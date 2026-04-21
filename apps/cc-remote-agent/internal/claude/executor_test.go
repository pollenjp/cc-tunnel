package claude

import (
	"strings"
	"testing"
)

// --- buildArgs ---

func TestBuildArgs_baseFlags(t *testing.T) {
	req := ExecuteRequest{Prompt: "hello"}
	args := buildArgs(req, false)

	// must contain -p, --output-format=stream-json, --verbose
	mustContain(t, args, "-p")
	mustContain(t, args, "--output-format=stream-json")
	mustContain(t, args, "--verbose")
	mustContain(t, args, "--dangerously-skip-permissions")
}

func TestBuildArgs_withSessionID_includesResume(t *testing.T) {
	req := ExecuteRequest{Prompt: "hello", SessionID: "abc123"}
	args := buildArgs(req, false)

	if !containsSeq(args, "--resume", "abc123") {
		t.Errorf("expected --resume abc123 in args, got: %v", args)
	}
}

func TestBuildArgs_isFallback_omitsResume(t *testing.T) {
	req := ExecuteRequest{Prompt: "hello", SessionID: "abc123"}
	args := buildArgs(req, true)

	for _, arg := range args {
		if arg == "--resume" {
			t.Errorf("fallback build should not include --resume, got: %v", args)
			return
		}
	}
}

func TestBuildArgs_withoutSessionID_noResume(t *testing.T) {
	req := ExecuteRequest{Prompt: "hello"}
	args := buildArgs(req, false)

	for _, arg := range args {
		if arg == "--resume" {
			t.Errorf("should not include --resume when no session ID, got: %v", args)
			return
		}
	}
}

func TestBuildArgs_model_included(t *testing.T) {
	req := ExecuteRequest{Prompt: "hello", Model: "claude-opus-4-6"}
	args := buildArgs(req, false)

	if !containsSeq(args, "--model", "claude-opus-4-6") {
		t.Errorf("expected --model claude-opus-4-6 in args, got: %v", args)
	}
}

func TestBuildArgs_emptyModel_omitted(t *testing.T) {
	req := ExecuteRequest{Prompt: "hello"}
	args := buildArgs(req, false)

	for _, arg := range args {
		if arg == "--model" {
			t.Errorf("should not include --model when empty, got: %v", args)
			return
		}
	}
}

func TestBuildArgs_systemPrompt_included(t *testing.T) {
	req := ExecuteRequest{Prompt: "hello", SystemPrompt: "You are helpful."}
	args := buildArgs(req, false)

	if !containsSeq(args, "--system-prompt", "You are helpful.") {
		t.Errorf("expected --system-prompt in args, got: %v", args)
	}
}

func TestBuildArgs_allowedTools_eachToolHasFlag(t *testing.T) {
	req := ExecuteRequest{
		Prompt:       "hello",
		AllowedTools: []string{"Bash", "Read", "Write"},
	}
	args := buildArgs(req, false)

	for _, tool := range []string{"Bash", "Read", "Write"} {
		if !containsSeq(args, "--allowedTools", tool) {
			t.Errorf("expected --allowedTools %s in args, got: %v", tool, args)
		}
	}
}

func TestBuildArgs_permissionMode_included(t *testing.T) {
	req := ExecuteRequest{Prompt: "hello", PermissionMode: "bypassPermissions"}
	args := buildArgs(req, false)

	if !containsSeq(args, "--permission-mode", "bypassPermissions") {
		t.Errorf("expected --permission-mode bypassPermissions in args, got: %v", args)
	}
}

func TestBuildArgs_includePartialMessages_flag(t *testing.T) {
	req := ExecuteRequest{Prompt: "hello", IncludePartialMessages: true}
	args := buildArgs(req, false)

	mustContain(t, args, "--include-partial-messages")
}

func TestBuildArgs_includePartialMessages_false_omitted(t *testing.T) {
	req := ExecuteRequest{Prompt: "hello", IncludePartialMessages: false}
	args := buildArgs(req, false)

	for _, arg := range args {
		if arg == "--include-partial-messages" {
			t.Errorf("should not include --include-partial-messages when false, got: %v", args)
			return
		}
	}
}

func TestBuildArgs_includeHookEvents_flag(t *testing.T) {
	req := ExecuteRequest{Prompt: "hello", IncludeHookEvents: true}
	args := buildArgs(req, false)

	mustContain(t, args, "--include-hook-events")
}

