package docker

import "context"

// ContainerCreateOpts はコンテナ作成のオプション。
type ContainerCreateOpts struct {
	Name         string
	Image        string
	Env          []string
	Network      string
	VolumeMounts []VolumeMount
}

// VolumeMount はコンテナへのボリュームマウント設定。
type VolumeMount struct {
	Source string // volume name or host path
	Target string // container path
}

// ContainerInfo はコンテナの状態情報。
type ContainerInfo struct {
	ID    string
	Name  string
	State string // "running", "exited", "dead", etc.
}

// ContainerSummary はコンテナ一覧用の軽量情報。
type ContainerSummary struct {
	ID    string
	Name  string
	State string
}

// DockerRunner abstracts Docker daemon operations for testability.
type DockerRunner interface {
	// ContainerCreate creates a container and returns its ID.
	ContainerCreate(ctx context.Context, opts ContainerCreateOpts) (string, error)
	// ContainerStart starts a previously created container.
	ContainerStart(ctx context.Context, containerID string) error
	// ContainerStop stops a running container (10s timeout).
	ContainerStop(ctx context.Context, containerID string) error
	// ContainerRemove removes a container forcefully.
	ContainerRemove(ctx context.Context, containerID string) error
	// ContainerInspect returns the current state of a container.
	ContainerInspect(ctx context.Context, containerID string) (*ContainerInfo, error)
	// ContainerList lists containers whose name contains namePrefix.
	// If all is true, includes stopped containers.
	ContainerList(ctx context.Context, namePrefix string, all bool) ([]ContainerSummary, error)
}
