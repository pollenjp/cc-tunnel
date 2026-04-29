package provider

import (
	"context"
	"errors"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

// ErrSessionNotFound is returned by GetSessionClient when no session container
// exists for the given conversationID. Callers should map this to HTTP 404.
var ErrSessionNotFound = errors.New("session not found")

// ExecutionProvider abstracts the execution backend for claude CLI.
// Implementations: local (via cc-remote-agent), cloud_run_sandbox (mock), docker_gce (mock).
type ExecutionProvider interface {
	Execute(ctx context.Context, req remoteclient.Request, onEvent func(remoteclient.StreamEvent)) (string, error)
	// PrepareForRelogin starts or reuses a session container for the given
	// conversation without injecting credentials, so the frontend can trigger
	// a PTY-based re-login flow against that container.
	PrepareForRelogin(ctx context.Context, conversationID string) error
	// PullCredentialsFromSession reads the credentials.json written by the PTY
	// login flow from the session container and returns the raw JSON string.
	PullCredentialsFromSession(ctx context.Context, conversationID string) (string, error)
	// GetSessionClient returns the remoteclient.Client for an existing per-session
	// container identified by conversationID. Returns ErrSessionNotFound if no
	// session container exists for the given ID.
	GetSessionClient(ctx context.Context, conversationID string) (*remoteclient.Client, error)
}
