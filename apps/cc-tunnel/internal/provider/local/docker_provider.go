package local

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

// sessionProvider abstracts SessionManager operations for testability.
type sessionProvider interface {
	GetOrCreate(ctx context.Context, convID string, credentials []byte) (*remoteclient.Client, error)
	GetClient(convID string) (*remoteclient.Client, bool)
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

// PrepareForRelogin starts or reuses a session container for the given conversation
// without injecting credentials (credentials=nil), so the frontend can initiate a
// PTY-based re-login flow against that container.
func (p *LocalDockerProvider) PrepareForRelogin(ctx context.Context, conversationID string) error {
	_, err := p.sessions.GetOrCreate(ctx, conversationID, nil)
	return err
}

// PullCredentialsFromSession fetches the credentials.json written by the PTY
// login flow from the session container.
func (p *LocalDockerProvider) PullCredentialsFromSession(ctx context.Context, conversationID string) (string, error) {
	client, ok := p.sessions.GetClient(conversationID)
	if !ok {
		return "", errors.New("no session container found for conversation")
	}
	return client.FinalizeCredentials(ctx)
}

func (p *LocalDockerProvider) Close() error {
	return p.sessions.StopAll(context.Background())
}

// CleanupOrphans removes stopped cctunnel-session-* containers.
// Implements the orphanCleaner interface used by main().
func (p *LocalDockerProvider) CleanupOrphans(ctx context.Context) error {
	return p.sessions.CleanupOrphans(ctx)
}
