package cloudrunsandbox

import (
	"context"

	"github.com/google/uuid"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

// MockProvider is a stub Cloud Run Sandbox execution provider.
// It returns a fixed response without actually executing any remote process.
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
				{Type: "text", Text: "This is a mock response from cloud_run_sandbox provider"},
			},
		},
	})
	onEvent(remoteclient.StreamEvent{
		Type:   "result",
		Result: "success",
	})
	return "mock-session-" + uuid.New().String(), nil
}
