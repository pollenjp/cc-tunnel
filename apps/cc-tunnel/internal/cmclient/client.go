package cmclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client talks to the container-manager HTTP API on a single GCE VM.
type Client struct {
	baseURL string
	http    *http.Client
}

var _ ContainerManager = (*Client)(nil)

// NewClient constructs a Client for the given base URL,
// e.g. "http://10.0.0.2:9090".
func NewClient(baseURL string) (*Client, error) {
	if _, err := url.Parse(baseURL); err != nil {
		return nil, fmt.Errorf("cmclient: parse base URL %q: %w", baseURL, err)
	}
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 5 * time.Minute},
	}, nil
}

type createAgentRequest struct {
	Image         string `json:"image"`
	Name          string `json:"name"`
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
}

// RunAgentContainer issues POST /v1/agents on container-manager.
// container-manager pulls the image (using its VM service-account credentials)
// and starts the container; no registry credentials cross this boundary.
func (c *Client) RunAgentContainer(ctx context.Context, image, name string, hostPort, containerPort int) error {
	body, err := json.Marshal(createAgentRequest{
		Image: image, Name: name, HostPort: hostPort, ContainerPort: containerPort,
	})
	if err != nil {
		return fmt.Errorf("cmclient: marshal create request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/agents", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("cmclient: build create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("cmclient: create container %q: %w", name, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("cmclient: create container %q: %s", name, statusError(resp))
	}
	return nil
}

// StopContainer issues POST /v1/agents/{name}/stop.
func (c *Client) StopContainer(ctx context.Context, name string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/agents/"+url.PathEscape(name)+"/stop", nil)
	if err != nil {
		return fmt.Errorf("cmclient: build stop request: %w", err)
	}
	resp, err := c.http.Do(req)
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
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/v1/agents/"+url.PathEscape(name), nil)
	if err != nil {
		return fmt.Errorf("cmclient: build remove request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("cmclient: remove container %q: %w", name, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("cmclient: remove container %q: %s", name, statusError(resp))
	}
	return nil
}

// IsReady issues GET /healthz with the parent context's deadline.
func (c *Client) IsReady(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/healthz", nil)
	if err != nil {
		return false
	}
	resp, err := c.http.Do(req)
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
