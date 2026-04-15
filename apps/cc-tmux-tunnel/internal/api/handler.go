package api

//go:generate go tool oapi-codegen -config ../../../openapi/internal-oapi-codegen-server.yaml -o gen.go ../../../openapi/internal-openapi.yaml

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pollenjp/cc-tunnel/apps/cc-tmux-tunnel/internal/session"
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
	var body CreateSessionRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	opts := session.CreateOptions{}
	if body.Type != nil {
		opts.Type = string(*body.Type)
	}
	if body.TmuxName != nil {
		opts.TmuxName = *body.TmuxName
	}
	if body.Width != nil {
		opts.Width = *body.Width
	}
	if body.Height != nil {
		opts.Height = *body.Height
	}

	s, err := h.manager.Create(opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, Session{
		Id:        s.ID,
		Type:      SessionType(s.Type),
		TmuxName:  s.TmuxName,
		PaneCount: s.PaneCount,
		CreatedAt: s.CreatedAt,
	})
}

func (h *Server) ListSessions(w http.ResponseWriter, r *http.Request) {
	list := h.manager.List()
	result := make([]Session, 0, len(list))
	for _, s := range list {
		result = append(result, Session{
			Id:        s.ID,
			Type:      SessionType(s.Type),
			TmuxName:  s.TmuxName,
			PaneCount: s.PaneCount,
			CreatedAt: s.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Server) DiscoverSessions(w http.ResponseWriter, r *http.Request) {
	discovered, err := h.manager.DiscoverSessions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result := make([]DiscoveredSession, 0, len(discovered))
	for _, d := range discovered {
		result = append(result, DiscoveredSession{
			Type:      SessionType(d.Type),
			TmuxNames: d.TmuxNames,
		})
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Server) ResizeSession(w http.ResponseWriter, r *http.Request, sessionId SessionId, params ResizeSessionParams) {
	var body ResizeRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var colWidths, rowHeights []int
	if body.ColWidths != nil {
		colWidths = *body.ColWidths
	}
	if body.RowHeights != nil {
		rowHeights = *body.RowHeights
	}

	var paneIndex *int
	if params.PaneIndex != nil {
		v := *params.PaneIndex
		paneIndex = &v
	}

	if err := h.manager.Resize(sessionId, body.Width, body.Height, paneIndex, colWidths, rowHeights); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, StatusResponse{Status: "ok"})
}

func (h *Server) SendInput(w http.ResponseWriter, r *http.Request, sessionId SessionId, params SendInputParams) {
	var body SendInputRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(body.Keys) == 0 {
		writeError(w, http.StatusBadRequest, "keys must not be empty")
		return
	}

	paneIndex := 0
	if params.PaneIndex != nil {
		paneIndex = *params.PaneIndex
	}

	if err := h.manager.SendKeysToPane(sessionId, paneIndex, body.Keys); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, StatusResponse{Status: "ok"})
}

func (h *Server) GetOutput(w http.ResponseWriter, r *http.Request, sessionId SessionId, params GetOutputParams) {
	paneIndex := 0
	if params.PaneIndex != nil {
		paneIndex = *params.PaneIndex
	}

	output, err := h.manager.GetPaneOutput(sessionId, paneIndex)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, OutputResponse{Output: output})
}

func (h *Server) GetAllOutputs(w http.ResponseWriter, r *http.Request, sessionId SessionId) {
	outputs, err := h.manager.GetAllPaneOutputs(sessionId)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	panes := make(map[string]string, len(outputs))
	for idx, output := range outputs {
		panes[fmt.Sprintf("%d", idx)] = output
	}

	writeJSON(w, http.StatusOK, AllOutputsResponse{Panes: panes})
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
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, Error{Error: message})
}
