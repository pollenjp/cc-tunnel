package credential_test

import (
	"context"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/credential"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func setupE2ETestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	_, err = pool.Exec(ctx, `
		CREATE TABLE credentials (
			id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
			username        TEXT        NOT NULL UNIQUE,
			encrypted_data  BYTEA       NOT NULL,
			nonce           BYTEA       NOT NULL,
			key_version     INTEGER     NOT NULL DEFAULT 1,
			is_valid        BOOLEAN     NOT NULL DEFAULT TRUE,
			last_validated  TIMESTAMPTZ,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		t.Fatalf("failed to create credentials table: %v", err)
	}

	return pool
}

// TestCredentialFlow_SaveAndFetch tests the end-to-end flow:
// TODO: cc-login is removed (integrated into session container). Update this test to reflect new design.
// cc-tunnel fetches and decrypts credentials via CredentialService.
func TestCredentialFlow_SaveAndFetch(t *testing.T) {
	pool := setupE2ETestDB(t)
	ctx := context.Background()

	// Shared encryption key
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	const keyVersion = 1
	username := "e2e-user"
	plaintext := []byte(`{"access_token":"e2e-token","refresh_token":"e2e-refresh"}`)

	// --- encrypt and save ---
	encryptor, err := credential.NewEncryptor(key, keyVersion)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	ciphertext, nonce, err := encryptor.Seal(plaintext, username)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	repo := credential.NewCredentialRepository(pool)
	if err := repo.Upsert(ctx, &credential.Credential{
		Username:      username,
		EncryptedData: ciphertext,
		Nonce:         nonce,
		KeyVersion:    keyVersion,
	}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	// --- cc-tunnel side: fetch and decrypt ---
	svc := credential.NewCredentialService(repo, encryptor)
	got, err := svc.FetchAndDecrypt(ctx, username)
	if err != nil {
		t.Fatalf("FetchAndDecrypt: %v", err)
	}

	if string(got) != string(plaintext) {
		t.Errorf("decrypted data mismatch: got %q, want %q", got, plaintext)
	}
}

// TestCredentialReloginFlow_E2E tests the re-login flow end-to-end:
// 1. Store initial credentials and mark them invalid (simulating expired/invalidated session).
// 2. Simulate relogin/finalize: mock cc-remote-agent returns new credJSON → StoreCredential saves to DB.
// 3. Verify FetchAndDecrypt succeeds with the newly stored credentials.
func TestCredentialReloginFlow_E2E(t *testing.T) {
	pool := setupE2ETestDB(t)
	ctx := context.Background()

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	const keyVersion = 1
	username := "relogin-user"

	encryptor, err := credential.NewEncryptor(key, keyVersion)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	repo := credential.NewCredentialRepository(pool)
	svc := credential.NewCredentialService(repo, encryptor)

	// Step 1: store initial credentials and mark them invalid.
	initialPlain := []byte(`{"access_token":"old-token"}`)
	ciphertext, nonce, err := encryptor.Seal(initialPlain, username)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if err := repo.Upsert(ctx, &credential.Credential{
		Username:      username,
		EncryptedData: ciphertext,
		Nonce:         nonce,
		KeyVersion:    keyVersion,
	}); err != nil {
		t.Fatalf("Upsert initial: %v", err)
	}
	if err := repo.MarkInvalid(ctx, username); err != nil {
		t.Fatalf("MarkInvalid: %v", err)
	}

	// Verify that FetchAndDecrypt returns ErrCredentialsInvalid before re-login.
	if _, err := svc.FetchAndDecrypt(ctx, username); !errors.Is(err, credential.ErrCredentialsInvalid) {
		t.Fatalf("expected ErrCredentialsInvalid before relogin, got %v", err)
	}

	// Step 2: simulate relogin/finalize — mock cc-remote-agent returned new credJSON.
	newCredJSON := `{"access_token":"new-token","refresh_token":"new-refresh"}`
	if err := svc.StoreCredential(ctx, username, newCredJSON); err != nil {
		t.Fatalf("StoreCredential (relogin finalize): %v", err)
	}

	// Step 3: FetchAndDecrypt must now succeed with the new credentials.
	got, err := svc.FetchAndDecrypt(ctx, username)
	if err != nil {
		t.Fatalf("FetchAndDecrypt after relogin: %v", err)
	}
	if string(got) != newCredJSON {
		t.Errorf("decrypted data mismatch: got %q, want %q", got, newCredJSON)
	}
}
