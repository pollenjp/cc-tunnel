package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/db"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

// --- Test doubles for repository ---

// mockRepoCheckCtx responds to context cancellation in CreateMessage and
// UpdateConversationUpdatedAt to detect whether the caller passes a live vs.
// cancelled context.
type mockRepoCheckCtx struct {
	conv                       *db.Conversation
	msgs                       []*db.Message
	assistantMsgSaved          bool
	updatedTitle               string
	statusHistory              []string
	streamingMsgCreated        bool
	updateContentBlocksCalled  bool
	msgStatusHistory           []string
	mergeDataHistory           []map[string]interface{}
}

func (m *mockRepoCheckCtx) CreateConversation(_ context.Context, _, _ string, _ *string) (*db.Conversation, error) {
	return m.conv, nil
}

func (m *mockRepoCheckCtx) GetConversation(_ context.Context, _ string) (*db.Conversation, error) {
	if m.conv == nil {
		return nil, fmt.Errorf("not found")
	}
	return m.conv, nil
}

func (m *mockRepoCheckCtx) ListConversations(_ context.Context) ([]*db.Conversation, error) {
	return []*db.Conversation{m.conv}, nil
}

func (m *mockRepoCheckCtx) DeleteConversation(_ context.Context, _ string) error {
	return nil
}

func (m *mockRepoCheckCtx) UpdateConversationUpdatedAt(ctx context.Context, _ string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return nil
}

func (m *mockRepoCheckCtx) UpdateConversationTitle(_ context.Context, _ string, title string) error {
	m.updatedTitle = title
	return nil
}

func (m *mockRepoCheckCtx) UpdateConversationStatus(_ context.Context, _ string, status string) error {
	m.statusHistory = append(m.statusHistory, status)
	return nil
}

func (m *mockRepoCheckCtx) CreateMessage(ctx context.Context, _ string, role string, _ map[string]interface{}) (*db.Message, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if role == "assistant" {
		m.assistantMsgSaved = true
	}
	return &db.Message{
		ID:             uuid.New().String(),
		ConversationID: m.conv.ID,
		Role:           role,
		MessageData:    map[string]interface{}{},
		CreatedAt:      time.Now(),
	}, nil
}

func (m *mockRepoCheckCtx) ListMessages(_ context.Context, _ string) ([]*db.Message, error) {
	return m.msgs, nil
}

func (m *mockRepoCheckCtx) CreateStreamingMessage(_ context.Context, convID, role string, _ map[string]interface{}) (*db.Message, error) {
	m.streamingMsgCreated = true
	return &db.Message{
		ID:             uuid.New().String(),
		ConversationID: convID,
		Role:           role,
		MessageData:    map[string]interface{}{},
		Status:         "streaming",
		CreatedAt:      time.Now(),
	}, nil
}

func (m *mockRepoCheckCtx) UpdateMessageContentBlocks(_ context.Context, _ string, _ []map[string]interface{}) error {
	m.updateContentBlocksCalled = true
	return nil
}

func (m *mockRepoCheckCtx) UpdateMessageStatus(_ context.Context, _ string, status string) error {
	m.msgStatusHistory = append(m.msgStatusHistory, status)
	return nil
}

func (m *mockRepoCheckCtx) MergeMessageData(_ context.Context, _ string, data map[string]interface{}) error {
	m.mergeDataHistory = append(m.mergeDataHistory, data)
	return nil
}

func (m *mockRepoCheckCtx) UpdateSessionEndpointLastActivity(_ context.Context, _ string) error {
	return nil
}

// --- Test doubles for remoteClient ---

// mockRemoteWithCancel cancels the caller's r.Context() mid-execution to simulate
// a frontend disconnect, then still completes execution normally.
type mockRemoteWithCancel struct {
	events    []remoteclient.StreamEvent
	sessionID string
	cancel    context.CancelFunc
}

