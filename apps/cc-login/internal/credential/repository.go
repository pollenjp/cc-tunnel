package credential

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("credential not found")

type Credential struct {
	ID            string
	Username      string
	EncryptedData []byte
	Nonce         []byte
	KeyVersion    int
	IsValid       bool
	LastValidated *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type CredentialRepository struct {
	pool *pgxpool.Pool
}

func NewCredentialRepository(pool *pgxpool.Pool) *CredentialRepository {
	return &CredentialRepository{pool: pool}
}

// Upsert inserts or replaces a credential row for username.
func (r *CredentialRepository) Upsert(ctx context.Context, c *Credential) error {
	const q = `
		INSERT INTO credentials (username, encrypted_data, nonce, key_version, is_valid, updated_at)
		VALUES ($1, $2, $3, $4, TRUE, NOW())
		ON CONFLICT (username) DO UPDATE SET
			encrypted_data = EXCLUDED.encrypted_data,
			nonce          = EXCLUDED.nonce,
			key_version    = EXCLUDED.key_version,
			is_valid       = TRUE,
			updated_at     = NOW()
	`
	_, err := r.pool.Exec(ctx, q, c.Username, c.EncryptedData, c.Nonce, c.KeyVersion)
	return err
}

func (r *CredentialRepository) GetByUsername(ctx context.Context, username string) (*Credential, error) {
	const q = `
		SELECT id, username, encrypted_data, nonce, key_version, is_valid, last_validated, created_at, updated_at
		FROM credentials WHERE username = $1
	`
	row := r.pool.QueryRow(ctx, q, username)
	c := &Credential{}
	err := row.Scan(&c.ID, &c.Username, &c.EncryptedData, &c.Nonce, &c.KeyVersion, &c.IsValid, &c.LastValidated, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *CredentialRepository) MarkInvalid(ctx context.Context, username string) error {
	_, err := r.pool.Exec(ctx, `UPDATE credentials SET is_valid = FALSE, updated_at = NOW() WHERE username = $1`, username)
	return err
}

func (r *CredentialRepository) Delete(ctx context.Context, username string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM credentials WHERE username = $1`, username)
	return err
}

func (r *CredentialRepository) UpdateLastValidated(ctx context.Context, username string) error {
	_, err := r.pool.Exec(ctx, `UPDATE credentials SET last_validated = NOW(), updated_at = NOW() WHERE username = $1`, username)
	return err
}
