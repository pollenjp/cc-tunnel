package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pollenjp/cc-tunnel/apps/cc-login/internal/api"
	"github.com/pollenjp/cc-tunnel/apps/cc-login/internal/auth"
	"github.com/pollenjp/cc-tunnel/apps/cc-login/internal/credential"
)

// --- Mocks ---

type mockPTYManager struct {
	startLoginFn       func(ctx context.Context, method string) (auth.LoginResponse, error)
	getOutputFn        func(since int) (string, int, auth.LoginStatus)
	submitInputFn      func(input string) error
	cancelFn           func()
	readCredentialsFn  func() ([]byte, error)
	clearLocalCredFn   func() error
	statusFn           func() auth.LoginStatus
}

func (m *mockPTYManager) StartLogin(ctx context.Context, method string) (auth.LoginResponse, error) {
	return m.startLoginFn(ctx, method)
}
func (m *mockPTYManager) GetOutput(since int) (string, int, auth.LoginStatus) {
	return m.getOutputFn(since)
}
func (m *mockPTYManager) SubmitInput(input string) error { return m.submitInputFn(input) }
func (m *mockPTYManager) Cancel()                         { m.cancelFn() }
func (m *mockPTYManager) ReadCredentials() ([]byte, error) {
	return m.readCredentialsFn()
}
func (m *mockPTYManager) ClearLocalCredentials() error { return m.clearLocalCredFn() }
func (m *mockPTYManager) Status() auth.LoginStatus     { return m.statusFn() }

type mockCredentialStore struct {
	upsertFn              func(ctx context.Context, c *credential.Credential) error
	getByUsernameFn       func(ctx context.Context, username string) (*credential.Credential, error)
	markInvalidFn         func(ctx context.Context, username string) error
	deleteFn              func(ctx context.Context, username string) error
	updateLastValidatedFn func(ctx context.Context, username string) error
}

func (m *mockCredentialStore) Upsert(ctx context.Context, c *credential.Credential) error {
	return m.upsertFn(ctx, c)
}
func (m *mockCredentialStore) GetByUsername(ctx context.Context, username string) (*credential.Credential, error) {
	return m.getByUsernameFn(ctx, username)
}
func (m *mockCredentialStore) MarkInvalid(ctx context.Context, username string) error {
	return m.markInvalidFn(ctx, username)
}
func (m *mockCredentialStore) Delete(ctx context.Context, username string) error {
	return m.deleteFn(ctx, username)
}
func (m *mockCredentialStore) UpdateLastValidated(ctx context.Context, username string) error {
	return m.updateLastValidatedFn(ctx, username)
}

type mockEncryptor struct {
	sealFn func(plaintext []byte, username string) (ciphertext, nonce []byte, err error)
	openFn func(ciphertext, nonce []byte, username string) ([]byte, error)
}

func (m *mockEncryptor) Seal(plaintext []byte, username string) ([]byte, []byte, error) {
	return m.sealFn(plaintext, username)
}
func (m *mockEncryptor) Open(ciphertext, nonce []byte, username string) ([]byte, error) {
	return m.openFn(ciphertext, nonce, username)
}
func (m *mockEncryptor) KeyVersion() int { return 1 }

// mockTokenResolver always returns the given username.
type mockTokenResolver struct {
	username string
	err      error
}

func (m *mockTokenResolver) ResolveUsername(r *http.Request) (string, error) {
	return m.username, m.err
}

// --- Tests ---

