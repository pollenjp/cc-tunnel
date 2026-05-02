package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
)

type AuthStatus struct {
	LoggedIn         bool   `json:"loggedIn"`
	AuthMethod       string `json:"authMethod"`
	ApiProvider      string `json:"apiProvider,omitempty"`
	Email            string `json:"email,omitempty"`
	OrgName          string `json:"orgName,omitempty"`
	SubscriptionType string `json:"subscriptionType,omitempty"`
	ApiKeySource     string `json:"apiKeySource,omitempty"`
	LoginPending     bool   `json:"loginPending"`
	LoginUrl         string `json:"loginUrl,omitempty"`
}

type LoginResponse struct {
	LoggedIn bool   `json:"loggedIn,omitempty"`
	Message  string `json:"message"`
}

type AuthManager struct {
	mu           sync.Mutex
	loginCmd     *exec.Cmd
	ptyFd        *os.File // PTY ファイルディスクリプタ（読み書き両用）
	loginPending bool
	cancelFunc   context.CancelFunc

	// SSE fan-out
	subscribersMu sync.Mutex
	subscribers   map[chan<- []byte]struct{}
}

func NewAuthManager() *AuthManager {
	return &AuthManager{
		subscribers: make(map[chan<- []byte]struct{}),
	}
}

// GetStatus runs `claude auth status --json` and returns the parsed AuthStatus.
func (m *AuthManager) GetStatus(ctx context.Context) (AuthStatus, error) {
	out, err := exec.CommandContext(ctx, "claude", "auth", "status", "--json").Output()
	if err != nil {
		var exitErr *exec.ExitError
		if e, ok := err.(*exec.ExitError); ok {
			exitErr = e
			_ = exitErr
			if len(out) == 0 {
				slog.Error("auth status check failed", "err", err)
				return AuthStatus{}, err
			}
		} else if len(out) == 0 {
			slog.Error("auth status check failed", "err", err)
			return AuthStatus{}, err
		}
	}

	var status AuthStatus
	if err := json.Unmarshal(bytes.TrimSpace(out), &status); err != nil {
		slog.Error("auth status check failed", "err", err)
		return AuthStatus{}, err
	}

	m.mu.Lock()
	status.LoginPending = m.loginPending
	m.mu.Unlock()

	return status, nil
}

// StartLogin launches `claude auth login` in a PTY.
// No --claudeai/--console flags — user interacts with TUI menu directly.
func (m *AuthManager) StartLogin(ctx context.Context, method string) (LoginResponse, error) {
	// Check current auth status
	status, err := m.GetStatus(ctx)
	if err == nil && status.LoggedIn {
		return LoginResponse{LoggedIn: true, Message: "Already authenticated"}, nil
	}

	m.mu.Lock()
	if m.loginPending {
		m.mu.Unlock()
		return LoginResponse{Message: "Login already in progress"}, nil
	}
	m.mu.Unlock()

	slog.Info("auth login started", "method", method)

	cancelCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)

	cmd := exec.CommandContext(cancelCtx, "claude", "/auth")

	// PTY で起動（TUI メニュー対応）
	ptmx, err := pty.Start(cmd)
	if err != nil {
		cancel()
		return LoginResponse{}, fmt.Errorf("failed to start pty: %w", err)
	}
	slog.Info("auth login PTY started", "pid", cmd.Process.Pid)

	// PTY ウィンドウサイズを固定（80x24）
	if err := pty.Setsize(ptmx, &pty.Winsize{Rows: 24, Cols: 80}); err != nil {
		slog.Warn("pty.Setsize failed", "error", err)
	}

	m.mu.Lock()
	m.loginCmd = cmd
	m.loginPending = true
	m.cancelFunc = cancel
	m.ptyFd = ptmx
	m.mu.Unlock()

	// PTY 出力を非同期で読み取り、全 subscriber へ fan-out する goroutine
	go func() {
		defer func() {
			if err := ptmx.Close(); err != nil {
				slog.Warn("ptmx.Close failed", "error", err)
			}
			if err := cmd.Wait(); err != nil {
				slog.Warn("cmd.Wait failed", "error", err)
			}
			cancel()
			m.mu.Lock()
			m.loginPending = false
			m.loginCmd = nil
			m.cancelFunc = nil
			m.ptyFd = nil
			m.mu.Unlock()
		}()

		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				m.subscribersMu.Lock()
				for ch := range m.subscribers {
					select {
					case ch <- data:
					default:
						slog.Warn("subscriber buffer full, dropping chunk")
					}
				}
				m.subscribersMu.Unlock()
			}
			if err != nil {
				break
			}
		}
	}()

	return LoginResponse{Message: "Login started"}, nil
}

// Subscribe returns a channel that receives raw PTY bytes until ctx is cancelled.
// The channel is buffered (64) and dropped chunks are logged.
func (m *AuthManager) Subscribe(ctx context.Context) <-chan []byte {
	ch := make(chan []byte, 64)
	m.subscribersMu.Lock()
	m.subscribers[ch] = struct{}{}
	m.subscribersMu.Unlock()
	go func() {
		<-ctx.Done()
		m.subscribersMu.Lock()
		delete(m.subscribers, ch)
		m.subscribersMu.Unlock()
		close(ch)
	}()
	return ch
}

// BroadcastForTest sends data to all subscribers. Used only in tests.
func (m *AuthManager) BroadcastForTest(data []byte) {
	m.subscribersMu.Lock()
	for ch := range m.subscribers {
		select {
		case ch <- data:
		default:
		}
	}
	m.subscribersMu.Unlock()
}

// SubmitInput writes input bytes to the PTY stdin.
// Accepts raw keystrokes (e.g. "\x1b[A" for up arrow, "\r" for Enter).
func (m *AuthManager) SubmitInput(input string) error {
	m.mu.Lock()
	fd := m.ptyFd
	pending := m.loginPending
	m.mu.Unlock()

	if !pending || fd == nil {
		return fmt.Errorf("no login in progress")
	}

	_, err := io.WriteString(fd, input)
	return err
}

// CancelLogin kills the PTY process and clears state immediately.
func (m *AuthManager) CancelLogin() {
	m.mu.Lock()
	cancel := m.cancelFunc
	fd := m.ptyFd
	m.loginPending = false
	m.cancelFunc = nil
	m.ptyFd = nil
	m.mu.Unlock()

	slog.Info("auth login cancelled")

	if fd != nil {
		if err := fd.Close(); err != nil {
			slog.Warn("fd.Close failed", "error", err)
		}
	}
	if cancel != nil {
		cancel()
	}
}

// Logout cancels any pending login then runs `claude auth logout`.
func (m *AuthManager) Logout(ctx context.Context) (AuthStatus, error) {
	m.mu.Lock()
	pending := m.loginPending
	m.mu.Unlock()
	if pending {
		m.CancelLogin()
	}
	slog.Info("auth logout executed")
	if err := exec.CommandContext(ctx, "claude", "auth", "logout").Run(); err != nil {
		slog.Warn("command failed", "error", err)
	}
	return m.GetStatus(ctx)
}
