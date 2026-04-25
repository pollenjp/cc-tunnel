package dockergce

import (
	"context"

	"github.com/google/uuid"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

// MockProvider is a stub Docker on GCE execution provider.
// It returns a fixed response without actually spinning up any container.
type MockProvider struct{}

func New() *MockProvider {
	return &MockProvider{}
}

func (p *MockProvider) Execute(_ context.Context, _ remoteclient.Request, onEvent func(remoteclient.StreamEvent)) (string, error) {
	onEvent(remoteclient.StreamEvent{
		Type: "assistant",
		Message: &struct {
			Content []remoteclient.ContentBlock `json:"content"`
		}{
			Content: []remoteclient.ContentBlock{
				{Type: "text", Text: "This is a mock response from docker_gce provider"},
			},
		},
	})
	onEvent(remoteclient.StreamEvent{
		Type:   "result",
		Result: "success",
	})
	return "mock-session-" + uuid.New().String(), nil
}
