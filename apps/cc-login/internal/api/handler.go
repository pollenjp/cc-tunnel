package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/pollenjp/cc-tunnel/apps/cc-login/internal/credential"
)

var ErrUnauthorized = errors.New("unauthorized")

// Encryptor interface for testability.
type Encryptor interface {
	Seal(plaintext []byte, username string) (ciphertext, nonce []byte, err error)
	Open(ciphertext, nonce []byte, username string) ([]byte, error)
	KeyVersion() int
}

// TokenResolver resolves a username from a request's Bearer token.
type TokenResolver interface {
	ResolveUsername(r *http.Request) (string, error)
}

type Handler struct {
	ptyMgr    PTYManager
	credStore CredentialStore
	encryptor Encryptor
	resolver  TokenResolver
}

func NewHandler(ptyMgr PTYManager, credStore CredentialStore, encryptor Encryptor, resolver TokenResolver) *Handler {
	return &Handler{
		ptyMgr:    ptyMgr,
		credStore: credStore,
		encryptor: encryptor,
		resolver:  resolver,
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("writeJSON failed", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// resolveUsername extracts the username from the request.
func (h *Handler) resolveUsername(r *http.Request) (string, error) {
	return h.resolver.ResolveUsername(r)
}

// POST /credentials/login
func (h *Handler) PostCredentialsLogin(w http.ResponseWriter, r *http.Request) {
	username, err := h.resolveUsername(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	slog.Info("credentials login request", "username", username)

	resp, err := h.ptyMgr.StartLogin(r.Context(), "claude")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start login")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// POST /credentials/input
func (h *Handler) PostCredentialsInput(w http.ResponseWriter, r *http.Request) {
	username, err := h.resolveUsername(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	slog.Info("credentials input request", "username", username)

	var body struct {
		Input string `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.ptyMgr.SubmitInput(body.Input); err != nil {
		writeError(w, http.StatusConflict, "no login in progress")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Input submitted"})
}

// GET /credentials/output?since=N
func (h *Handler) GetCredentialsOutput(w http.ResponseWriter, r *http.Request) {
	username, err := h.resolveUsername(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	slog.Info("credentials output request", "username", username)

	since := 0
	if s := r.URL.Query().Get("since"); s != "" {
		if _, err := fmt.Sscanf(s, "%d", &since); err != nil {
			slog.Warn("invalid since param", "error", err)
		}
	}

	data, cursor, status := h.ptyMgr.GetOutput(since)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":   data,
		"cursor": cursor,
		"status": string(status),
	})
}

// POST /credentials/cancel
func (h *Handler) PostCredentialsCancel(w http.ResponseWriter, r *http.Request) {
	username, err := h.resolveUsername(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	slog.Info("credentials cancel request", "username", username)

	h.ptyMgr.Cancel()
	if err := h.ptyMgr.ClearLocalCredentials(); err != nil {
		slog.Warn("ClearLocalCredentials failed", "error", err)
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Login cancelled"})
}

// POST /credentials/finalize
func (h *Handler) PostCredentialsFinalize(w http.ResponseWriter, r *http.Request) {
	username, err := h.resolveUsername(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	slog.Info("credentials finalize request", "username", username)

	credJSON, err := h.ptyMgr.ReadCredentials()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read credentials: "+err.Error())
		return
	}

	ct, nonce, err := h.encryptor.Seal(credJSON, username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encryption failed")
		return
	}

	cred := &credential.Credential{
		Username:      username,
		EncryptedData: ct,
		Nonce:         nonce,
		KeyVersion:    h.encryptor.KeyVersion(),
		IsValid:       true,
	}
	if err := h.credStore.Upsert(r.Context(), cred); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save credential")
		return
	}

	// Clean up local credentials file
	if err := h.ptyMgr.ClearLocalCredentials(); err != nil {
		slog.Warn("ClearLocalCredentials failed", "error", err)
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Credentials saved"})
}

// GET /credentials/status
func (h *Handler) GetCredentialsStatus(w http.ResponseWriter, r *http.Request) {
	username, err := h.resolveUsername(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	slog.Info("credentials status request", "username", username)

	cred, err := h.credStore.GetByUsername(r.Context(), username)
	if errors.Is(err, credential.ErrNotFound) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"registered":      false,
			"isValid":         false,
			"lastValidatedAt": nil,
		})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get credential status")
		return
	}

	var lastValidatedAt interface{}
	if cred.LastValidated != nil {
		lastValidatedAt = cred.LastValidated.Format("2006-01-02T15:04:05Z07:00")
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"registered":      true,
		"isValid":         cred.IsValid,
		"lastValidatedAt": lastValidatedAt,
	})
}

// DELETE /credentials
func (h *Handler) DeleteCredentials(w http.ResponseWriter, r *http.Request) {
	username, err := h.resolveUsername(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	slog.Info("credentials delete request", "username", username)

	if err := h.credStore.Delete(r.Context(), username); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete credential")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Credentials deleted"})
}

// bearerToken extracts the Bearer token from Authorization header.
func bearerToken(r *http.Request) (string, bool) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", false
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", false
	}
	return parts[1], true
}
