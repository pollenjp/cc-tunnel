package credential_test

import (
	"crypto/rand"
	"errors"
	"testing"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/credential"
)

func TestEncryptor_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	enc, err := credential.NewEncryptor(key, 1)
	if err != nil {
		t.Fatal(err)
	}

	plaintext := []byte(`{"access_token":"abc","refresh_token":"def"}`)
	username := "alice"

	ct, nonce, err := enc.Seal(plaintext, username)
	if err != nil {
		t.Fatal(err)
	}
	if string(ct) == string(plaintext) {
		t.Fatal("ciphertext equals plaintext")
	}

	out, err := enc.Open(ct, nonce, username)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(plaintext) {
		t.Fatalf("got %q, want %q", out, plaintext)
	}
}

func TestEncryptor_RejectsTampering(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	enc, err := credential.NewEncryptor(key, 1)
	if err != nil {
		t.Fatal(err)
	}

	ct, nonce, err := enc.Seal([]byte("secret data"), "alice")
	if err != nil {
		t.Fatal(err)
	}

	ct[0] ^= 0xff
	_, err = enc.Open(ct, nonce, "alice")
	if !errors.Is(err, credential.ErrDecryptionFailed) {
		t.Fatalf("expected ErrDecryptionFailed, got %v", err)
	}
}

func TestEncryptor_RejectsCrossUserNonce(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	enc, err := credential.NewEncryptor(key, 1)
	if err != nil {
		t.Fatal(err)
	}

	ct, nonce, err := enc.Seal([]byte("alice's credentials"), "alice")
	if err != nil {
		t.Fatal(err)
	}

	_, err = enc.Open(ct, nonce, "bob")
	if !errors.Is(err, credential.ErrDecryptionFailed) {
		t.Fatalf("expected ErrDecryptionFailed when using wrong username, got %v", err)
	}
}

func TestNewEncryptor_RejectsInvalidKeyLength(t *testing.T) {
	_, err := credential.NewEncryptor(make([]byte, 16), 1)
	if !errors.Is(err, credential.ErrInvalidKeyLength) {
		t.Fatalf("got %v, want ErrInvalidKeyLength", err)
	}
}

func TestEncryptor_KeyVersion(t *testing.T) {
	key := make([]byte, 32)
	enc, err := credential.NewEncryptor(key, 42)
	if err != nil {
		t.Fatal(err)
	}
	if enc.KeyVersion() != 42 {
		t.Fatalf("expected key version 42, got %d", enc.KeyVersion())
	}
}
