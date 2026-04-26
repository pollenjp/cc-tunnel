package main

import (
	"context"
	"testing"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider/dockergce"
	cloudrunsandbox "github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider/cloudrunsandbox"
	localprovider "github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider/local"
)

func TestNewProviderFromEnv_local(t *testing.T) {
	p, err := newProviderFromEnv(context.Background(), "local", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(*localprovider.LocalDockerProvider); !ok {
		t.Errorf("expected *local.LocalDockerProvider, got %T", p)
	}
}

func TestNewProviderFromEnv_cloudRunSandbox(t *testing.T) {
	p, err := newProviderFromEnv(context.Background(), "cloud_run_sandbox", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(*cloudrunsandbox.MockProvider); !ok {
		t.Errorf("expected *cloudrunsandbox.MockProvider, got %T", p)
	}
}

func TestNewProviderFromEnv_dockerGce(t *testing.T) {
	// docker_gce requires Application Default Credentials (ADC).
	// Skip this test if GCE client creation fails (e.g., in CI without ADC).
	p, err := newProviderFromEnv(context.Background(), "docker_gce", nil)
	if err != nil {
		t.Skipf("skipping docker_gce test (GCE client unavailable): %v", err)
	}
	if _, ok := p.(*dockergce.DockerGCEProvider); !ok {
		t.Errorf("expected *dockergce.DockerGCEProvider, got %T", p)
	}
}

func TestNewProviderFromEnv_empty(t *testing.T) {
	_, err := newProviderFromEnv(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected error for empty envVal, got nil")
	}
}

func TestNewProviderFromEnv_unknown(t *testing.T) {
	_, err := newProviderFromEnv(context.Background(), "unknown", nil)
	if err == nil {
		t.Fatal("expected error for unknown envVal, got nil")
	}
}
