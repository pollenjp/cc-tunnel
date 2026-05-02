package api

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pollenjp/cc-tunnel/apps/cc-remote-agent/internal/auth"
)

// TestAuthSSEPtyStream_E2E_ANSINotStripped verifies that PTY output containing ANSI escape
// sequences is delivered via SSE as base64 without stripping the ANSI codes.
func TestAuthSSEPtyStream_E2E_ANSINotStripped(t *testing.T) {
	authMgr := auth.NewAuthManager()
	h := NewHandler(authMgr)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.AuthPtyStream(w, r)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/auth/pty/stream", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	// Connect to SSE endpoint
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
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for SSE response")
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("resp.Body.Close: %v", err)
		}
	}()

	// Verify SSE headers
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	// Send ANSI-containing data via fan-out
	ansiData := []byte("\x1b[31mred text\x1b[0m \x1b[32mgreen\x1b[0m normal")
	authMgr.BroadcastForTest(ansiData)

	// Read SSE event and verify ANSI is NOT stripped (preserved in base64)
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		encoded := strings.TrimPrefix(line, "data: ")
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatalf("base64 decode failed: %v (encoded=%q)", err, encoded)
		}
		got := string(decoded)
		// ANSI escape sequences must be present (not stripped)
		if !strings.Contains(got, "\x1b[31m") {
			t.Errorf("ANSI escape \\x1b[31m not found in decoded data; got %q", got)
		}
		if !strings.Contains(got, "\x1b[0m") {
			t.Errorf("ANSI reset \\x1b[0m not found in decoded data; got %q", got)
		}
		// Actual text content must be intact
		if !strings.Contains(got, "red text") {
			t.Errorf("text content 'red text' not found; got %q", got)
		}
		return // success
	}
	t.Error("no SSE data event received")
}

// authPtyStreamWithHeartbeat is a test-only SSE handler that uses a configurable heartbeat
// interval so that the heartbeat behaviour can be tested without waiting 30 seconds.
func authPtyStreamWithHeartbeat(authMgr *auth.AuthManager, heartbeatInterval time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		ch := authMgr.Subscribe(r.Context())
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case data, ok := <-ch:
				if !ok {
					return
				}
				encoded := base64.StdEncoding.EncodeToString(data)
				if _, err := fmt.Fprintf(w, "data: %s\n\n", encoded); err != nil {
					return
				}
				flusher.Flush()
			case <-ticker.C:
				if _, err := fmt.Fprintf(w, ": keepalive\n\n"); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	}
}

// TestAuthSSEPtyStream_E2E_HeartbeatSent verifies that keepalive comments are sent
// periodically over the SSE stream even when no PTY data arrives.
func TestAuthSSEPtyStream_E2E_HeartbeatSent(t *testing.T) {
	authMgr := auth.NewAuthManager()

	// Use 100ms heartbeat interval to avoid waiting 30 seconds.
	srv := httptest.NewServer(authPtyStreamWithHeartbeat(authMgr, 100*time.Millisecond))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/auth/pty/stream", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

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
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for SSE response")
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("resp.Body.Close: %v", err)
		}
	}()

	// Read lines and look for a ": keepalive" comment within reasonable time.
	type result struct {
		found bool
		line  string
	}
	resultCh := make(chan result, 1)
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, ": keepalive") {
				resultCh <- result{found: true, line: line}
				return
			}
		}
		resultCh <- result{found: false}
	}()

	select {
	case r := <-resultCh:
		if !r.found {
			t.Error("no keepalive heartbeat received")
		}
		// success: keepalive comment arrived
	case <-time.After(3 * time.Second):
		t.Error("timeout: no keepalive heartbeat received within 3 seconds")
	}
}
