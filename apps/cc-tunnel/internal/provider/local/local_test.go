package local_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider/local"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

// TestProvider_Execute_delegatesToClient verifies that local.Provider.Execute
// delegates to the underlying remoteclient.Client.Execute by standing up a
// fake cc-remote-agent HTTP server and asserting that the events it returns
// are forwarded to the onEvent callback and that the session_id is returned.
func TestProvider_Execute_delegatesToClient(t *testing.T) {
	assistantEvent := remoteclient.StreamEvent{
		Type: "assistant",
		Message: &struct {
			Content []remoteclient.ContentBlock `json:"content"`
		}{
			Content: []remoteclient.ContentBlock{
				{Type: "text", Text: "hello from fake agent"},
			},
		},
	}
	resultEvent := remoteclient.StreamEvent{
		Type:      "result",
		SessionID: "sess-local-test",
	}

	assistantLine, err := json.Marshal(assistantEvent)
	if err != nil {
		t.Fatalf("json.Marshal assistantEvent: %v", err)
	}
	resultLine, err := json.Marshal(resultEvent)
	if err != nil {
		t.Fatalf("json.Marshal resultEvent: %v", err)
	}
	ndjson := string(assistantLine) + "\n" + string(resultLine) + "\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(ndjson)); err != nil {
			t.Errorf("fake server write: %v", err)
		}
	}))
	defer srv.Close()

	client := remoteclient.NewClient(srv.URL)
	p := local.New(client)

	var received []remoteclient.StreamEvent
	sessionID, err := p.Execute(
		context.Background(),
		remoteclient.Request{Prompt: "test prompt"},
		func(e remoteclient.StreamEvent) {
			received = append(received, e)
		},
	)

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if sessionID != "sess-local-test" {
		t.Errorf("sessionID = %q, want %q", sessionID, "sess-local-test")
	}
	if len(received) != 2 {
		t.Fatalf("received %d events, want 2", len(received))
	}
	if received[0].Type != "assistant" {
		t.Errorf("received[0].Type = %q, want %q", received[0].Type, "assistant")
	}
	if received[1].Type != "result" {
		t.Errorf("received[1].Type = %q, want %q", received[1].Type, "result")
	}
}
