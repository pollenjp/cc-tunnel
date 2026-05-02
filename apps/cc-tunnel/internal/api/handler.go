package api

//go:generate go tool oapi-codegen -config ../../../openapi/oapi-codegen.yaml -o gen.go ../../../openapi/openapi.yaml

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/credential"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/db"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

// getSessionClientOrRespond is a helper that calls executionProvider.GetSessionClient
// and writes the appropriate HTTP error if the session is not found or another error occurs.
// Returns the client and true on success, or nil and false (with the error already written) on failure.
func getSessionClientOrRespond(w http.ResponseWriter, r *http.Request, ep provider.ExecutionProvider, convID string) (*remoteclient.Client, bool) {
	client, err := ep.GetSessionClient(r.Context(), convID)
	if err != nil {
		if errors.Is(err, provider.ErrSessionNotFound) {
			writeError(w, http.StatusNotFound, fmt.Sprintf("session not found for conversation %s", convID))
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return nil, false
	}
	return client, true
}

// AppSession is an in-memory session store for mock authentication.
// It maps Bearer tokens to AppUser values. Safe for concurrent use.
type AppSession struct {
	mu    sync.RWMutex
	store map[string]AppUser // token → user
}

func newAppSession() *AppSession {
	return &AppSession{store: make(map[string]AppUser)}
}

func (s *AppSession) set(token string, user AppUser) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store[token] = user
}

func (s *AppSession) get(token string) (AppUser, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.store[token]
	return u, ok
}

func (s *AppSession) delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.store, token)
}

type Server struct {
	repo              repository
	executionProvider provider.ExecutionProvider
	session           *AppSession
	credService       credentialService // nil = skip credential check (testing / no-auth mode)
	credStorer        credentialStorer  // nil = credential storage unavailable
	batchInterval     time.Duration     // 0 = default 2s; override for testing
	doneCh            chan struct{}      // closed when Execute goroutine completes; for testing only
}

var _ ServerInterface = (*Server)(nil)

func NewHandler(repo *db.Repository, execProvider provider.ExecutionProvider) *Server {
	return &Server{repo: repo, executionProvider: execProvider, session: newAppSession()}
}

// NewHandlerWithCredentials creates a Server with credential validation enabled.
func NewHandlerWithCredentials(repo *db.Repository, execProvider provider.ExecutionProvider, credSvc credentialService) *Server {
	return &Server{repo: repo, executionProvider: execProvider, session: newAppSession(), credService: credSvc}
}

// NewHandlerFull creates a Server with both credential validation and credential storage enabled.
func NewHandlerFull(repo *db.Repository, execProvider provider.ExecutionProvider, credSvc credentialService, credStore credentialStorer) *Server {
	return &Server{repo: repo, executionProvider: execProvider, session: newAppSession(), credService: credSvc, credStorer: credStore}
}

