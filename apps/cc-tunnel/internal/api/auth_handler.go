package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// PTY-based auth flow proxied to the per-session cc-remote-agent container.
// These endpoints all resolve the session client via executionProvider and
// forward the call.

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
