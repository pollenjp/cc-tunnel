package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/db"
)

// createTestAssistantMessage inserts a streaming assistant message linked to
// the conversation, returning the message ID for FK use.
func createTestAssistantMessage(t *testing.T, repo *db.Repository, conversationID string) string {
	t.Helper()
	ctx := context.Background()
	m, err := repo.CreateStreamingMessage(ctx, conversationID, "assistant", map[string]interface{}{})
	if err != nil {
		t.Fatalf("CreateStreamingMessage: %v", err)
	}
	return m.ID
}

func TestCreateAndGetAgentDispatch(t *testing.T) {
	repo, cleanup := setupRepo(t)
	defer cleanup()
	ctx := context.Background()

	convID := createTestConversation(t, repo)
	msgID := createTestAssistantMessage(t, repo, convID)

	d, err := repo.CreateAgentDispatch(ctx, convID, msgID, "hello world", nil)
	if err != nil {
		t.Fatalf("CreateAgentDispatch: %v", err)
	}
	if d.Status != db.DispatchStatusPending {
		t.Errorf("status = %q, want %q", d.Status, db.DispatchStatusPending)
	}
	if d.Prompt != "hello world" {
		t.Errorf("prompt = %q", d.Prompt)
	}

	got, err := repo.GetAgentDispatch(ctx, d.ID)
	if err != nil {
		t.Fatalf("GetAgentDispatch: %v", err)
	}
	if got.ID != d.ID {
		t.Errorf("id mismatch: got %q want %q", got.ID, d.ID)
	}
}

func TestClaimNextPendingDispatch_Empty(t *testing.T) {
	repo, cleanup := setupRepo(t)
	defer cleanup()
	ctx := context.Background()

	convID := createTestConversation(t, repo)

	_, err := repo.ClaimNextPendingDispatch(ctx, convID)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected pgx.ErrNoRows, got %v", err)
	}
}

func TestClaimNextPendingDispatch_FIFO(t *testing.T) {
	repo, cleanup := setupRepo(t)
	defer cleanup()
	ctx := context.Background()

	convID := createTestConversation(t, repo)
	msgID := createTestAssistantMessage(t, repo, convID)

	d1, err := repo.CreateAgentDispatch(ctx, convID, msgID, "first", nil)
	if err != nil {
		t.Fatalf("CreateAgentDispatch(first): %v", err)
	}
	d2, err := repo.CreateAgentDispatch(ctx, convID, msgID, "second", nil)
	if err != nil {
		t.Fatalf("CreateAgentDispatch(second): %v", err)
	}

	claimed, err := repo.ClaimNextPendingDispatch(ctx, convID)
	if err != nil {
		t.Fatalf("ClaimNextPendingDispatch: %v", err)
	}
	if claimed.ID != d1.ID {
		t.Fatalf("claimed wrong dispatch: got %q want %q", claimed.ID, d1.ID)
	}
	if claimed.Status != db.DispatchStatusDelivered {
		t.Errorf("status = %q, want %q", claimed.Status, db.DispatchStatusDelivered)
	}
	if claimed.DeliveredAt == nil {
		t.Error("delivered_at was not set")
	}

	// Second claim should pick up d2 (d1 is already delivered).
	claimed2, err := repo.ClaimNextPendingDispatch(ctx, convID)
	if err != nil {
		t.Fatalf("ClaimNextPendingDispatch(2): %v", err)
	}
	if claimed2.ID != d2.ID {
		t.Errorf("second claim wrong: got %q want %q", claimed2.ID, d2.ID)
	}

	// Third claim returns ErrNoRows.
	_, err = repo.ClaimNextPendingDispatch(ctx, convID)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("expected pgx.ErrNoRows after exhausting pending, got %v", err)
	}
}

