package api

//go:generate go tool oapi-codegen -config ../../../openapi/oapi-codegen.yaml -o gen.go ../../../openapi/openapi.yaml

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/db"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

type Server struct {
	repo          repository
	remote        remoteClient
	batchInterval time.Duration // 0 = default 2s; override for testing
	doneCh        chan struct{}  // closed when Execute goroutine completes; for testing only
}

var _ ServerInterface = (*Server)(nil)

func NewHandler(repo *db.Repository, remote *remoteclient.Client) *Server {
	return &Server{repo: repo, remote: remote}
}

func (h *Server) GetAuthStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.remote.GetAuthStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *Server) InitiateLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}
	method := ""
	if req.Method != nil {
		method = string(*req.Method)
	}
	resp, err := h.remote.InitiateLogin(r.Context(), method)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Server) Logout(w http.ResponseWriter, r *http.Request) {
	status, err := h.remote.Logout(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *Server) CancelLogin(w http.ResponseWriter, r *http.Request) {
	resp, err := h.remote.CancelLogin(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Server) SubmitAuthInput(w http.ResponseWriter, r *http.Request) {
	var req AuthInputRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.remote.SubmitAuthInput(r.Context(), req.Input)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Server) GetAuthOutput(w http.ResponseWriter, r *http.Request, params GetAuthOutputParams) {
	since := 0
	if params.Since != nil {
		since = *params.Since
	}
	resp, err := h.remote.GetAuthOutput(r.Context(), since)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Server) CreateConversation(w http.ResponseWriter, r *http.Request) {
	var req CreateConversationRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	title := ""
	if req.Title != nil {
		title = *req.Title
	}
	model := "claude-sonnet-4-6"
	if req.Model != nil {
		model = *req.Model
	}
	var systemPrompt *string
	if req.SystemPrompt != nil {
		systemPrompt = req.SystemPrompt
	}

	conv, err := h.repo.CreateConversation(r.Context(), title, model, systemPrompt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	slog.Info("conversation created", "conversation_id", conv.ID)
	writeJSON(w, http.StatusCreated, dbConvToAPI(conv))
}

func (h *Server) ListConversations(w http.ResponseWriter, r *http.Request) {
	convs, err := h.repo.ListConversations(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result := make([]Conversation, 0, len(convs))
	for _, c := range convs {
		result = append(result, dbConvToAPI(c))
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Server) GetConversation(w http.ResponseWriter, r *http.Request, conversationId ConversationId) {
	conv, err := h.repo.GetConversation(r.Context(), conversationId.String())
	if err != nil {
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}
	msgs, err := h.repo.ListMessages(r.Context(), conversationId.String())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, newConversationDetail(conv, msgs))
}

func (h *Server) DeleteConversation(w http.ResponseWriter, r *http.Request, conversationId ConversationId) {
	if err := h.repo.DeleteConversation(r.Context(), conversationId.String()); err != nil {
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}
	slog.Info("conversation deleted", "conversation_id", conversationId.String())
	writeJSON(w, http.StatusOK, StatusResponse{Status: "ok"})
}

func (h *Server) SendMessage(w http.ResponseWriter, r *http.Request, conversationId ConversationId) {
	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	convIDStr := conversationId.String()

	// 会話の存在確認 + 過去メッセージ取得
	conv, err := h.repo.GetConversation(r.Context(), convIDStr)
	if err != nil {
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}
	history, err := h.repo.ListMessages(r.Context(), convIDStr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// ユーザーメッセージを DB に保存
	_, err = h.repo.CreateMessage(r.Context(), convIDStr, "user", map[string]interface{}{"content": req.Content})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 過去メッセージから --resume 用 session_id を取得
	var resumeSessionID string
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "assistant" {
			if sid, ok := history[i].MessageData["session_id"].(string); ok && sid != "" {
				resumeSessionID = sid
				break
			}
		}
	}

	// cc-remote-agent への会話履歴（フォールバック用）
	var convHistory []remoteclient.Message
	for _, m := range history {
		content := ""
		switch m.Role {
		case "user":
			content, _ = m.MessageData["content"].(string)
		case "assistant":
			if cbs, ok := m.MessageData["content_blocks"].([]interface{}); ok {
				for _, cb := range cbs {
					if block, ok := cb.(map[string]interface{}); ok {
						if block["type"] == "text" {
							if t, ok := block["content"].(string); ok {
								content += t
							}
						}
					}
				}
			}
		}
		convHistory = append(convHistory, remoteclient.Message{
			Role:    m.Role,
			Content: content,
		})
	}

	executeReq := remoteclient.Request{
		Prompt:                 req.Content,
		SessionID:              resumeSessionID,
		Model:                  conv.Model,
		ConversationHistory:    convHistory,
		IncludePartialMessages: true,
		IncludeHookEvents:      true,
	}
	if conv.SystemPrompt != nil {
		executeReq.SystemPrompt = *conv.SystemPrompt
	}

	// execCtx is independent of r.Context() so that a frontend disconnect (which
	// cancels r.Context()) does not abort the Claude CLI execution or the DB save.
	execCtx := context.WithoutCancel(r.Context())

	// 早期アシスタントメッセージ作成（streaming 状態）
	assistantMsg, err := h.repo.CreateStreamingMessage(execCtx, convIDStr, "assistant", map[string]interface{}{})
	if err != nil {
		slog.Error("failed to create streaming assistant message", "err", err, "conversation_id", convIDStr)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 202 即時返却
	msgUUID, _ := uuid.Parse(assistantMsg.ID)
	writeJSON(w, http.StatusAccepted, SendMessageResponse{MessageId: msgUUID})

	// goroutine で Execute + DB保存
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic in Execute goroutine", "err", r, "conversation_id", convIDStr)
			}
			if h.doneCh != nil {
				close(h.doneCh)
			}
		}()

		// Mark conversation as running before CLI execution starts.
		if err := h.repo.UpdateConversationStatus(execCtx, convIDStr, "running"); err != nil {
			slog.Warn("failed to update conversation status to running", "err", err, "conversation_id", convIDStr)
		}
		// Ensure status is set to completed when execution finishes, regardless of outcome.
		defer func() {
			if err := h.repo.UpdateConversationStatus(execCtx, convIDStr, "completed"); err != nil {
				slog.Warn("failed to update conversation status to completed", "err", err, "conversation_id", convIDStr)
			}
		}()

		// cc-remote-agent に実行依頼
		var mu sync.Mutex
		var (
			assistantContent  string
			thinkingContent   string
			modelName         string
			costUSD           float64
			durationMs        int64
			hookEventsList    []map[string]any
			thinkingBlocks    []string
			toolCallsData     []ToolCallData
			contentBlocksList []map[string]interface{}
		)

		// 2秒バッチ: contentBlocksList と toolCallsData を定期的に DB 保存
		batchInterval := h.batchInterval
		if batchInterval <= 0 {
			batchInterval = 2 * time.Second
		}
		ticker := time.NewTicker(batchInterval)
		defer ticker.Stop()
		go func() {
			for range ticker.C {
				mu.Lock()
				snapshot := cloneBlocks(contentBlocksList)
				snapshotTools := cloneToolCalls(toolCallsData)
				mu.Unlock()
				if err := h.repo.UpdateMessageContentBlocks(execCtx, assistantMsg.ID, snapshot); err != nil {
					slog.Warn("batch content_blocks update failed", "err", err, "message_id", assistantMsg.ID)
				}
				if err := h.repo.MergeMessageData(execCtx, assistantMsg.ID, map[string]interface{}{
					"tool_calls": snapshotTools,
				}); err != nil {
					slog.Warn("batch tool_calls update failed", "err", err, "message_id", assistantMsg.ID)
				}
			}
		}()

		slog.Info("message processing started", "conversation_id", convIDStr, "has_resume_session", resumeSessionID != "")
		newSessionID, err := h.remote.Execute(execCtx, executeReq, func(event remoteclient.StreamEvent) {
			slog.Info("stream event received", "type", event.Type, "subtype", event.SubType)
			switch event.Type {
			case "assistant":
				if event.Message != nil {
					for _, block := range event.Message.Content {
						if block.Type == "thinking" && block.Thinking != "" {
							mu.Lock()
							contentBlocksList = append(contentBlocksList, map[string]interface{}{
								"type":    "thinking",
								"content": block.Thinking,
							})
							mu.Unlock()
							thinkingContent += block.Thinking
						}
						if block.Type == "text" && block.Text != "" {
							mu.Lock()
							contentBlocksList = append(contentBlocksList, map[string]interface{}{
								"type":    "text",
								"content": block.Text,
							})
							mu.Unlock()
							assistantContent += block.Text
						}
						if block.Type == "tool_use" && block.ID != "" {
							mu.Lock()
							contentBlocksList = append(contentBlocksList, map[string]interface{}{
								"type":        "tool_use",
								"tool_use_id": block.ID,
							})
							mu.Unlock()
						}
						if block.Type == "tool_result" {
							content := ""
							switch v := block.Content.(type) {
							case string:
								content = v
							case []any:
								if len(v) > 0 {
									if m, ok := v[0].(map[string]any); ok {
										content, _ = m["text"].(string)
									}
								}
							}
							if len(content) > 1000 {
								content = content[:1000] + "...[truncated]"
							}
							for i, tc := range toolCallsData {
								if tc.ToolUseId == block.ToolUseID {
									result := content
									toolCallsData[i].Result = &result
									break
								}
							}
						}
					}
				}
			case "user":
				if event.Message != nil {
					for _, block := range event.Message.Content {
						if block.Type == "tool_result" {
							content := ""
							switch v := block.Content.(type) {
							case string:
								content = v
							case []any:
								if len(v) > 0 {
									if m, ok := v[0].(map[string]any); ok {
										content, _ = m["text"].(string)
									}
								}
							}
							if len(content) > 2000 {
								content = content[:2000] + "...[truncated]"
							}
							for i, tc := range toolCallsData {
								if tc.ToolUseId == block.ToolUseID {
									result := content
									toolCallsData[i].Result = &result
									break
								}
							}
						}
					}
				}
			case "system":
				switch event.SubType {
				case "init":
					if event.Model != "" {
						modelName = event.Model
					}
				case "hook_started", "hook_response", "notification", "status":
					hookEventsList = append(hookEventsList, map[string]any{
						"subtype":    event.SubType,
						"hook_id":    event.HookID,
						"hook_name":  event.HookName,
						"hook_event": event.HookEvent,
					})
				}
			case "stream_event":
				var inner remoteclient.InnerEvent
				if err := json.Unmarshal(event.Event, &inner); err != nil {
					break
				}
				switch inner.Type {
				case "content_block_start":
					var cbInner struct {
						Type string `json:"type"`
						ID   string `json:"id,omitempty"`
						Name string `json:"name,omitempty"`
					}
					if err := json.Unmarshal(inner.ContentBlock, &cbInner); err != nil {
						break
					}
					if cbInner.Type == "tool_use" && cbInner.Name != "" {
						toolCallsData = append(toolCallsData, ToolCallData{
							ToolUseId: cbInner.ID,
							ToolName:  cbInner.Name,
							InputJson: "",
						})
					}
				case "content_block_delta":
					var delta map[string]string
					if err := json.Unmarshal(inner.Delta, &delta); err != nil {
						break
					}
					switch delta["type"] {
					case "thinking_delta":
						if delta["thinking"] != "" {
							if len(thinkingBlocks) == 0 {
								thinkingBlocks = append(thinkingBlocks, "")
							}
							thinkingBlocks[len(thinkingBlocks)-1] += delta["thinking"]
						}
					case "input_json_delta":
						if delta["partial_json"] != "" {
							if len(toolCallsData) > 0 {
								toolCallsData[len(toolCallsData)-1].InputJson += delta["partial_json"]
							}
						}
					}
				}
			case "result":
				costUSD = event.CostUSD
				durationMs = event.DurationMs
			}
		})

		if err != nil {
			slog.Error("message processing error", "conversation_id", convIDStr, "err", err)
			if serr := h.repo.UpdateMessageStatus(execCtx, assistantMsg.ID, "error"); serr != nil {
				slog.Error("failed to update message status to error", "err", serr, "message_id", assistantMsg.ID)
			}
			return
		}

		slog.Info("message processing completed", "conversation_id", convIDStr)

		// 最終 content_blocks 保存 + message_data マージ + status 更新
		mu.Lock()
		finalBlocks := cloneBlocks(contentBlocksList)
		mu.Unlock()

		if err := h.repo.UpdateMessageContentBlocks(execCtx, assistantMsg.ID, finalBlocks); err != nil {
			slog.Error("failed to update final content_blocks", "err", err, "message_id", assistantMsg.ID)
		}

		// session_id, model 等のメタデータをマージ
		extra := map[string]interface{}{"session_id": newSessionID}
		if assistantContent != "" {
			extra["content"] = assistantContent
		}
		if len(thinkingBlocks) > 0 {
			extra["thinking"] = thinkingBlocks
		} else if thinkingContent != "" {
			extra["thinking"] = []string{thinkingContent}
		}
		if modelName != "" {
			extra["model"] = modelName
		}
		if costUSD > 0 {
			extra["cost_usd"] = costUSD
		}
		if durationMs > 0 {
			extra["duration_ms"] = durationMs
		}
		if len(hookEventsList) > 0 {
			extra["hook_events"] = hookEventsList
		}
		if len(toolCallsData) > 0 {
			extra["tool_calls"] = toolCallsData
		}
		if err := h.repo.MergeMessageData(execCtx, assistantMsg.ID, extra); err != nil {
			slog.Error("failed to merge assistant message data", "err", err, "message_id", assistantMsg.ID)
		}

		if err := h.repo.UpdateMessageStatus(execCtx, assistantMsg.ID, "completed"); err != nil {
			slog.Error("failed to update message status to completed", "err", err, "message_id", assistantMsg.ID)
		}

		if assistantContent != "" {
			title := generateTitle(assistantContent)
			if err := h.repo.UpdateConversationTitle(execCtx, convIDStr, title); err != nil {
				slog.Error("failed to update conversation title", "err", err, "conversation_id", convIDStr)
			}
		}
		if err := h.repo.UpdateConversationUpdatedAt(execCtx, convIDStr); err != nil {
			slog.Error("failed to update conversation updated_at", "err", err, "conversation_id", convIDStr)
		}
	}()
}

// --- helper functions ---

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("json encode failed", "error", err)
	}
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, Error{Error: msg})
}

func cloneToolCalls(calls []ToolCallData) []ToolCallData {
	if calls == nil {
		return nil
	}
	clone := make([]ToolCallData, len(calls))
	copy(clone, calls)
	return clone
}

func cloneBlocks(blocks []map[string]interface{}) []map[string]interface{} {
	if blocks == nil {
		return nil
	}
	clone := make([]map[string]interface{}, len(blocks))
	for i, b := range blocks {
		m := make(map[string]interface{}, len(b))
		for k, v := range b {
			m[k] = v
		}
		clone[i] = m
	}
	return clone
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}
