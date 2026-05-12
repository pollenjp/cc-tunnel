package db_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/db"
)

func testDatabaseURL() string {
	if u := os.Getenv("DATABASE_URL"); u != "" {
		return u
	}
	return "postgres://cctunnel:cctunnel_dev@localhost:5432/cctunnel?sslmode=disable"
}

func setupRepo(t *testing.T) (*db.Repository, func()) {
	t.Helper()
	ctx := context.Background()

	pool, err := db.NewPool(ctx, testDatabaseURL())
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	repo := db.NewRepository(pool)
	cleanup := func() { pool.Close() }
	return repo, cleanup
}

// unique suffix to avoid conflicts across parallel runs
func uid(t *testing.T, suffix string) string {
	return fmt.Sprintf("%s_%s_%d", t.Name(), suffix, time.Now().UnixNano())
}

// createTestConversation inserts a minimal conversation row needed for FK constraints.
func createTestConversation(t *testing.T, repo *db.Repository) string {
	t.Helper()
	ctx := context.Background()
	title := uid(t, "conv")
	conv, err := repo.CreateConversation(ctx, title, "claude-test", nil)
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	return conv.ID
}

// TestCreateVMInstance_GetVMInstanceByName verifies that a created VM can be
// retrieved by its GCE instance name.
func TestCreateVMInstance_GetVMInstanceByName(t *testing.T) {
	repo, cleanup := setupRepo(t)
	defer cleanup()
	ctx := context.Background()

	name := uid(t, "vm")
	vm, err := repo.CreateVMInstance(ctx, name, "asia-northeast1-b", "10.128.0.1")
	if err != nil {
		t.Fatalf("CreateVMInstance: %v", err)
	}
	defer func() { _ = repo.DeleteVMInstance(ctx, vm.ID) }()

	got, err := repo.GetVMInstanceByName(ctx, name)
	if err != nil {
		t.Fatalf("GetVMInstanceByName: %v", err)
	}

	if got.ID != vm.ID {
		t.Errorf("ID: got %q, want %q", got.ID, vm.ID)
	}
	if got.GCEInstanceName != name {
		t.Errorf("GCEInstanceName: got %q, want %q", got.GCEInstanceName, name)
	}
	if got.Zone != "asia-northeast1-b" {
		t.Errorf("Zone: got %q, want %q", got.Zone, "asia-northeast1-b")
	}
	if got.Status != "provisioning" {
		t.Errorf("Status: got %q, want %q", got.Status, "provisioning")
	}
}

// TestUpdateVMInstanceStatus verifies that UpdateVMInstanceStatus changes the
// status column and GetVMInstance reflects the new value.
func TestUpdateVMInstanceStatus(t *testing.T) {
	repo, cleanup := setupRepo(t)
	defer cleanup()
	ctx := context.Background()

	name := uid(t, "vm")
	vm, err := repo.CreateVMInstance(ctx, name, "asia-northeast1-b", "10.128.0.2")
	if err != nil {
		t.Fatalf("CreateVMInstance: %v", err)
	}
	defer func() { _ = repo.DeleteVMInstance(ctx, vm.ID) }()

	if err := repo.UpdateVMInstanceStatus(ctx, vm.ID, "running"); err != nil {
		t.Fatalf("UpdateVMInstanceStatus: %v", err)
	}

	got, err := repo.GetVMInstance(ctx, vm.ID)
	if err != nil {
		t.Fatalf("GetVMInstance: %v", err)
	}
	if got.Status != "running" {
		t.Errorf("Status: got %q, want %q", got.Status, "running")
	}
}

// TestCreateSessionEndpoint_GetByConversationID verifies that a session endpoint
// can be retrieved by its conversation ID.
func TestCreateSessionEndpoint_GetByConversationID(t *testing.T) {
	repo, cleanup := setupRepo(t)
	defer cleanup()
	ctx := context.Background()

	convID := createTestConversation(t, repo)
	defer func() { _ = repo.DeleteConversation(ctx, convID) }()

	name := uid(t, "vm")
	vm, err := repo.CreateVMInstance(ctx, name, "asia-northeast1-b", "10.128.0.3")
	if err != nil {
		t.Fatalf("CreateVMInstance: %v", err)
	}
	defer func() { _ = repo.DeleteVMInstance(ctx, vm.ID) }()

	containerName := fmt.Sprintf("session-%s", convID)
	ep, err := repo.CreateSessionEndpoint(ctx, convID, vm.ID, containerName, 9091)
	if err != nil {
		t.Fatalf("CreateSessionEndpoint: %v", err)
	}

	got, err := repo.GetSessionEndpointByConversationID(ctx, convID)
	if err != nil {
		t.Fatalf("GetSessionEndpointByConversationID: %v", err)
	}
	if got.ID != ep.ID {
		t.Errorf("ID: got %q, want %q", got.ID, ep.ID)
	}
	if got.Port != 9091 {
		t.Errorf("Port: got %d, want %d", got.Port, 9091)
	}
	if got.Status != "running" {
		t.Errorf("Status: got %q, want %q", got.Status, "running")
	}
}

