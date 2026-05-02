package api

import (
	"context"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/db"
)

// credentialService abstracts credential fetching for testability.
type credentialService interface {
	FetchAndDecrypt(ctx context.Context, username string) ([]byte, error)
	// Deprecated: MarkInvalid is currently not invoked by any handler. It was
	// added as part of the credential-validation roadmap but the SendMessage
	// gate now redirects to /login/credentials instead of flipping the DB
	// flag. Keep the method on the interface until an ADR decides whether
	// to wire it up (proactive invalidation) or remove it.
	MarkInvalid(ctx context.Context, username string) error
}

// credentialStorer abstracts credential encryption and storage for testability.
type credentialStorer interface {
	StoreCredential(ctx context.Context, username, credJSON string) error
}

type repository interface {
	conversationRepository
	messageRepository
	sessionEndpointRepository
}

// conversationRepository handles conversation-level CRUD and metadata updates.
type conversationRepository interface {
	CreateConversation(ctx context.Context, title, model string, systemPrompt *string) (*db.Conversation, error)
	GetConversation(ctx context.Context, id string) (*db.Conversation, error)
	ListConversations(ctx context.Context) ([]*db.Conversation, error)
	DeleteConversation(ctx context.Context, id string) error
	UpdateConversationUpdatedAt(ctx context.Context, id string) error
	UpdateConversationTitle(ctx context.Context, id string, title string) error
	UpdateConversationStatus(ctx context.Context, id, status string) error
}

// messageRepository handles message persistence including the streaming
// lifecycle (create / batch-update content blocks / status / merge metadata).
type messageRepository interface {
	CreateMessage(ctx context.Context, conversationID, role string, messageData map[string]interface{}) (*db.Message, error)
	ListMessages(ctx context.Context, conversationID string) ([]*db.Message, error)
	CreateStreamingMessage(ctx context.Context, conversationID, role string, messageData map[string]interface{}) (*db.Message, error)
	UpdateMessageContentBlocks(ctx context.Context, messageID string, contentBlocks []map[string]interface{}) error
	UpdateMessageStatus(ctx context.Context, messageID, status string) error
	MergeMessageData(ctx context.Context, messageID string, extra map[string]interface{}) error
}

// sessionEndpointRepository tracks per-session endpoint activity for the
// idle-cleanup loop in the docker_gce / local providers.
type sessionEndpointRepository interface {
	UpdateSessionEndpointLastActivity(ctx context.Context, conversationID string) error
}
