package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/credential"
)

// TestGetCredentialsStatus_Success_Returns200WithRegisteredValid verifies that
// GetCredentialsStatus returns 200 with registered=true and isValid=true when
// credentials are found and valid.
func TestGetCredentialsStatus_Success_Returns200WithRegisteredValid(t *testing.T) {
	credSvc := &mockCredService{credJSON: []byte(`{"api_key":"valid"}`)}
	server := &Server{
		session:     newAppSession(),
		credService: credSvc,
	}
	token := "test-token-status-ok"
	server.session.set(token, AppUser{Id: uuid.New().String(), Name: "alice"})

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/credentials/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	server.GetCredentialsStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"registered":true`) {
		t.Errorf("expected registered:true in body, got %q", body)
	}
	if !strings.Contains(body, `"isValid":true`) {
		t.Errorf("expected isValid:true in body, got %q", body)
	}
}

// TestGetCredentialsStatus_NotFound_ReturnsRegisteredFalse verifies that
// GetCredentialsStatus returns 200 with registered=false and isValid=false
// when credentials are not found.
func TestGetCredentialsStatus_NotFound_ReturnsRegisteredFalse(t *testing.T) {
	credSvc := &mockCredService{err: credential.ErrNotFound}
	server := &Server{
		session:     newAppSession(),
		credService: credSvc,
	}
	token := "test-token-status-notfound"
	server.session.set(token, AppUser{Id: uuid.New().String(), Name: "bob"})

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/credentials/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	server.GetCredentialsStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"registered":false`) {
		t.Errorf("expected registered:false in body, got %q", body)
	}
	if !strings.Contains(body, `"isValid":false`) {
		t.Errorf("expected isValid:false in body, got %q", body)
	}
}

// TestGetCredentialsStatus_Invalid_ReturnsIsValidFalse verifies that
// GetCredentialsStatus returns 200 with registered=true and isValid=false
// when credentials exist but are invalid.
func TestGetCredentialsStatus_Invalid_ReturnsIsValidFalse(t *testing.T) {
	credSvc := &mockCredService{err: credential.ErrCredentialsInvalid}
	server := &Server{
		session:     newAppSession(),
		credService: credSvc,
	}
	token := "test-token-status-invalid"
	server.session.set(token, AppUser{Id: uuid.New().String(), Name: "carol"})

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/credentials/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	server.GetCredentialsStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"registered":true`) {
		t.Errorf("expected registered:true in body, got %q", body)
	}
	if !strings.Contains(body, `"isValid":false`) {
		t.Errorf("expected isValid:false in body, got %q", body)
	}
}

// TestGetCredentialsStatus_NoCredService_ReturnsRegisteredTrue verifies that
// GetCredentialsStatus returns 200 with registered=true and isValid=true
// when credService is nil (skip mode / no-auth mode).
func TestGetCredentialsStatus_NoCredService_ReturnsRegisteredTrue(t *testing.T) {
	server := &Server{
		session: newAppSession(),
		// credService is nil
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/credentials/status", nil)

	server.GetCredentialsStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"registered":true`) {
		t.Errorf("expected registered:true in body, got %q", body)
	}
	if !strings.Contains(body, `"isValid":true`) {
		t.Errorf("expected isValid:true in body, got %q", body)
	}
}
