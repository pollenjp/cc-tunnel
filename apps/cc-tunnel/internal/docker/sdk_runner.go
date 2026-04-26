package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	dockerclient "github.com/moby/moby/client"
)

// SDKRunner implements DockerRunner using the Docker SDK.
type SDKRunner struct {
	cli *dockerclient.Client
}

var _ DockerRunner = (*SDKRunner)(nil) // コンパイル時インターフェース確認

// NewSDKRunner creates a new SDKRunner connected to the local Docker daemon.
func NewSDKRunner() (*SDKRunner, error) {
	cli, err := dockerclient.New(
		dockerclient.FromEnv,
	)
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	return &SDKRunner{cli: cli}, nil
}

// NewSDKRunnerWithTCP creates a new SDKRunner connected to a remote Docker daemon via TCP.
func NewSDKRunnerWithTCP(tcpEndpoint string) (*SDKRunner, error) {
	cli, err := dockerclient.New(
		dockerclient.WithHost("tcp://" + tcpEndpoint),
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

	resp, err := r.cli.ContainerCreate(ctx, dockerclient.ContainerCreateOptions{
		Config: &container.Config{
			Image: opts.Image,
			Env:   opts.Env,
		},
		HostConfig: &container.HostConfig{
			Mounts: mounts,
		},
		NetworkingConfig: netConfig,
		Name:             opts.Name,
	})
	if err != nil {
		return "", fmt.Errorf("container create: %w", err)
	}
	return resp.ID, nil
}

// ContainerStart starts a previously created container.
func (r *SDKRunner) ContainerStart(ctx context.Context, containerID string) error {
	if _, err := r.cli.ContainerStart(ctx, containerID, dockerclient.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("container start: %w", err)
	}
	return nil
}

// ContainerStop stops a running container (10s timeout).
func (r *SDKRunner) ContainerStop(ctx context.Context, containerID string) error {
	timeout := 10
	if _, err := r.cli.ContainerStop(ctx, containerID, dockerclient.ContainerStopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("container stop: %w", err)
	}
	return nil
}

// ContainerRemove removes a container forcefully.
func (r *SDKRunner) ContainerRemove(ctx context.Context, containerID string) error {
	if _, err := r.cli.ContainerRemove(ctx, containerID, dockerclient.ContainerRemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("container remove: %w", err)
	}
	return nil
}

// ContainerInspect returns the current state of a container.
func (r *SDKRunner) ContainerInspect(ctx context.Context, containerID string) (*ContainerInfo, error) {
	resp, err := r.cli.ContainerInspect(ctx, containerID, dockerclient.ContainerInspectOptions{})
	if err != nil {
		return nil, fmt.Errorf("container inspect: %w", err)
	}

	info := &ContainerInfo{
		ID:   resp.Container.ID,
		Name: strings.TrimPrefix(resp.Container.Name, "/"),
	}
	if resp.Container.State != nil {
		info.State = string(resp.Container.State.Status)
	}
	return info, nil
}

// ContainerList lists containers whose name contains namePrefix.
// If all is true, includes stopped containers.
func (r *SDKRunner) ContainerList(ctx context.Context, namePrefix string, all bool) ([]ContainerSummary, error) {
	result, err := r.cli.ContainerList(ctx, dockerclient.ContainerListOptions{
		All:     all,
		Filters: make(dockerclient.Filters).Add("name", namePrefix),
	})
	if err != nil {
		return nil, fmt.Errorf("container list: %w", err)
	}

	summaries := make([]ContainerSummary, 0, len(result.Items))
	for _, c := range result.Items {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		summaries = append(summaries, ContainerSummary{
			ID:    c.ID,
			Name:  name,
			State: string(c.State),
		})
	}
	return summaries, nil
}
