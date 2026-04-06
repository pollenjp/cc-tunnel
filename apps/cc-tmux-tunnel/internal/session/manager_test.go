package session

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if len(m.sessions) != 0 {
		t.Fatalf("expected empty sessions, got %d", len(m.sessions))
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()
	if id1 == id2 {
		t.Fatal("expected unique IDs")
	}
	if len(id1) != 16 {
		t.Fatalf("expected 16 hex chars, got %d: %q", len(id1), id1)
	}
}

func TestManagerGetNotFound(t *testing.T) {
	m := NewManager()
	_, ok := m.Get("nonexistent")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestManagerListEmpty(t *testing.T) {
	m := NewManager()
	list := m.List()
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %d", len(list))
	}
}

func TestResolvePaneTargetOutOfRange(t *testing.T) {
	m := NewManager()
	s := &Session{Type: SessionTypeClaudeCode, PaneCount: 1}

	_, _, err := m.resolvePaneTarget(s, 1)
	if err == nil {
		t.Fatal("expected error for out of range pane index")
	}

	_, _, err = m.resolvePaneTarget(s, -1)
	if err == nil {
		t.Fatal("expected error for negative pane index")
	}
}

func TestResolvePaneTargetClaudeCode(t *testing.T) {
	m := NewManager()
	s := &Session{
		Type:      SessionTypeClaudeCode,
		TmuxName:  "test-session",
		PaneCount: 1,
	}

	name, idx, err := m.resolvePaneTarget(s, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "test-session" {
		t.Fatalf("expected session name 'test-session', got %q", name)
	}
	if idx != 0 {
		t.Fatalf("expected pane index 0, got %d", idx)
	}
}

func TestResolvePaneTargetMultiAgentShogun(t *testing.T) {
	m := NewManager()
	s := &Session{
		Type:               SessionTypeMultiAgentShogun,
		TmuxName:           "shogun",
		MultiagentTmuxName: "multiagent",
		PaneCount:          10,
	}

	// Pane 0 -> shogun session, pane 0
	name, idx, err := m.resolvePaneTarget(s, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "shogun" || idx != 0 {
		t.Fatalf("expected (shogun, 0), got (%s, %d)", name, idx)
	}

	// Pane 1 -> multiagent session, pane 0
	name, idx, err = m.resolvePaneTarget(s, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "multiagent" || idx != 0 {
		t.Fatalf("expected (multiagent, 0), got (%s, %d)", name, idx)
	}

	// Pane 5 -> multiagent session, pane 4
	name, idx, err = m.resolvePaneTarget(s, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "multiagent" || idx != 4 {
		t.Fatalf("expected (multiagent, 4), got (%s, %d)", name, idx)
	}

	// Pane 9 -> multiagent session, pane 8
	name, idx, err = m.resolvePaneTarget(s, 9)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "multiagent" || idx != 8 {
		t.Fatalf("expected (multiagent, 8), got (%s, %d)", name, idx)
	}

	// Pane 10 -> out of range
	_, _, err = m.resolvePaneTarget(s, 10)
	if err == nil {
		t.Fatal("expected error for pane index 10")
	}
}
