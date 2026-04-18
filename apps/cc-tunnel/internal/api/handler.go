package api

//go:generate go tool oapi-codegen -config ../../../openapi/oapi-codegen.yaml -o gen.go ../../../openapi/openapi.yaml

import (
	"encoding/json"
	"fmt"
	"net/http"

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
		json.NewDecoder(r.Body).Decode(&req)
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
		json.NewDecoder(r.Body).Decode(&req)
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
	var assistantContent string
	var thinkingContent string
	executeReq := remoteclient.Request{
		Prompt:              req.Content,
		SessionID:           resumeSessionID,
		Model:               conv.Model,
		ConversationHistory: convHistory,
	}
	if conv.SystemPrompt != nil {
		executeReq.SystemPrompt = *conv.SystemPrompt
	}

	newSessionID, err := h.remote.Execute(r.Context(), executeReq, func(event remoteclient.StreamEvent) {
		switch event.Type {
		case "assistant":
			if event.Message != nil {
				for _, block := range event.Message.Content {
					if block.Type == "thinking" && block.Thinking != "" {
						thinkingContent += block.Thinking
						sseEvent := map[string]string{"type": "thinking", "content": block.Thinking}
						data, _ := json.Marshal(sseEvent)
						fmt.Fprintf(w, "data: %s\n\n", data)
						flusher.Flush()
					}
					if block.Type == "text" && block.Text != "" {
						assistantContent += block.Text
						sseEvent := map[string]string{"type": "text", "content": block.Text}
						data, _ := json.Marshal(sseEvent)
						fmt.Fprintf(w, "data: %s\n\n", data)
						flusher.Flush()
					}
				}
			}
		case "result":
			doneEvent := map[string]interface{}{
				"type":       "done",
				"session_id": event.SessionID,
				"cost_usd":   event.CostUSD,
			}
			data, _ := json.Marshal(doneEvent)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	})

	if err != nil {
		errEvent := map[string]string{"type": "error", "message": err.Error()}
		data, _ := json.Marshal(errEvent)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		return
	}

	// assistant メッセージを DB に保存（session_id を metadata に含める）
	metadata := map[string]interface{}{"session_id": newSessionID}
	if thinkingContent != "" {
		metadata["thinking"] = thinkingContent
	}
	h.repo.CreateMessage(r.Context(), convIDStr, "assistant", assistantContent, metadata)
	h.repo.UpdateConversationUpdatedAt(r.Context(), convIDStr)
}

// --- helper functions ---

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
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
