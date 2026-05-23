package cmclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/cmclient/genclient"
)

// Client adapts the oapi-codegen-generated genclient.ClientInterface to the
// higher-level ContainerManager interface that cc-tunnel callers consume.
type Client struct {
	gen genclient.ClientInterface
}

var _ ContainerManager = (*Client)(nil)

// NewClient constructs a Client that talks to the container-manager HTTP API
// at baseURL (e.g. "http://10.0.0.2:9090").
func NewClient(baseURL string) (*Client, error) {
	if _, err := url.Parse(baseURL); err != nil {
		return nil, fmt.Errorf("cmclient: parse base URL %q: %w", baseURL, err)
	}
	gen, err := genclient.NewClient(baseURL, genclient.WithHTTPClient(&http.Client{Timeout: 5 * time.Minute}))
	if err != nil {
		return nil, fmt.Errorf("cmclient: build generated client: %w", err)
	}
	return &Client{gen: gen}, nil
}

// RunAgent issues POST /v1/agents on container-manager. container-manager
// pulls the image (using its own credentials) and starts the container; no
// registry credentials cross this boundary.
func (c *Client) RunAgent(ctx context.Context, req RunAgentRequest) error {
	body := genclient.CreateAgentJSONRequestBody{
		Image:         req.Image,
		Name:          req.Name,
		ContainerPort: int32(req.ContainerPort),
	}
	if req.HostPort != 0 {
		hp := int32(req.HostPort)
		body.HostPort = &hp
	}
	if req.Network != "" {
		n := req.Network
		body.Network = &n
	}
	if len(req.Env) > 0 {
		env := append([]string(nil), req.Env...)
		body.Env = &env
	}
	if len(req.Labels) > 0 {
		labels := make(map[string]string, len(req.Labels))
		for k, v := range req.Labels {
			labels[k] = v
		}
		body.Labels = &labels
	}

	resp, err := c.gen.CreateAgent(ctx, body)
	if err != nil {
		return fmt.Errorf("cmclient: create container %q: %w", req.Name, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("cmclient: create container %q: %s", req.Name, statusError(resp))
	}
	return nil
}

// RunAgentContainer is the production GCE path's convenience wrapper.
//
// cc-remote-agent reads the `PORT` env var and falls back to 9090 when unset
// (apps/cc-remote-agent/cmd/cc-remote-agent/main.go), but container-manager
// publishes the host-side port to `containerPort` inside the container. To
// keep the listener and the published port in sync we inject PORT here; the
// agent then binds to the same port container-manager exposes.
func (c *Client) RunAgentContainer(ctx context.Context, image, name string, hostPort, containerPort int) error {
	return c.RunAgent(ctx, RunAgentRequest{
		Image:         image,
		Name:          name,
		HostPort:      hostPort,
		ContainerPort: containerPort,
		Env:           []string{fmt.Sprintf("PORT=%d", containerPort)},
	})
}

// StopContainer issues POST /v1/agents/{name}/stop.
func (c *Client) StopContainer(ctx context.Context, name string) error {
	resp, err := c.gen.StopAgent(ctx, name)
	if err != nil {
		return fmt.Errorf("cmclient: stop container %q: %w", name, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("cmclient: stop container %q: %s", name, statusError(resp))
	}
	return nil
}

// RemoveContainer issues DELETE /v1/agents/{name}.
func (c *Client) RemoveContainer(ctx context.Context, name string) error {
	resp, err := c.gen.RemoveAgent(ctx, name)
	if err != nil {
		return fmt.Errorf("cmclient: remove container %q: %w", name, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("cmclient: remove container %q: %s", name, statusError(resp))
	}
	return nil
}

// ListAgents issues GET /v1/agents.
func (c *Client) ListAgents(ctx context.Context) ([]AgentInfo, error) {
	resp, err := c.gen.ListAgents(ctx)
	if err != nil {
		return nil, fmt.Errorf("cmclient: list agents: %w", err)
	}
	// ParseListAgentsResponse drains and closes the body.
	parsed, err := genclient.ParseListAgentsResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("cmclient: parse list agents response: %w", err)
	}
	if parsed.StatusCode()/100 != 2 || parsed.JSON200 == nil {
		return nil, fmt.Errorf("cmclient: list agents: %s", parsed.Status())
	}
	out := make([]AgentInfo, 0, len(parsed.JSON200.Agents))
	for _, a := range parsed.JSON200.Agents {
		out = append(out, AgentInfo{Name: a.Name})
	}
	return out, nil
}

// IsReady issues GET /healthz with the parent context's deadline.
func (c *Client) IsReady(ctx context.Context) bool {
	resp, err := c.gen.GetHealthz(ctx)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode == http.StatusOK
}

func statusError(resp *http.Response) string {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return fmt.Sprintf("%s: %s", resp.Status, bytes.TrimSpace(body))
}