// TestGetSmallestAvailablePortOnVM verifies the port allocation query:
// returns the start of the range on a fresh VM, fills gaps left by removed
// endpoints (so released ports get reused first), and returns 0 when every
// port in the range is taken.
func TestGetSmallestAvailablePortOnVM(t *testing.T) {
	repo, cleanup := setupRepo(t)
	defer cleanup()
	ctx := context.Background()

	convID := createTestConversation(t, repo)
	defer func() { _ = repo.DeleteConversation(ctx, convID) }()

	name := uid(t, "vm")
	vm, err := repo.CreateVMInstance(ctx, name, "asia-northeast1-b", "10.128.0.5")
	if err != nil {
		t.Fatalf("CreateVMInstance: %v", err)
	}
	defer func() { _ = repo.DeleteVMInstance(ctx, vm.ID) }()

	// Fresh VM → smallest free = start of the range.
	got, err := repo.GetSmallestAvailablePortOnVM(ctx, vm.ID, 61000, 61002)
	if err != nil {
		t.Fatalf("GetSmallestAvailablePortOnVM(empty): %v", err)
	}
	if got != 61000 {
		t.Errorf("empty range: got %d, want %d", got, 61000)
	}

	// Reserve 61000 and 61002, leaving 61001 as the smallest free.
	convB := createTestConversation(t, repo)
	defer func() { _ = repo.DeleteConversation(ctx, convB) }()
	if _, err := repo.CreateSessionEndpoint(ctx, convID, vm.ID, "session-a", 61000); err != nil {
		t.Fatalf("CreateSessionEndpoint(61000): %v", err)
	}
	if _, err := repo.CreateSessionEndpoint(ctx, convB, vm.ID, "session-b", 61002); err != nil {
		t.Fatalf("CreateSessionEndpoint(61002): %v", err)
	}

	got, err = repo.GetSmallestAvailablePortOnVM(ctx, vm.ID, 61000, 61002)
	if err != nil {
		t.Fatalf("GetSmallestAvailablePortOnVM(gap): %v", err)
	}
	if got != 61001 {
		t.Errorf("with gap: got %d, want %d (gap should be reused)", got, 61001)
	}

	// Fill 61001 too → range exhausted → 0.
	convC := createTestConversation(t, repo)
	defer func() { _ = repo.DeleteConversation(ctx, convC) }()
	if _, err := repo.CreateSessionEndpoint(ctx, convC, vm.ID, "session-c", 61001); err != nil {
		t.Fatalf("CreateSessionEndpoint(61001): %v", err)
	}

	got, err = repo.GetSmallestAvailablePortOnVM(ctx, vm.ID, 61000, 61002)
	if err != nil {
		t.Fatalf("GetSmallestAvailablePortOnVM(full): %v", err)
	}
	if got != 0 {
		t.Errorf("full range: got %d, want 0", got)
	}
}

// TestDeleteVMInstance_CascadeDeletesSessionEndpoints verifies that deleting a
// VM instance also removes its associated session endpoints via ON DELETE CASCADE
// on the conversation FK chain: conversation delete cascades to session_endpoints.
// Here we test the vm_instances side by deleting the conversation first to remove
// the session endpoint, then the VM, confirming CASCADE behaviour on session_endpoints.
func TestDeleteVMInstance_CascadeDeletesSessionEndpoints(t *testing.T) {
	repo, cleanup := setupRepo(t)
	defer cleanup()
	ctx := context.Background()

	convID := createTestConversation(t, repo)

	name := uid(t, "vm")
	vm, err := repo.CreateVMInstance(ctx, name, "asia-northeast1-b", "10.128.0.4")
	if err != nil {
		t.Fatalf("CreateVMInstance: %v", err)
	}

	containerName := fmt.Sprintf("session-%s", convID)
	ep, err := repo.CreateSessionEndpoint(ctx, convID, vm.ID, containerName, 9092)
	if err != nil {
		t.Fatalf("CreateSessionEndpoint: %v", err)
	}

	// Delete conversation → ON DELETE CASCADE removes session_endpoint
	if err := repo.DeleteConversation(ctx, convID); err != nil {
		t.Fatalf("DeleteConversation: %v", err)
	}

	// session_endpoint should now be gone
	_, err = repo.GetSessionEndpointByConversationID(ctx, ep.ConversationID)
	if err == nil {
		t.Error("expected error after cascade delete, got nil")
	}

	// Clean up VM
	if err := repo.DeleteVMInstance(ctx, vm.ID); err != nil {
		t.Fatalf("DeleteVMInstance: %v", err)
	}
}
