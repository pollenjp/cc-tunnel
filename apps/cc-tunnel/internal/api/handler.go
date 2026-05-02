package api

//go:generate go tool oapi-codegen -config ../../../openapi/oapi-codegen.yaml -o gen.go ../../../openapi/openapi.yaml

import (
	"time"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/db"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider"
)

// Server implements the generated ServerInterface. The struct intentionally
// holds only collaborators (repo, executionProvider, credential plumbing) and
// a couple of test-only fields. Each handler lives in its own file:
//
//   - auth_handler.go         PTY auth proxy (per-session)
//   - conversation_handler.go conversation CRUD
//   - credentials_handler.go  credential status + relogin flow
//   - app_auth_handler.go     app-level Bearer auth (login/logout/me)
//   - message_handler.go      SendMessage (thin: request → service → 202)
//   - message_service.go      executeAndPersist + streamAggregator
//   - app_session.go          AppSession + bearerToken + requireAppAuthIfEnabled
//   - helpers.go              writeJSON/writeError/clone helpers + LoggingMiddleware
type Server struct {
	repo              repository
	executionProvider provider.ExecutionProvider
	session           *AppSession
	credService       credentialService // nil = skip credential check (testing / no-auth mode)
	credStorer        credentialStorer  // nil = credential storage unavailable
	batchInterval     time.Duration     // 0 = default 2s; override for testing
	doneCh            chan struct{}     // closed when executeAndPersist completes; for testing only
}

var _ ServerInterface = (*Server)(nil)

func NewHandler(repo *db.Repository, execProvider provider.ExecutionProvider) *Server {
	return &Server{repo: repo, executionProvider: execProvider, session: newAppSession()}
}

// NewHandlerWithCredentials creates a Server with credential validation enabled.
func NewHandlerWithCredentials(repo *db.Repository, execProvider provider.ExecutionProvider, credSvc credentialService) *Server {
	return &Server{repo: repo, executionProvider: execProvider, session: newAppSession(), credService: credSvc}
}

// NewHandlerFull creates a Server with both credential validation and credential storage enabled.
func NewHandlerFull(repo *db.Repository, execProvider provider.ExecutionProvider, credSvc credentialService, credStore credentialStorer) *Server {
	return &Server{repo: repo, executionProvider: execProvider, session: newAppSession(), credService: credSvc, credStorer: credStore}
}
