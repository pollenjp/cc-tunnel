package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/credential"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

// mockCredService is a test double for credentialService.
type mockCredService struct {
	credJSON []byte
	err      error
}

func (m *mockCredService) FetchAndDecrypt(_ context.Context, _ string) ([]byte, error) {
	return m.credJSON, m.err
}

func (m *mockCredService) MarkInvalid(_ context.Context, _ string) error {
	return nil
}

// TestSendMessage_CredentialsRequired_Returns401 verifies that SendMessage returns
// HTTP 401 with "credentials_required" when credService reports ErrNotFound.
func TestSendMessage_CredentialsRequired_Returns401(t *testing.T) {
	const convIDStr = "d0000001-0000-0000-0000-000000000001"

	repo := &mockRepoCheckCtx{conv: makeConv(convIDStr)}
	remote := &mockRemoteWithCancel{cancel: func() {}}
	credSvc := &mockCredService{err: credential.ErrNotFound}

	// Create server with credService set.
	server := &Server{
		repo:              repo,
		remote:            remote,
		executionProvider: remote,
		session:           newAppSession(),
		credService:       credSvc,
	}
	// Register a session so bearerToken + session.get succeeds.
	token := "test-token-cred"
	server.session.set(token, AppUser{Id: uuid.New().String(), Name: "alice"})

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"content":"hello"}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost,
		"/conversations/"+convIDStr+"/messages", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	convID := ConversationId(uuid.MustParse(convIDStr))
	server.SendMessage(w, req, convID)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "credentials_required") {
		t.Errorf("expected credentials_required in body, got %q", w.Body.String())
	}
}

// TestSendMessage_CredentialsInvalid_Returns401 verifies that SendMessage returns
// HTTP 401 with "credentials_invalid" when credService reports ErrCredentialsInvalid.
func TestSendMessage_CredentialsInvalid_Returns401(t *testing.T) {
	const convIDStr = "d0000002-0000-0000-0000-000000000002"

	repo := &mockRepoCheckCtx{conv: makeConv(convIDStr)}
	remote := &mockRemoteWithCancel{cancel: func() {}}
	credSvc := &mockCredService{err: credential.ErrCredentialsInvalid}

	server := &Server{
		repo:              repo,
		remote:            remote,
		executionProvider: remote,
		session:           newAppSession(),
		credService:       credSvc,
	}
	token := "test-token-invalid"
	server.session.set(token, AppUser{Id: uuid.New().String(), Name: "alice"})

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"content":"hello"}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost,
		"/conversations/"+convIDStr+"/messages", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	convID := ConversationId(uuid.MustParse(convIDStr))
	server.SendMessage(w, req, convID)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "credentials_invalid") {
		t.Errorf("expected credentials_invalid in body, got %q", w.Body.String())
	}
}

// TestSendMessage_CredentialServiceError_Returns500 verifies that unexpected errors
// from credService produce HTTP 500.
func TestSendMessage_CredentialServiceError_Returns500(t *testing.T) {
	const convIDStr = "d0000003-0000-0000-0000-000000000003"

	repo := &mockRepoCheckCtx{conv: makeConv(convIDStr)}
	remote := &mockRemoteWithCancel{cancel: func() {}}
	credSvc := &mockCredService{err: errors.New("db connection lost")}

	server := &Server{
		repo:              repo,
		remote:            remote,
		executionProvider: remote,
		session:           newAppSession(),
		credService:       credSvc,
	}
	token := "test-token-err"
	server.session.set(token, AppUser{Id: uuid.New().String(), Name: "alice"})

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"content":"hello"}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost,
		"/conversations/"+convIDStr+"/messages", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	convID := ConversationId(uuid.MustParse(convIDStr))
	server.SendMessage(w, req, convID)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// TestSendMessage_NoCredService_SkipsCheck verifies that existing tests still work
// when credService is nil (no-auth mode).
func TestSendMessage_NoCredService_SkipsCheck(t *testing.T) {
	const convIDStr = "d0000004-0000-0000-0000-000000000004"

	repo := &mockRepoCheckCtx{conv: makeConv(convIDStr)}
	remote := &mockRemoteWithCancel{
		events:    []remoteclient.StreamEvent{makeResultEvent("sess-no-cred")},
		sessionID: "sess-no-cred",
		cancel:    func() {},
	}

	done := make(chan struct{})
	server := &Server{
		repo:              repo,
		remote:            remote,
		executionProvider: remote,
		// credService is nil
		doneCh: done,
	}

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"content":"hello"}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost,
		"/conversations/"+convIDStr+"/messages", body)
	req.Header.Set("Content-Type", "application/json")

	convID := ConversationId(uuid.MustParse(convIDStr))
	server.SendMessage(w, req, convID)
	<-done

	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", w.Code)
	}
}
