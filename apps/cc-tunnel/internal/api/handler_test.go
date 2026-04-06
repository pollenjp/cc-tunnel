package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, 200, map[string]string{"key": "value"})

	if w.Code != 200 {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %s", ct)
	}

	var result map[string]string
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result["key"] != "value" {
		t.Fatalf("expected 'value', got %q", result["key"])
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, 400, "bad request")

	if w.Code != 400 {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	var result Error
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.Error != "bad request" {
		t.Fatalf("expected 'bad request', got %q", result.Error)
	}
}

func TestProxyErrorResponse(t *testing.T) {
	w := httptest.NewRecorder()
	body := []byte(`{"error":"upstream error"}`)
	proxyErrorResponse(w, 502, body)

	if w.Code != 502 {
		t.Fatalf("expected status 502, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %s", ct)
	}
	if w.Body.String() != string(body) {
		t.Fatalf("expected body %q, got %q", string(body), w.Body.String())
	}
}
