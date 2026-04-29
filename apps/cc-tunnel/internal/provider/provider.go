package provider

import (
	"context"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

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
}
