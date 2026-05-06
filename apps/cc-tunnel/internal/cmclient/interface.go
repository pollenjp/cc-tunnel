// Package cmclient is the cc-tunnel-side HTTP client for the
// container-manager service that runs on each GCE VM. The interface mirrors
// the previous dockerhost package so the dockergce provider could be migrated
// in place.
package cmclient

import "context"

// ContainerManager abstracts cc-remote-agent container operations on a
// remote VM, executed via the container-manager HTTP API.
type ContainerManager interface {
	// RunAgentContainer pulls the image and starts a new cc-remote-agent
	// container on the VM. image is the container image URL, name is the
	// container name, hostPort is the port exposed on the VM host, and
	// containerPort is the port inside the container.
	RunAgentContainer(ctx context.Context, image, name string, hostPort, containerPort int) error

	// StopContainer stops a running container gracefully.
	StopContainer(ctx context.Context, name string) error

	// RemoveContainer force-removes a container.
	RemoveContainer(ctx context.Context, name string) error

	// IsReady checks if the container-manager (and the underlying Docker
	// daemon) is reachable.
	IsReady(ctx context.Context) bool
}
