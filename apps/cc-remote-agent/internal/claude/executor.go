package claude

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
	"sync"
	"time"
)

type ExecuteRequest struct {
	Prompt                 string            `json:"prompt"`
	SessionID              string            `json:"session_id,omitempty"`           // --resume 用
	Model                  string            `json:"model,omitempty"`
	SystemPrompt           string            `json:"system_prompt,omitempty"`
	ConversationHistory    []ConversationMsg `json:"conversation_history,omitempty"` // フォールバック用
	AllowedTools           []string          `json:"allowed_tools,omitempty"`
	PermissionMode         string            `json:"permission_mode,omitempty"`
	MaxBudgetUSD           float64           `json:"max_budget_usd,omitempty"`
	IncludePartialMessages bool              `json:"include_partial_messages,omitempty"`
	IncludeHookEvents      bool              `json:"include_hook_events,omitempty"`
}

type ConversationMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// StreamToWriter executes claude CLI and streams ndjson lines to w.
// Uses --resume if SessionID is set. Falls back to prompt stuffing on resume failure.
func StreamToWriter(ctx context.Context, req ExecuteRequest, w http.ResponseWriter) error {
	args := buildArgs(req, false)
	if req.SessionID != "" {
		slog.Info("claude CLI resume attempt", "session_id", req.SessionID)
	}
	return runStream(ctx, args, w, func() error {
		// フォールバック: prompt stuffing
		if req.SessionID != "" && len(req.ConversationHistory) > 0 {
			slog.Info("claude CLI fallback triggered", "reason", "resume not found", "history_count", len(req.ConversationHistory))
			fallbackReq := req
			fallbackReq.SessionID = ""
			fallbackReq.Prompt = buildFallbackPrompt(req.ConversationHistory, req.Prompt)
			fallbackArgs := buildArgs(fallbackReq, false)
			return runStream(ctx, fallbackArgs, w, nil)
		}
		return fmt.Errorf("claude execution failed")
	})
}

func buildArgs(req ExecuteRequest, isFallback bool) []string {
	args := []string{"-p", "--output-format=stream-json", "--verbose"}

	// --resume (セッション継続) or 新規セッション
	if req.SessionID != "" && !isFallback {
		args = append(args, "--resume", req.SessionID)
	}

	// モデル指定
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}

	// システムプロンプト
	if req.SystemPrompt != "" {
		args = append(args, "--system-prompt", req.SystemPrompt)
	}

	// 許可ツール
	for _, tool := range req.AllowedTools {
		args = append(args, "--allowedTools", tool)
	}

	// パーミッションモード
	if req.PermissionMode != "" {
		args = append(args, "--permission-mode", req.PermissionMode)
	}

	// --include-partial-messages（stream_event デルタ有効化）
	if req.IncludePartialMessages {
		args = append(args, "--include-partial-messages")
	}
	// --include-hook-events（フックイベント有効化）
	if req.IncludeHookEvents {
		args = append(args, "--include-hook-events")
	}

	// プロンプト（最後の引数）
	args = append(args, "--", req.Prompt)
	return args
}

func buildFallbackPrompt(history []ConversationMsg, currentPrompt string) string {
	// 過去メッセージをテキスト形式で組み立て
	prompt := "You are continuing a conversation. Here is the conversation history:\n\n"
	for _, msg := range history {
		switch msg.Role {
		case "user":
			prompt += fmt.Sprintf("[User]: %s\n", msg.Content)
		case "assistant":
			prompt += fmt.Sprintf("[Assistant]: %s\n", msg.Content)
		case "system":
			prompt += fmt.Sprintf("[System]: %s\n", msg.Content)
		}
	}
	prompt += fmt.Sprintf("\nPlease respond to the latest user message:\n%s", currentPrompt)
	return prompt
}

func runStream(ctx context.Context, args []string, w http.ResponseWriter, onError func() error) error {
	cmd := exec.CommandContext(ctx, "claude", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	slog.Info("claude CLI command", "args", cmd.Args)
	startTime := time.Now()
	if err := cmd.Start(); err != nil {
		return err
	}
	slog.Info("claude CLI process started", "pid", cmd.Process.Pid)

	// ヘッダー設定（まだ書いていない場合）
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Transfer-Encoding", "chunked")
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	// stdout を ndjson として転送
	scanner := bufio.NewScanner(stdout)
	resumeFailed := false
	for scanner.Scan() {
		line := scanner.Bytes()
		slog.Info("claude CLI stdout", "line", string(line))
		// "session not found" エラー検知
		if containsResumeError(line) {
			resumeFailed = true
			break
		}
		w.Write(line)
		w.Write([]byte("\n"))
		flusher.Flush()
	}

	// stderr を読み捨て（ログには出力）
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		stderrScanner := bufio.NewScanner(stderr)
		for stderrScanner.Scan() {
			slog.Warn("claude CLI stderr", "line", stderrScanner.Text())
		}
	}()

	cmdErr := cmd.Wait()
	wg.Wait()

	exitCode := 0
	var ee *exec.ExitError
	if errors.As(cmdErr, &ee) {
		exitCode = ee.ExitCode()
	}
	slog.Info("claude CLI process exited", "exit_code", exitCode, "duration_ms", time.Since(startTime).Milliseconds())

	if resumeFailed || (cmdErr != nil && onError != nil) {
		return onError()
	}
	return cmdErr
}

// containsResumeError checks if a ndjson line indicates --resume failure.
func containsResumeError(line []byte) bool {
	s := string(line)
	return containsAny(s, []string{"session not found", "resume failed", "invalid session"})
}

func containsAny(s string, substrings []string) bool {
	for _, sub := range substrings {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
