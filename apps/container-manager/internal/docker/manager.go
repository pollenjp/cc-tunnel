// Package docker performs container lifecycle operations on the local Docker
// daemon (Unix socket), and pulls images from Artifact Registry using the
// VM's GCE service-account credentials obtained from the metadata server.
package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"cloud.google.com/go/compute/metadata"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	dockerclient "github.com/moby/moby/client"
)

// TokenSource returns an OAuth2 access token for Artifact Registry.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// MetadataTokenSource fetches an access token from the GCE metadata server
// (i.e. the VM's default service account).
type MetadataTokenSource struct{}

// Token implements TokenSource.
func (MetadataTokenSource) Token(ctx context.Context) (string, error) {
	tok, err := metadata.GetWithContext(ctx, "instance/service-accounts/default/token")
	if err != nil {
		return "", fmt.Errorf("metadata token: %w", err)
	}
	var resp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal([]byte(tok), &resp); err != nil {
		return "", fmt.Errorf("decode metadata token: %w", err)
	}
	if resp.AccessToken == "" {
		return "", fmt.Errorf("metadata token: empty access_token")
	}
	return resp.AccessToken, nil
}

// Manager wraps the Docker SDK with image-pull authentication and the small
// set of container lifecycle operations container-manager exposes over HTTP.
type Manager struct {
	cli            *dockerclient.Client
	token          TokenSource
	defaultNetwork string
}

// NewManager constructs a Manager that talks to the local Docker daemon
// (defaults to /var/run/docker.sock unless DOCKER_HOST is set) and uses
// MetadataTokenSource for registry auth. defaultNetwork is the Docker network
// new containers join when the request does not specify one (use "" for
// "bridge").
func NewManager(defaultNetwork string) (*Manager, error) {
	cli, err := dockerclient.New(dockerclient.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	if defaultNetwork == "" {
		defaultNetwork = "bridge"
	}
	return &Manager{cli: cli, token: MetadataTokenSource{}, defaultNetwork: defaultNetwork}, nil
}

// NewManagerWithDeps is for tests.
func NewManagerWithDeps(cli *dockerclient.Client, token TokenSource, defaultNetwork string) *Manager {
	if defaultNetwork == "" {
		defaultNetwork = "bridge"
	}
	return &Manager{cli: cli, token: token, defaultNetwork: defaultNetwork}
}

// Ping checks the local Docker daemon is reachable.
func (m *Manager) Ping(ctx context.Context) error {
	_, err := m.cli.Ping(ctx, dockerclient.PingOptions{})
	return err
}

// RunAgentRequest specifies how to start a cc-remote-agent container.
type RunAgentRequest struct {
	Image         string
	Name          string
	HostPort      int      // 0 = no host port mapping (only the container is exposed via the docker network)
	ContainerPort int
	MemoryBytes   int64
	NanoCPUs      int64
	Network       string   // overrides the manager's default network when non-empty
	Env           []string // additional environment variables (e.g. "PORT=9090")
}

// RunAgent pulls the image (with VM-SA-derived auth) only when it is not
// already present locally, then starts a new container. Skipping the pull on
// a cache hit means that in local docker-compose development the
// pre-built cc-remote-agent image is used directly without a (failing)
// registry lookup.
func (m *Manager) RunAgent(ctx context.Context, req RunAgentRequest) (string, error) {
	if err := m.ensureImage(ctx, req.Image); err != nil {
		return "", fmt.Errorf("ensure image %q: %w", req.Image, err)
	}

	portProto, err := network.ParsePort(fmt.Sprintf("%d/tcp", req.ContainerPort))
	if err != nil {
		return "", fmt.Errorf("parse container port: %w", err)
	}

	memory := req.MemoryBytes
	if memory == 0 {
		memory = 512 * 1024 * 1024
	}
	nanoCPU := req.NanoCPUs
	if nanoCPU == 0 {
		nanoCPU = 500_000_000
	}

	netMode := req.Network
	if netMode == "" {
		netMode = m.defaultNetwork
	}

	hostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(netMode),
		Resources: container.Resources{
			Memory:   memory,
			NanoCPUs: nanoCPU,
		},
	}
	if req.HostPort > 0 {
		hostConfig.PortBindings = network.PortMap{
			portProto: []network.PortBinding{
				{HostPort: strconv.Itoa(req.HostPort)},
			},
		}
	}

	resp, err := m.cli.ContainerCreate(ctx, dockerclient.ContainerCreateOptions{
		Config: &container.Config{
			Image: req.Image,
			Env:   req.Env,
			ExposedPorts: network.PortSet{
				portProto: struct{}{},
			},
		},
		HostConfig: hostConfig,
		Name:       req.Name,
	})
	if err != nil {
		return "", fmt.Errorf("create container %q: %w", req.Name, err)
	}

	if _, err := m.cli.ContainerStart(ctx, resp.ID, dockerclient.ContainerStartOptions{}); err != nil {
		return "", fmt.Errorf("start container %q (id=%s): %w", req.Name, resp.ID, err)
	}
	return resp.ID, nil
}

