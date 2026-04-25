package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
)

// SDKRunner implements DockerRunner using the Docker SDK.
type SDKRunner struct {
	cli *dockerclient.Client
}

var _ DockerRunner = (*SDKRunner)(nil) // コンパイル時インターフェース確認

// NewSDKRunner creates a new SDKRunner connected to the local Docker daemon.
func NewSDKRunner() (*SDKRunner, error) {
	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	return &SDKRunner{cli: cli}, nil
}

// ContainerCreate creates a container and returns its ID.
func (r *SDKRunner) ContainerCreate(ctx context.Context, opts ContainerCreateOpts) (string, error) {
	mounts := make([]mount.Mount, 0, len(opts.VolumeMounts))
	for _, vm := range opts.VolumeMounts {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeVolume,
			Source: vm.Source,
			Target: vm.Target,
		})
	}

	var netConfig *network.NetworkingConfig
	if opts.Network != "" {
		netConfig = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				opts.Network: {},
			},
		}
	}

	resp, err := r.cli.ContainerCreate(
		ctx,
		&container.Config{
			Image: opts.Image,
			Env:   opts.Env,
		},
		&container.HostConfig{
			Mounts: mounts,
		},
		netConfig,
		nil,
		opts.Name,
	)
	if err != nil {
		return "", fmt.Errorf("container create: %w", err)
	}
	return resp.ID, nil
}

// ContainerStart starts a previously created container.
func (r *SDKRunner) ContainerStart(ctx context.Context, containerID string) error {
	if err := r.cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("container start: %w", err)
	}
	return nil
}

// ContainerStop stops a running container (10s timeout).
func (r *SDKRunner) ContainerStop(ctx context.Context, containerID string) error {
	timeout := 10
	if err := r.cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("container stop: %w", err)
	}
	return nil
}

// ContainerRemove removes a container forcefully.
func (r *SDKRunner) ContainerRemove(ctx context.Context, containerID string) error {
	if err := r.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("container remove: %w", err)
	}
	return nil
}

// ContainerInspect returns the current state of a container.
func (r *SDKRunner) ContainerInspect(ctx context.Context, containerID string) (*ContainerInfo, error) {
	resp, err := r.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("container inspect: %w", err)
	}

	info := &ContainerInfo{
		ID:   resp.ID,
		Name: strings.TrimPrefix(resp.Name, "/"),
	}
	if resp.State != nil {
		info.State = resp.State.Status
	}
	return info, nil
}

// ContainerList lists containers whose name contains namePrefix.
// If all is true, includes stopped containers.
func (r *SDKRunner) ContainerList(ctx context.Context, namePrefix string, all bool) ([]ContainerSummary, error) {
	containers, err := r.cli.ContainerList(ctx, container.ListOptions{
		All:     all,
		Filters: filters.NewArgs(filters.Arg("name", namePrefix)),
	})
	if err != nil {
		return nil, fmt.Errorf("container list: %w", err)
	}

	summaries := make([]ContainerSummary, 0, len(containers))
	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		summaries = append(summaries, ContainerSummary{
			ID:    c.ID,
			Name:  name,
			State: c.State,
		})
	}
	return summaries, nil
}
