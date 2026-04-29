package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
