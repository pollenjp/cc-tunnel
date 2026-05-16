package dockergce_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/cmclient"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/db"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/gce"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider/dockergce"
)

// runningVM is a small helper for tests in this file.
func runningVM(id, name, ip string, zeroAgentsSince *time.Time) *db.VMInstance {
	return &db.VMInstance{
		ID:              id,
		GCEInstanceName: name,
		Zone:            "asia-northeast1-b",
		InternalIP:      ip,
		Status:          "running",
		ZeroAgentsSince: zeroAgentsSince,
	}
}

// newReconcileProvider wires together a DockerGCEProvider with a mock GCE
// client, mock DB, and the supplied container-manager factory. It uses
// IdleCheckInterval=0 / VMReconcileInterval=0 so background goroutines are
// not started — the tests drive ReconcileVMs synchronously.
func newReconcileProvider(t *testing.T, repo *mockDBRepo, gceClient gce.GCEClient, cmFactory func(string) (cmclient.ContainerManager, error)) *dockergce.DockerGCEProvider {
	t.Helper()
	cfg := dockergce.DockerGCEConfig{
		ProjectID:                    "p",
		Zone:                         "asia-northeast1-b",
		MachineType:                  "e2-standard-2",
		VMImage:                      "img",
		AgentImage:                   "ar/agent:latest",
		AgentPort:                    9091,
		ContainerManagerPort:         9090,
		ZeroAgentsTimeout:            10 * time.Minute,
		ContainerManagerProbeTimeout: 1 * time.Second,
		ContainerManagerFactory:      cmFactory,
	}
	return dockergce.NewDockerGCEProvider(cfg, gceClient, repo)
}

// TestReconcileVMs_FirstZeroObservation sets zero_agents_since when the VM
// has no agents and the field is currently NULL.
func TestReconcileVMs_FirstZeroObservation(t *testing.T) {
	repo := newMockDBRepo()
	vm := runningVM("vm-1", "cc-tunnel-aaaa", "10.0.0.1", nil)
	repo.vms[vm.ID] = vm
	repo.vmByName[vm.GCEInstanceName] = vm

	gceClient := gce.NewMockGCEClient()
	cmMock := &cmclient.MockContainerManager{
		ListAgentsFunc: func(_ context.Context) ([]cmclient.AgentInfo, error) {
			return nil, nil // zero agents
		},
	}
	p := newReconcileProvider(t, repo, gceClient, func(string) (cmclient.ContainerManager, error) {
		return cmMock, nil
	})

	if err := p.ReconcileVMs(context.Background()); err != nil {
		t.Fatalf("ReconcileVMs: %v", err)
	}
	if vm.ZeroAgentsSince == nil {
		t.Fatalf("expected zero_agents_since to be set, got nil")
	}
	if time.Since(*vm.ZeroAgentsSince) > 5*time.Second {
		t.Fatalf("zero_agents_since looks stale: %v", *vm.ZeroAgentsSince)
	}
}

// TestReconcileVMs_DeletesAfterTimeout deletes the VM once zero_agents_since
// has aged past ZeroAgentsTimeout.
func TestReconcileVMs_DeletesAfterTimeout(t *testing.T) {
	repo := newMockDBRepo()
	old := time.Now().Add(-15 * time.Minute)
	vm := runningVM("vm-1", "cc-tunnel-aaaa", "10.0.0.1", &old)
	repo.vms[vm.ID] = vm
	repo.vmByName[vm.GCEInstanceName] = vm

	gceClient := gce.NewMockGCEClient()
	// Seed the GCE mock so DeleteInstance succeeds.
	if _, err := gceClient.CreateInstance(context.Background(), &gce.CreateInstanceRequest{
		Project: "p", Zone: "asia-northeast1-b", Name: vm.GCEInstanceName,
	}); err != nil {
		t.Fatalf("seed GCE mock: %v", err)
	}

	cmMock := &cmclient.MockContainerManager{
		ListAgentsFunc: func(_ context.Context) ([]cmclient.AgentInfo, error) {
			return nil, nil // zero agents
		},
	}
	p := newReconcileProvider(t, repo, gceClient, func(string) (cmclient.ContainerManager, error) {
		return cmMock, nil
	})

	if err := p.ReconcileVMs(context.Background()); err != nil {
		t.Fatalf("ReconcileVMs: %v", err)
	}
	if _, ok := repo.vms[vm.ID]; ok {
		t.Fatalf("expected VM to be deleted from DB, but it is still present")
	}
	if _, err := gceClient.GetInstance(context.Background(), "p", "asia-northeast1-b", vm.GCEInstanceName); err == nil {
		t.Fatalf("expected GCE instance to be deleted, but it still exists")
	}
}

