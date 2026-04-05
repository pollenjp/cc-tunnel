package api

//go:generate oapi-codegen -package api -generate std-http-server,models,spec -o gen.go ../../../openapi/openapi.yaml

import (
	"encoding/json"
	"net/http"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/session"
)

// Server implements the generated ServerInterface.
type Server struct {
	manager *session.Manager
}

var _ ServerInterface = (*Server)(nil)

func NewHandler(m *session.Manager) *Server {
	return &Server{manager: m}
}

func (h *Server) CreateSession(w http.ResponseWriter, r *http.Request) {
	s, err := h.manager.Create()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, Session{
		Id:        s.ID,
		TmuxName:  s.TmuxName,
		CreatedAt: s.CreatedAt,
	})
}

func (h *Server) ListSessions(w http.ResponseWriter, r *http.Request) {
	list := h.manager.List()
	result := make([]Session, 0, len(list))
	for _, s := range list {
		result = append(result, Session{
			Id:        s.ID,
			TmuxName:  s.TmuxName,
			CreatedAt: s.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Server) SendInput(w http.ResponseWriter, r *http.Request, sessionId SessionId) {
	var body SendInputRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.manager.SendInput(sessionId, body.Text); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, StatusResponse{Status: "ok"})
}

func (h *Server) GetOutput(w http.ResponseWriter, r *http.Request, sessionId SessionId) {
	output, err := h.manager.GetOutput(sessionId)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, OutputResponse{Output: output})
}

func (h *Server) DeleteSession(w http.ResponseWriter, r *http.Request, sessionId SessionId) {
	if err := h.manager.Delete(sessionId); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, StatusResponse{Status: "deleted"})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, Error{Error: message})
}