func (m *mockRemoteWithCancel) GetAuthStatus(_ context.Context) (*remoteclient.AuthStatus, error) {
	return &remoteclient.AuthStatus{}, nil
}
func (m *mockRemoteWithCancel) InitiateLogin(_ context.Context, _ string) (*remoteclient.LoginResponse, error) {
	return &remoteclient.LoginResponse{}, nil
}
func (m *mockRemoteWithCancel) Logout(_ context.Context) (*remoteclient.AuthStatus, error) {
	return &remoteclient.AuthStatus{}, nil
}
func (m *mockRemoteWithCancel) CancelLogin(_ context.Context) (*remoteclient.AuthCancelResponse, error) {
	return &remoteclient.AuthCancelResponse{}, nil
}
func (m *mockRemoteWithCancel) SubmitAuthInput(_ context.Context, _ string) (*remoteclient.AuthInputResponse, error) {
	return &remoteclient.AuthInputResponse{}, nil
}
func (m *mockRemoteWithCancel) GetAuthOutput(_ context.Context, _ int) (*remoteclient.AuthOutputResponse, error) {
	return &remoteclient.AuthOutputResponse{}, nil
}

func (m *mockRemoteWithCancel) Execute(_ context.Context, _ remoteclient.Request, onEvent func(remoteclient.StreamEvent)) (string, error) {
	// Simulate frontend disconnect by cancelling r.Context().
	// A correct implementation uses execCtx (context.WithoutCancel), so this cancel
	// must not affect Execute's context or subsequent DB operations.
	m.cancel()
	for _, e := range m.events {
		onEvent(e)
	}
	return m.sessionID, nil
}

func (m *mockRemoteWithCancel) PrepareForRelogin(_ context.Context, _ string) error {
	return nil
}

func (m *mockRemoteWithCancel) PullCredentialsFromSession(_ context.Context, _ string) (string, error) {
	return "", nil
}

// mockRemoteWithCancelAndCtxCheck cancels r.Context() then reports whether the
// context Execute received was already cancelled at that point.
type mockRemoteWithCancelAndCtxCheck struct {
	events                   []remoteclient.StreamEvent
	sessionID                string
	cancel                   context.CancelFunc
	executeCtxCancelledAtEntry bool
}

func (m *mockRemoteWithCancelAndCtxCheck) GetAuthStatus(_ context.Context) (*remoteclient.AuthStatus, error) {
	return &remoteclient.AuthStatus{}, nil
}
func (m *mockRemoteWithCancelAndCtxCheck) InitiateLogin(_ context.Context, _ string) (*remoteclient.LoginResponse, error) {
	return &remoteclient.LoginResponse{}, nil
}
func (m *mockRemoteWithCancelAndCtxCheck) Logout(_ context.Context) (*remoteclient.AuthStatus, error) {
	return &remoteclient.AuthStatus{}, nil
}
func (m *mockRemoteWithCancelAndCtxCheck) CancelLogin(_ context.Context) (*remoteclient.AuthCancelResponse, error) {
	return &remoteclient.AuthCancelResponse{}, nil
}
func (m *mockRemoteWithCancelAndCtxCheck) SubmitAuthInput(_ context.Context, _ string) (*remoteclient.AuthInputResponse, error) {
	return &remoteclient.AuthInputResponse{}, nil
}
func (m *mockRemoteWithCancelAndCtxCheck) GetAuthOutput(_ context.Context, _ int) (*remoteclient.AuthOutputResponse, error) {
	return &remoteclient.AuthOutputResponse{}, nil
}

func (m *mockRemoteWithCancelAndCtxCheck) Execute(ctx context.Context, _ remoteclient.Request, onEvent func(remoteclient.StreamEvent)) (string, error) {
	// Cancel r.Context() first.
	m.cancel()
	// Then check whether the context Execute received is cancelled.
	// With the fix: ctx is context.WithoutCancel → never cancelled → test passes.
	// Without the fix: ctx is r.Context() → cancelled above → test fails.
	select {
	case <-ctx.Done():
		m.executeCtxCancelledAtEntry = true
	default:
		m.executeCtxCancelledAtEntry = false
	}
	for _, e := range m.events {
		onEvent(e)
	}
	return m.sessionID, nil
}

func (m *mockRemoteWithCancelAndCtxCheck) PrepareForRelogin(_ context.Context, _ string) error {
	return nil
}

