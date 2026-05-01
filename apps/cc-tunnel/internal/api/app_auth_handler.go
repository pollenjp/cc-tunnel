package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
)

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
