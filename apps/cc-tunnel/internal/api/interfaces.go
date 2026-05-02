package api

import (
	"context"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/db"
)

// credentialService abstracts credential fetching for testability.
type credentialService interface {
	FetchAndDecrypt(ctx context.Context, username string) ([]byte, error)
	MarkInvalid(ctx context.Context, username string) error
}

// credentialStorer abstracts credential encryption and storage for testability.
type credentialStorer interface {
	StoreCredential(ctx context.Context, username, credJSON string) error
}

type repository interface {
	CreateConversation(ctx context.Context, title, model string, systemPrompt *string) (*db.Conversation, error)
	GetConversation(ctx context.Context, id string) (*db.Conversation, error)
	ListConversations(ctx context.Context) ([]*db.Conversation, error)
	DeleteConversation(ctx context.Context, id string) error
	UpdateConversationUpdatedAt(ctx context.Context, id string) error
	UpdateConversationTitle(ctx context.Context, id string, title string) error
	UpdateConversationStatus(ctx context.Context, id, status string) error
	CreateMessage(ctx context.Context, conversationID, role string, messageData map[string]interface{}) (*db.Message, error)
	ListMessages(ctx context.Context, conversationID string) ([]*db.Message, error)
	CreateStreamingMessage(ctx context.Context, conversationID, role string, messageData map[string]interface{}) (*db.Message, error)
	UpdateMessageContentBlocks(ctx context.Context, messageID string, contentBlocks []map[string]interface{}) error
	UpdateMessageStatus(ctx context.Context, messageID, status string) error
	MergeMessageData(ctx context.Context, messageID string, extra map[string]interface{}) error
	UpdateSessionEndpointLastActivity(ctx context.Context, conversationID string) error
}
