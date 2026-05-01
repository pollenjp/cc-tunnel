package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

func (h *Server) CreateConversation(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAppAuthIfEnabled(w, r); !ok {
		return
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
	systemPrompt := req.SystemPrompt

	conv, err := h.repo.CreateConversation(r.Context(), title, model, systemPrompt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	slog.Info("conversation created", "conversation_id", conv.ID)
	writeJSON(w, http.StatusCreated, dbConvToAPI(conv))
}

func (h *Server) ListConversations(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAppAuthIfEnabled(w, r); !ok {
		return
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
	if _, ok := h.requireAppAuthIfEnabled(w, r); !ok {
		return
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
	if _, ok := h.requireAppAuthIfEnabled(w, r); !ok {
		return
	}

	if err := h.repo.DeleteConversation(r.Context(), conversationId.String()); err != nil {
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}
	slog.Info("conversation deleted", "conversation_id", conversationId.String())
	writeJSON(w, http.StatusOK, StatusResponse{Status: "ok"})
}
