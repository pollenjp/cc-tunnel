package main

import (
	"testing"

	cloudrunsandbox "github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider/cloudrunsandbox"
	dockergce "github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider/dockergce"
	localprovider "github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider/local"
)

func TestNewProviderFromEnv_local(t *testing.T) {
	p, remote, err := newProviderFromEnv("local", "http://localhost:9091")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(*localprovider.Provider); !ok {
		t.Errorf("expected *local.Provider, got %T", p)
	}
	if remote == nil {
		t.Error("expected non-nil remote for local provider")
	}
}

func TestNewProviderFromEnv_cloudRunSandbox(t *testing.T) {
	p, remote, err := newProviderFromEnv("cloud_run_sandbox", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(*cloudrunsandbox.MockProvider); !ok {
		t.Errorf("expected *cloudrunsandbox.MockProvider, got %T", p)
	}
	if remote != nil {
		t.Errorf("expected nil remote for cloud_run_sandbox, got %v", remote)
	}
}

func TestNewProviderFromEnv_dockerGce(t *testing.T) {
	p, remote, err := newProviderFromEnv("docker_gce", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(*dockergce.MockProvider); !ok {
		t.Errorf("expected *dockergce.MockProvider, got %T", p)
	}
	if remote != nil {
		t.Errorf("expected nil remote for docker_gce, got %v", remote)
	}
}

func TestNewProviderFromEnv_empty(t *testing.T) {
	_, _, err := newProviderFromEnv("", "")
	if err == nil {
		t.Fatal("expected error for empty envVal, got nil")
	}
}

func TestNewProviderFromEnv_unknown(t *testing.T) {
	_, _, err := newProviderFromEnv("unknown", "")
	if err == nil {
		t.Fatal("expected error for unknown envVal, got nil")
	}
}