func (m *mockRemoteWithCancelAndCtxCheck) PullCredentialsFromSession(_ context.Context, _ string) (string, error) {
	return "", nil
}

// --- Failing ResponseWriter (simulates broken SSE connection) ---

type failingResponseWriter struct {
	header     http.Header
	statusCode int
}

func newFailingResponseWriter() *failingResponseWriter {
	return &failingResponseWriter{header: make(http.Header)}
}

func (f *failingResponseWriter) Header() http.Header         { return f.header }
func (f *failingResponseWriter) WriteHeader(code int)         { f.statusCode = code }
func (f *failingResponseWriter) Write(_ []byte) (int, error) { return 0, fmt.Errorf("broken pipe") }
func (f *failingResponseWriter) Flush()                       {}

// --- Helpers ---

func makeConv(id string) *db.Conversation {
	return &db.Conversation{
		ID:    id,
		Title: "Test Conversation",
		Model: "claude-sonnet-4-6",
	}
}

func makeTextEvent(text string) remoteclient.StreamEvent {
	return remoteclient.StreamEvent{
		Type: "assistant",
		Message: &struct {
			Content []remoteclient.ContentBlock `json:"content"`
		}{
			Content: []remoteclient.ContentBlock{
				{Type: "text", Text: text},
			},
		},
	}
}

func makeResultEvent(sessionID string) remoteclient.StreamEvent {
	return remoteclient.StreamEvent{
		Type:      "result",
		SessionID: sessionID,
	}
}

// =============================================================================
// 202 レスポンス + message_id 検証
// =============================================================================

// TestSendMessage_Returns202WithMessageID verifies that SendMessage returns HTTP 202
// with a JSON body containing "message_id".
func TestSendMessage_Returns202WithMessageID(t *testing.T) {
	const convIDStr = "00000001-0000-0000-0000-000000000001"
	ctx := context.Background()

	repo := &mockRepoCheckCtx{
		conv: makeConv(convIDStr),
		msgs: []*db.Message{},
	}
	remote := &mockRemoteWithCancel{
		events:    []remoteclient.StreamEvent{makeResultEvent("sess-202")},
		sessionID: "sess-202",
		cancel:    func() {},
	}

	done := make(chan struct{})
	w := httptest.NewRecorder()
	body := strings.NewReader(`{"content":"hi"}`)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/conversations/"+convIDStr+"/messages", body)
	req.Header.Set("Content-Type", "application/json")

	server := &Server{repo: repo, remote: remote, executionProvider: remote, doneCh: done}
	convID := ConversationId(uuid.MustParse(convIDStr))
	server.SendMessage(w, req, convID)
	<-done

	if w.Code != http.StatusAccepted {
		t.Errorf("FAIL: status = %d, want %d (202 Accepted)", w.Code, http.StatusAccepted)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("FAIL: response body is not valid JSON: %v", err)
	}
	if _, ok := resp["message_id"]; !ok {
		t.Errorf("FAIL: response body missing 'message_id' field: %v", resp)
	}
}

// =============================================================================
// Cycle 1: SSE切断後もDB保存が完了すること
// =============================================================================

// TestSendMessage_ContextCancelledDuringExecution_AssistantMessageSaved verifies
// that when r.Context() is cancelled mid-execution (simulating frontend disconnect),
// the assistant message is still saved to the DB.
//
// Without the fix: CreateMessage(assistant) is called with r.Context() which is
// cancelled → returns context.Canceled → assistant message is NOT saved.
//
// With the fix (context.WithoutCancel for post-Execute DB ops): CreateMessage is
// called with execCtx which is never cancelled → save succeeds.
func TestSendMessage_ContextCancelledDuringExecution_AssistantMessageSaved(t *testing.T) {
	const convIDStr = "11111111-1111-1111-1111-111111111111"
	ctx, cancel := context.WithCancel(context.Background())

	repo := &mockRepoCheckCtx{
		conv: makeConv(convIDStr),
		msgs: []*db.Message{},
	}
	remote := &mockRemoteWithCancel{
		events: []remoteclient.StreamEvent{
			makeTextEvent("Hello from AI"),
			makeResultEvent("sess-abc"),
		},
		sessionID: "sess-abc",
		cancel:    cancel,
	}

	done := make(chan struct{})
	w := newFailingResponseWriter()
	body := strings.NewReader(`{"content":"hi"}`)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/conversations/"+convIDStr+"/messages", body)
	req.Header.Set("Content-Type", "application/json")

	server := &Server{repo: repo, remote: remote, executionProvider: remote, doneCh: done}
	convID := ConversationId(uuid.MustParse(convIDStr))
	server.SendMessage(w, req, convID)
	<-done

	// With the streaming approach, message is created at start via CreateStreamingMessage
	// (before Execute), so it's always persisted regardless of frontend disconnect.
	if !repo.streamingMsgCreated {
		t.Error("FAIL: assistant message was NOT saved to DB after frontend disconnect\n" +
			"Expected: CreateStreamingMessage called before Execute with execCtx\n" +
			"Got: CreateStreamingMessage never called")
	}
}

