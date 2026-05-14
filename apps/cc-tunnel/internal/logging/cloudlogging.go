package logging

import (
	"io"
	"log/slog"
	"strings"
)

// NewCloudLoggingHandler returns a slog handler whose JSON output uses the
// field names Cloud Logging recognises for structured payloads:
//
//	level → severity (uppercased, e.g. INFO/ERROR)
//	msg   → message
//	time  → timestamp (RFC3339Nano via slog default)
//
// It also wraps the JSON handler with ErrorStackHandler so any error-typed
// attribute attaches a compact call stack.
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
	return &ErrorStackHandler{Next: slog.NewJSONHandler(w, &base)}
}
