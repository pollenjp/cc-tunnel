package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

// mockCredStorer is a test double for credentialStorer.
type mockCredStorer struct {
	stored map[string]string // username → credJSON
	err    error
}

func newMockCredStorer() *mockCredStorer {
	return &mockCredStorer{stored: make(map[string]string)}
}

func (m *mockCredStorer) StoreCredential(_ context.Context, username, credJSON string) error {
	if m.err != nil {
		return m.err
	}
	m.stored[username] = credJSON
	return nil
}

// mockProviderRelogin is a test double for provider.ExecutionProvider that supports
// PrepareForRelogin and PullCredentialsFromSession.
type mockProviderRelogin struct {
	prepareErr   error
	pullCredJSON string
	pullErr      error
}

func (m *mockProviderRelogin) Execute(_ context.Context, _ remoteclient.Request, _ func(remoteclient.StreamEvent)) (string, error) {
	return "", nil
}

func (m *mockProviderRelogin) PrepareForRelogin(_ context.Context, _ string) error {
	return m.prepareErr
}

func (m *mockProviderRelogin) PullCredentialsFromSession(_ context.Context, _ string) (string, error) {
	return m.pullCredJSON, m.pullErr
}

// TestPostReloginStart_Unauthorized verifies 401 when no Bearer token is present.
func TestPostReloginStart_Unauthorized(t *testing.T) {
	server := &Server{
		repo:              &mockRepoCheckCtx{},
		executionProvider: &mockProviderRelogin{},
		session:           newAppSession(),
	}

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"conversationId":"` + uuid.New().String() + `"}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/credentials/relogin/start", body)

	server.PostReloginStart(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// TestPostReloginStart_OK verifies successful start.
func TestPostReloginStart_OK(t *testing.T) {
	convID := uuid.New().String()
	server := &Server{
		repo:              &mockRepoCheckCtx{},
		executionProvider: &mockProviderRelogin{},
		session:           newAppSession(),
	}
	token := "test-relogin-token"
	server.session.set(token, AppUser{Id: uuid.New().String(), Name: "alice"})

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"conversationId":"` + convID + `"}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/credentials/relogin/start", body)
	req.Header.Set("Authorization", "Bearer "+token)

	server.PostReloginStart(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"ready":true`) {
		t.Errorf("expected ready:true in body, got %q", w.Body.String())
	}
}

// TestPostReloginFinalize_CredentialsNotReady verifies 400 when ErrCredentialsNotReady.
func TestPostReloginFinalize_CredentialsNotReady(t *testing.T) {
	convID := uuid.New().String()
	server := &Server{
		repo: &mockRepoCheckCtx{},
		executionProvider: &mockProviderRelogin{
			pullErr: remoteclient.ErrCredentialsNotReady,
		},
		session:    newAppSession(),
		credStorer: newMockCredStorer(),
	}
	token := "test-finalize-token"
	server.session.set(token, AppUser{Id: uuid.New().String(), Name: "bob"})

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"conversationId":"` + convID + `"}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/credentials/relogin/finalize", body)
	req.Header.Set("Authorization", "Bearer "+token)

	server.PostReloginFinalize(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestPostReloginFinalize_OK verifies successful finalize.
func TestPostReloginFinalize_OK(t *testing.T) {
	convID := uuid.New().String()
	storer := newMockCredStorer()
	server := &Server{
		repo: &mockRepoCheckCtx{},
		executionProvider: &mockProviderRelogin{
			pullCredJSON: `{"apiKey":"real-key"}`,
		},
		session:    newAppSession(),
		credStorer: storer,
	}
	token := "test-finalize-ok-token"
	server.session.set(token, AppUser{Id: uuid.New().String(), Name: "carol"})

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"conversationId":"` + convID + `"}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/credentials/relogin/finalize", body)
	req.Header.Set("Authorization", "Bearer "+token)

	server.PostReloginFinalize(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"registered":true`) {
		t.Errorf("expected registered:true in body, got %q", w.Body.String())
	}
	if storer.stored["carol"] != `{"apiKey":"real-key"}` {
		t.Errorf("expected stored credential for carol, got %q", storer.stored["carol"])
	}
}
