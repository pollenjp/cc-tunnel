package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/pollenjp/cc-tunnel/apps/cc-remote-agent/internal/api"
	"github.com/pollenjp/cc-tunnel/apps/cc-remote-agent/internal/auth"
	"github.com/pollenjp/cc-tunnel/apps/cc-remote-agent/internal/logging"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
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
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	stackHandler := &logging.ErrorStackHandler{Next: jsonHandler}
	slog.SetDefault(slog.New(stackHandler))

	port := os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}
	addr := ":" + port

	authMgr := auth.NewAuthManager()
	handler := api.NewHandler(authMgr)

	mux := http.NewServeMux()
	mux.HandleFunc("/init", handler.Init)
	mux.HandleFunc("/execute", handler.Execute)
	mux.HandleFunc("/health", handler.Health)
	mux.HandleFunc("/auth/status", handler.AuthStatus)
	mux.HandleFunc("/auth/login", handler.AuthLogin)
	mux.HandleFunc("/auth/logout", handler.AuthLogout)
	mux.HandleFunc("/auth/pty/input", handler.AuthPtyInput)
	mux.HandleFunc("/auth/pty/stream", handler.AuthPtyStream)
	mux.HandleFunc("/auth/cancel", handler.AuthCancel)
	mux.HandleFunc("/auth/finalize-credentials", handler.FinalizeCredentials)

	slog.Info("cc-remote-agent listening", "addr", addr)
	if err := http.ListenAndServe(addr, loggingMiddleware(mux)); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}
