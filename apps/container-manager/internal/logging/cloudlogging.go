// Package logging configures slog for Cloud Logging structured ingestion.
//
// Mirrors apps/cc-tunnel/internal/logging/cloudlogging.go intentionally;
// duplicated rather than shared via a Go module so the two apps can be built
// independently.
package logging

import (
	"io"
	"log/slog"
	"strings"
)

// NewCloudLoggingHandler returns a slog handler whose JSON output uses the
// field names Cloud Logging recognises for structured payloads:
//
//	level → severity (uppercased)
//	msg   → message
//	time  → timestamp
func NewCloudLoggingHandler(w io.Writer, opts *slog.HandlerOptions) slog.Handler {
	if opts == nil {
		opts = &slog.HandlerOptions{Level: slog.LevelInfo}
	}
	base := *opts
	prev := base.ReplaceAttr
	base.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
		if len(groups) == 0 {
			switch a.Key {
			case slog.LevelKey:
				return slog.Attr{Key: "severity", Value: slog.StringValue(strings.ToUpper(a.Value.String()))}
			case slog.MessageKey:
				return slog.Attr{Key: "message", Value: a.Value}
			case slog.TimeKey:
				return slog.Attr{Key: "timestamp", Value: a.Value}
			}
		}
		if prev != nil {
			return prev(groups, a)
		}
		return a
	}
	return slog.NewJSONHandler(w, &base)
}