func TestMarkDispatchConsumed(t *testing.T) {
	repo, cleanup := setupRepo(t)
	defer cleanup()
	ctx := context.Background()

	convID := createTestConversation(t, repo)
	msgID := createTestAssistantMessage(t, repo, convID)

	d, err := repo.CreateAgentDispatch(ctx, convID, msgID, "x", nil)
	if err != nil {
		t.Fatalf("CreateAgentDispatch: %v", err)
	}
	if err := repo.MarkDispatchConsumed(ctx, d.ID); err != nil {
		t.Fatalf("MarkDispatchConsumed: %v", err)
	}
	got, err := repo.GetAgentDispatch(ctx, d.ID)
	if err != nil {
		t.Fatalf("GetAgentDispatch: %v", err)
	}
	if got.Status != db.DispatchStatusConsumed {
		t.Errorf("status = %q, want %q", got.Status, db.DispatchStatusConsumed)
	}
	if got.ConsumedAt == nil {
		t.Error("consumed_at was not set")
	}
}

func TestAppendAgentOutput_AndList(t *testing.T) {
	repo, cleanup := setupRepo(t)
	defer cleanup()
	ctx := context.Background()

	convID := createTestConversation(t, repo)
	msgID := createTestAssistantMessage(t, repo, convID)
	d, err := repo.CreateAgentDispatch(ctx, convID, msgID, "x", nil)
	if err != nil {
		t.Fatalf("CreateAgentDispatch: %v", err)
	}

	seq, err := repo.NextEventSeq(ctx, d.ID)
	if err != nil {
		t.Fatalf("NextEventSeq: %v", err)
	}
	if seq != 1 {
		t.Errorf("first seq = %d, want 1", seq)
	}

	o1, err := repo.AppendAgentOutput(ctx, d.ID, msgID, seq,
		db.OutputTypeSessionStart,
		map[string]interface{}{"session_id": "abc-123"},
		db.OutputStatusPartial,
	)
	if err != nil {
		t.Fatalf("AppendAgentOutput(session_start): %v", err)
	}
	if o1.Payload["session_id"] != "abc-123" {
		t.Errorf("payload not round-tripped: %v", o1.Payload)
	}

	seq2, err := repo.NextEventSeq(ctx, d.ID)
	if err != nil {
		t.Fatalf("NextEventSeq(2): %v", err)
	}
	if seq2 != 2 {
		t.Errorf("second seq = %d, want 2", seq2)
	}

	if _, err := repo.AppendAgentOutput(ctx, d.ID, msgID, seq2,
		db.OutputTypeStop,
		map[string]interface{}{"reason": "done"},
		db.OutputStatusFinal,
	); err != nil {
		t.Fatalf("AppendAgentOutput(stop): %v", err)
	}

	list, err := repo.ListAgentOutputsByMessage(ctx, msgID)
	if err != nil {
		t.Fatalf("ListAgentOutputsByMessage: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2", len(list))
	}
	if list[0].EventType != db.OutputTypeSessionStart {
		t.Errorf("list[0] = %q", list[0].EventType)
	}
	if list[1].EventType != db.OutputTypeStop {
		t.Errorf("list[1] = %q", list[1].EventType)
	}
	if list[1].Status != db.OutputStatusFinal {
		t.Errorf("list[1].status = %q", list[1].Status)
	}
}

func TestAppendAgentOutput_DuplicateSeqRejected(t *testing.T) {
	repo, cleanup := setupRepo(t)
	defer cleanup()
	ctx := context.Background()

	convID := createTestConversation(t, repo)
	msgID := createTestAssistantMessage(t, repo, convID)
	d, err := repo.CreateAgentDispatch(ctx, convID, msgID, "x", nil)
	if err != nil {
		t.Fatalf("CreateAgentDispatch: %v", err)
	}

	if _, err := repo.AppendAgentOutput(ctx, d.ID, msgID, 1,
		db.OutputTypeSessionStart,
		map[string]interface{}{},
		db.OutputStatusPartial,
	); err != nil {
		t.Fatalf("first AppendAgentOutput: %v", err)
	}
	_, err = repo.AppendAgentOutput(ctx, d.ID, msgID, 1,
		db.OutputTypeSessionStart,
		map[string]interface{}{},
		db.OutputStatusPartial,
	)
	if err == nil {
		t.Fatal("expected UNIQUE violation for duplicate (dispatch_id, event_seq)")
	}
}