// StopAgent stops a container with a 10-second graceful timeout.
func (m *Manager) StopAgent(ctx context.Context, name string) error {
	timeout := 10
	if _, err := m.cli.ContainerStop(ctx, name, dockerclient.ContainerStopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("stop container %q: %w", name, err)
	}
	return nil
}

// RemoveAgent force-removes a container.
func (m *Manager) RemoveAgent(ctx context.Context, name string) error {
	if _, err := m.cli.ContainerRemove(ctx, name, dockerclient.ContainerRemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("remove container %q: %w", name, err)
	}
	return nil
}

// ensureImage skips the pull when the image already exists on the local
// daemon. This is critical for local development (where cc-remote-agent is
// built locally and has no upstream registry to pull from) and is a useful
// optimisation in production (subsequent sessions on the same VM reuse the
// cached image).
func (m *Manager) ensureImage(ctx context.Context, image string) error {
	if _, err := m.cli.ImageInspect(ctx, image); err == nil {
		return nil
	}
	return m.pullImage(ctx, image)
}

// pullImage executes ImagePull with X-Registry-Auth derived from the VM's
// metadata token. The token is only attached for hostnames that look like
// Google-hosted registries; for others (e.g. mirror.gcr.io public images,
// localhost) the pull is unauthenticated.
func (m *Manager) pullImage(ctx context.Context, image string) error {
	opts := dockerclient.ImagePullOptions{}
	if needsGoogleAuth(image) {
		auth, err := m.googleRegistryAuth(ctx, registryHost(image))
		if err != nil {
			return fmt.Errorf("registry auth: %w", err)
		}
		opts.RegistryAuth = auth
	}
	body, err := m.cli.ImagePull(ctx, image, opts)
	if err != nil {
		return err
	}
	defer func() { _ = body.Close() }()
	// Drain the pull progress so the operation completes before we return.
	if _, err := io.Copy(io.Discard, body); err != nil {
		return fmt.Errorf("drain pull stream: %w", err)
	}
	return nil
}

func (m *Manager) googleRegistryAuth(ctx context.Context, host string) (string, error) {
	tok, err := m.token.Token(ctx)
	if err != nil {
		return "", err
	}
	cfg := struct {
		Username      string `json:"username"`
		Password      string `json:"password"`
		ServerAddress string `json:"serveraddress"`
	}{
		Username:      "oauth2accesstoken",
		Password:      tok,
		ServerAddress: host,
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func registryHost(image string) string {
	if i := strings.IndexByte(image, '/'); i > 0 {
		head := image[:i]
		if strings.ContainsAny(head, ".:") || head == "localhost" {
			return head
		}
	}
	return "docker.io"
}

// needsGoogleAuth reports whether the image is hosted on Artifact Registry
// (the only Google-hosted registry we currently push to). mirror.gcr.io and
// docker.io are explicitly *not* matched: the former is a public read-through
// cache that rejects bearer tokens, the latter has no Google credentials.
func needsGoogleAuth(image string) bool {
	host := registryHost(image)
	return strings.HasSuffix(host, ".pkg.dev")
}
