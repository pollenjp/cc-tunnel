package dockerhost

import (
	"context"
	"fmt"
	"strconv"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	dockerclient "github.com/moby/moby/client"
)

// Client wraps Docker daemon TCP communication.
type Client struct {
	dockerClient *dockerclient.Client
	host         string // e.g. "tcp://10.0.0.2:2375"
}

var _ ContainerManager = (*Client)(nil)

// NewClient creates a Docker TCP client for the given host (e.g. "tcp://10.0.0.2:2375").
func NewClient(host string) (*Client, error) {
	cli, err := dockerclient.New(
		dockerclient.WithHost(host),
	)
	if err != nil {
		return nil, fmt.Errorf("dockerhost: new client for %q: %w", host, err)
	}
	return &Client{dockerClient: cli, host: host}, nil
}

// RunAgentContainer starts a new cc-remote-agent container on the remote Docker daemon.
// image is the container image to run, name is the container name,
// hostPort is the port to expose on the VM host, containerPort is the port inside the container.
func (c *Client) RunAgentContainer(ctx context.Context, image, name string, hostPort, containerPort int) error {
	portProto, err := network.ParsePort(fmt.Sprintf("%d/tcp", containerPort))
	if err != nil {
		return fmt.Errorf("dockerhost: parse container port: %w", err)
	}

	resp, err := c.dockerClient.ContainerCreate(ctx, dockerclient.ContainerCreateOptions{
		Config: &container.Config{
			Image: image,
			ExposedPorts: network.PortSet{
				portProto: struct{}{},
			},
		},
		HostConfig: &container.HostConfig{
			PortBindings: network.PortMap{
				portProto: []network.PortBinding{
					{HostPort: strconv.Itoa(hostPort)},
				},
			},
			NetworkMode: "bridge",
			Resources: container.Resources{
				Memory:   512 * 1024 * 1024, // 512 MiB
				NanoCPUs: 500_000_000,        // 0.5 CPU
			},
		},
		Name: name,
	})
	if err != nil {
		return fmt.Errorf("dockerhost: create container %q: %w", name, err)
	}

	if _, err := c.dockerClient.ContainerStart(ctx, resp.ID, dockerclient.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("dockerhost: start container %q (id=%s): %w", name, resp.ID, err)
	}
	return nil
}

// StopContainer stops a running container gracefully (10s timeout).
func (c *Client) StopContainer(ctx context.Context, name string) error {
	timeout := 10
	if _, err := c.dockerClient.ContainerStop(ctx, name, dockerclient.ContainerStopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("dockerhost: stop container %q: %w", name, err)
	}
	return nil
}

// RemoveContainer removes a container forcefully.
func (c *Client) RemoveContainer(ctx context.Context, name string) error {
	if _, err := c.dockerClient.ContainerRemove(ctx, name, dockerclient.ContainerRemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("dockerhost: remove container %q: %w", name, err)
	}
	return nil
}

// IsReady checks if the Docker daemon is reachable via ping.
func (c *Client) IsReady(ctx context.Context) bool {
	_, err := c.dockerClient.Ping(ctx, dockerclient.PingOptions{})
	return err == nil
}
