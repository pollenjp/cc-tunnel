package cloudrunsandbox

import (
	"context"
	"strings"
	"testing"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

func TestMockProvider_Execute_returnsFixedResponse(t *testing.T) {
	p := New()

	var events []remoteclient.StreamEvent
	sessionID, err := p.Execute(context.Background(), remoteclient.Request{}, func(e remoteclient.StreamEvent) {
		events = append(events, e)
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	// assistant event
	assistantEvent := events[0]
	if assistantEvent.Type != "assistant" {
		t.Errorf("expected events[0].Type == 'assistant', got %q", assistantEvent.Type)
	}
	if assistantEvent.Message == nil || len(assistantEvent.Message.Content) == 0 {
		t.Fatal("expected events[0].Message.Content to be non-empty")
	}
	if got, want := assistantEvent.Message.Content[0].Text, "This is a mock response from cloud_run_sandbox provider"; got != want {
		t.Errorf("expected Content[0].Text == %q, got %q", want, got)
	}

	// result event
	resultEvent := events[1]
	if resultEvent.Type != "result" {
		t.Errorf("expected events[1].Type == 'result', got %q", resultEvent.Type)
	}
	if resultEvent.Result != "success" {
		t.Errorf("expected events[1].Result == 'success', got %q", resultEvent.Result)
	}

	// session ID
	if sessionID == "" {
		t.Error("expected non-empty sessionID")
	}
	if !strings.HasPrefix(sessionID, "mock-session-") {
		t.Errorf("expected sessionID to have prefix 'mock-session-', got %q", sessionID)
	}
}
