package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pollenjp/cc-tunnel/apps/cc-remote-agent/internal/auth"
	"github.com/pollenjp/cc-tunnel/apps/cc-remote-agent/internal/claude"
)

type Handler struct {
	authManager *auth.AuthManager
}

func NewHandler(authManager *auth.AuthManager) *Handler {
	return &Handler{authManager: authManager}
}

// POST /execute — claude CLI を実行して ndjson をストリーミング
func (h *Handler) Execute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 認証チェック
	status, err := h.authManager.GetStatus(r.Context())
	if err != nil {
		slog.Error("failed to get auth status", "error", err)
		http.Error(w, `{"error":"failed to get auth status"}`, http.StatusInternalServerError)
		return
	}
	if !status.LoggedIn {
		http.Error(w, `{"error":"not authenticated"}`, http.StatusUnauthorized)
		return
	}

	var req claude.ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		http.Error(w, "prompt is required", http.StatusBadRequest)
		return
	}
	slog.Info("execute request", "prompt_len", len(req.Prompt), "has_session_id", req.SessionID != "", "model", req.Model)

	if err := claude.StreamToWriter(r.Context(), req, w); err != nil {
		// ストリーミング中のエラーはヘッダー送信済みなのでログのみ
		slog.Error("streaming error", "error", err)
		return
	}
}

// GET /health — ヘルスチェック + claude バージョン確認
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	resp := map[string]string{"status": "ok"}

	// claude --version の確認
	out, err := exec.Command("claude", "--version").Output()
	if err == nil {
		resp["claude_version"] = strings.TrimSpace(string(out))
	} else {
		resp["claude_version"] = "unavailable"
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to encode health response", "error", err)
	}
}

// GET /auth/status — Claude CLI 認証状態を返す
func (h *Handler) AuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	slog.Info("auth status request")

	status, err := h.authManager.GetStatus(r.Context())
	if err != nil {
		http.Error(w, `{"error":"failed to get auth status"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		slog.Error("failed to encode auth status response", "error", err)
	}
}

// POST /auth/login — Claude CLI OAuth ログインを開始する
// body: {"method":"claudeai"|"console"}
func (h *Handler) AuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Method string `json:"method"`
	}
	// body は optional; デコード失敗は無視してデフォルトを使う
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		slog.Debug("auth login body decode failed, using defaults", "error", err)
	}
	if body.Method == "" {
		body.Method = "claudeai"
	}
	slog.Info("auth login request", "method", body.Method)

	resp, err := h.authManager.StartLogin(r.Context(), body.Method)
	if err != nil {
		http.Error(w, `{"error":"failed to start login"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to encode login response", "error", err)
	}
}

// POST /auth/pty/input — login プロセスの stdin に任意の入力を送信する
func (h *Handler) AuthPtyInput(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Input string `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	// 空文字列も許容（Enter キーのみ送信のユースケース）
	slog.Info("auth pty input request", "input_len", len(body.Input))

	if err := h.authManager.SubmitInput(body.Input); err != nil {
		http.Error(w, `{"error":"no login in progress"}`, http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"message": "Input submitted"}); err != nil {
		slog.Error("failed to encode input response", "error", err)
	}
}

// GET /auth/pty/stream — PTY 出力を SSE (Server-Sent Events) でストリーミングする
func (h *Handler) AuthPtyStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Connection", "keep-alive")

	// Subscribe before flushing headers so any broadcast that happens
	// after the client observes the response is delivered to this handler.
	ch := h.authManager.Subscribe(r.Context())

	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case data, ok := <-ch:
			if !ok {
				return
			}
			encoded := base64.StdEncoding.EncodeToString(data)
			if _, err := fmt.Fprintf(w, "data: %s\n\n", encoded); err != nil {
				return
			}
			flusher.Flush()
		case <-ticker.C:
			if _, err := fmt.Fprintf(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// POST /auth/cancel — PTY プロセスを強制終了する
func (h *Handler) AuthCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	slog.Info("auth cancel request")
	h.authManager.CancelLogin()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"message": "Login cancelled"}); err != nil {
		slog.Error("failed to encode cancel response", "error", err)
	}
}

// POST /init — credentials JSON をコンテナ内 ~/.claude/.credentials.json に書き込む
// body: {"credentialsJson": "<JSON string>"}
func (h *Handler) Init(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		CredentialsJSON string `json:"credentialsJson"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if body.CredentialsJSON == "" {
		http.Error(w, `{"error":"credentialsJson is required"}`, http.StatusBadRequest)
		return
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		slog.Error("failed to get home dir", "error", err)
		http.Error(w, `{"error":"failed to resolve home directory"}`, http.StatusInternalServerError)
		return
	}
	claudeDir := filepath.Join(homeDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o700); err != nil {
		slog.Error("failed to create .claude dir", "error", err)
		http.Error(w, `{"error":"failed to create .claude directory"}`, http.StatusInternalServerError)
		return
	}
	credPath := filepath.Join(claudeDir, ".credentials.json")
	if err := os.WriteFile(credPath, []byte(body.CredentialsJSON), 0o600); err != nil {
		slog.Error("failed to write credentials file", "error", err)
		http.Error(w, `{"error":"failed to write credentials file"}`, http.StatusInternalServerError)
		return
	}

	slog.Info("credentials initialized", "path", credPath)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"message": "credentials initialized"}); err != nil {
		slog.Error("failed to encode init response", "error", err)
	}
}

// POST /auth/finalize-credentials — PTY ログイン完了後、tmpfs 上の credentials.json を読み返す
// response: {"credentialsJson": "<file content>"}
func (h *Handler) FinalizeCredentials(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		slog.Error("failed to get home dir", "error", err)
		http.Error(w, `{"error":"failed to resolve home directory"}`, http.StatusInternalServerError)
		return
	}
	credPath := filepath.Join(homeDir, ".claude", ".credentials.json")
	data, err := os.ReadFile(credPath)
	if errors.Is(err, os.ErrNotExist) {
		http.Error(w, `{"error":"credentials file not found","hint":"complete /auth/login flow first"}`, http.StatusNotFound)
		return
	}
	if err != nil {
		slog.Error("failed to read credentials", "error", err)
		http.Error(w, `{"error":"failed to read credentials"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"credentialsJson": string(data),
	}); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

// POST /auth/logout — Claude CLI からログアウトする
func (h *Handler) AuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	slog.Info("auth logout request")

	status, err := h.authManager.Logout(r.Context())
	if err != nil {
		http.Error(w, `{"error":"logout failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		slog.Error("failed to encode logout response", "error", err)
	}
}
