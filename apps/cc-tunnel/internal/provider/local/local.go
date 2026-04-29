package local

import (
	"context"
	"errors"

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

func (p *Provider) PrepareForRelogin(_ context.Context, _ string) error {
	return errors.New("relogin not supported by single-client local provider")
}

func (p *Provider) PullCredentialsFromSession(_ context.Context, _ string) (string, error) {
	return "", errors.New("relogin not supported by single-client local provider")
}
