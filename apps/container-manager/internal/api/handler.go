// Package api exposes the container-manager HTTP surface.
//
// container-manager runs on each GCE VM and is the only client of the local
// Docker daemon (Unix socket). cc-tunnel (Cloud Run) calls these endpoints
// over the VPC; no registry credentials cross the cc-tunnel <-> VM boundary.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	dockerops "github.com/pollenjp/cc-tunnel/apps/container-manager/internal/docker"
)

// AgentManager is the subset of operations Handler needs.
type AgentManager interface {
	Ping(ctx context.Context) error
	RunAgent(ctx context.Context, req dockerops.RunAgentRequest) (string, error)
	StopAgent(ctx context.Context, name string) error
	RemoveAgent(ctx context.Context, name string) error
}

// Handler serves the container-manager HTTP API.
type Handler struct {
	mgr AgentManager
}

// NewHandler constructs a Handler.
func NewHandler(mgr AgentManager) *Handler {
	return &Handler{mgr: mgr}
}

// Routes returns a configured ServeMux.
func (h *Handler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("POST /v1/agents", h.createAgent)
	mux.HandleFunc("POST /v1/agents/{name}/stop", h.stopAgent)
	mux.HandleFunc("DELETE /v1/agents/{name}", h.removeAgent)
	return mux
}

func (h *Handler) healthz(w http.ResponseWriter, r *http.Request) {
	if err := h.mgr.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "docker daemon unreachable: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type createAgentRequest struct {
	Image         string   `json:"image"`
	Name          string   `json:"name"`
	HostPort      int      `json:"host_port"`
	ContainerPort int      `json:"container_port"`
	MemoryMiB     int64    `json:"memory_mib,omitempty"`
	NanoCPUs      int64    `json:"nano_cpus,omitempty"`
	Network       string   `json:"network,omitempty"`
	Env           []string `json:"env,omitempty"`
}

type createAgentResponse struct {
	ID string `json:"id"`
}

func (h *Handler) createAgent(w http.ResponseWriter, r *http.Request) {
	var req createAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decode body: "+err.Error())
		return
	}
	if req.Image == "" || req.Name == "" || req.ContainerPort == 0 {
		writeError(w, http.StatusBadRequest, "image, name, container_port are required")
		return
	}
	if strings.ContainsAny(req.Name, "/ \t\n") {
		writeError(w, http.StatusBadRequest, "invalid container name")
		return
	}

	id, err := h.mgr.RunAgent(r.Context(), dockerops.RunAgentRequest{
		Image:         req.Image,
		Name:          req.Name,
		HostPort:      req.HostPort,
		ContainerPort: req.ContainerPort,
		MemoryBytes:   req.MemoryMiB * 1024 * 1024,
		NanoCPUs:      req.NanoCPUs,
		Network:       req.Network,
		Env:           req.Env,
	})
	if err != nil {
		slog.Error("RunAgent failed", "err", err, "name", req.Name)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, createAgentResponse{ID: id})
}

func (h *Handler) stopAgent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := h.mgr.StopAgent(r.Context(), name); err != nil {
		writeError(w, statusFor(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) removeAgent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := h.mgr.RemoveAgent(r.Context(), name); err != nil {
		writeError(w, statusFor(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// statusFor maps a few common Docker error patterns to HTTP statuses;
// everything else collapses to 500.
func statusFor(err error) int {
	if err == nil {
		return http.StatusOK
	}
	msg := err.Error()
	if strings.Contains(msg, "No such container") || errors.Is(err, errNotFound) {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}

var errNotFound = errors.New("not found")

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
