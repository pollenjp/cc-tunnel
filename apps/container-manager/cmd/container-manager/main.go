package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/pollenjp/cc-tunnel/apps/container-manager/internal/api"
	dockerops "github.com/pollenjp/cc-tunnel/apps/container-manager/internal/docker"
	"github.com/pollenjp/cc-tunnel/apps/container-manager/internal/logging"
	"github.com/pollenjp/cc-tunnel/apps/container-manager/internal/selfreaper"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

func main() {
	logHandler := logging.NewCloudLoggingHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(logHandler).With("component", "container-manager"))

	mgr, err := dockerops.NewManager(os.Getenv("DEFAULT_NETWORK"))
	if err != nil {
		slog.Error("docker manager init", "err", err)
		os.Exit(1)
	}

	rootCtx, cancelRoot := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancelRoot()

	startSelfReaper(rootCtx, mgr)

	srv := api.NewServer(mgr)

	addr := ":" + getenv("PORT", "9090")
	slog.Info("container-manager listening", "addr", addr)
	if err := http.ListenAndServe(addr, loggingMiddleware(srv.Routes())); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}

// startSelfReaper spawns the per-VM self-termination goroutine when
// SELF_REAP_ENABLED is truthy (default true on GCE, off when
// container-manager is run outside of a GCE VM — e.g. local dev /
// tests).
//
// Failures during startup (e.g. metadata server unreachable because we
// are not on GCE) are logged and swallowed: the HTTP server keeps
// serving so container-manager remains usable. The Cloud Scheduler
// safety-net path (every 6 h) will still reap such VMs.
func startSelfReaper(ctx context.Context, mgr *dockerops.Manager) {
	if !getenvBool("SELF_REAP_ENABLED", true) {
		slog.Info("self-reaper: disabled via SELF_REAP_ENABLED=false")
		return
	}
	cfg := selfreaper.Config{
		Interval: time.Duration(getenvInt("SELF_REAP_INTERVAL_SECONDS", 60)) * time.Second,
		Timeout:  time.Duration(getenvInt("SELF_REAP_TIMEOUT_SECONDS", 600)) * time.Second,
	}
	deleter, err := selfreaper.NewSDKDeleter(ctx)
	if err != nil {
		slog.Warn("self-reaper: GCE compute client init failed, reaper disabled", "err", err)
		return
	}
	r := selfreaper.New(
		selfreaper.DockerAgentLister{Manager: mgr},
		deleter,
		selfreaper.SDKMetadata{},
		cfg,
	)
	go func() {
		if err := r.Run(ctx); err != nil {
			slog.Warn("self-reaper: stopped with error", "err", err)
		}
	}()
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getenvBool(key string, def bool) bool {
	switch os.Getenv(key) {
	case "":
		return def
	case "0", "false", "FALSE", "False", "no", "NO":
		return false
	default:
		return true
	}
}