// TestReconcileVMs_BelowTimeout_LeavesVM keeps the VM when zero_agents_since
// is set but has not yet aged past the timeout.
func TestReconcileVMs_BelowTimeout_LeavesVM(t *testing.T) {
	repo := newMockDBRepo()
	recent := time.Now().Add(-1 * time.Minute)
	vm := runningVM("vm-1", "cc-tunnel-aaaa", "10.0.0.1", &recent)
	repo.vms[vm.ID] = vm
	repo.vmByName[vm.GCEInstanceName] = vm

	gceClient := gce.NewMockGCEClient()
	deleteCalled := false
	gceClient.DeleteInstanceFn = func(_ context.Context, _, _, _ string) error {
		deleteCalled = true
		return nil
	}

	cmMock := &cmclient.MockContainerManager{
		ListAgentsFunc: func(_ context.Context) ([]cmclient.AgentInfo, error) {
			return nil, nil
		},
	}
	p := newReconcileProvider(t, repo, gceClient, func(string) (cmclient.ContainerManager, error) {
		return cmMock, nil
	})

	if err := p.ReconcileVMs(context.Background()); err != nil {
		t.Fatalf("ReconcileVMs: %v", err)
	}
	if deleteCalled {
		t.Fatalf("VM was deleted before timeout elapsed")
	}
	if _, ok := repo.vms[vm.ID]; !ok {
		t.Fatalf("expected VM to still be in DB")
	}
}

// TestReconcileVMs_NonZeroClearsTimestamp resets zero_agents_since back to
// NULL when at least one agent is observed.
func TestReconcileVMs_NonZeroClearsTimestamp(t *testing.T) {
	repo := newMockDBRepo()
	old := time.Now().Add(-3 * time.Minute)
	vm := runningVM("vm-1", "cc-tunnel-aaaa", "10.0.0.1", &old)
	repo.vms[vm.ID] = vm
	repo.vmByName[vm.GCEInstanceName] = vm

	gceClient := gce.NewMockGCEClient()
	cmMock := &cmclient.MockContainerManager{
		ListAgentsFunc: func(_ context.Context) ([]cmclient.AgentInfo, error) {
			return []cmclient.AgentInfo{{Name: "session-conv-1"}}, nil
		},
	}
	p := newReconcileProvider(t, repo, gceClient, func(string) (cmclient.ContainerManager, error) {
		return cmMock, nil
	})

	if err := p.ReconcileVMs(context.Background()); err != nil {
		t.Fatalf("ReconcileVMs: %v", err)
	}
	if vm.ZeroAgentsSince != nil {
		t.Fatalf("expected zero_agents_since to be cleared, got %v", *vm.ZeroAgentsSince)
	}
}

// TestReconcileVMs_ProbeFailure_LeavesVM treats container-manager errors as
// fail-safe: the VM is not deleted, and zero_agents_since is not advanced.
func TestReconcileVMs_ProbeFailure_LeavesVM(t *testing.T) {
	repo := newMockDBRepo()
	old := time.Now().Add(-1 * time.Hour)
	vm := runningVM("vm-1", "cc-tunnel-aaaa", "10.0.0.1", &old)
	repo.vms[vm.ID] = vm
	repo.vmByName[vm.GCEInstanceName] = vm

	gceClient := gce.NewMockGCEClient()
	deleteCalled := false
	gceClient.DeleteInstanceFn = func(_ context.Context, _, _, _ string) error {
		deleteCalled = true
		return nil
	}

	cmMock := &cmclient.MockContainerManager{
		ListAgentsFunc: func(_ context.Context) ([]cmclient.AgentInfo, error) {
			return nil, errors.New("connection refused")
		},
	}
	p := newReconcileProvider(t, repo, gceClient, func(string) (cmclient.ContainerManager, error) {
		return cmMock, nil
	})

	if err := p.ReconcileVMs(context.Background()); err != nil {
		t.Fatalf("ReconcileVMs: %v", err)
	}
	if deleteCalled {
		t.Fatalf("VM should not be deleted when container-manager probe fails")
	}
	if vm.ZeroAgentsSince == nil || !vm.ZeroAgentsSince.Equal(old) {
		t.Fatalf("zero_agents_since should be unchanged on probe failure; got %v want %v",
			vm.ZeroAgentsSince, old)
	}
}
