package dockerhost

import "context"

// ContainerManager abstracts Docker daemon container operations for testability.
type ContainerManager interface {
	// RunAgentContainer starts a new cc-remote-agent container on the remote Docker daemon.
	// image is the container image, name is the container name,
	// hostPort is the port exposed on the VM host, containerPort is the port inside the container.
	RunAgentContainer(ctx context.Context, image, name string, hostPort, containerPort int) error

	// StopContainer stops a running container gracefully.
	StopContainer(ctx context.Context, name string) error

	// RemoveContainer removes a container (must be stopped first, or force is used).
	RemoveContainer(ctx context.Context, name string) error

	// IsReady checks if the Docker daemon is reachable.
	IsReady(ctx context.Context) bool
}
