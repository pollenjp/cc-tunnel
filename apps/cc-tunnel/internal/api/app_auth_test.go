package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestServer() *Server {
	return &Server{session: newAppSession()}
}

func TestAppAuthLogin_success(t *testing.T) {
	s := newTestServer()
	body, _ := json.Marshal(AppAuthLoginRequest{Username: "alice"})
	req := httptest.NewRequest(http.MethodPost, "/app-auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.AppAuthLogin(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp AppAuthLoginResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Token == "" {
		t.Error("token should not be empty")
	}
	if resp.User.Name != "alice" {
		t.Errorf("user name = %q, want %q", resp.User.Name, "alice")
	}
	if resp.User.Id == "" {
		t.Error("user id should not be empty")
	}
}

func TestAppAuthLogin_emptyUsername(t *testing.T) {
	s := newTestServer()
	body, _ := json.Marshal(AppAuthLoginRequest{Username: ""})
	req := httptest.NewRequest(http.MethodPost, "/app-auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.AppAuthLogin(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAppAuthGetMe_success(t *testing.T) {
	s := newTestServer()

	// First login to get a valid token
	loginBody, _ := json.Marshal(AppAuthLoginRequest{Username: "bob"})
	loginReq := httptest.NewRequest(http.MethodPost, "/app-auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	s.AppAuthLogin(loginW, loginReq)

	var loginResp AppAuthLoginResponse
	if err := json.NewDecoder(loginW.Body).Decode(&loginResp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}

	// Now call GetMe
	meReq := httptest.NewRequest(http.MethodGet, "/app-auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+loginResp.Token)
	meW := httptest.NewRecorder()
	s.AppAuthGetMe(meW, meReq)

	if meW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", meW.Code, meW.Body.String())
	}
	var meResp AppAuthMeResponse
	if err := json.NewDecoder(meW.Body).Decode(&meResp); err != nil {
		t.Fatalf("failed to decode me response: %v", err)
	}
	if meResp.User.Name != "bob" {
		t.Errorf("user name = %q, want %q", meResp.User.Name, "bob")
	}
}

func TestAppAuthGetMe_unauthorized(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/app-auth/me", nil)
	req.Header.Set("Authorization", "Bearer invalidtoken")
	w := httptest.NewRecorder()

	s.AppAuthGetMe(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
	var errResp AppAuthError
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Message == "" {
		t.Error("error message should not be empty")
	}
}

func TestAppAuthLogout_success(t *testing.T) {
	s := newTestServer()

	// Login first
	loginBody, _ := json.Marshal(AppAuthLoginRequest{Username: "charlie"})
	loginReq := httptest.NewRequest(http.MethodPost, "/app-auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	s.AppAuthLogin(loginW, loginReq)

	var loginResp AppAuthLoginResponse
	if err := json.NewDecoder(loginW.Body).Decode(&loginResp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}

	// Logout
	logoutReq := httptest.NewRequest(http.MethodPost, "/app-auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+loginResp.Token)
	logoutW := httptest.NewRecorder()
	s.AppAuthLogout(logoutW, logoutReq)

	if logoutW.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", logoutW.Code, logoutW.Body.String())
	}

	// Token should now be invalid
	meReq := httptest.NewRequest(http.MethodGet, "/app-auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+loginResp.Token)
	meW := httptest.NewRecorder()
	s.AppAuthGetMe(meW, meReq)
	if meW.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 after logout, got %d", meW.Code)
	}
}

func TestAppAuthUpdateMe_success(t *testing.T) {
	s := newTestServer()

	// Login first
	loginBody, _ := json.Marshal(AppAuthLoginRequest{Username: "dave"})
	loginReq := httptest.NewRequest(http.MethodPost, "/app-auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	s.AppAuthLogin(loginW, loginReq)

	var loginResp AppAuthLoginResponse
	if err := json.NewDecoder(loginW.Body).Decode(&loginResp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}

	// Update nickname
	updateBody, _ := json.Marshal(AppAuthUpdateMeRequest{Nickname: "dave-new"})
	updateReq := httptest.NewRequest(http.MethodPatch, "/app-auth/me", bytes.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Authorization", "Bearer "+loginResp.Token)
	updateW := httptest.NewRecorder()
	s.AppAuthUpdateMe(updateW, updateReq)

	if updateW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", updateW.Code, updateW.Body.String())
	}
	var updateResp AppAuthMeResponse
	if err := json.NewDecoder(updateW.Body).Decode(&updateResp); err != nil {
		t.Fatalf("failed to decode update response: %v", err)
	}
	if updateResp.User.Name != "dave-new" {
		t.Errorf("user name = %q, want %q", updateResp.User.Name, "dave-new")
	}
}

func TestAppAuthUpdateMe_unauthorized(t *testing.T) {
	s := newTestServer()
	updateBody, _ := json.Marshal(AppAuthUpdateMeRequest{Nickname: "hacker"})
	req := httptest.NewRequest(http.MethodPatch, "/app-auth/me", bytes.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer badtoken")
	w := httptest.NewRecorder()

	s.AppAuthUpdateMe(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
	var errResp AppAuthError
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Message == "" {
		t.Error("error message should not be empty")
	}
}

// TestAppAuthGetMe_noAuthHeader verifies 401 when Authorization header is missing.
func TestAppAuthGetMe_noAuthHeader(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/app-auth/me", nil)
	w := httptest.NewRecorder()
	s.AppAuthGetMe(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
