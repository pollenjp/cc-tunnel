package local

import (
	"context"
	"fmt"
	"io"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

// sessionProvider abstracts SessionManager operations for testability.
type sessionProvider interface {
	GetOrCreate(ctx context.Context, convID string, credentials []byte) (*remoteclient.Client, error)
	StopAll(ctx context.Context) error
	CleanupOrphans(ctx context.Context) error
}

// LocalDockerProvider implements ExecutionProvider using per-session Docker containers.
type LocalDockerProvider struct {
	sessions sessionProvider
}

var _ io.Closer = (*LocalDockerProvider)(nil) // io.Closer 実装確認

// NewLocalDockerProvider creates a LocalDockerProvider.
func NewLocalDockerProvider(sessions sessionProvider) *LocalDockerProvider {
	return &LocalDockerProvider{sessions: sessions}
}

func (p *LocalDockerProvider) Execute(ctx context.Context, req remoteclient.Request, onEvent func(remoteclient.StreamEvent)) (string, error) {
	client, err := p.sessions.GetOrCreate(ctx, req.ConversationID, req.Credentials)
	if err != nil {
		return "", fmt.Errorf("get session: %w", err)
	}
	return client.Execute(ctx, req, onEvent)
}

func (p *LocalDockerProvider) Close() error {
	return p.sessions.StopAll(context.Background())
}

// CleanupOrphans removes stopped cctunnel-session-* containers.
// Implements the orphanCleaner interface used by main().
func (p *LocalDockerProvider) CleanupOrphans(ctx context.Context) error {
	return p.sessions.CleanupOrphans(ctx)
}
