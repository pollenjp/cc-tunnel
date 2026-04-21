package logging

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
)

// recordingHandler captures slog.Records for test inspection.
type recordingHandler struct {
	records []slog.Record
}

func (h *recordingHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r.Clone())
	return nil
}
func (h *recordingHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(_ string) slog.Handler       { return h }

func TestErrorStackHandler_withErrorAttr_addsStack(t *testing.T) {
	recorder := &recordingHandler{}
	handler := &ErrorStackHandler{Next: recorder}

	r := slog.NewRecord(time.Now(), slog.LevelError, "test error", 0)
	r.AddAttrs(slog.Any("err", errors.New("something broke")))

	if err := handler.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if len(recorder.records) == 0 {
		t.Fatal("no records captured by Next handler")
	}

	var stackFound bool
	recorder.records[0].Attrs(func(a slog.Attr) bool {
		if a.Key == "stack" {
			stackFound = true
			return false
		}
		return true
	})
	if !stackFound {
		t.Error("expected 'stack' attribute when error attribute is present, but not found")
	}
}

func TestErrorStackHandler_withoutErrorAttr_noStack(t *testing.T) {
	recorder := &recordingHandler{}
	handler := &ErrorStackHandler{Next: recorder}

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "info message", 0)
	r.AddAttrs(slog.String("key", "value"))

	if err := handler.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	var stackFound bool
	recorder.records[0].Attrs(func(a slog.Attr) bool {
		if a.Key == "stack" {
			stackFound = true
			return false
		}
		return true
	})
	if stackFound {
		t.Error("expected no 'stack' attribute when no error attribute is present")
	}
}

func TestErrorStackHandler_noAttrs_noStack(t *testing.T) {
	recorder := &recordingHandler{}
	handler := &ErrorStackHandler{Next: recorder}

	// error level but no attributes at all
	r := slog.NewRecord(time.Now(), slog.LevelError, "error with no attrs", 0)

	if err := handler.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	var stackFound bool
	recorder.records[0].Attrs(func(a slog.Attr) bool {
		if a.Key == "stack" {
			stackFound = true
			return false
		}
		return true
	})
	if stackFound {
		t.Error("expected no 'stack' when no attributes, even at error level")
	}
}

func TestErrorStackHandler_stackIsNonEmpty(t *testing.T) {
	recorder := &recordingHandler{}
	handler := &ErrorStackHandler{Next: recorder}

	r := slog.NewRecord(time.Now(), slog.LevelError, "test", 0)
	r.AddAttrs(slog.Any("err", errors.New("oops")))

	if err := handler.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	recorder.records[0].Attrs(func(a slog.Attr) bool {
		if a.Key != "stack" {
			return true
		}
		stack, ok := a.Value.Any().([]string)
		if !ok {
			t.Errorf("stack attribute should be []string, got %T", a.Value.Any())
			return false
		}
		if len(stack) == 0 {
			t.Error("stack should be non-empty when error attribute is present")
		}
		return false
	})
}

func TestErrorStackHandler_WithAttrs_returnsErrorStackHandler(t *testing.T) {
	recorder := &recordingHandler{}
	handler := &ErrorStackHandler{Next: recorder}

	wrapped := handler.WithAttrs([]slog.Attr{slog.String("k", "v")})
	if _, ok := wrapped.(*ErrorStackHandler); !ok {
		t.Errorf("WithAttrs should return *ErrorStackHandler, got %T", wrapped)
	}
}

func TestErrorStackHandler_WithGroup_returnsErrorStackHandler(t *testing.T) {
	recorder := &recordingHandler{}
	handler := &ErrorStackHandler{Next: recorder}

	wrapped := handler.WithGroup("mygroup")
	if _, ok := wrapped.(*ErrorStackHandler); !ok {
		t.Errorf("WithGroup should return *ErrorStackHandler, got %T", wrapped)
	}
}

func TestErrorStackHandler_nonErrorAny_noStack(t *testing.T) {
	recorder := &recordingHandler{}
	handler := &ErrorStackHandler{Next: recorder}

	// slog.Any with a non-error value should NOT trigger stack injection
	r := slog.NewRecord(time.Now(), slog.LevelError, "test", 0)
	r.AddAttrs(slog.Any("data", map[string]string{"key": "val"}))

	if err := handler.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	var stackFound bool
	recorder.records[0].Attrs(func(a slog.Attr) bool {
		if a.Key == "stack" {
			stackFound = true
			return false
		}
		return true
	})
	if stackFound {
		t.Error("expected no 'stack' attribute when slog.Any value is not an error")
	}
}