// =============================================================================
// Cycle 2: ctx.Done() でCLI実行が止まらないこと
// =============================================================================

// TestSendMessage_ExecuteContextIsIndependentOfRequestContext verifies that the
// context passed to Execute is independent of r.Context(): it must remain
// uncancelled even after r.Context() is cancelled by a frontend disconnect.
//
// Without the fix: Execute receives r.Context() → after m.cancel(), the ctx
// Execute holds is already Done → executeCtxCancelledAtEntry = true → test fails.
//
// With the fix: Execute receives context.WithoutCancel(r.Context()) → ctx.Done()
// is never closed → executeCtxCancelledAtEntry = false → test passes.
func TestSendMessage_ExecuteContextIsIndependentOfRequestContext(t *testing.T) {
	const convIDStr = "33333333-3333-3333-3333-333333333333"
	ctx, cancel := context.WithCancel(context.Background())

	remote := &mockRemoteWithCancelAndCtxCheck{
		events:    []remoteclient.StreamEvent{makeResultEvent("sess-def")},
		sessionID: "sess-def",
		cancel:    cancel,
	}
	repo := &mockRepoCheckCtx{
		conv: makeConv(convIDStr),
		msgs: []*db.Message{},
	}

	done := make(chan struct{})
	w := newFailingResponseWriter()
	body := strings.NewReader(`{"content":"cycle2 test"}`)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/conversations/"+convIDStr+"/messages", body)
	req.Header.Set("Content-Type", "application/json")

	server := &Server{repo: repo, remote: remote, executionProvider: remote, doneCh: done}
	convID := ConversationId(uuid.MustParse(convIDStr))
	server.SendMessage(w, req, convID)
	<-done

	if remote.executeCtxCancelledAtEntry {
		t.Error("FAIL: Execute received a context that was already cancelled\n" +
			"Expected: Execute receives context.WithoutCancel(r.Context()) — never cancelled\n" +
			"Got: Execute received r.Context() which was cancelled by the simulated disconnect")
	}
}

// =============================================================================
// Cycle 3: アシスタント応答後にタイトルが自動更新されること
// =============================================================================

// TestSendMessage_AssistantResponse_TitleUpdated verifies that after the assistant
// message is saved, UpdateConversationTitle is called with the generated title.
func TestSendMessage_AssistantResponse_TitleUpdated(t *testing.T) {
	const convIDStr = "55555555-5555-5555-5555-555555555555"
	ctx := context.Background()

	repo := &mockRepoCheckCtx{
		conv: makeConv(convIDStr),
		msgs: []*db.Message{},
	}
	remote := &mockRemoteWithCancel{
		events: []remoteclient.StreamEvent{
			makeTextEvent("Hello from AI"),
			makeResultEvent("sess-title"),
		},
		sessionID: "sess-title",
		cancel:    func() {}, // no-op: no disconnect simulation
	}

	done := make(chan struct{})
	w := httptest.NewRecorder()
	body := strings.NewReader(`{"content":"hi"}`)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/conversations/"+convIDStr+"/messages", body)
	req.Header.Set("Content-Type", "application/json")

	server := &Server{repo: repo, remote: remote, executionProvider: remote, doneCh: done}
	convID := ConversationId(uuid.MustParse(convIDStr))
	server.SendMessage(w, req, convID)
	<-done

	wantTitle := generateTitle("Hello from AI")
	if repo.updatedTitle != wantTitle {
		t.Errorf("FAIL: UpdateConversationTitle not called with expected title\nGot: %q\nWant: %q", repo.updatedTitle, wantTitle)
	}
}

