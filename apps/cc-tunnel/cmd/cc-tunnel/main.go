package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/api"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/db"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

func main() {
	defaultAddr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		defaultAddr = ":" + p
	}
	addr := flag.String("addr", defaultAddr, "listen address")
	agentURL := flag.String("agent-url", "http://localhost:9091", "cc-remote-agent URL")
	dbURL := flag.String("db-url", "", "PostgreSQL connection URL")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	if *dbURL == "" {
		if v := os.Getenv("DATABASE_URL"); v != "" {
			*dbURL = v
		} else {
			*dbURL = "postgres://cctunnel:cctunnel_dev@localhost:5432/cctunnel?sslmode=disable"
		}
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, *dbURL)
	if err != nil {
		slog.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	repo := db.NewRepository(pool)
	remote := remoteclient.NewClient(*agentURL)
	handler := api.NewHandler(repo, remote)

	mux := http.NewServeMux()
	api.HandlerFromMux(handler, mux)

	slog.Info("cc-tunnel listening", "addr", *addr, "agent", *agentURL)
	if err := http.ListenAndServe(*addr, api.LoggingMiddleware(mux)); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}