func TestHandler_PostCredentialsLogin(t *testing.T) {
	ptyMgr := &mockPTYManager{
		startLoginFn: func(_ context.Context, method string) (auth.LoginResponse, error) {
			return auth.LoginResponse{Message: "Login started"}, nil
		},
	}
	credStore := &mockCredentialStore{}
	enc := &mockEncryptor{}
	resolver := &mockTokenResolver{username: "alice"}

	h := api.NewHandler(ptyMgr, credStore, enc, resolver)

	req := httptest.NewRequest(http.MethodPost, "/credentials/login", nil)
	rec := httptest.NewRecorder()
	h.PostCredentialsLogin(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["message"] != "Login started" {
		t.Errorf("unexpected message: %v", resp["message"])
	}
}

func TestHandler_PostCredentialsInput_NoLogin(t *testing.T) {
	ptyMgr := &mockPTYManager{
		submitInputFn: func(input string) error {
			return errors.New("no login in progress")
		},
	}
	credStore := &mockCredentialStore{}
	enc := &mockEncryptor{}
	resolver := &mockTokenResolver{username: "alice"}

	h := api.NewHandler(ptyMgr, credStore, enc, resolver)

	body := bytes.NewBufferString(`{"input":"hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/credentials/input", body)
	rec := httptest.NewRecorder()
	h.PostCredentialsInput(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestHandler_GetCredentialsOutput(t *testing.T) {
	ptyMgr := &mockPTYManager{
		getOutputFn: func(since int) (string, int, auth.LoginStatus) {
			return "aGVsbG8=", 5, auth.StatusPending
		},
	}
	credStore := &mockCredentialStore{}
	enc := &mockEncryptor{}
	resolver := &mockTokenResolver{username: "alice"}

	h := api.NewHandler(ptyMgr, credStore, enc, resolver)

	req := httptest.NewRequest(http.MethodGet, "/credentials/output?since=0", nil)
	rec := httptest.NewRecorder()
	h.GetCredentialsOutput(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["data"] != "aGVsbG8=" {
		t.Errorf("unexpected data: %v", resp["data"])
	}
	if resp["status"] != string(auth.StatusPending) {
		t.Errorf("unexpected status: %v", resp["status"])
	}
}

func TestHandler_PostCredentialsFinalize(t *testing.T) {
	ptyMgr := &mockPTYManager{
		readCredentialsFn: func() ([]byte, error) {
			return []byte(`{"access_token":"tok"}`), nil
		},
		clearLocalCredFn: func() error { return nil },
	}
	upsertCalled := false
	credStore := &mockCredentialStore{
		upsertFn: func(_ context.Context, c *credential.Credential) error {
			upsertCalled = true
			if c.Username != "alice" {
				return errors.New("unexpected username")
			}
			return nil
		},
	}
	enc := &mockEncryptor{
		sealFn: func(plaintext []byte, username string) ([]byte, []byte, error) {
			return []byte("ct"), []byte("nonce"), nil
		},
	}
	resolver := &mockTokenResolver{username: "alice"}

	h := api.NewHandler(ptyMgr, credStore, enc, resolver)

	req := httptest.NewRequest(http.MethodPost, "/credentials/finalize", nil)
	rec := httptest.NewRecorder()
	h.PostCredentialsFinalize(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !upsertCalled {
		t.Error("expected Upsert to be called")
	}
}

func TestHandler_GetCredentialsStatus_NotRegistered(t *testing.T) {
	ptyMgr := &mockPTYManager{}
	credStore := &mockCredentialStore{
		getByUsernameFn: func(_ context.Context, username string) (*credential.Credential, error) {
			return nil, credential.ErrNotFound
		},
	}
	enc := &mockEncryptor{}
	resolver := &mockTokenResolver{username: "alice"}

	h := api.NewHandler(ptyMgr, credStore, enc, resolver)

	req := httptest.NewRequest(http.MethodGet, "/credentials/status", nil)
	rec := httptest.NewRecorder()
	h.GetCredentialsStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["registered"] != false {
		t.Errorf("expected registered=false, got %v", resp["registered"])
	}
}

func TestHandler_DeleteCredentials(t *testing.T) {
	deleted := false
	credStore := &mockCredentialStore{
		deleteFn: func(_ context.Context, username string) error {
			if username == "alice" {
				deleted = true
			}
			return nil
		},
	}
	enc := &mockEncryptor{}
	resolver := &mockTokenResolver{username: "alice"}

	h := api.NewHandler(&mockPTYManager{}, credStore, enc, resolver)

	req := httptest.NewRequest(http.MethodDelete, "/credentials", nil)
	rec := httptest.NewRecorder()
	h.DeleteCredentials(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !deleted {
		t.Error("expected Delete to be called with 'alice'")
	}
}

func TestHandler_Unauthorized(t *testing.T) {
	resolver := &mockTokenResolver{err: api.ErrUnauthorized}
	h := api.NewHandler(&mockPTYManager{}, &mockCredentialStore{}, &mockEncryptor{}, resolver)

	req := httptest.NewRequest(http.MethodPost, "/credentials/login", nil)
	rec := httptest.NewRecorder()
	h.PostCredentialsLogin(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}
