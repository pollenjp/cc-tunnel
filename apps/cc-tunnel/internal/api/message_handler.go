package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/credential"
)

// SendMessage validates the request, persists the user message, kicks off the
// CLI execution in a goroutine, and returns 202 with the assistant message id.
// Heavy lifting (streaming aggregation, batch persistence, finalization) lives
// in executeAndPersist.
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

	credJSON, ok := h.fetchCredentialOrRespond(w, r, convIDStr)
	if !ok {
		return
	}

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

	if _, err := h.repo.CreateMessage(r.Context(), convIDStr, "user", map[string]any{"content": req.Content}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	executeReq := buildExecuteRequest(req.Content, convIDStr, conv, history, credJSON)

	// execCtx is independent of r.Context() so a frontend disconnect (which
	// cancels r.Context()) does not abort the Claude CLI execution or the DB save.
	execCtx := context.WithoutCancel(r.Context())

	assistantMsg, err := h.repo.CreateStreamingMessage(execCtx, convIDStr, "assistant", map[string]interface{}{})
	if err != nil {
		slog.Error("failed to create streaming assistant message", "err", err, "conversation_id", convIDStr)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	msgUUID, _ := uuid.Parse(assistantMsg.ID)
	writeJSON(w, http.StatusAccepted, SendMessageResponse{MessageId: msgUUID})

	go h.executeAndPersist(execCtx, executeReq, convIDStr, assistantMsg.ID)
}

// fetchCredentialOrRespond performs the SendMessage credential gate. Returns
// (credJSON, true) on success. On rejection the response is already written
// and ok=false is returned. When credService is nil, returns (nil, true).
//
// Response shapes are intentional and used by the frontend for routing:
//   - missing bearer            → Error{Error: "unauthorized"}                   401
//   - unknown bearer            → AppAuthError{Message: "unauthorized"}          401
//   - credential row missing    → {error: "credentials_required",  redirect}    401
//   - credential row invalid    → {error: "credentials_invalid",   redirect}    401
//   - decryption / DB error     → Error{Error: <msg>}                            500
func (h *Server) fetchCredentialOrRespond(w http.ResponseWriter, r *http.Request, convIDStr string) ([]byte, bool) {
	if h.credService == nil {
		return nil, true
	}
	token, ok := bearerToken(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return nil, false
	}
	user, found := h.session.get(token)
	if !found {
		writeJSON(w, http.StatusUnauthorized, AppAuthError{Message: "unauthorized"})
		return nil, false
	}
	credJSON, err := h.credService.FetchAndDecrypt(r.Context(), user.Name)
	switch {
	case err == nil:
		return credJSON, true
	case errors.Is(err, credential.ErrNotFound):
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error":    "credentials_required",
			"redirect": fmt.Sprintf("/login/credentials?reason=missing&conversationId=%s", convIDStr),
		})
		return nil, false
	case errors.Is(err, credential.ErrCredentialsInvalid):
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error":    "credentials_invalid",
			"redirect": fmt.Sprintf("/login/credentials?reason=expired&conversationId=%s", convIDStr),
		})
		return nil, false
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
		return nil, false
	}
}
