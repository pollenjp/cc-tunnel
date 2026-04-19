package api

import (
	"context"
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
	conv              *db.Conversation
	msgs              []*db.Message
	assistantMsgSaved bool
	updatedTitle      string
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

	w := newFailingResponseWriter()
	body := strings.NewReader(`{"content":"hi"}`)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/conversations/"+convIDStr+"/messages", body)
	req.Header.Set("Content-Type", "application/json")

	server := &Server{repo: repo, remote: remote}
	convID := ConversationId(uuid.MustParse(convIDStr))
	server.SendMessage(w, req, convID)

	if !repo.assistantMsgSaved {
		t.Error("FAIL: assistant message was NOT saved to DB after frontend disconnect\n" +
			"Expected: CreateMessage(assistant) called with a non-cancelled execCtx\n" +
			"Got: called with cancelled r.Context() — use context.WithoutCancel for DB ops after Execute")
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

	w := newFailingResponseWriter()
	body := strings.NewReader(`{"content":"cycle2 test"}`)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/conversations/"+convIDStr+"/messages", body)
	req.Header.Set("Content-Type", "application/json")

	server := &Server{repo: repo, remote: remote}
	convID := ConversationId(uuid.MustParse(convIDStr))
	server.SendMessage(w, req, convID)

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

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"content":"hi"}`)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/conversations/"+convIDStr+"/messages", body)
	req.Header.Set("Content-Type", "application/json")

	server := &Server{repo: repo, remote: remote}
	convID := ConversationId(uuid.MustParse(convIDStr))
	server.SendMessage(w, req, convID)

	wantTitle := generateTitle("Hello from AI")
	if repo.updatedTitle != wantTitle {
		t.Errorf("FAIL: UpdateConversationTitle not called with expected title\nGot: %q\nWant: %q", repo.updatedTitle, wantTitle)
	}
}
