package remoteclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFinalizeCredentials_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/finalize-credentials" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"credentialsJson": `{"apiKey":"abc123"}`})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	got, err := c.FinalizeCredentials(context.Background())
	if err != nil {
		t.Fatalf("FinalizeCredentials: %v", err)
	}
	// The encoded JSON includes a trailing newline from json.Encoder; trim for comparison.
	if got != `{"apiKey":"abc123"}` {
		t.Errorf("got %q, want %q", got, `{"apiKey":"abc123"}`)
	}
}

func TestFinalizeCredentials_NotReady(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.FinalizeCredentials(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrCredentialsNotReady) {
		t.Errorf("expected ErrCredentialsNotReady, got %v", err)
	}
}

func TestFinalizeCredentials_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.FinalizeCredentials(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, ErrCredentialsNotReady) {
		t.Errorf("did not expect ErrCredentialsNotReady for 500 response")
	}
}
