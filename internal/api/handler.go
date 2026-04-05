package api

import (
	"encoding/json"
	"net/http"

	"github.com/pollenjp/cc-tunnel/internal/session"
)

type Handler struct {
	manager *session.Manager
}

func NewHandler(m *session.Manager) *Handler {
	return &Handler{manager: m}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /sessions", h.createSession)
	mux.HandleFunc("GET /sessions", h.listSessions)
	mux.HandleFunc("POST /sessions/{id}/input", h.sendInput)
	mux.HandleFunc("GET /sessions/{id}/output", h.getOutput)
	mux.HandleFunc("DELETE /sessions/{id}", h.deleteSession)
}

func (h *Handler) createSession(w http.ResponseWriter, r *http.Request) {
	s, err := h.manager.Create()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, s)
}

func (h *Handler) listSessions(w http.ResponseWriter, r *http.Request) {
	sessions := h.manager.List()
	writeJSON(w, http.StatusOK, sessions)
}

func (h *Handler) sendInput(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.manager.SendInput(id, body.Text); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) getOutput(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	output, err := h.manager.GetOutput(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"output": output})
}

func (h *Handler) deleteSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.manager.Delete(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
