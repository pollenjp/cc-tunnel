package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/api"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/credential"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/db"
	dockerpkg "github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/docker"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/gce"
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

	// Auth remote: always points to the auth agent (cc-remote-agent-auth in compose).
	remote := remoteclient.NewClient(*agentURL)

	execProvider, err := newProviderFromEnv(ctx, os.Getenv("EXECUTION_PROVIDER"), repo)
	if err != nil {
		slog.Error("invalid EXECUTION_PROVIDER", "err", err)
		os.Exit(1)
	}

	// Cleanup orphaned containers from previous sessions at startup.
	type orphanCleaner interface {
		CleanupOrphans(ctx context.Context) error
	}
	if cleaner, ok := execProvider.(orphanCleaner); ok {
		if err := cleaner.CleanupOrphans(ctx); err != nil {
			slog.Warn("orphan cleanup failed", "err", err)
		}
	}

	// Graceful shutdown on SIGTERM/SIGINT.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		slog.Info("received shutdown signal, cleaning up")
		if c, ok := execProvider.(io.Closer); ok {
			if err := c.Close(); err != nil {
				slog.Error("provider close failed", "err", err)
			}
		}
		pool.Close()
		os.Exit(0)
	}()

	// CC_LOGIN_ENCRYPTION_KEY must be set (64 hex chars = 32 bytes for AES-256).
	encKeyHex := os.Getenv("CC_LOGIN_ENCRYPTION_KEY")
	if encKeyHex == "" {
		slog.Error("CC_LOGIN_ENCRYPTION_KEY is not set; set a 64-char hex key (32 bytes)")
		os.Exit(1)
	}
	encKeyBytes, err := hex.DecodeString(encKeyHex)
	if err != nil {
		slog.Error("CC_LOGIN_ENCRYPTION_KEY is not valid hex", "err", err)
		os.Exit(1)
	}
	encryptor, err := credential.NewEncryptor(encKeyBytes, 1)
	if err != nil {
		slog.Error("failed to initialize encryptor", "err", err)
		os.Exit(1)
	}
	credRepo := credential.NewCredentialRepository(pool)
	credSvc := credential.NewCredentialService(credRepo, encryptor)

	handler := api.NewHandlerFull(repo, remote, execProvider, credSvc, credSvc)

	mux := http.NewServeMux()
	api.HandlerFromMux(handler, mux)

	slog.Info("cc-tunnel listening", "addr", *addr)
	if err := http.ListenAndServe(*addr, api.LoggingMiddleware(mux)); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// newProviderFromEnv selects the ExecutionProvider based on envVal.
// Returns an error for unknown or empty envVal.
func newProviderFromEnv(ctx context.Context, envVal string, repo *db.Repository) (provider.ExecutionProvider, error) {
	switch envVal {
	case "local":
		runner, err := dockerpkg.NewSDKRunner()
		if err != nil {
			return nil, fmt.Errorf("docker runner: %w", err)
		}
		sm := dockerpkg.NewSessionManager(runner, dockerpkg.SessionManagerConfig{
			Image:         getEnvOrDefault("CC_REMOTE_AGENT_IMAGE", "cc-remote-agent:latest"),
			Network:       getEnvOrDefault("DOCKER_NETWORK", "apps_default"),
			// apps_claude-sessions
			VolumeName:    getEnvOrDefault("CLAUDE_SESSIONS_VOLUME", "claude-sessions"),
			ContainerPort: getEnvOrDefault("CC_REMOTE_AGENT_PORT", "9091"),
		})
		return localprovider.NewLocalDockerProvider(sm), nil
	case "cloud_run_sandbox":
		return cloudrunsandbox.New(), nil
	case "docker_gce":
		gceClient, err := gce.NewSDKGCEClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("GCE client: %w", err)
		}
		gceProjectID := os.Getenv("GCE_PROJECT_ID")
		if gceProjectID == "" {
			return nil, fmt.Errorf("GCE_PROJECT_ID environment variable is required for docker_gce provider")
		}
		agentImage := os.Getenv("CC_REMOTE_AGENT_IMAGE")
		if agentImage == "" {
			return nil, fmt.Errorf("CC_REMOTE_AGENT_IMAGE environment variable is required for docker_gce provider")
		}
		cfg := dockergce.DockerGCEConfig{
			ProjectID:   gceProjectID,
			Zone:        getEnvOrDefault("GCE_ZONE", "us-central1-a"),
			MachineType: getEnvOrDefault("GCE_MACHINE_TYPE", "e2-medium"),
			AgentImage:  agentImage,
			AgentPort:   9091,
			IdleTimeout: 15 * time.Minute,
		}
		return dockergce.NewDockerGCEProvider(cfg, gceClient, repo), nil
	default:
		return nil, fmt.Errorf("unknown EXECUTION_PROVIDER: %q", envVal)
	}
}
