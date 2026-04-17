package api

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"

	"github.com/pollenjp/cc-tunnel/apps/cc-remote-agent/internal/claude"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

// POST /execute — claude CLI を実行して ndjson をストリーミング
func (h *Handler) Execute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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

	if err := claude.StreamToWriter(r.Context(), req, w); err != nil {
		// ストリーミング中のエラーはヘッダー送信済みなのでログのみ
		// http.Error は使えない
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
	json.NewEncoder(w).Encode(resp)
}
