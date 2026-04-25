package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/api"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/db"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/logging"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider/cloudrunsandbox"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider/dockergce"
	localprovider "github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider/local"
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

	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	stackHandler := &logging.ErrorStackHandler{Next: jsonHandler}
	slog.SetDefault(slog.New(stackHandler))

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

	// Select execution provider via EXECUTION_PROVIDER env var.
	execProvider, remote, err := newProviderFromEnv(os.Getenv("EXECUTION_PROVIDER"), *agentURL)
	if err != nil {
		slog.Error("invalid EXECUTION_PROVIDER", "err", err)
		os.Exit(1)
	}

	handler := api.NewHandler(repo, remote, execProvider)

	mux := http.NewServeMux()
	api.HandlerFromMux(handler, mux)

	slog.Info("cc-tunnel listening", "addr", *addr)
	if err := http.ListenAndServe(*addr, api.LoggingMiddleware(mux)); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}

// newProviderFromEnv selects the ExecutionProvider based on envVal.
// agentURL is only used when envVal is "local".
// Returns an error for unknown or empty envVal.
func newProviderFromEnv(envVal, agentURL string) (provider.ExecutionProvider, *remoteclient.Client, error) {
	switch envVal {
	case "local":
		remote := remoteclient.NewClient(agentURL)
		return localprovider.New(remote), remote, nil
	case "cloud_run_sandbox":
		return cloudrunsandbox.New(), nil, nil
	case "docker_gce":
		return dockergce.New(), nil, nil
	default:
		return nil, nil, fmt.Errorf("unknown EXECUTION_PROVIDER: %q", envVal)
	}
}