func (h *Server) GetAuthStatus(w http.ResponseWriter, r *http.Request, params GetAuthStatusParams) {
	convID := params.ConversationId.String()
	sessionClient, ok := getSessionClientOrRespond(w, r, h.executionProvider, convID)
	if !ok {
		return
	}
	status, err := sessionClient.GetAuthStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *Server) InitiateLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	convID := req.ConversationId.String()
	sessionClient, ok := getSessionClientOrRespond(w, r, h.executionProvider, convID)
	if !ok {
		return
	}
	method := ""
	if req.Method != nil {
		method = string(*req.Method)
	}
	resp, err := sessionClient.InitiateLogin(r.Context(), method)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Server) Logout(w http.ResponseWriter, r *http.Request, params LogoutParams) {
	convID := params.ConversationId.String()
	sessionClient, ok := getSessionClientOrRespond(w, r, h.executionProvider, convID)
	if !ok {
		return
	}
	status, err := sessionClient.Logout(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *Server) CancelLogin(w http.ResponseWriter, r *http.Request, params CancelLoginParams) {
	convID := params.ConversationId.String()
	sessionClient, ok := getSessionClientOrRespond(w, r, h.executionProvider, convID)
	if !ok {
		return
	}
	resp, err := sessionClient.CancelLogin(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Server) SubmitAuthPtyInput(w http.ResponseWriter, r *http.Request) {
	var req AuthInputRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	convID := req.ConversationId.String()
	sessionClient, ok := getSessionClientOrRespond(w, r, h.executionProvider, convID)
	if !ok {
		return
	}
	resp, err := sessionClient.SubmitAuthInput(r.Context(), req.Input)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Server) GetAuthPtyStream(w http.ResponseWriter, r *http.Request, params GetAuthPtyStreamParams) {
	convID := params.ConversationId.String()
	sessionClient, ok := getSessionClientOrRespond(w, r, h.executionProvider, convID)
	if !ok {
		return
	}
	rc, err := sessionClient.GetAuthPtyStream(r.Context(), convID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer func() {
		if err := rc.Close(); err != nil {
			slog.Warn("rc.Close failed", "error", err)
		}
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	buf := make([]byte, 4096)
	for {
		n, err := rc.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return
			}
			flusher.Flush()
		}
		if err != nil {
			return
		}
	}
}

func (h *Server) CreateConversation(w http.ResponseWriter, r *http.Request) {
	if h.credService != nil {
		token, ok := bearerToken(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		_, found := h.session.get(token)
		if !found {
			writeJSON(w, http.StatusUnauthorized, AppAuthError{Message: "unauthorized"})
			return
		}
	}

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
	if h.credService != nil {
		token, ok := bearerToken(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		_, found := h.session.get(token)
		if !found {
			writeJSON(w, http.StatusUnauthorized, AppAuthError{Message: "unauthorized"})
			return
		}
	}

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
	if h.credService != nil {
		token, ok := bearerToken(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		_, found := h.session.get(token)
		if !found {
			writeJSON(w, http.StatusUnauthorized, AppAuthError{Message: "unauthorized"})
			return
		}
	}

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
	if h.credService != nil {
		token, ok := bearerToken(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		_, found := h.session.get(token)
		if !found {
			writeJSON(w, http.StatusUnauthorized, AppAuthError{Message: "unauthorized"})
			return
		}
	}

	if err := h.repo.DeleteConversation(r.Context(), conversationId.String()); err != nil {
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}
	slog.Info("conversation deleted", "conversation_id", conversationId.String())
	writeJSON(w, http.StatusOK, StatusResponse{Status: "ok"})
}

func (h *Server) GetCredentialsStatus(w http.ResponseWriter, r *http.Request) {
	if h.credService == nil {
		writeJSON(w, http.StatusOK, CredentialsStatusResponse{Registered: true, IsValid: true})
		return
	}
	token, ok := bearerToken(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	user, found := h.session.get(token)
	if !found {
		writeJSON(w, http.StatusUnauthorized, AppAuthError{Message: "unauthorized"})
		return
	}
	_, err := h.credService.FetchAndDecrypt(r.Context(), user.Name)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, CredentialsStatusResponse{Registered: true, IsValid: true})
	case errors.Is(err, credential.ErrNotFound):
		writeJSON(w, http.StatusOK, CredentialsStatusResponse{Registered: false, IsValid: false})
	case errors.Is(err, credential.ErrCredentialsInvalid):
		writeJSON(w, http.StatusOK, CredentialsStatusResponse{Registered: true, IsValid: false})
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
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

	// Credential check: fetch and decrypt if credService is configured.
	var credJSON []byte
	if h.credService != nil {
		token, ok := bearerToken(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		user, found := h.session.get(token)
		if !found {
			writeJSON(w, http.StatusUnauthorized, AppAuthError{Message: "unauthorized"})
			return
		}
		var credErr error
		credJSON, credErr = h.credService.FetchAndDecrypt(r.Context(), user.Name)
		if errors.Is(credErr, credential.ErrNotFound) {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error":    "credentials_required",
				"redirect": fmt.Sprintf("/login/credentials?reason=missing&conversationId=%s", convIDStr),
			})
			return
		}
		if errors.Is(credErr, credential.ErrCredentialsInvalid) {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error":    "credentials_invalid",
				"redirect": fmt.Sprintf("/login/credentials?reason=expired&conversationId=%s", convIDStr),
			})
			return
		}
		if credErr != nil {
			writeError(w, http.StatusInternalServerError, credErr.Error())
			return
		}
	}

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
	_, err = h.repo.CreateMessage(r.Context(), convIDStr, "user", map[string]any{"content": req.Content})
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
			if cbs, ok := m.MessageData["content_blocks"].([]any); ok {
				for _, cb := range cbs {
					if block, ok := cb.(map[string]any); ok {
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
		ConversationID:         convIDStr,
		Model:                  conv.Model,
		ConversationHistory:    convHistory,
		IncludePartialMessages: true,
		IncludeHookEvents:      true,
		Credentials:            credJSON,
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
		newSessionID, err := h.executionProvider.Execute(execCtx, executeReq, func(event remoteclient.StreamEvent) {
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

// PostReloginStart starts a re-login flow for a conversation by ensuring its
// session container is running (without credentials), so the frontend can
// trigger the PTY-based /auth/* flow against it.
func (h *Server) PostReloginStart(w http.ResponseWriter, r *http.Request) {
	token, ok := bearerToken(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	_, found := h.session.get(token)
	if !found {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body ReloginStartRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if body.ConversationId == (uuid.UUID{}) {
		writeError(w, http.StatusBadRequest, "conversationId is required")
		return
	}

	convIDStr := body.ConversationId.String()
	if err := h.executionProvider.PrepareForRelogin(r.Context(), convIDStr); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	defer func() {
		if err := h.repo.UpdateSessionEndpointLastActivity(r.Context(), convIDStr); err != nil {
			slog.Warn("failed to update last_activity on relogin start", "err", err, "conversation_id", convIDStr)
		}
	}()

	writeJSON(w, http.StatusOK, ReloginStartResponse{Ready: true})
}

// PostReloginFinalize reads the credentials written by the PTY login flow from
// the session container, encrypts them, and stores them in the DB.
func (h *Server) PostReloginFinalize(w http.ResponseWriter, r *http.Request) {
	token, ok := bearerToken(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	user, found := h.session.get(token)
	if !found {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body ReloginFinalizeRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if body.ConversationId == (uuid.UUID{}) {
		writeError(w, http.StatusBadRequest, "conversationId is required")
		return
	}

	convIDStr := body.ConversationId.String()
	defer func() {
		if err := h.repo.UpdateSessionEndpointLastActivity(r.Context(), convIDStr); err != nil {
			slog.Warn("failed to update last_activity on relogin finalize", "err", err, "conversation_id", convIDStr)
		}
	}()

	credJSON, err := h.executionProvider.PullCredentialsFromSession(r.Context(), convIDStr)
	if errors.Is(err, remoteclient.ErrCredentialsNotReady) {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "credentials not ready, complete /auth/login first",
		})
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	if h.credStorer == nil {
		writeError(w, http.StatusInternalServerError, "credential storage not configured")
		return
	}
	if err := h.credStorer.StoreCredential(r.Context(), user.Name, credJSON); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, ReloginFinalizeResponse{Registered: true, IsValid: true})
}

func (h *Server) AppAuthLogin(w http.ResponseWriter, r *http.Request) {
	var req AppAuthLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Username == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	token := hex.EncodeToString(tokenBytes)

	user := AppUser{Id: uuid.New().String(), Name: req.Username}
	h.session.set(token, user)

	writeJSON(w, http.StatusOK, AppAuthLoginResponse{Token: token, User: user})
}

func (h *Server) AppAuthLogout(w http.ResponseWriter, r *http.Request) {
	token, ok := bearerToken(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, AppAuthError{Message: "unauthorized"})
		return
	}
	h.session.delete(token)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Server) AppAuthGetMe(w http.ResponseWriter, r *http.Request) {
	token, ok := bearerToken(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, AppAuthError{Message: "unauthorized"})
		return
	}
	user, found := h.session.get(token)
	if !found {
		writeJSON(w, http.StatusUnauthorized, AppAuthError{Message: "unauthorized"})
		return
	}
	writeJSON(w, http.StatusOK, AppAuthMeResponse{User: user})
}

func (h *Server) AppAuthUpdateMe(w http.ResponseWriter, r *http.Request) {
	token, ok := bearerToken(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, AppAuthError{Message: "unauthorized"})
		return
	}
	user, found := h.session.get(token)
	if !found {
		writeJSON(w, http.StatusUnauthorized, AppAuthError{Message: "unauthorized"})
		return
	}
	var req AppAuthUpdateMeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	user.Name = req.Nickname
	h.session.set(token, user)
	writeJSON(w, http.StatusOK, AppAuthMeResponse{User: user})
}

// bearerToken extracts the Bearer token from the Authorization header.
func bearerToken(r *http.Request) (string, bool) {
	auth := r.Header.Get("Authorization")
	token, found := strings.CutPrefix(auth, "Bearer ")
	if !found || token == "" {
		return "", false
	}
	return token, true
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
		maps.Copy(m, b)
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
