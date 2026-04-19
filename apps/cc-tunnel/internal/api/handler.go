package api

//go:generate go tool oapi-codegen -config ../../../openapi/oapi-codegen.yaml -o gen.go ../../../openapi/openapi.yaml

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/db"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

type Server struct {
	repo   *db.Repository
	remote *remoteclient.Client
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
	convUUID, _ := uuid.Parse(conv.ID)
	detail := ConversationDetail{
		Id:           convUUID,
		Title:        conv.Title,
		Model:        conv.Model,
		CreatedAt:    conv.CreatedAt,
		UpdatedAt:    conv.UpdatedAt,
		SystemPrompt: conv.SystemPrompt,
		Messages:     make([]Message, 0, len(msgs)),
	}
	for _, m := range msgs {
		detail.Messages = append(detail.Messages, dbMsgToAPI(m))
	}
	writeJSON(w, http.StatusOK, detail)
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
	_, err = h.repo.CreateMessage(r.Context(), convIDStr, "user", req.Content, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 過去メッセージから --resume 用 session_id を取得
	// 最新の assistant メッセージの metadata["session_id"] を使う
	var resumeSessionID string
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "assistant" {
			if sid, ok := history[i].Metadata["session_id"].(string); ok && sid != "" {
				resumeSessionID = sid
				break
			}
		}
	}

	// cc-remote-agent への会話履歴（フォールバック用）
	var convHistory []remoteclient.Message
	for _, m := range history {
		convHistory = append(convHistory, remoteclient.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	// SSE ストリーミングレスポンス開始
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// cc-remote-agent に実行依頼
	var (
		assistantContent string
		thinkingContent  string
		modelName        string
		costUSD          float64
		durationMs       int64
		hookEventsList   []map[string]any
		thinkingBlocks   []string
		toolCallsData    []map[string]any
	)
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

	slog.Info("message streaming started", "conversation_id", convIDStr, "has_resume_session", resumeSessionID != "")
	newSessionID, err := h.remote.Execute(r.Context(), executeReq, func(event remoteclient.StreamEvent) {
		slog.Info("stream event received", "type", event.Type, "subtype", event.SubType)
		switch event.Type {
		case "assistant":
			if event.Message != nil {
				for _, block := range event.Message.Content {
					if block.Type == "thinking" && block.Thinking != "" {
						thinkingContent += block.Thinking
						sseEvent := map[string]string{"type": "thinking", "content": block.Thinking}
						data, _ := json.Marshal(sseEvent)
						slog.Info("SSE sent", "data", string(data))
						if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
							slog.Warn("SSE write failed", "error", err)
							return
						}
						flusher.Flush()
					}
					if block.Type == "text" && block.Text != "" {
						assistantContent += block.Text
						sseEvent := map[string]string{"type": "text", "content": block.Text}
						data, _ := json.Marshal(sseEvent)
						slog.Info("SSE sent", "data", string(data))
						if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
							slog.Warn("SSE write failed", "error", err)
							return
						}
						flusher.Flush()
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
						sseData, _ := json.Marshal(map[string]any{
							"type":        "tool_result",
							"tool_use_id": block.ToolUseID,
							"content":     content,
						})
						slog.Info("SSE sent", "data", string(sseData))
						if _, err := fmt.Fprintf(w, "data: %s\n\n", sseData); err != nil {
							slog.Warn("SSE write failed", "error", err)
							return
						}
						flusher.Flush()
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
						sseData, _ := json.Marshal(map[string]any{
							"type":        "tool_result",
							"tool_use_id": block.ToolUseID,
							"content":     content,
						})
						slog.Info("SSE sent", "data", string(sseData))
						if _, err := fmt.Fprintf(w, "data: %s\n\n", sseData); err != nil {
							slog.Warn("SSE write failed", "error", err)
							return
						}
						flusher.Flush()
						for i, tc := range toolCallsData {
							if tc["tool_use_id"] == block.ToolUseID {
								toolCallsData[i]["result"] = content
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
					sseData, _ := json.Marshal(map[string]any{
						"type":       "init",
						"model":      event.Model,
						"session_id": event.SessionID,
					})
					slog.Info("SSE sent", "data", string(sseData))
					if _, err := fmt.Fprintf(w, "data: %s\n\n", sseData); err != nil {
						slog.Warn("SSE write failed", "error", err)
						return
					}
					flusher.Flush()
				}
			case "hook_started", "hook_response", "notification", "status":
				hookEventsList = append(hookEventsList, map[string]any{
					"subtype":    event.SubType,
					"hook_id":    event.HookID,
					"hook_name":  event.HookName,
					"hook_event": event.HookEvent,
				})
				sseData, _ := json.Marshal(map[string]any{
					"type":       "hook_event",
					"subtype":    event.SubType,
					"hook_id":    event.HookID,
					"hook_name":  event.HookName,
					"hook_event": event.HookEvent,
					"session_id": event.SessionID,
				})
				slog.Info("SSE sent", "data", string(sseData))
				if _, err := fmt.Fprintf(w, "data: %s\n\n", sseData); err != nil {
					slog.Warn("SSE write failed", "error", err)
					return
				}
				flusher.Flush()
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
					sseData, _ := json.Marshal(map[string]any{
						"type":        "tool_use_start",
						"index":       inner.Index,
						"tool_use_id": cbInner.ID,
						"tool_name":   cbInner.Name,
					})
					slog.Info("SSE sent", "data", string(sseData))
					if _, err := fmt.Fprintf(w, "data: %s\n\n", sseData); err != nil {
						slog.Warn("SSE write failed", "error", err)
						return
					}
					flusher.Flush()
					toolCallsData = append(toolCallsData, map[string]any{
						"tool_use_id": cbInner.ID,
						"tool_name":   cbInner.Name,
						"input_json":  "",
						"result":      nil,
					})
				}
			case "content_block_delta":
				var delta map[string]string
				if err := json.Unmarshal(inner.Delta, &delta); err != nil {
					break
				}
				switch delta["type"] {
				case "text_delta":
					if delta["text"] != "" {
						sseData, _ := json.Marshal(map[string]string{
							"type":    "text_delta",
							"content": delta["text"],
						})
						slog.Info("SSE sent", "data", string(sseData))
						if _, err := fmt.Fprintf(w, "data: %s\n\n", sseData); err != nil {
							slog.Warn("SSE write failed", "error", err)
							return
						}
						flusher.Flush()
					}
				case "thinking_delta":
					if delta["thinking"] != "" {
						sseData, _ := json.Marshal(map[string]string{
							"type":    "thinking_delta",
							"content": delta["thinking"],
						})
						slog.Info("SSE sent", "data", string(sseData))
						if _, err := fmt.Fprintf(w, "data: %s\n\n", sseData); err != nil {
							slog.Warn("SSE write failed", "error", err)
							return
						}
						flusher.Flush()
						if len(thinkingBlocks) == 0 {
							thinkingBlocks = append(thinkingBlocks, "")
						}
						thinkingBlocks[len(thinkingBlocks)-1] += delta["thinking"]
					}
				case "input_json_delta":
					if delta["partial_json"] != "" {
						sseData, _ := json.Marshal(map[string]any{
							"type":         "tool_input_delta",
							"index":        inner.Index,
							"partial_json": delta["partial_json"],
						})
						slog.Info("SSE sent", "data", string(sseData))
						if _, err := fmt.Fprintf(w, "data: %s\n\n", sseData); err != nil {
							slog.Warn("SSE write failed", "error", err)
							return
						}
						flusher.Flush()
						if len(toolCallsData) > 0 {
							last := toolCallsData[len(toolCallsData)-1]
							if prev, ok := last["input_json"].(string); ok {
								last["input_json"] = prev + delta["partial_json"]
							}
							toolCallsData[len(toolCallsData)-1] = last
						}
					}
				}
			}
		case "rate_limit_event":
			if event.RateLimitInfo != nil {
				sseData, _ := json.Marshal(map[string]any{
					"type":            "rate_limit",
					"status":          event.RateLimitInfo.Status,
					"resets_at":       event.RateLimitInfo.ResetsAt,
					"rate_limit_type": event.RateLimitInfo.Type,
				})
				slog.Info("SSE sent", "data", string(sseData))
				if _, err := fmt.Fprintf(w, "data: %s\n\n", sseData); err != nil {
					slog.Warn("SSE write failed", "error", err)
					return
				}
				flusher.Flush()
			}
		case "result":
			costUSD = event.CostUSD
			durationMs = event.DurationMs
			costData, _ := json.Marshal(map[string]any{
				"type":           "cost",
				"total_cost_usd": event.CostUSD,
				"duration_ms":    event.DurationMs,
			})
			slog.Info("SSE sent", "data", string(costData))
			if _, err := fmt.Fprintf(w, "data: %s\n\n", costData); err != nil {
				slog.Warn("SSE write failed", "error", err)
				return
			}
			flusher.Flush()
			doneEvent := map[string]interface{}{
				"type":       "done",
				"session_id": event.SessionID,
				"cost_usd":   event.CostUSD,
			}
			data, _ := json.Marshal(doneEvent)
			slog.Info("SSE sent", "data", string(data))
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				slog.Warn("SSE write failed", "error", err)
				return
			}
			flusher.Flush()
		}
	})

	if err != nil {
		slog.Error("message streaming error", "conversation_id", convIDStr, "err", err)
		errEvent := map[string]string{"type": "error", "message": err.Error()}
		data, _ := json.Marshal(errEvent)
		if _, werr := fmt.Fprintf(w, "data: %s\n\n", data); werr != nil {
			slog.Warn("SSE write failed", "error", werr)
		}
		flusher.Flush()
		return
	}

	slog.Info("message streaming completed", "conversation_id", convIDStr)
	// assistant メッセージを DB に保存（session_id を metadata に含める）
	metadata := map[string]interface{}{"session_id": newSessionID}
	if len(thinkingBlocks) > 0 {
		metadata["thinking"] = thinkingBlocks
	} else if thinkingContent != "" {
		metadata["thinking"] = []string{thinkingContent}
	}
	if modelName != "" {
		metadata["model"] = modelName
	}
	if costUSD > 0 {
		metadata["cost_usd"] = costUSD
	}
	if durationMs > 0 {
		metadata["duration_ms"] = durationMs
	}
	if len(hookEventsList) > 0 {
		metadata["hook_events"] = hookEventsList
	}
	if len(toolCallsData) > 0 {
		metadata["tool_calls"] = toolCallsData
	}
	if _, err := h.repo.CreateMessage(r.Context(), convIDStr, "assistant", assistantContent, metadata); err != nil {
		slog.Error("failed to save assistant message", "err", err, "conversation_id", convIDStr)
	}
	if err := h.repo.UpdateConversationUpdatedAt(r.Context(), convIDStr); err != nil {
		slog.Error("failed to update conversation updated_at", "err", err, "conversation_id", convIDStr)
	}
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

func dbConvToAPI(c *db.Conversation) Conversation {
	id, _ := uuid.Parse(c.ID)
	conv := Conversation{
		Id:        id,
		Title:     c.Title,
		Model:     c.Model,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
	if c.SystemPrompt != nil {
		conv.SystemPrompt = c.SystemPrompt
	}
	return conv
}

func dbMsgToAPI(m *db.Message) Message {
	msgID, _ := uuid.Parse(m.ID)
	convID, _ := uuid.Parse(m.ConversationID)
	msg := Message{
		Id:             msgID,
		ConversationId: convID,
		Role:           MessageRole(m.Role),
		Content:        m.Content,
		CreatedAt:      m.CreatedAt,
	}
	if len(m.Metadata) > 0 {
		msg.Metadata = &m.Metadata
	}
	return msg
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