// =============================================================================
// Cycle 1 (status): SendMessage が 'running' → 'completed' の順でステータスを更新すること
// =============================================================================

// TestSendMessage_StatusUpdatedToRunningThenCompleted verifies that SendMessage
// calls UpdateConversationStatus("running") before Execute starts, and then
// calls UpdateConversationStatus("completed") after Execute finishes (even on
// a normal completion path).
func TestSendMessage_StatusUpdatedToRunningThenCompleted(t *testing.T) {
	const convIDStr = "77777777-7777-7777-7777-777777777777"
	ctx := context.Background()

	repo := &mockRepoCheckCtx{
		conv: makeConv(convIDStr),
		msgs: []*db.Message{},
	}
	remote := &mockRemoteWithCancel{
		events: []remoteclient.StreamEvent{
			makeTextEvent("Hello"),
			makeResultEvent("sess-status"),
		},
		sessionID: "sess-status",
		cancel:    func() {}, // no disconnect
	}

	done := make(chan struct{})
	w := httptest.NewRecorder()
	body := strings.NewReader(`{"content":"status test"}`)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/conversations/"+convIDStr+"/messages", body)
	req.Header.Set("Content-Type", "application/json")

	server := &Server{repo: repo, remote: remote, executionProvider: remote, doneCh: done}
	convID := ConversationId(uuid.MustParse(convIDStr))
	server.SendMessage(w, req, convID)
	<-done

	// Expect exactly: ["running", "completed"]
	wantHistory := []string{"running", "completed"}
	if len(repo.statusHistory) != len(wantHistory) {
		t.Fatalf("FAIL: expected %d status updates, got %d: %v", len(wantHistory), len(repo.statusHistory), repo.statusHistory)
	}
	for i, want := range wantHistory {
		if repo.statusHistory[i] != want {
			t.Errorf("FAIL: statusHistory[%d] = %q, want %q", i, repo.statusHistory[i], want)
		}
	}
}

// =============================================================================
// TDD Cycle 2: SendMessage 冒頭で assistant メッセージが streaming 状態で作成されること
// =============================================================================

// TestSendMessage_StreamingMessageCreatedAtStart verifies that SendMessage calls
// CreateStreamingMessage before Execute (not after), so that a crash during
// execution still leaves a 'streaming' record in the DB for recovery.
func TestSendMessage_StreamingMessageCreatedAtStart(t *testing.T) {
	const convIDStr = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	ctx := context.Background()

	repo := &mockRepoCheckCtx{
		conv: makeConv(convIDStr),
		msgs: []*db.Message{},
	}
	remote := &mockRemoteWithCancel{
		events: []remoteclient.StreamEvent{
			makeTextEvent("Hello"),
			makeResultEvent("sess-stream"),
		},
		sessionID: "sess-stream",
		cancel:    func() {},
	}

	done := make(chan struct{})
	w := httptest.NewRecorder()
	body := strings.NewReader(`{"content":"stream test"}`)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/conversations/"+convIDStr+"/messages", body)
	req.Header.Set("Content-Type", "application/json")

	server := &Server{repo: repo, remote: remote, executionProvider: remote, doneCh: done}
	convID := ConversationId(uuid.MustParse(convIDStr))
	server.SendMessage(w, req, convID)
	<-done

	if !repo.streamingMsgCreated {
		t.Error("FAIL: CreateStreamingMessage was NOT called\n" +
			"Expected: assistant message created with status='streaming' before Execute\n" +
			"Got: CreateStreamingMessage never called")
	}
}

// =============================================================================
// TDD Cycle 3: CLI実行中に UpdateMessageContentBlocks が呼ばれること
// =============================================================================

