package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/pollenjp/cc-tunnel/apps/container-manager/internal/api"
	dockerops "github.com/pollenjp/cc-tunnel/apps/container-manager/internal/docker"
	"github.com/pollenjp/cc-tunnel/apps/container-manager/internal/logging"
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

	srv := api.NewServer(mgr)

	addr := ":" + getenv("PORT", "9090")
	slog.Info("container-manager listening", "addr", addr)
	if err := http.ListenAndServe(addr, loggingMiddleware(srv.Routes())); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
