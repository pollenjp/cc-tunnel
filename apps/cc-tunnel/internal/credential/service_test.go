package credential_test

import (
	"context"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/credential"
)

// mockRepo is a test double for credentialRepo.
type mockRepo struct {
	cred        *credential.Credential
	getErr      error
	markInvalid bool
}

func (m *mockRepo) GetByUsername(_ context.Context, _ string) (*credential.Credential, error) {
	return m.cred, m.getErr
}

func (m *mockRepo) MarkInvalid(_ context.Context, _ string) error {
	m.markInvalid = true
	return nil
}

func newTestEncryptor(t *testing.T) *credential.Encryptor {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	enc, err := credential.NewEncryptor(key, 1)
	if err != nil {
		t.Fatal(err)
	}
	return enc
}

func TestCredentialService_FetchAndDecrypt_Success(t *testing.T) {
	enc := newTestEncryptor(t)
	username := "alice"
	plaintext := []byte(`{"access_token":"tok","refresh_token":"ref"}`)

	ct, nonce, err := enc.Seal(plaintext, username)
	if err != nil {
		t.Fatal(err)
	}

	repo := &mockRepo{
		cred: &credential.Credential{
			Username:      username,
			EncryptedData: ct,
			Nonce:         nonce,
			KeyVersion:    1,
			IsValid:       true,
		},
	}
	svc := credential.NewCredentialService(repo, enc)

	got, err := svc.FetchAndDecrypt(context.Background(), username)
	if err != nil {
		t.Fatalf("FetchAndDecrypt: %v", err)
	}
	if string(got) != string(plaintext) {
		t.Errorf("got %q, want %q", got, plaintext)
	}
}

func TestCredentialService_FetchAndDecrypt_NotFound(t *testing.T) {
	enc := newTestEncryptor(t)
	repo := &mockRepo{getErr: credential.ErrNotFound}
	svc := credential.NewCredentialService(repo, enc)

	_, err := svc.FetchAndDecrypt(context.Background(), "alice")
	if !errors.Is(err, credential.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestCredentialService_FetchAndDecrypt_Invalid(t *testing.T) {
	enc := newTestEncryptor(t)
	repo := &mockRepo{
		cred: &credential.Credential{
			Username:      "alice",
			EncryptedData: []byte("x"),
			Nonce:         make([]byte, 12),
			KeyVersion:    1,
			IsValid:       false,
		},
	}
	svc := credential.NewCredentialService(repo, enc)

	_, err := svc.FetchAndDecrypt(context.Background(), "alice")
	if !errors.Is(err, credential.ErrCredentialsInvalid) {
		t.Fatalf("expected ErrCredentialsInvalid, got %v", err)
	}
}

func TestCredentialService_FetchAndDecrypt_KeyVersionMismatch(t *testing.T) {
	enc := newTestEncryptor(t) // keyVersion = 1
	repo := &mockRepo{
		cred: &credential.Credential{
			Username:      "alice",
			EncryptedData: []byte("x"),
			Nonce:         make([]byte, 12),
			KeyVersion:    2, // mismatch with enc's version 1
			IsValid:       true,
		},
	}
	svc := credential.NewCredentialService(repo, enc)

	_, err := svc.FetchAndDecrypt(context.Background(), "alice")
	if !errors.Is(err, credential.ErrKeyVersionMismatch) {
		t.Fatalf("expected ErrKeyVersionMismatch, got %v", err)
	}
}

func TestCredentialService_MarkInvalid(t *testing.T) {
	enc := newTestEncryptor(t)
	repo := &mockRepo{}
	svc := credential.NewCredentialService(repo, enc)

	if err := svc.MarkInvalid(context.Background(), "alice"); err != nil {
		t.Fatalf("MarkInvalid: %v", err)
	}
	if !repo.markInvalid {
		t.Error("expected MarkInvalid to be called on repo")
	}
}
