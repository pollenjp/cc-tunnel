package main

import (
	"testing"

	cloudrunsandbox "github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider/cloudrunsandbox"
	dockergce "github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider/dockergce"
	localprovider "github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider/local"
)

func TestNewProviderFromEnv_local(t *testing.T) {
	p, err := newProviderFromEnv("local")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(*localprovider.LocalDockerProvider); !ok {
		t.Errorf("expected *local.LocalDockerProvider, got %T", p)
	}
}

func TestNewProviderFromEnv_cloudRunSandbox(t *testing.T) {
	p, err := newProviderFromEnv("cloud_run_sandbox")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(*cloudrunsandbox.MockProvider); !ok {
		t.Errorf("expected *cloudrunsandbox.MockProvider, got %T", p)
	}
}

func TestNewProviderFromEnv_dockerGce(t *testing.T) {
	p, err := newProviderFromEnv("docker_gce")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(*dockergce.MockProvider); !ok {
		t.Errorf("expected *dockergce.MockProvider, got %T", p)
	}
}

func TestNewProviderFromEnv_empty(t *testing.T) {
	_, err := newProviderFromEnv("")
	if err == nil {
		t.Fatal("expected error for empty envVal, got nil")
	}
}

func TestNewProviderFromEnv_unknown(t *testing.T) {
	_, err := newProviderFromEnv("unknown")
	if err == nil {
		t.Fatal("expected error for unknown envVal, got nil")
	}
}
