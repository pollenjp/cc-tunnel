package auth

import (
	"bytes"
	"context"
	"encoding/base64"
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

// LoginStatus represents the state of a PTY login session.
type LoginStatus string

const (
	StatusIdle      LoginStatus = "idle"
	StatusPending   LoginStatus = "pending"
	StatusCompleted LoginStatus = "completed"
	StatusFailed    LoginStatus = "failed"
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
	LoginID  string `json:"loginId,omitempty"`
}

type AuthManager struct {
	mu         sync.Mutex
	loginCmd   *exec.Cmd
	ptyFd      *os.File
	outputBuf  []byte
	status     LoginStatus
	cancelFunc context.CancelFunc
}

func NewAuthManager() *AuthManager {
	return &AuthManager{status: StatusIdle}
}

// GetStatus runs `claude auth status --json` and returns the parsed AuthStatus.
func (m *AuthManager) GetStatus(ctx context.Context) (AuthStatus, error) {
	out, err := exec.CommandContext(ctx, "claude", "auth", "status", "--json").Output()
	if err != nil {
		if len(out) == 0 {
			slog.Error("auth status check failed", "err", err)
			return AuthStatus{}, err
		}
	}

	var status AuthStatus
	if err := json.Unmarshal(bytes.TrimSpace(out), &status); err != nil {
		slog.Error("auth status unmarshal failed", "err", err)
		return AuthStatus{}, err
	}

	m.mu.Lock()
	status.LoginPending = m.status == StatusPending
	m.mu.Unlock()

	return status, nil
}

// StartLogin launches `claude /auth` in a PTY.
func (m *AuthManager) StartLogin(ctx context.Context, method string) (LoginResponse, error) {
	m.mu.Lock()
	if m.status == StatusPending {
		m.mu.Unlock()
		return LoginResponse{Message: "Login already in progress"}, nil
	}
	m.mu.Unlock()

	slog.Info("auth login started", "method", method)

	cancelCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)

	cmd := exec.CommandContext(cancelCtx, "claude", "/auth")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		cancel()
		return LoginResponse{}, fmt.Errorf("failed to start pty: %w", err)
	}
	slog.Info("auth login PTY started", "pid", cmd.Process.Pid)

	if err := pty.Setsize(ptmx, &pty.Winsize{Rows: 24, Cols: 80}); err != nil {
		slog.Warn("pty.Setsize failed", "error", err)
	}

	m.mu.Lock()
	m.loginCmd = cmd
	m.status = StatusPending
	m.cancelFunc = cancel
	m.ptyFd = ptmx
	m.outputBuf = nil
	m.mu.Unlock()

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
			if m.status == StatusPending {
				m.status = StatusCompleted
			}
			m.loginCmd = nil
			m.cancelFunc = nil
			m.ptyFd = nil
			m.mu.Unlock()
		}()

		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				m.mu.Lock()
				m.outputBuf = append(m.outputBuf, chunk...)
				m.mu.Unlock()
			}
			if err != nil {
				break
			}
		}
	}()

	return LoginResponse{Message: "Login started"}, nil
}

// GetOutput returns PTY output bytes since the given cursor position.
// The data is base64-encoded to safely transport binary/ANSI content.
func (m *AuthManager) GetOutput(since int) (data string, cursor int, status LoginStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	total := len(m.outputBuf)
	if since < 0 {
		since = 0
	}
	if since >= total {
		return "", total, m.status
	}
	chunk := make([]byte, total-since)
	copy(chunk, m.outputBuf[since:])
	return base64.StdEncoding.EncodeToString(chunk), total, m.status
}

// SubmitInput writes input bytes to the PTY stdin.
func (m *AuthManager) SubmitInput(input string) error {
	m.mu.Lock()
	fd := m.ptyFd
	status := m.status
	m.mu.Unlock()

	if status != StatusPending || fd == nil {
		return fmt.Errorf("no login in progress")
	}

	_, err := io.WriteString(fd, input)
	return err
}

// Cancel kills the PTY process and clears state.
func (m *AuthManager) Cancel() {
	m.mu.Lock()
	cancel := m.cancelFunc
	fd := m.ptyFd
	m.status = StatusIdle
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

// ReadCredentials reads ~/.claude/.credentials.json and returns the raw JSON.
func (m *AuthManager) ReadCredentials() ([]byte, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("UserHomeDir: %w", err)
	}
	path := home + "/.claude/.credentials.json"
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ReadFile %s: %w", path, err)
	}
	return data, nil
}

// ClearLocalCredentials removes ~/.claude/.credentials.json.
func (m *AuthManager) ClearLocalCredentials() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("UserHomeDir: %w", err)
	}
	path := home + "/.claude/.credentials.json"
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

// Status returns the current login status.
func (m *AuthManager) Status() LoginStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}
