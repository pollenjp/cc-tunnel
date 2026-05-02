package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"time"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

// getSessionClientOrRespond resolves a remoteclient.Client for the given conversation.
// On error, it writes the appropriate HTTP response and returns ok=false.
func getSessionClientOrRespond(w http.ResponseWriter, r *http.Request, ep provider.ExecutionProvider, convID string) (*remoteclient.Client, bool) {
	client, err := ep.GetSessionClient(r.Context(), convID)
	if err != nil {
		if errors.Is(err, provider.ErrSessionNotFound) {
			writeError(w, http.StatusNotFound, fmt.Sprintf("session not found for conversation %s", convID))
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return nil, false
	}
	return client, true
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("json encode failed", "error", err)
	}
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, Error{Error: msg})
}

func cloneToolCalls(calls []ToolCallData) []ToolCallData {
	if calls == nil {
		return nil
	}
	clone := make([]ToolCallData, len(calls))
	copy(clone, calls)
	return clone
}

func cloneBlocks(blocks []map[string]interface{}) []map[string]interface{} {
	if blocks == nil {
		return nil
	}
	clone := make([]map[string]interface{}, len(blocks))
	for i, b := range blocks {
		m := make(map[string]interface{}, len(b))
		maps.Copy(m, b)
		clone[i] = m
	}
	return clone
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
