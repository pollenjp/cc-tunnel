// Package cmclient is the cc-tunnel-side HTTP client for the
// container-manager service that runs on each GCE VM. The interface mirrors
// the previous dockerhost package so the dockergce provider could be migrated
// in place.
package cmclient

//go:generate go tool oapi-codegen -config ../../../openapi/container-manager.client.yaml -o genclient/gen.go ../../../openapi/container-manager.yaml

import "context"

// RunAgentRequest mirrors container-manager's POST /v1/agents body. HostPort=0
// means no host port mapping. Network and Env fall back to the
// container-manager's own defaults when zero-valued.
type RunAgentRequest struct {
	Image         string
	Name          string
	HostPort      int
	ContainerPort int
	Network       string
	Env           []string
	// Labels are forwarded to the container-manager and applied both to the
	// Docker container metadata and the gcplogs log driver so the keys
	// surface as Cloud Logging entry labels. Restrict to operational keys
	// (conversation_id, vm_instance_id, component); values are not redacted.
	Labels map[string]string
}

// ContainerManager abstracts cc-remote-agent container operations on a
// remote VM, executed via the container-manager HTTP API.
type ContainerManager interface {
	// RunAgent pulls the image and starts a new cc-remote-agent container
	// on the VM.
	RunAgent(ctx context.Context, req RunAgentRequest) error

	// RunAgentContainer is a convenience wrapper around RunAgent for the
	// production GCE path that only needs the four core parameters.
	RunAgentContainer(ctx context.Context, image, name string, hostPort, containerPort int) error

	// StopContainer stops a running container gracefully.
	StopContainer(ctx context.Context, name string) error

	// RemoveContainer force-removes a container.
	RemoveContainer(ctx context.Context, name string) error

	// IsReady checks if the container-manager (and the underlying Docker
	// daemon) is reachable.
	IsReady(ctx context.Context) bool
}
