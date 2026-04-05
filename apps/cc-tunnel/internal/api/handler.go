package api

//go:generate go tool oapi-codegen -config ../../../openapi/oapi-codegen.yaml -o gen.go ../../../openapi/openapi.yaml

import (
	"encoding/json"
	"net/http"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/tmuxclient"
)

// Server implements the generated ServerInterface.
type Server struct {
	client *tmuxclient.ClientWithResponses
}

var _ ServerInterface = (*Server)(nil)

func NewHandler(client *tmuxclient.ClientWithResponses) *Server {
	return &Server{client: client}
}

func (h *Server) CreateSession(w http.ResponseWriter, r *http.Request) {
	var body tmuxclient.CreateSessionJSONRequestBody
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	resp, err := h.client.CreateSessionWithResponse(r.Context(), body)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	if resp.JSON201 != nil {
		s := resp.JSON201
		writeJSON(w, http.StatusCreated, Session{
			Id:        s.Id,
			Type:      SessionType(s.Type),
			TmuxName:  s.TmuxName,
			PaneCount: s.PaneCount,
			CreatedAt: s.CreatedAt,
		})
		return
	}
	proxyErrorResponse(w, resp.StatusCode(), resp.Body)
}

func (h *Server) ListSessions(w http.ResponseWriter, r *http.Request) {
	resp, err := h.client.ListSessionsWithResponse(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	if resp.JSON200 != nil {
		result := make([]Session, 0, len(*resp.JSON200))
		for _, s := range *resp.JSON200 {
			result = append(result, Session{
				Id:        s.Id,
				Type:      SessionType(s.Type),
				TmuxName:  s.TmuxName,
				PaneCount: s.PaneCount,
				CreatedAt: s.CreatedAt,
			})
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	proxyErrorResponse(w, resp.StatusCode(), resp.Body)
}

func (h *Server) DiscoverSessions(w http.ResponseWriter, r *http.Request) {
	resp, err := h.client.DiscoverSessionsWithResponse(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	if resp.JSON200 != nil {
		result := make([]DiscoveredSession, 0, len(*resp.JSON200))
		for _, d := range *resp.JSON200 {
			result = append(result, DiscoveredSession{
				Type:      SessionType(d.Type),
				TmuxNames: d.TmuxNames,
			})
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	proxyErrorResponse(w, resp.StatusCode(), resp.Body)
}

func (h *Server) ResizeSession(w http.ResponseWriter, r *http.Request, sessionId SessionId) {
	var body tmuxclient.ResizeSessionJSONRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.client.ResizeSessionWithResponse(r.Context(), sessionId, body)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	if resp.JSON200 != nil {
		writeJSON(w, http.StatusOK, StatusResponse{Status: resp.JSON200.Status})
		return
	}
	proxyErrorResponse(w, resp.StatusCode(), resp.Body)
}

func (h *Server) SendInput(w http.ResponseWriter, r *http.Request, sessionId SessionId, params SendInputParams) {
	var body tmuxclient.SendInputJSONRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(body.Keys) == 0 {
		writeError(w, http.StatusBadRequest, "keys must not be empty")
		return
	}

	// Forward paneIndex to internal API
	upstreamParams := &tmuxclient.SendInputParams{}
	if params.PaneIndex != nil {
		upstreamParams.PaneIndex = params.PaneIndex
	}

	resp, err := h.client.SendInputWithResponse(r.Context(), sessionId, upstreamParams, body)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	if resp.JSON200 != nil {
		writeJSON(w, http.StatusOK, StatusResponse{Status: resp.JSON200.Status})
		return
	}
	proxyErrorResponse(w, resp.StatusCode(), resp.Body)
}

func (h *Server) GetOutput(w http.ResponseWriter, r *http.Request, sessionId SessionId, params GetOutputParams) {
	// Forward paneIndex to internal API
	upstreamParams := &tmuxclient.GetOutputParams{}
	if params.PaneIndex != nil {
		upstreamParams.PaneIndex = params.PaneIndex
	}

	resp, err := h.client.GetOutputWithResponse(r.Context(), sessionId, upstreamParams)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	if resp.JSON200 != nil {
		writeJSON(w, http.StatusOK, OutputResponse{Output: resp.JSON200.Output})
		return
	}
	proxyErrorResponse(w, resp.StatusCode(), resp.Body)
}

func (h *Server) GetAllOutputs(w http.ResponseWriter, r *http.Request, sessionId SessionId) {
	resp, err := h.client.GetAllOutputsWithResponse(r.Context(), sessionId)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	if resp.JSON200 != nil {
		writeJSON(w, http.StatusOK, AllOutputsResponse{Panes: resp.JSON200.Panes})
		return
	}
	proxyErrorResponse(w, resp.StatusCode(), resp.Body)
}

func (h *Server) DeleteSession(w http.ResponseWriter, r *http.Request, sessionId SessionId) {
	resp, err := h.client.DeleteSessionWithResponse(r.Context(), sessionId)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	if resp.JSON200 != nil {
		writeJSON(w, http.StatusOK, StatusResponse{Status: resp.JSON200.Status})
		return
	}
	proxyErrorResponse(w, resp.StatusCode(), resp.Body)
}

// proxyErrorResponse forwards the upstream error response as-is.
func proxyErrorResponse(w http.ResponseWriter, statusCode int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(body)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, Error{Error: message})
}
