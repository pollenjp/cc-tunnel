package api

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/db"
)

// --- dbMsgToAPI mapping ---

func TestDbMsgToAPI_basicFields(t *testing.T) {
	msgID := "11111111-1111-1111-1111-111111111111"
	convID := "22222222-2222-2222-2222-222222222222"
	now := time.Now().UTC().Truncate(time.Millisecond)

	m := &db.Message{
		ID:             msgID,
		ConversationID: convID,
		Role:           "user",
		MessageData:    map[string]interface{}{"content": "hello"},
		CreatedAt:      now,
	}

	msg := dbMsgToAPI(m)

	if msg.Id.String() != msgID {
		t.Errorf("Id = %q, want %q", msg.Id.String(), msgID)
	}
	if msg.ConversationId.String() != convID {
		t.Errorf("ConversationId = %q, want %q", msg.ConversationId.String(), convID)
	}
	if msg.Role != "user" {
		t.Errorf("Role = %q, want %q", msg.Role, "user")
	}
	if msg.MessageData == nil {
		t.Error("MessageData should not be nil when source has data")
	}
}

func TestDbMsgToAPI_emptyMessageData_nilPointer(t *testing.T) {
	m := &db.Message{
		ID:             "11111111-1111-1111-1111-111111111111",
		ConversationID: "22222222-2222-2222-2222-222222222222",
		Role:           "assistant",
		MessageData:    map[string]interface{}{},
		CreatedAt:      time.Now(),
	}

	msg := dbMsgToAPI(m)

	if msg.MessageData != nil {
		t.Errorf("MessageData should be nil when source map is empty, got %v", *msg.MessageData)
	}
}

func TestDbMsgToAPI_invalidUUID_zeroValue(t *testing.T) {
	m := &db.Message{
		ID:             "not-a-uuid",
		ConversationID: "also-not-a-uuid",
		Role:           "user",
		MessageData:    nil,
		CreatedAt:      time.Now(),
	}

	msg := dbMsgToAPI(m)

	// uuid.Parse failure results in zero UUID
	if msg.Id != (uuid.UUID{}) {
		t.Errorf("expected zero UUID for invalid ID, got %s", msg.Id.String())
	}
	if msg.ConversationId != (uuid.UUID{}) {
		t.Errorf("expected zero UUID for invalid ConversationID, got %s", msg.ConversationId.String())
	}
}

func TestDbMsgToAPI_assistantRole(t *testing.T) {
	m := &db.Message{
		ID:             "11111111-1111-1111-1111-111111111111",
		ConversationID: "22222222-2222-2222-2222-222222222222",
		Role:           "assistant",
		MessageData:    map[string]interface{}{"content": "reply"},
		CreatedAt:      time.Now(),
	}

	msg := dbMsgToAPI(m)

	if msg.Role != Assistant {
		t.Errorf("Role = %q, want %q", msg.Role, Assistant)
	}
}

// --- dbConvToAPI mapping ---

func TestDbConvToAPI_basicFields(t *testing.T) {
	convID := "33333333-3333-3333-3333-333333333333"
	now := time.Now().UTC().Truncate(time.Millisecond)

	c := &db.Conversation{
		ID:        convID,
		Title:     "My Conversation",
		Model:     "claude-sonnet-4-6",
		CreatedAt: now,
		UpdatedAt: now,
	}

	conv := dbConvToAPI(c)

	if conv.Id.String() != convID {
		t.Errorf("Id = %q, want %q", conv.Id.String(), convID)
	}
	if conv.Title != "My Conversation" {
		t.Errorf("Title = %q, want %q", conv.Title, "My Conversation")
	}
	if conv.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want %q", conv.Model, "claude-sonnet-4-6")
	}
	if conv.SystemPrompt != nil {
		t.Errorf("SystemPrompt should be nil when not set, got %v", *conv.SystemPrompt)
	}
}

