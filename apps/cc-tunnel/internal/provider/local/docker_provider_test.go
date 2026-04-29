package local

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

// mockSessionProvider implements sessionProvider for testing.
type mockSessionProvider struct {
	client      *remoteclient.Client
	getCalled   bool
	lastConvID  string
	lastCredLen int
	getErr      error
	stopErr     error
}

func (m *mockSessionProvider) GetOrCreate(ctx context.Context, convID string, credentials []byte) (*remoteclient.Client, error) {
	m.getCalled = true
	m.lastConvID = convID
	m.lastCredLen = len(credentials)
	return m.client, m.getErr
}

func (m *mockSessionProvider) StopAll(_ context.Context) error {
	return m.stopErr
}

func (m *mockSessionProvider) CleanupOrphans(_ context.Context) error {
	return nil
}

func TestLocalDockerProvider_Execute_delegatesToSession(t *testing.T) {
	resultEvent := remoteclient.StreamEvent{
		Type:      "result",
		SessionID: "sess-docker-test",
	}
	line, err := json.Marshal(resultEvent)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(append(line, '\n')); err != nil {
			t.Errorf("fake server write: %v", err)
		}
	}))
	defer srv.Close()

	mock := &mockSessionProvider{
		client: remoteclient.NewClient(srv.URL),
	}
	p := &LocalDockerProvider{sessions: mock}

	req := remoteclient.Request{ConversationID: "conv-abc", Prompt: "hello"}
	var received []remoteclient.StreamEvent
	sessionID, err := p.Execute(context.Background(), req, func(e remoteclient.StreamEvent) {
		received = append(received, e)
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sessionID != "sess-docker-test" {
		t.Errorf("sessionID = %q, want %q", sessionID, "sess-docker-test")
	}
	if !mock.getCalled {
		t.Error("GetOrCreate was not called")
	}
	if mock.lastConvID != "conv-abc" {
		t.Errorf("lastConvID = %q, want %q", mock.lastConvID, "conv-abc")
	}
	if len(received) != 1 || received[0].Type != "result" {
		t.Errorf("unexpected events: %v", received)
	}
}

func TestLocalDockerProvider_Execute_propagatesGetOrCreateError(t *testing.T) {
	expectedErr := errors.New("docker unavailable")
	mock := &mockSessionProvider{getErr: expectedErr}
	p := &LocalDockerProvider{sessions: mock}

	_, err := p.Execute(context.Background(), remoteclient.Request{ConversationID: "conv-1"}, func(remoteclient.StreamEvent) {})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected wrapped docker unavailable error, got %v", err)
	}
}

func TestLocalDockerProvider_Close(t *testing.T) {
	mock := &mockSessionProvider{}
	p := &LocalDockerProvider{sessions: mock}

	if err := p.Close(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLocalDockerProvider_Close_propagatesError(t *testing.T) {
	expectedErr := errors.New("stop all failed")
	mock := &mockSessionProvider{stopErr: expectedErr}
	p := &LocalDockerProvider{sessions: mock}

	if err := p.Close(); !errors.Is(err, expectedErr) {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
}
