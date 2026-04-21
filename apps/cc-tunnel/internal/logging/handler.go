package logging

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
)

// ErrorStackHandler wraps a slog.Handler and automatically adds a "stack"
// attribute when an error attribute is present in the record.
type ErrorStackHandler struct {
	Next slog.Handler
}

func (h *ErrorStackHandler) Handle(ctx context.Context, r slog.Record) error {
	r.Attrs(func(a slog.Attr) bool {
		if _, ok := a.Value.Any().(error); ok {
			r.AddAttrs(slog.Any("stack", extractStack()))
			return false
		}
		return true
	})
	return h.Next.Handle(ctx, r)
}

func (h *ErrorStackHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.Next.Enabled(ctx, l)
}

func (h *ErrorStackHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ErrorStackHandler{Next: h.Next.WithAttrs(attrs)}
}

func (h *ErrorStackHandler) WithGroup(name string) slog.Handler {
	return &ErrorStackHandler{Next: h.Next.WithGroup(name)}
}

// extractStack returns a compact call stack as a slice of "file:line" strings,
// skipping slog/runtime internal frames.
func extractStack() []string {
	pcs := make([]uintptr, 32)
	// skip: runtime.Callers, extractStack, Handle
	n := runtime.Callers(3, pcs)
	if n == 0 {
		return nil
	}
	frames := runtime.CallersFrames(pcs[:n])
	var stack []string
	for {
		frame, more := frames.Next()
		if frame.File != "" {
			stack = append(stack, fmt.Sprintf("%s:%d", frame.File, frame.Line))
		}
		if !more || len(stack) >= 8 {
			break
		}
	}
	return stack
}
