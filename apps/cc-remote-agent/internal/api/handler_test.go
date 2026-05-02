package api

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pollenjp/cc-tunnel/apps/cc-remote-agent/internal/auth"
)

func TestFinalizeCredentials_NotFound(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	h := &Handler{}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/finalize-credentials", nil)
	h.FinalizeCredentials(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFinalizeCredentials_ReadAndReturn(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	claudeDir := filepath.Join(tmpHome, ".claude")
	if err := os.MkdirAll(claudeDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	credJSON := `{"apiKey":"test-key-abc"}`
	if err := os.WriteFile(filepath.Join(claudeDir, ".credentials.json"), []byte(credJSON), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	h := &Handler{}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/finalize-credentials", nil)
	h.FinalizeCredentials(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "test-key-abc") {
		t.Errorf("expected credentialsJson in body, got %q", w.Body.String())
	}
}

func TestAuthPtyStream_SendsSSEEvents(t *testing.T) {
	authMgr := auth.NewAuthManager()
	h := NewHandler(authMgr)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.AuthPtyStream(w, r)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/auth/pty/stream", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	// Start the SSE connection in background
	respCh := make(chan *http.Response, 1)
	go func() {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Logf("request error (expected on cancel): %v", err)
			return
		}
		respCh <- resp
	}()

	var resp *http.Response
	select {
	case resp = <-respCh:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for SSE response")
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("resp.Body.Close: %v", err)
		}
	}()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	// Push data to subscriber via authMgr
	testData := []byte("hello\x1b[31mANSI\x1b[0m")
	authMgr.BroadcastForTest(testData)

	// Read one SSE event
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			// Got an event - success
			return
		}
	}
	t.Error("no SSE data event received")
}

func TestAuthPtyStream_StatusOK(t *testing.T) {
	authMgr := auth.NewAuthManager()
	h := NewHandler(authMgr)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.AuthPtyStream(w, r)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/auth/pty/stream", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	respCh := make(chan *http.Response, 1)
	go func() {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		respCh <- resp
	}()

	var resp *http.Response
	select {
	case resp = <-respCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for SSE headers")
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("resp.Body.Close: %v", err)
		}
	}()
	cancel()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
}
