package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
)

func TestNewCloudLoggingHandler_FieldRenames(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewCloudLoggingHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	logger.Info("hello", "k", "v")

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v (raw=%q)", err, buf.String())
	}

	if sev, ok := got["severity"].(string); !ok || sev != "INFO" {
		t.Errorf("severity = %v, want %q", got["severity"], "INFO")
	}
	if msg, ok := got["message"].(string); !ok || msg != "hello" {
		t.Errorf("message = %v, want %q", got["message"], "hello")
	}
	if _, ok := got["timestamp"]; !ok {
		t.Errorf("timestamp missing in %v", got)
	}

	for _, banned := range []string{"level", "msg", "time"} {
		if _, ok := got[banned]; ok {
			t.Errorf("unexpected key %q present (Cloud Logging would not auto-promote it)", banned)
		}
	}
}

func TestNewCloudLoggingHandler_AttachesStackOnError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewCloudLoggingHandler(&buf, nil))

	logger.Error("boom", "err", errors.New("synthetic"))

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v (raw=%q)", err, buf.String())
	}
	if got["severity"] != "ERROR" {
		t.Errorf("severity = %v, want ERROR", got["severity"])
	}
	stack, ok := got["stack"].([]any)
	if !ok || len(stack) == 0 {
		t.Errorf("stack missing or empty: %v", got["stack"])
	}
}