// TestSendMessage_UpdateContentBlocksCalledOnCompletion verifies that
// UpdateMessageContentBlocks is called at least once (at completion) so that
// content_blocks are persisted even if the ticker never fires in unit tests.
func TestSendMessage_UpdateContentBlocksCalledOnCompletion(t *testing.T) {
	const convIDStr = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	ctx := context.Background()

	repo := &mockRepoCheckCtx{
		conv: makeConv(convIDStr),
		msgs: []*db.Message{},
	}
	remote := &mockRemoteWithCancel{
		events: []remoteclient.StreamEvent{
			makeTextEvent("Final answer"),
			makeResultEvent("sess-batch"),
		},
		sessionID: "sess-batch",
		cancel:    func() {},
	}

	done := make(chan struct{})
	w := httptest.NewRecorder()
	body := strings.NewReader(`{"content":"batch test"}`)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/conversations/"+convIDStr+"/messages", body)
	req.Header.Set("Content-Type", "application/json")

	server := &Server{repo: repo, remote: remote, executionProvider: remote, doneCh: done}
	convID := ConversationId(uuid.MustParse(convIDStr))
	server.SendMessage(w, req, convID)
	<-done

	if !repo.updateContentBlocksCalled {
		t.Error("FAIL: UpdateMessageContentBlocks was NOT called\n" +
			"Expected: content_blocks persisted at completion via UpdateMessageContentBlocks\n" +
			"Got: UpdateMessageContentBlocks never called")
	}
}

// TestSendMessage_MessageStatusCompletedOnSuccess verifies that
// UpdateMessageStatus("completed") is called on the assistant message after
// successful execution.
func TestSendMessage_MessageStatusCompletedOnSuccess(t *testing.T) {
	const convIDStr = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	ctx := context.Background()

	repo := &mockRepoCheckCtx{
		conv: makeConv(convIDStr),
		msgs: []*db.Message{},
	}
	remote := &mockRemoteWithCancel{
		events: []remoteclient.StreamEvent{
			makeTextEvent("Done"),
			makeResultEvent("sess-msgstatus"),
		},
		sessionID: "sess-msgstatus",
		cancel:    func() {},
	}

	done := make(chan struct{})
	w := httptest.NewRecorder()
	body := strings.NewReader(`{"content":"msg status test"}`)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/conversations/"+convIDStr+"/messages", body)
	req.Header.Set("Content-Type", "application/json")

	server := &Server{repo: repo, remote: remote, executionProvider: remote, doneCh: done}
	convID := ConversationId(uuid.MustParse(convIDStr))
	server.SendMessage(w, req, convID)
	<-done

	if len(repo.msgStatusHistory) == 0 {
		t.Error("FAIL: UpdateMessageStatus was NOT called on assistant message\n" +
			"Expected: UpdateMessageStatus('completed') called after execution\n" +
			"Got: msgStatusHistory is empty")
		return
	}
	last := repo.msgStatusHistory[len(repo.msgStatusHistory)-1]
	if last != "completed" {
		t.Errorf("FAIL: last message status = %q, want %q", last, "completed")
	}
}

// =============================================================================
// TDD Cycle (polling_tool_display): バッチ ticker が tool_calls も保存すること
// =============================================================================

// mockRemoteSlowExec fires events then sleeps so that the batch ticker goroutine
// has time to fire multiple times before Execute returns.
type mockRemoteSlowExec struct {
	events    []remoteclient.StreamEvent
	sessionID string
	sleepDur  time.Duration
}

func (m *mockRemoteSlowExec) GetAuthStatus(_ context.Context) (*remoteclient.AuthStatus, error) {
	return &remoteclient.AuthStatus{}, nil
}
func (m *mockRemoteSlowExec) InitiateLogin(_ context.Context, _ string) (*remoteclient.LoginResponse, error) {
	return &remoteclient.LoginResponse{}, nil
}
func (m *mockRemoteSlowExec) Logout(_ context.Context) (*remoteclient.AuthStatus, error) {
	return &remoteclient.AuthStatus{}, nil
}
func (m *mockRemoteSlowExec) CancelLogin(_ context.Context) (*remoteclient.AuthCancelResponse, error) {
	return &remoteclient.AuthCancelResponse{}, nil
}
func (m *mockRemoteSlowExec) SubmitAuthInput(_ context.Context, _ string) (*remoteclient.AuthInputResponse, error) {
	return &remoteclient.AuthInputResponse{}, nil
}
func (m *mockRemoteSlowExec) GetAuthOutput(_ context.Context, _ int) (*remoteclient.AuthOutputResponse, error) {
	return &remoteclient.AuthOutputResponse{}, nil
}
func (m *mockRemoteSlowExec) Execute(_ context.Context, _ remoteclient.Request, onEvent func(remoteclient.StreamEvent)) (string, error) {
	for _, e := range m.events {
		onEvent(e)
	}
	time.Sleep(m.sleepDur)
	return m.sessionID, nil
}

