package api

import (
	"context"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/credential"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// setupSessionAuthGateTestDB starts a postgres:16-alpine testcontainer and creates
// the credentials table required by CredentialService.
func setupSessionAuthGateTestDB(t *testing.T) *pgxpool.Pool {
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
			t.Logf("failed to terminate postgres container: %v", err)
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

// TestSessionAuthGate_E2E verifies the GET /credentials/status endpoint end-to-end
// using a real PostgreSQL database via testcontainers.
func TestSessionAuthGate_E2E(t *testing.T) {
	pool := setupSessionAuthGateTestDB(t)
	ctx := context.Background()

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	const keyVersion = 1

	encryptor, err := credential.NewEncryptor(key, keyVersion)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	repo := credential.NewCredentialRepository(pool)
	credSvc := credential.NewCredentialService(repo, encryptor)

	t.Run("unregistered user returns registered=false isValid=false", func(t *testing.T) {
		server := &Server{
			session:     newAppSession(),
			credService: credSvc,
		}
		token := "e2e-token-unregistered"
		server.session.set(token, AppUser{Id: uuid.New().String(), Name: "unregistered-user"})

		w := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/credentials/status", nil)
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
	})

	t.Run("registered valid user returns registered=true isValid=true", func(t *testing.T) {
		username := "registered-user"
		plaintext := []byte(`{"access_token":"valid-token"}`)

		ciphertext, nonce, err := encryptor.Seal(plaintext, username)
		if err != nil {
			t.Fatalf("Seal: %v", err)
		}
		if err := repo.Upsert(ctx, &credential.Credential{
			Username:      username,
			EncryptedData: ciphertext,
			Nonce:         nonce,
			KeyVersion:    keyVersion,
		}); err != nil {
			t.Fatalf("Upsert: %v", err)
		}

		server := &Server{
			session:     newAppSession(),
			credService: credSvc,
		}
		token := "e2e-token-registered"
		server.session.set(token, AppUser{Id: uuid.New().String(), Name: username})

		w := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/credentials/status", nil)
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
	})

	t.Run("nil credService (skip mode) returns registered=true isValid=true", func(t *testing.T) {
		server := &Server{
			session: newAppSession(),
			// credService is nil — no-auth / skip mode
		}

		w := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/credentials/status", nil)

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
	})
}
