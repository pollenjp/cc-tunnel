package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/credential"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

func (h *Server) GetCredentialsStatus(w http.ResponseWriter, r *http.Request) {
	if h.credService == nil {
		writeJSON(w, http.StatusOK, CredentialsStatusResponse{Registered: true, IsValid: true})
		return
	}
	user, ok := h.requireAppAuthIfEnabled(w, r)
	if !ok {
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
