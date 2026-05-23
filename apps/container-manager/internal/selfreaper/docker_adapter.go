package selfreaper

import (
	"context"

	dockerops "github.com/pollenjp/cc-tunnel/apps/container-manager/internal/docker"
)

// DockerAgentLister adapts docker.Manager to the AgentLister interface
// used by Reaper. It is a thin shim — the source-of-truth filter
// (label=component=cc-remote-agent + status=running) lives in
// Manager.ListAgents so cc-tunnel and self-reaper see the exact same
// count.
type DockerAgentLister struct {
	Manager *dockerops.Manager
}

func (d DockerAgentLister) AgentCount(ctx context.Context) (int, error) {
	agents, err := d.Manager.ListAgents(ctx)
	if err != nil {
		return 0, err
	}
	return len(agents), nil
}
