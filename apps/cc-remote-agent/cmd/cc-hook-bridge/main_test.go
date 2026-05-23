package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReadWriteState_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	want := &State{
		DispatchID:         "11111111-1111-1111-1111-111111111111",
		AssistantMessageID: "22222222-2222-2222-2222-222222222222",
	}
	if err := writeState(path, want); err != nil {
		t.Fatalf("writeState: %v", err)
	}

	got, err := readState(path)
	if err != nil {
		t.Fatalf("readState: %v", err)
	}
	if got == nil {
		t.Fatal("readState returned nil")
	}
	if got.DispatchID != want.DispatchID || got.AssistantMessageID != want.AssistantMessageID {
		t.Errorf("round-trip mismatch: got %+v want %+v", got, want)
	}
}

func TestReadState_MissingFileIsNil(t *testing.T) {
	dir := t.TempDir()
	got, err := readState(filepath.Join(dir, "no-such-file.json"))
	if err != nil {
		t.Fatalf("readState: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil state for missing file, got %+v", got)
	}
}

func TestWriteState_ModeIs0600(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := writeState(path, &State{DispatchID: "d", AssistantMessageID: "m"}); err != nil {
		t.Fatalf("writeState: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("perm = %o, want 0600", info.Mode().Perm())
	}
}

func TestParsePayload_Empty(t *testing.T) {
	got := parsePayload(nil)
	if len(got) != 0 {
		t.Errorf("empty input should map to empty map, got %v", got)
	}
}

func TestParsePayload_ValidJSON(t *testing.T) {
	got := parsePayload([]byte(`{"session_id":"abc","cwd":"/home/user"}`))
	if got["session_id"] != "abc" {
		t.Errorf("session_id = %v, want abc", got["session_id"])
	}
	if got["cwd"] != "/home/user" {
		t.Errorf("cwd = %v", got["cwd"])
	}
}

func TestParsePayload_InvalidFallsBackToRaw(t *testing.T) {
	got := parsePayload([]byte("not json"))
	if got["raw"] != "not json" {
		t.Errorf("expected raw fallback, got %+v", got)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	t.Setenv("CC_HOOK_BRIDGE_DATABASE_URL", "postgres://x")
	t.Setenv("CC_HOOK_BRIDGE_CONVERSATION_ID", "abc")
	t.Setenv("CC_HOOK_BRIDGE_STATE_FILE", "")
	t.Setenv("CC_HOOK_BRIDGE_STOP_TIMEOUT_SEC", "")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.StateFile != "/tmp/cc-hook-bridge-state.json" {
		t.Errorf("default state file = %q", cfg.StateFile)
	}
	if cfg.StopTimeout.Seconds() != 55 {
		t.Errorf("default stop timeout = %v", cfg.StopTimeout)
	}
}

func TestLoadConfig_MissingRequired(t *testing.T) {
	t.Setenv("CC_HOOK_BRIDGE_DATABASE_URL", "")
	t.Setenv("CC_HOOK_BRIDGE_CONVERSATION_ID", "x")
	if _, err := loadConfig(); err == nil {
		t.Error("expected error when DATABASE_URL is empty")
	}

	t.Setenv("CC_HOOK_BRIDGE_DATABASE_URL", "postgres://x")
	t.Setenv("CC_HOOK_BRIDGE_CONVERSATION_ID", "")
	if _, err := loadConfig(); err == nil {
		t.Error("expected error when CONVERSATION_ID is empty")
	}
}

func TestLoadConfig_BadTimeout(t *testing.T) {
	t.Setenv("CC_HOOK_BRIDGE_DATABASE_URL", "postgres://x")
	t.Setenv("CC_HOOK_BRIDGE_CONVERSATION_ID", "abc")
	t.Setenv("CC_HOOK_BRIDGE_STOP_TIMEOUT_SEC", "not-a-number")
	if _, err := loadConfig(); err == nil {
		t.Error("expected error for non-numeric timeout")
	}
}

// TestState_JSONShape locks down the JSON field names. Other hooks
// depend on this serialization, so changing it is a contract break.
func TestState_JSONShape(t *testing.T) {
	st := &State{DispatchID: "d", AssistantMessageID: "m"}
	buf, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"dispatch_id":"d","assistant_message_id":"m"}`
	if string(buf) != want {
		t.Errorf("JSON = %s, want %s", buf, want)
	}
}