func (m *mockRemoteSlowExec) PrepareForRelogin(_ context.Context, _ string) error {
	return nil
}

func (m *mockRemoteSlowExec) PullCredentialsFromSession(_ context.Context, _ string) (string, error) {
	return "", nil
}

// makeToolUseStartStreamEvent builds a stream_event/content_block_start event
// that triggers tool_use processing in SendMessage.
func makeToolUseStartStreamEvent(id, name string) remoteclient.StreamEvent {
	cbInner, _ := json.Marshal(map[string]string{
		"type": "tool_use",
		"id":   id,
		"name": name,
	})
	innerEvent, _ := json.Marshal(map[string]interface{}{
		"type":          "content_block_start",
		"index":         0,
		"content_block": json.RawMessage(cbInner),
	})
	return remoteclient.StreamEvent{
		Type:  "stream_event",
		Event: json.RawMessage(innerEvent),
	}
}

// TestSendMessage_BatchTickerSavesToolCalls verifies that the batch ticker goroutine
// calls MergeMessageData with "tool_calls" in addition to UpdateMessageContentBlocks.
//
// Without the fix: the batch goroutine only calls UpdateMessageContentBlocks.
//   → MergeMessageData with "tool_calls" is called exactly once (at completion only).
// With the fix: the batch goroutine also calls MergeMessageData with "tool_calls".
//   → MergeMessageData with "tool_calls" is called >= 2 times (batch + completion).
func TestSendMessage_BatchTickerSavesToolCalls(t *testing.T) {
	const convIDStr = "ffff0001-ffff-ffff-ffff-ffffffffffff"
	ctx := context.Background()

	repo := &mockRepoCheckCtx{
		conv: makeConv(convIDStr),
		msgs: []*db.Message{},
	}
	// mockRemoteSlowExec fires events then sleeps 30ms so that the 1ms batch ticker
	// has time to fire multiple times before Execute returns.
	remote := &mockRemoteSlowExec{
		events: []remoteclient.StreamEvent{
			makeToolUseStartStreamEvent("toolu_001", "bash"),
			makeResultEvent("sess-batch-tool"),
		},
		sessionID: "sess-batch-tool",
		sleepDur:  30 * time.Millisecond,
	}

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"content":"batch tool test"}`)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/conversations/"+convIDStr+"/messages", body)
	req.Header.Set("Content-Type", "application/json")

	done := make(chan struct{})
	server := &Server{
		repo:              repo,
		remote:            remote,
		executionProvider: remote,
		batchInterval:     1 * time.Millisecond,
		doneCh:            done,
	}
	convID := ConversationId(uuid.MustParse(convIDStr))
	server.SendMessage(w, req, convID)
	<-done

	// Count how many MergeMessageData calls included "tool_calls" key.
	toolCallsMergeCount := 0
	for _, data := range repo.mergeDataHistory {
		if _, ok := data["tool_calls"]; ok {
			toolCallsMergeCount++
		}
	}

	// With the fix: batch ticker also calls MergeMessageData(tool_calls), so count >= 2.
	// Without the fix: only completion saves tool_calls → count == 1.
	if toolCallsMergeCount < 2 {
		t.Errorf("FAIL: MergeMessageData with 'tool_calls' called %d time(s), want >= 2\n"+
			"Expected: batch ticker calls MergeMessageData(tool_calls) during streaming\n"+
			"Got: MergeMessageData(tool_calls) called only at completion (not in batch)",
			toolCallsMergeCount)
	}
}
