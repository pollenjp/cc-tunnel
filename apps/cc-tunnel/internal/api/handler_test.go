package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/db"
)

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, 200, map[string]string{"key": "value"})

	if w.Code != 200 {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %s", ct)
	}

	var result map[string]string
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result["key"] != "value" {
		t.Fatalf("expected 'value', got %q", result["key"])
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, 400, "bad request")

	if w.Code != 400 {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	var result Error
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.Error != "bad request" {
		t.Fatalf("expected 'bad request', got %q", result.Error)
	}
}

// TestGetConversation_returnsStatus verifies that GET /conversations/:id
// includes the conversation status in its response.
// This is a regression test for the bug where ConversationDetail.Status was
// not set in the handler, causing the frontend poller to never detect completion.
func TestGetConversation_returnsStatus(t *testing.T) {
	const convIDStr = "00000001-0000-0000-0000-000000000001"

	conv := makeConv(convIDStr)
	conv.Status = "completed"

	repo := &mockRepoCheckCtx{
		conv: conv,
		msgs: nil,
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/conversations/"+convIDStr, nil)

	server := &Server{repo: repo, remote: nil}
	convID := ConversationId(uuid.MustParse(convIDStr))
	server.GetConversation(w, req, convID)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var detail ConversationDetail
	if err := json.NewDecoder(w.Body).Decode(&detail); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if detail.Status != ConversationDetailStatusCompleted {
		t.Errorf("Status = %q, want %q", detail.Status, ConversationDetailStatusCompleted)
	}
}

// TestGetConversation_hasAllFields verifies that GET /conversations/:id response
// includes all required fields: id, status, created_at, and messages with status.
func TestGetConversation_hasAllFields(t *testing.T) {
	const convIDStr = "10000001-0000-0000-0000-000000000001"
	now := time.Now().UTC().Truncate(time.Second)

	conv := makeConv(convIDStr)
	conv.Status = "completed"
	conv.CreatedAt = now
	conv.UpdatedAt = now

	msgID := "20000001-0000-0000-0000-000000000001"
	msgs := []*db.Message{
		{
			ID:             msgID,
			ConversationID: convIDStr,
			Role:           "user",
			MessageData:    map[string]interface{}{"content": "hi"},
			Status:         "completed",
			CreatedAt:      now,
		},
	}

	repo := &mockRepoCheckCtx{conv: conv, msgs: msgs}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/conversations/"+convIDStr, nil)
	server := &Server{repo: repo, remote: nil}
	server.GetConversation(w, req, ConversationId(uuid.MustParse(convIDStr)))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var detail ConversationDetail
	if err := json.NewDecoder(w.Body).Decode(&detail); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if detail.Id == (uuid.UUID{}) {
		t.Error("Id should not be zero UUID")
	}
	if string(detail.Status) == "" {
		t.Error("Status should not be empty")
	}
	if detail.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if detail.Model == "" {
		t.Error("Model should not be empty")
	}
	if len(detail.Messages) != 1 {
		t.Fatalf("Messages len = %d, want 1", len(detail.Messages))
	}
	if detail.Messages[0].Status == nil {
		t.Error("Message.Status should not be nil when DB has status")
	}
}

// TestListConversations_hasAllFields verifies that GET /conversations response
// includes id, status, and created_at for each conversation.
func TestListConversations_hasAllFields(t *testing.T) {
	const convIDStr = "30000001-0000-0000-0000-000000000001"
	now := time.Now().UTC().Truncate(time.Second)

	conv := makeConv(convIDStr)
	conv.Status = "idle"
	conv.CreatedAt = now
	conv.UpdatedAt = now

	repo := &mockRepoCheckCtx{conv: conv}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/conversations", nil)
	server := &Server{repo: repo, remote: nil}
	server.ListConversations(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var convs []Conversation
	if err := json.NewDecoder(w.Body).Decode(&convs); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if len(convs) == 0 {
		t.Fatal("expected at least 1 conversation")
	}
	c := convs[0]
	if c.Id == (uuid.UUID{}) {
		t.Error("Id should not be zero UUID")
	}
	if string(c.Status) == "" {
		t.Error("Status should not be empty")
	}
	if c.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}
