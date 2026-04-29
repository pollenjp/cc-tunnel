package credential_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pollenjp/cc-tunnel/apps/cc-login/internal/credential"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func setupTestDB(t *testing.T) *pgxpool.Pool {
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

	// Create credentials table
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

func TestCredentialRepository_UpsertAndGet(t *testing.T) {
	pool := setupTestDB(t)
	repo := credential.NewCredentialRepository(pool)
	ctx := context.Background()

	cred := &credential.Credential{
		Username:      "alice",
		EncryptedData: []byte("encrypted"),
		Nonce:         []byte("nonce123nonce"),
		KeyVersion:    1,
		IsValid:       true,
	}

	if err := repo.Upsert(ctx, cred); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	got, err := repo.GetByUsername(ctx, "alice")
	if err != nil {
		t.Fatalf("GetByUsername failed: %v", err)
	}
	if string(got.EncryptedData) != string(cred.EncryptedData) {
		t.Errorf("encrypted_data mismatch: got %q, want %q", got.EncryptedData, cred.EncryptedData)
	}
	if string(got.Nonce) != string(cred.Nonce) {
		t.Errorf("nonce mismatch")
	}
	if got.KeyVersion != 1 {
		t.Errorf("key_version mismatch: got %d, want 1", got.KeyVersion)
	}
	if !got.IsValid {
		t.Error("expected is_valid=true")
	}
}

func TestCredentialRepository_UpsertOverwrites(t *testing.T) {
	pool := setupTestDB(t)
	repo := credential.NewCredentialRepository(pool)
	ctx := context.Background()

	cred1 := &credential.Credential{
		Username:      "bob",
		EncryptedData: []byte("old-encrypted"),
		Nonce:         []byte("nonce123nonce1"),
		KeyVersion:    1,
		IsValid:       true,
	}
	if err := repo.Upsert(ctx, cred1); err != nil {
		t.Fatalf("first Upsert failed: %v", err)
	}

	cred2 := &credential.Credential{
		Username:      "bob",
		EncryptedData: []byte("new-encrypted"),
		Nonce:         []byte("nonce123nonce2"),
		KeyVersion:    2,
		IsValid:       true,
	}
	if err := repo.Upsert(ctx, cred2); err != nil {
		t.Fatalf("second Upsert failed: %v", err)
	}

	got, err := repo.GetByUsername(ctx, "bob")
	if err != nil {
		t.Fatalf("GetByUsername failed: %v", err)
	}
	if string(got.EncryptedData) != "new-encrypted" {
		t.Errorf("expected new-encrypted, got %q", got.EncryptedData)
	}
	if got.KeyVersion != 2 {
		t.Errorf("expected key_version=2, got %d", got.KeyVersion)
	}
}

func TestCredentialRepository_GetNotFound(t *testing.T) {
	pool := setupTestDB(t)
	repo := credential.NewCredentialRepository(pool)
	ctx := context.Background()

	_, err := repo.GetByUsername(ctx, "nonexistent")
	if !errors.Is(err, credential.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestCredentialRepository_MarkInvalid(t *testing.T) {
	pool := setupTestDB(t)
	repo := credential.NewCredentialRepository(pool)
	ctx := context.Background()

	cred := &credential.Credential{
		Username:      "charlie",
		EncryptedData: []byte("enc"),
		Nonce:         []byte("nonce123nonc1"),
		KeyVersion:    1,
		IsValid:       true,
	}
	if err := repo.Upsert(ctx, cred); err != nil {
		t.Fatal(err)
	}

	if err := repo.MarkInvalid(ctx, "charlie"); err != nil {
		t.Fatalf("MarkInvalid failed: %v", err)
	}

	got, err := repo.GetByUsername(ctx, "charlie")
	if err != nil {
		t.Fatal(err)
	}
	if got.IsValid {
		t.Error("expected is_valid=false after MarkInvalid")
	}
}

func TestCredentialRepository_Delete(t *testing.T) {
	pool := setupTestDB(t)
	repo := credential.NewCredentialRepository(pool)
	ctx := context.Background()

	cred := &credential.Credential{
		Username:      "diana",
		EncryptedData: []byte("enc"),
		Nonce:         []byte("nonce123nonc1"),
		KeyVersion:    1,
		IsValid:       true,
	}
	if err := repo.Upsert(ctx, cred); err != nil {
		t.Fatal(err)
	}

	if err := repo.Delete(ctx, "diana"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := repo.GetByUsername(ctx, "diana")
	if !errors.Is(err, credential.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestCredentialRepository_UpdateLastValidated(t *testing.T) {
	pool := setupTestDB(t)
	repo := credential.NewCredentialRepository(pool)
	ctx := context.Background()

	cred := &credential.Credential{
		Username:      "eve",
		EncryptedData: []byte("enc"),
		Nonce:         []byte("nonce123nonc1"),
		KeyVersion:    1,
		IsValid:       true,
	}
	if err := repo.Upsert(ctx, cred); err != nil {
		t.Fatal(err)
	}

	if err := repo.UpdateLastValidated(ctx, "eve"); err != nil {
		t.Fatalf("UpdateLastValidated failed: %v", err)
	}

	got, err := repo.GetByUsername(ctx, "eve")
	if err != nil {
		t.Fatal(err)
	}
	if got.LastValidated == nil {
		t.Error("expected last_validated to be set")
	}
	if time.Since(*got.LastValidated) > 5*time.Second {
		t.Error("last_validated is too old")
	}
}
