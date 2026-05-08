package docker

import (
	"context"
	"fmt"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/cmclient"
)

// CMRunner adapts a cmclient.ContainerManager to the DockerRunner interface
// so SessionManager (used by the local execution provider) can drive a remote
// container-manager instead of the local Docker SDK.
//
// container-manager is responsible for the actual image pull and container
// lifecycle on its host. SessionManager addresses the resulting cc-remote-agent
// containers by name on a shared Docker network, so port mapping is not
// required (HostPort=0).
//
// Inspect / List are stubbed because container-manager does not expose them
// and SessionManager's only use of those methods is best-effort cache
// validation and orphan cleanup. A stale cached entry surfaces as an error on
// the next remote call, which is acceptable for local development.
type CMRunner struct {
	cm            cmclient.ContainerManager
	containerPort int
}

var _ DockerRunner = (*CMRunner)(nil)

// NewCMRunner constructs a CMRunner. containerPort is the port the
// cc-remote-agent process listens on inside the container (typically 9090,
// matching cc-remote-agent's default). SessionManager builds the agent URL as
// http://<container-name>:<containerPort> on the shared Docker network.
func NewCMRunner(cm cmclient.ContainerManager, containerPort int) *CMRunner {
	return &CMRunner{cm: cm, containerPort: containerPort}
}

// ContainerCreate calls container-manager to pull the image and start the
// container. The container-manager API does pull+create+start in one step, so
// this returns the container name as the canonical "ID" used by
// SessionManager. ContainerStart is a no-op because the container is already
// running once this returns.
func (r *CMRunner) ContainerCreate(ctx context.Context, opts ContainerCreateOpts) (string, error) {
	if err := r.cm.RunAgent(ctx, cmclient.RunAgentRequest{
		Image:         opts.Image,
		Name:          opts.Name,
		HostPort:      0,
		ContainerPort: r.containerPort,
		Network:       opts.Network,
		Env:           opts.Env,
	}); err != nil {
		return "", fmt.Errorf("cmrunner: run agent: %w", err)
	}
	return opts.Name, nil
}

// ContainerStart is a no-op (RunAgent already starts the container).
func (r *CMRunner) ContainerStart(_ context.Context, _ string) error { return nil }

func (r *CMRunner) ContainerStop(ctx context.Context, containerID string) error {
	return r.cm.StopContainer(ctx, containerID)
}

func (r *CMRunner) ContainerRemove(ctx context.Context, containerID string) error {
	return r.cm.RemoveContainer(ctx, containerID)
}

// ContainerInspect returns a stub "running" state. container-manager has no
// inspect endpoint; if the container is actually dead, the next call to
// cc-remote-agent through the cached client will surface the failure.
func (r *CMRunner) ContainerInspect(_ context.Context, containerID string) (*ContainerInfo, error) {
	return &ContainerInfo{ID: containerID, Name: containerID, State: "running"}, nil
}

// ContainerList returns an empty slice. SessionManager's only consumer is
// CleanupOrphans on startup; in container-manager mode orphan cleanup is the
// container-manager host's responsibility (or a manual operation).
func (r *CMRunner) ContainerList(_ context.Context, _ string, _ bool) ([]ContainerSummary, error) {
	return nil, nil
}
