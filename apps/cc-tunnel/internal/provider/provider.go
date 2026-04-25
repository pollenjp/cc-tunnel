package provider

import (
	"context"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

// ExecutionProvider abstracts the execution backend for claude CLI.
// Implementations: local (via cc-remote-agent), cloud_run_sandbox (mock), docker_gce (mock).
type ExecutionProvider interface {
	Execute(ctx context.Context, req remoteclient.Request, onEvent func(remoteclient.StreamEvent)) (string, error)
}
