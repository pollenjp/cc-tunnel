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
	"strconv"
	"syscall"
	"time"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/api"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/cmclient"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/credential"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/db"
	dockerpkg "github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/docker"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/gce"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/logging"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider/cloudrunsandbox"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider/dockergce"
	localprovider "github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider/local"
)

func main() {
	defaultAddr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		defaultAddr = ":" + p
	}
	addr := flag.String("addr", defaultAddr, "listen address")
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

	handler := api.NewHandlerFull(repo, execProvider, credSvc, credSvc)

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

func getEnvIntOrDefault(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}

// newLocalRunner returns a DockerRunner for local-mode SessionManager.
// When CONTAINER_MANAGER_URL is set, requests are proxied to a remote
// container-manager (matching the production architecture). The container
// port string returned is what SessionManager uses to build the agent URL
// (http://<container-name>:<port>); in container-manager mode, env injection
// of PORT is not used and we fall back to cc-remote-agent's default 9090.
func newLocalRunner() (dockerpkg.DockerRunner, string, error) {
	if cmURL := os.Getenv("CONTAINER_MANAGER_URL"); cmURL != "" {
		c, err := cmclient.NewClient(cmURL)
		if err != nil {
			return nil, "", fmt.Errorf("cmclient: %w", err)
		}
		port := getEnvOrDefault("CC_REMOTE_AGENT_PORT", "9090")
		portInt, err := strconv.Atoi(port)
		if err != nil {
			return nil, "", fmt.Errorf("parse CC_REMOTE_AGENT_PORT: %w", err)
		}
		return dockerpkg.NewCMRunner(c, portInt), port, nil
	}
	runner, err := dockerpkg.NewSDKRunner()
	if err != nil {
		return nil, "", fmt.Errorf("docker runner: %w", err)
	}
	return runner, getEnvOrDefault("CC_REMOTE_AGENT_PORT", "9091"), nil
}

// newProviderFromEnv selects the ExecutionProvider based on envVal.
// Returns an error for unknown or empty envVal.
func newProviderFromEnv(ctx context.Context, envVal string, repo *db.Repository) (provider.ExecutionProvider, error) {
	switch envVal {
	case "local":
		// When CONTAINER_MANAGER_URL is set, drive Docker through a remote
		// container-manager instance instead of the local Docker SDK. This
		// mirrors the production architecture during local development and
		// removes the need to bind-mount /var/run/docker.sock into cc-tunnel.
		runner, containerPortStr, err := newLocalRunner()
		if err != nil {
			return nil, err
		}
		sm := dockerpkg.NewSessionManager(runner, dockerpkg.SessionManagerConfig{
			Image:         getEnvOrDefault("CC_REMOTE_AGENT_IMAGE", "cc-remote-agent:latest"),
			Network:       getEnvOrDefault("DOCKER_NETWORK", "apps_default"),
			ContainerPort: containerPortStr,
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
		vmImage := os.Getenv("GCE_VM_IMAGE")
		if vmImage == "" {
			return nil, fmt.Errorf("GCE_VM_IMAGE environment variable is required for docker_gce provider")
		}
		cfg := dockergce.DockerGCEConfig{
			ProjectID:            gceProjectID,
			Zone:                 getEnvOrDefault("GCE_ZONE", "us-central1-a"),
			MachineType:          getEnvOrDefault("GCE_MACHINE_TYPE", "e2-medium"),
			VMImage:              vmImage,
			VMServiceAccount:     os.Getenv("GCE_VM_SERVICE_ACCOUNT"),
			VMSubnetwork:         os.Getenv("GCE_VM_SUBNETWORK"),
			AgentImage:           agentImage,
			AgentPort:            9091,
			IdleTimeout:          15 * time.Minute,
			MaxContainers:        getEnvIntOrDefault("GCE_MAX_CONTAINERS", 10),
			IdleCheckInterval:    time.Duration(getEnvIntOrDefault("GCE_IDLE_CHECK_INTERVAL_SECONDS", 300)) * time.Second,
			ContainerManagerPort: getEnvIntOrDefault("GCE_CONTAINER_MANAGER_PORT", 9090),
			ContainerNamePrefix:  getEnvOrDefault("GCE_CONTAINER_NAME_PREFIX", "cc-remote-agent"),
		}
		return dockergce.NewDockerGCEProvider(cfg, gceClient, repo), nil
	default:
		return nil, fmt.Errorf("unknown EXECUTION_PROVIDER: %q", envVal)
	}
}
