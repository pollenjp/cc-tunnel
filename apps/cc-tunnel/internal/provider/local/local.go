package local

import (
	"context"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

// Provider delegates execution to a cc-remote-agent via remoteclient.Client.
// This is the default provider that preserves existing behavior.
type Provider struct {
	client *remoteclient.Client
}

func New(client *remoteclient.Client) *Provider {
	return &Provider{client: client}
}

func (p *Provider) Execute(ctx context.Context, req remoteclient.Request, onEvent func(remoteclient.StreamEvent)) (string, error) {
	return p.client.Execute(ctx, req, onEvent)
}