func TestBuildArgs_promptIsLastAfterDoubleDash(t *testing.T) {
	req := ExecuteRequest{Prompt: "my prompt text"}
	args := buildArgs(req, false)

	n := len(args)
	if n < 2 {
		t.Fatalf("too few args: %v", args)
	}
	if args[n-1] != "my prompt text" {
		t.Errorf("last arg should be prompt, got %q", args[n-1])
	}
	if args[n-2] != "--" {
		t.Errorf("second-to-last arg should be --, got %q", args[n-2])
	}
}

// --- buildFallbackPrompt ---

func TestBuildFallbackPrompt_includesUserMessages(t *testing.T) {
	history := []ConversationMsg{
		{Role: "user", Content: "first question"},
	}
	prompt := buildFallbackPrompt(history, "follow up")

	if !strings.Contains(prompt, "[User]: first question") {
		t.Errorf("expected user message in fallback prompt, got:\n%s", prompt)
	}
}

func TestBuildFallbackPrompt_includesAssistantMessages(t *testing.T) {
	history := []ConversationMsg{
		{Role: "assistant", Content: "my reply"},
	}
	prompt := buildFallbackPrompt(history, "next")

	if !strings.Contains(prompt, "[Assistant]: my reply") {
		t.Errorf("expected assistant message in fallback prompt, got:\n%s", prompt)
	}
}

func TestBuildFallbackPrompt_includesSystemMessages(t *testing.T) {
	history := []ConversationMsg{
		{Role: "system", Content: "system instruction"},
	}
	prompt := buildFallbackPrompt(history, "question")

	if !strings.Contains(prompt, "[System]: system instruction") {
		t.Errorf("expected system message in fallback prompt, got:\n%s", prompt)
	}
}

func TestBuildFallbackPrompt_includesCurrentPrompt(t *testing.T) {
	prompt := buildFallbackPrompt(nil, "the current question")

	if !strings.Contains(prompt, "the current question") {
		t.Errorf("expected current prompt in fallback, got:\n%s", prompt)
	}
}

func TestBuildFallbackPrompt_conversationOrderPreserved(t *testing.T) {
	history := []ConversationMsg{
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "second"},
		{Role: "user", Content: "third"},
	}
	prompt := buildFallbackPrompt(history, "current")

	firstIdx := strings.Index(prompt, "first")
	secondIdx := strings.Index(prompt, "second")
	thirdIdx := strings.Index(prompt, "third")

	if firstIdx > secondIdx || secondIdx > thirdIdx {
		t.Errorf("messages not in order: first=%d, second=%d, third=%d", firstIdx, secondIdx, thirdIdx)
	}
}

// --- containsAny ---

func TestContainsAny(t *testing.T) {
	tests := []struct {
		name string
		s    string
		subs []string
		want bool
	}{
		{"exact match", "session not found", []string{"session not found"}, true},
		{"substring match", "error: resume failed here", []string{"resume failed"}, true},
		{"no match", "some other error", []string{"session not found", "resume failed"}, false},
		{"empty string", "", []string{"session not found"}, false},
		{"empty substrings list", "anything", []string{}, false},
		{"multiple subs one matches", "invalid session id", []string{"session not found", "invalid session"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsAny(tt.s, tt.subs)
			if got != tt.want {
				t.Errorf("containsAny(%q, %v) = %v, want %v", tt.s, tt.subs, got, tt.want)
			}
		})
	}
}

// --- containsResumeError ---

func TestContainsResumeError(t *testing.T) {
	tests := []struct {
		name string
		line []byte
		want bool
	}{
		{"session not found", []byte(`{"type":"error","message":"session not found"}`), true},
		{"resume failed", []byte(`{"type":"error","message":"resume failed unexpectedly"}`), true},
		{"invalid session", []byte(`{"type":"error","message":"invalid session id"}`), true},
		{"normal result", []byte(`{"type":"result","result":"success"}`), false},
		{"assistant message", []byte(`{"type":"assistant"}`), false},
		{"empty line", []byte{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsResumeError(tt.line)
			if got != tt.want {
				t.Errorf("containsResumeError(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

// helpers

func mustContain(t *testing.T, args []string, want string) {
	t.Helper()
	for _, arg := range args {
		if arg == want {
			return
		}
	}
	t.Errorf("expected %q in args, got: %v", want, args)
}

// containsSeq returns true if args contains flag immediately followed by val.
func containsSeq(args []string, flag, val string) bool {
	for i, arg := range args {
		if arg == flag && i+1 < len(args) && args[i+1] == val {
			return true
		}
	}
	return false
}
