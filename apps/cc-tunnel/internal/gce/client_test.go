package gce_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/gce"
)

func TestMockGCEClient_CreateInstance(t *testing.T) {
	ctx := context.Background()
	client := gce.NewMockGCEClient()

	req := &gce.CreateInstanceRequest{
		Project:     "my-project",
		Zone:        "us-central1-a",
		Name:        "test-vm",
		MachineType: "n1-standard-1",
		Labels:      map[string]string{"env": "test"},
	}

	inst, err := client.CreateInstance(ctx, req)
	if err != nil {
		t.Fatalf("CreateInstance: unexpected error: %v", err)
	}
	if inst == nil {
		t.Fatal("CreateInstance: expected non-nil instance")
	}
	if inst.Name != "test-vm" {
		t.Errorf("CreateInstance: Name = %q, want %q", inst.Name, "test-vm")
	}
	if inst.Status != "RUNNING" {
		t.Errorf("CreateInstance: Status = %q, want %q", inst.Status, "RUNNING")
	}
}

func TestMockGCEClient_GetInstance(t *testing.T) {
	ctx := context.Background()
	client := gce.NewMockGCEClient()

	req := &gce.CreateInstanceRequest{
		Project:     "my-project",
		Zone:        "us-central1-a",
		Name:        "test-vm",
		MachineType: "n1-standard-1",
	}
	if _, err := client.CreateInstance(ctx, req); err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}

	inst, err := client.GetInstance(ctx, "my-project", "us-central1-a", "test-vm")
	if err != nil {
		t.Fatalf("GetInstance: unexpected error: %v", err)
	}
	if inst.Name != "test-vm" {
		t.Errorf("GetInstance: Name = %q, want %q", inst.Name, "test-vm")
	}
}

func TestMockGCEClient_GetInstance_NotFound(t *testing.T) {
	ctx := context.Background()
	client := gce.NewMockGCEClient()

	_, err := client.GetInstance(ctx, "my-project", "us-central1-a", "nonexistent")
	if err == nil {
		t.Fatal("GetInstance: expected error for nonexistent instance, got nil")
	}
}

func TestMockGCEClient_DeleteInstance(t *testing.T) {
	ctx := context.Background()
	client := gce.NewMockGCEClient()

	req := &gce.CreateInstanceRequest{
		Project:     "my-project",
		Zone:        "us-central1-a",
		Name:        "test-vm",
		MachineType: "n1-standard-1",
	}
	if _, err := client.CreateInstance(ctx, req); err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}

	if err := client.DeleteInstance(ctx, "my-project", "us-central1-a", "test-vm"); err != nil {
		t.Fatalf("DeleteInstance: unexpected error: %v", err)
	}

	// 削除後は GetInstance で not found エラーになること
	_, err := client.GetInstance(ctx, "my-project", "us-central1-a", "test-vm")
	if err == nil {
		t.Fatal("GetInstance after delete: expected error, got nil")
	}
}

func TestMockGCEClient_ListInstances(t *testing.T) {
	ctx := context.Background()
	client := gce.NewMockGCEClient()

	names := []string{"vm-1", "vm-2", "vm-3"}
	for _, name := range names {
		req := &gce.CreateInstanceRequest{
			Project:     "my-project",
			Zone:        "us-central1-a",
			Name:        name,
			MachineType: "n1-standard-1",
		}
		if _, err := client.CreateInstance(ctx, req); err != nil {
			t.Fatalf("CreateInstance(%q): %v", name, err)
		}
	}

	instances, err := client.ListInstances(ctx, "my-project", "us-central1-a")
	if err != nil {
		t.Fatalf("ListInstances: unexpected error: %v", err)
	}
	if len(instances) != 3 {
		t.Errorf("ListInstances: got %d instances, want 3", len(instances))
	}

	// 作成したインスタンスが全て含まれていることを確認
	found := make(map[string]bool)
	for _, inst := range instances {
		found[inst.Name] = true
	}
	for _, name := range names {
		if !found[name] {
			t.Errorf("ListInstances: instance %q not found in result", name)
		}
	}
}

func TestCreateInstance_WithNetworkTags(t *testing.T) {
	ctx := context.Background()
	client := gce.NewMockGCEClient()

	var capturedReq *gce.CreateInstanceRequest
	client.CreateInstanceFn = func(ctx context.Context, req *gce.CreateInstanceRequest) (*gce.Instance, error) {
		capturedReq = req
		return &gce.Instance{
			Name:      req.Name,
			Status:    "RUNNING",
			NetworkIP: "10.0.0.1",
			Labels:    req.Labels,
		}, nil
	}

	req := &gce.CreateInstanceRequest{
		Project:     "my-project",
		Zone:        "us-central1-a",
		Name:        "test-vm-tags",
		MachineType: "n1-standard-1",
		Tags:        []string{"cc-tunnel-agent"},
	}

	inst, err := client.CreateInstance(ctx, req)
	if err != nil {
		t.Fatalf("CreateInstance: unexpected error: %v", err)
	}
	if inst == nil {
		t.Fatal("CreateInstance: expected non-nil instance")
	}
	if capturedReq == nil {
		t.Fatal("CreateInstanceFn was not called")
	}
	if len(capturedReq.Tags) != 1 || capturedReq.Tags[0] != "cc-tunnel-agent" {
		t.Errorf("Tags = %v, want [cc-tunnel-agent]", capturedReq.Tags)
	}
}

func TestMockGCEClient_CustomHook(t *testing.T) {
	ctx := context.Background()
	client := gce.NewMockGCEClient()

	// カスタムフックで特定のエラーを返す
	client.CreateInstanceFn = func(ctx context.Context, req *gce.CreateInstanceRequest) (*gce.Instance, error) {
		return nil, fmt.Errorf("quota exceeded")
	}

	_, err := client.CreateInstance(ctx, &gce.CreateInstanceRequest{Name: "test"})
	if err == nil {
		t.Fatal("expected error from custom hook")
	}
}
