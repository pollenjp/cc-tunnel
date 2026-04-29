package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

// mockProviderWithSessionClient is an ExecutionProvider that returns a configurable session client.
type mockProviderWithSessionClient struct {
	mockProviderRelogin
	sessionClient    *remoteclient.Client
	sessionClientErr error
}

func (m *mockProviderWithSessionClient) GetSessionClient(_ context.Context, _ string) (*remoteclient.Client, error) {
	return m.sessionClient, m.sessionClientErr
}

// TestGetAuthStatus_SessionNotFound verifies that GetAuthStatus returns 404 when session is missing.
func TestGetAuthStatus_SessionNotFound(t *testing.T) {
	convID := uuid.New()
	ep := &mockProviderWithSessionClient{
		sessionClientErr: fmt.Errorf("%w: %s", provider.ErrSessionNotFound, convID.String()),
	}
	server := &Server{
		repo:              &mockRepoCheckCtx{},
		executionProvider: ep,
		session:           newAppSession(),
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/status?conversationId="+convID.String(), nil)

	server.GetAuthStatus(w, req, GetAuthStatusParams{ConversationId: convID})

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestGetAuthStatus_OK verifies that GetAuthStatus proxies to session client.
func TestGetAuthStatus_OK(t *testing.T) {
	convID := uuid.New()

	// Minimal HTTP server acting as the cc-remote-agent session container.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/status" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(remoteclient.AuthStatus{LoggedIn: true, AuthMethod: "api_key"})
		}
	}))
	defer ts.Close()

	ep := &mockProviderWithSessionClient{
		sessionClient: remoteclient.NewClient(ts.URL),
	}
	server := &Server{
		repo:              &mockRepoCheckCtx{},
		executionProvider: ep,
		session:           newAppSession(),
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/status?conversationId="+convID.String(), nil)

	server.GetAuthStatus(w, req, GetAuthStatusParams{ConversationId: convID})

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestInitiateLogin_SessionNotFound verifies that InitiateLogin returns 404 when session is missing.
func TestInitiateLogin_SessionNotFound(t *testing.T) {
	convID := uuid.New()
	ep := &mockProviderWithSessionClient{
		sessionClientErr: fmt.Errorf("%w: %s", provider.ErrSessionNotFound, convID.String()),
	}
	server := &Server{
		repo:              &mockRepoCheckCtx{},
		executionProvider: ep,
		session:           newAppSession(),
	}

	body := strings.NewReader(`{"conversationId":"` + convID.String() + `"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/auth/login", body)

	server.InitiateLogin(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSubmitAuthInput_SessionNotFound verifies that SubmitAuthInput returns 404 when session is missing.
func TestSubmitAuthInput_SessionNotFound(t *testing.T) {
	convID := uuid.New()
	ep := &mockProviderWithSessionClient{
		sessionClientErr: fmt.Errorf("%w: %s", provider.ErrSessionNotFound, convID.String()),
	}
	server := &Server{
		repo:              &mockRepoCheckCtx{},
		executionProvider: ep,
		session:           newAppSession(),
	}

	body := strings.NewReader(`{"conversationId":"` + convID.String() + `","input":"enter"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/auth/input", body)

	server.SubmitAuthInput(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestGetAuthOutput_SessionNotFound verifies that GetAuthOutput returns 404 when session is missing.
func TestGetAuthOutput_SessionNotFound(t *testing.T) {
	convID := uuid.New()
	ep := &mockProviderWithSessionClient{
		sessionClientErr: fmt.Errorf("%w: %s", provider.ErrSessionNotFound, convID.String()),
	}
	server := &Server{
		repo:              &mockRepoCheckCtx{},
		executionProvider: ep,
		session:           newAppSession(),
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/output?conversationId="+convID.String(), nil)

	server.GetAuthOutput(w, req, GetAuthOutputParams{ConversationId: convID})

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestLogout_SessionNotFound verifies that Logout returns 404 when session is missing.
func TestLogout_SessionNotFound(t *testing.T) {
	convID := uuid.New()
	ep := &mockProviderWithSessionClient{
		sessionClientErr: fmt.Errorf("%w: %s", provider.ErrSessionNotFound, convID.String()),
	}
	server := &Server{
		repo:              &mockRepoCheckCtx{},
		executionProvider: ep,
		session:           newAppSession(),
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/auth/logout?conversationId="+convID.String(), nil)

	server.Logout(w, req, LogoutParams{ConversationId: convID})

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestCancelLogin_SessionNotFound verifies that CancelLogin returns 404 when session is missing.
func TestCancelLogin_SessionNotFound(t *testing.T) {
	convID := uuid.New()
	ep := &mockProviderWithSessionClient{
		sessionClientErr: fmt.Errorf("%w: %s", provider.ErrSessionNotFound, convID.String()),
	}
	server := &Server{
		repo:              &mockRepoCheckCtx{},
		executionProvider: ep,
		session:           newAppSession(),
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/auth/cancel?conversationId="+convID.String(), nil)

	server.CancelLogin(w, req, CancelLoginParams{ConversationId: convID})

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}
