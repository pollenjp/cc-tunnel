package credential_test

import (
	"context"
	"crypto/rand"
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
// cc-login encrypts and saves credentials to the DB,
// cc-tunnel fetches and decrypts them via CredentialService.
func TestCredentialFlow_SaveAndFetch(t *testing.T) {
	pool := setupE2ETestDB(t)
	ctx := context.Background()

	// Shared encryption key (same key used by both cc-login and cc-tunnel)
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	const keyVersion = 1
	username := "e2e-user"
	plaintext := []byte(`{"access_token":"e2e-token","refresh_token":"e2e-refresh"}`)

	// --- cc-login side: encrypt and save ---
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
