package db

import (
	"context"
	"embed"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

// NewPool creates a new pgxpool.Pool, runs goose migrations, and cleans up orphaned streaming messages.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}

	if err := runMigrations(pool); err != nil {
		pool.Close()
		return nil, err
	}

	cleanupOrphanedStreamingMessages(ctx, pool)

	return pool, nil
}

// cleanupOrphanedStreamingMessages marks streaming messages older than 30 minutes as error.
// These are messages that were left in 'streaming' state due to a server crash.
func cleanupOrphanedStreamingMessages(ctx context.Context, pool *pgxpool.Pool) {
	const q = `
		UPDATE messages SET status = 'error', updated_at = NOW()
		WHERE status = 'streaming' AND created_at < NOW() - INTERVAL '30 minutes'
	`
	if _, err := pool.Exec(ctx, q); err != nil {
		slog.Warn("failed to cleanup orphaned streaming messages", "error", err)
	}
}

func runMigrations(pool *pgxpool.Pool) error {
	db := stdlib.OpenDBFromPool(pool)
	defer func() {
		if err := db.Close(); err != nil {
			slog.Warn("db.Close failed", "error", err)
		}
	}()

	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(db, "migrations")
}
