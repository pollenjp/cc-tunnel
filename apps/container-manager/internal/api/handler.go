// Package api exposes the container-manager HTTP surface.
//
// container-manager runs on each GCE VM and is the only client of the local
// Docker daemon (Unix socket). cc-tunnel (Cloud Run) calls these endpoints
// over the VPC; no registry credentials cross the cc-tunnel <-> VM boundary.
//
// The HTTP surface is defined in apps/openapi/container-manager.yaml and the
// request/response types plus routing glue are generated into gen.go via
// oapi-codegen. This file implements the generated StrictServerInterface.
package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	dockerops "github.com/pollenjp/cc-tunnel/apps/container-manager/internal/docker"
)

//go:generate go tool oapi-codegen -config ../../../openapi/container-manager.server.yaml -o gen.go ../../../openapi/container-manager.yaml

// AgentManager is the subset of operations Server needs.
type AgentManager interface {
	Ping(ctx context.Context) error
	RunAgent(ctx context.Context, req dockerops.RunAgentRequest) (string, error)
	StopAgent(ctx context.Context, name string) error
	RemoveAgent(ctx context.Context, name string) error
}

// Server implements the generated StrictServerInterface backed by AgentManager.
type Server struct {
	mgr AgentManager
}

// NewServer constructs a Server.
func NewServer(mgr AgentManager) *Server {
	return &Server{mgr: mgr}
}

// Routes returns an http.Handler with routing matching the OpenAPI spec.
func (h *Server) Routes() http.Handler {
	return HandlerFromMux(NewStrictHandler(h, nil), http.NewServeMux())
}

// Compile-time check that Server satisfies the generated interface.
var _ StrictServerInterface = (*Server)(nil)

func (h *Server) GetHealthz(ctx context.Context, _ GetHealthzRequestObject) (GetHealthzResponseObject, error) {
	if err := h.mgr.Ping(ctx); err != nil {
		return GetHealthz503JSONResponse{Error: "docker daemon unreachable: " + err.Error()}, nil
	}
	return GetHealthz200JSONResponse{Status: "ok"}, nil
}

func (h *Server) CreateAgent(ctx context.Context, request CreateAgentRequestObject) (CreateAgentResponseObject, error) {
	if request.Body == nil {
		return CreateAgent400JSONResponse{Error: "missing request body"}, nil
	}
	req := *request.Body
	if req.Image == "" || req.Name == "" || req.ContainerPort == 0 {
		return CreateAgent400JSONResponse{Error: "image, name, container_port are required"}, nil
	}
	if strings.ContainsAny(req.Name, "/ \t\n") {
		return CreateAgent400JSONResponse{Error: "invalid container name"}, nil
	}

	id, err := h.mgr.RunAgent(ctx, dockerops.RunAgentRequest{
		Image:         req.Image,
		Name:          req.Name,
		HostPort:      int(deref(req.HostPort)),
		ContainerPort: int(req.ContainerPort),
		MemoryBytes:   deref(req.MemoryMib) * 1024 * 1024,
		NanoCPUs:      deref(req.NanoCpus),
		Network:       deref(req.Network),
		Env:           derefSlice(req.Env),
	})
	if err != nil {
		slog.Error("RunAgent failed", "err", err, "name", req.Name)
		return CreateAgent500JSONResponse{Error: err.Error()}, nil
	}
	return CreateAgent201JSONResponse{Id: id}, nil
}

func (h *Server) StopAgent(ctx context.Context, request StopAgentRequestObject) (StopAgentResponseObject, error) {
	if err := h.mgr.StopAgent(ctx, request.Name); err != nil {
		if isNotFound(err) {
			return StopAgent404JSONResponse{Error: err.Error()}, nil
		}
		return StopAgent500JSONResponse{Error: err.Error()}, nil
	}
	return StopAgent204Response{}, nil
}

func (h *Server) RemoveAgent(ctx context.Context, request RemoveAgentRequestObject) (RemoveAgentResponseObject, error) {
	if err := h.mgr.RemoveAgent(ctx, request.Name); err != nil {
		if isNotFound(err) {
			return RemoveAgent404JSONResponse{Error: err.Error()}, nil
		}
		return RemoveAgent500JSONResponse{Error: err.Error()}, nil
	}
	return RemoveAgent204Response{}, nil
}

var errNotFound = errors.New("not found")

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errNotFound) {
		return true
	}
	return strings.Contains(err.Error(), "No such container")
}

func deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

func derefSlice[T any](p *[]T) []T {
	if p == nil {
		return nil
	}
	return *p
}