func TestDbConvToAPI_withSystemPrompt(t *testing.T) {
	prompt := "You are a helpful assistant."
	c := &db.Conversation{
		ID:           "33333333-3333-3333-3333-333333333333",
		Title:        "Test",
		Model:        "claude-sonnet-4-6",
		SystemPrompt: &prompt,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	conv := dbConvToAPI(c)

	if conv.SystemPrompt == nil {
		t.Error("SystemPrompt should not be nil when source has system prompt")
	} else if *conv.SystemPrompt != prompt {
		t.Errorf("SystemPrompt = %q, want %q", *conv.SystemPrompt, prompt)
	}
}

func TestDbConvToAPI_invalidUUID_zeroValue(t *testing.T) {
	c := &db.Conversation{
		ID:        "not-valid-uuid",
		Title:     "Test",
		Model:     "claude-sonnet-4-6",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	conv := dbConvToAPI(c)

	if conv.Id != (uuid.UUID{}) {
		t.Errorf("expected zero UUID for invalid ID, got %s", conv.Id.String())
	}
}

// --- newMessage constructor ---

func TestNewMessage_allFields(t *testing.T) {
	msgID := "aaaaaaaa-1111-1111-1111-111111111111"
	convID := "bbbbbbbb-2222-2222-2222-222222222222"
	now := time.Now().UTC().Truncate(time.Millisecond)

	m := &db.Message{
		ID:             msgID,
		ConversationID: convID,
		Role:           "user",
		MessageData:    map[string]interface{}{"content": "hello"},
		Status:         "completed",
		CreatedAt:      now,
	}

	msg := newMessage(m)

	if msg.Id.String() != msgID {
		t.Errorf("Id = %q, want %q", msg.Id.String(), msgID)
	}
	if msg.ConversationId.String() != convID {
		t.Errorf("ConversationId = %q, want %q", msg.ConversationId.String(), convID)
	}
	if msg.Role != User {
		t.Errorf("Role = %q, want %q", msg.Role, User)
	}
	if msg.Status == nil {
		t.Error("Status should not be nil when source has status")
	} else if *msg.Status != MessageStatusCompleted {
		t.Errorf("Status = %q, want %q", *msg.Status, MessageStatusCompleted)
	}
	if msg.MessageData == nil {
		t.Error("MessageData should not be nil when source has data")
	}
	if msg.CreatedAt != now {
		t.Errorf("CreatedAt = %v, want %v", msg.CreatedAt, now)
	}
}

func TestNewMessage_emptyStatus_nilPointer(t *testing.T) {
	m := &db.Message{
		ID:             "aaaaaaaa-1111-1111-1111-111111111111",
		ConversationID: "bbbbbbbb-2222-2222-2222-222222222222",
		Role:           "user",
		MessageData:    map[string]interface{}{},
		Status:         "",
		CreatedAt:      time.Now(),
	}

	msg := newMessage(m)

	if msg.Status != nil {
		t.Errorf("Status should be nil when source status is empty, got %v", *msg.Status)
	}
}

// --- newConversationDetail constructor ---

func TestNewConversationDetail_allFields(t *testing.T) {
	convID := "cccccccc-3333-3333-3333-333333333333"
	msgID := "dddddddd-4444-4444-4444-444444444444"
	now := time.Now().UTC().Truncate(time.Millisecond)

	conv := &db.Conversation{
		ID:        convID,
		Title:     "Detail Test",
		Model:     "claude-sonnet-4-6",
		Status:    "completed",
		CreatedAt: now,
		UpdatedAt: now,
	}
	msgs := []*db.Message{
		{
			ID:             msgID,
			ConversationID: convID,
			Role:           "user",
			MessageData:    map[string]interface{}{"content": "hi"},
			Status:         "completed",
			CreatedAt:      now,
		},
	}

	detail := newConversationDetail(conv, msgs)

	if detail.Id.String() != convID {
		t.Errorf("Id = %q, want %q", detail.Id.String(), convID)
	}
	if detail.Status != ConversationDetailStatusCompleted {
		t.Errorf("Status = %q, want %q", detail.Status, ConversationDetailStatusCompleted)
	}
	if detail.Title != "Detail Test" {
		t.Errorf("Title = %q, want %q", detail.Title, "Detail Test")
	}
	if detail.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want %q", detail.Model, "claude-sonnet-4-6")
	}
	if len(detail.Messages) != 1 {
		t.Fatalf("Messages len = %d, want 1", len(detail.Messages))
	}
	if detail.Messages[0].Id.String() != msgID {
		t.Errorf("Messages[0].Id = %q, want %q", detail.Messages[0].Id.String(), msgID)
	}
	if detail.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestNewConversationDetail_emptyMessages(t *testing.T) {
	convID := "eeeeeeee-5555-5555-5555-555555555555"
	now := time.Now().UTC()

	conv := &db.Conversation{
		ID:        convID,
		Title:     "Empty",
		Model:     "claude-sonnet-4-6",
		Status:    "idle",
		CreatedAt: now,
		UpdatedAt: now,
	}

	detail := newConversationDetail(conv, nil)

	if detail.Messages == nil {
		t.Error("Messages should not be nil (should be empty slice)")
	}
	if len(detail.Messages) != 0 {
		t.Errorf("Messages len = %d, want 0", len(detail.Messages))
	}
	if detail.Status != ConversationDetailStatusIdle {
		t.Errorf("Status = %q, want %q", detail.Status, ConversationDetailStatusIdle)
	}
}
