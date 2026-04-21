package api

import (
	"github.com/google/uuid"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/db"
)

// newConversation converts a DB Conversation to an API Conversation.
// All fields are explicitly set to prevent zero-value omissions.
func newConversation(c *db.Conversation) Conversation {
	id, _ := uuid.Parse(c.ID)
	return Conversation{
		Id:           id,
		Title:        c.Title,
		Model:        c.Model,
		Status:       ConversationStatus(c.Status),
		CreatedAt:    c.CreatedAt,
		UpdatedAt:    c.UpdatedAt,
		SystemPrompt: c.SystemPrompt,
	}
}

// newMessage converts a DB Message to an API Message.
// All fields are explicitly set to prevent zero-value omissions.
func newMessage(m *db.Message) Message {
	msgID, _ := uuid.Parse(m.ID)
	convID, _ := uuid.Parse(m.ConversationID)
	var status *MessageStatus
	if m.Status != "" {
		s := MessageStatus(m.Status)
		status = &s
	}
	var msgData *map[string]interface{}
	if len(m.MessageData) > 0 {
		msgData = &m.MessageData
	}
	return Message{
		Id:             msgID,
		ConversationId: convID,
		Role:           MessageRole(m.Role),
		CreatedAt:      m.CreatedAt,
		Status:         status,
		MessageData:    msgData,
		UpdatedAt:      nil,
	}
}

// newConversationDetail converts a DB Conversation with its messages to a ConversationDetail.
// All fields are explicitly set to prevent zero-value omissions.
func newConversationDetail(conv *db.Conversation, msgs []*db.Message) ConversationDetail {
	convUUID, _ := uuid.Parse(conv.ID)
	messages := make([]Message, 0, len(msgs))
	for _, m := range msgs {
		messages = append(messages, newMessage(m))
	}
	return ConversationDetail{
		Id:           convUUID,
		Title:        conv.Title,
		Model:        conv.Model,
		Status:       ConversationDetailStatus(conv.Status),
		CreatedAt:    conv.CreatedAt,
		UpdatedAt:    conv.UpdatedAt,
		SystemPrompt: conv.SystemPrompt,
		Messages:     messages,
	}
}

// dbConvToAPI is kept as an alias for backward compatibility with existing tests.
// New code should use newConversation directly.
func dbConvToAPI(c *db.Conversation) Conversation {
	return newConversation(c)
}

// dbMsgToAPI is kept as an alias for backward compatibility with existing tests.
// New code should use newMessage directly.
func dbMsgToAPI(m *db.Message) Message {
	return newMessage(m)
}
