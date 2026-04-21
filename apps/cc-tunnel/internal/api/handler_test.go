package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
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
