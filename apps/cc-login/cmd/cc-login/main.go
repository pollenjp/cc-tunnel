package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/pollenjp/cc-tunnel/apps/cc-login/internal/api"
	"github.com/pollenjp/cc-tunnel/apps/cc-login/internal/auth"
	"github.com/pollenjp/cc-tunnel/apps/cc-login/internal/config"
	"github.com/pollenjp/cc-tunnel/apps/cc-login/internal/credential"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	// Startup self-test: verify encryption key works
	enc, err := credential.NewEncryptor(cfg.EncryptionKey, 1)
	if err != nil {
		slog.Error("failed to create encryptor", "err", err)
		os.Exit(1)
	}
	// Sanity test
	testPlain := []byte("startup-test")
	ct, nonce, err := enc.Seal(testPlain, "test")
	if err != nil {
		slog.Error("encryption self-test failed", "err", err)
		os.Exit(1)
	}
	if _, err := enc.Open(ct, nonce, "test"); err != nil {
		slog.Error("decryption self-test failed", "err", err)
		os.Exit(1)
	}
	slog.Info("encryption self-test passed")

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	credRepo := credential.NewCredentialRepository(pool)
	authMgr := auth.NewAuthManager()
	resolver := api.NewCCTunnelTokenResolver(cfg.CCTunnelURL)

	handler := api.NewHandler(authMgr, credRepo, enc, resolver)

	mux := http.NewServeMux()
	api.RegisterRoutes(mux, handler)

	addr := ":" + cfg.Port
	slog.Info("cc-login listening", "addr", addr)
	if err := http.ListenAndServe(addr, api.LoggingMiddleware(mux)); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}
