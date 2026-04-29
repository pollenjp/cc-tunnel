package api

import (
	"log/slog"
	"net/http"
	"time"
)

// RegisterRoutes registers all cc-login HTTP routes on the given mux.
func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	mux.HandleFunc("/credentials/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.PostCredentialsLogin(w, r)
	})
	mux.HandleFunc("/credentials/input", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.PostCredentialsInput(w, r)
	})
	mux.HandleFunc("/credentials/output", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.GetCredentialsOutput(w, r)
	})
	mux.HandleFunc("/credentials/cancel", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.PostCredentialsCancel(w, r)
	})
	mux.HandleFunc("/credentials/finalize", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.PostCredentialsFinalize(w, r)
	})
	mux.HandleFunc("/credentials/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.GetCredentialsStatus(w, r)
	})
	mux.HandleFunc("/credentials", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.DeleteCredentials(w, r)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
			slog.Error("health write failed", "error", err)
		}
	})
}

// LoggingMiddleware logs each request.
func LoggingMiddleware(next http.Handler) http.Handler {
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
