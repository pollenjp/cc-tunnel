package dockergce_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/db"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/dockerhost"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/gce"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider/dockergce"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

// --- mock DB repository ---

type mockDBRepo struct {
	mu        sync.Mutex
	endpoints map[string]*db.SessionEndpoint // convID → endpoint
	vms       map[string]*db.VMInstance       // id → vm
	vmByName  map[string]*db.VMInstance       // name → vm

	getEndpointErr    error
	createEndpointErr error
	availableVMErr    error // nil = return first vm, non-nil = return error
	createVMErr       error
	maxPortOnVM       int // returned by GetMaxPortOnVM (0 = no containers)

	// Optional overrides for more granular test control (if non-nil, used instead of defaults).
	createEndpointFn func(ctx context.Context, conversationID, vmInstanceID, containerName string, port int) (*db.SessionEndpoint, error)
	maxPortOnVMFn    func(ctx context.Context, vmID string) (int, error)
}

func newMockDBRepo() *mockDBRepo {
	return &mockDBRepo{
		endpoints: make(map[string]*db.SessionEndpoint),
		vms:       make(map[string]*db.VMInstance),
		vmByName:  make(map[string]*db.VMInstance),
	}
}

func (m *mockDBRepo) GetSessionEndpointByConversationID(_ context.Context, conversationID string) (*db.SessionEndpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getEndpointErr != nil {
		return nil, m.getEndpointErr
	}
	ep, ok := m.endpoints[conversationID]
	if !ok {
		return nil, errors.New("not found")
	}
	return ep, nil
}

func (m *mockDBRepo) CreateSessionEndpoint(ctx context.Context, conversationID, vmInstanceID, containerName string, port int) (*db.SessionEndpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createEndpointFn != nil {
		return m.createEndpointFn(ctx, conversationID, vmInstanceID, containerName, port)
	}
	if m.createEndpointErr != nil {
		return nil, m.createEndpointErr
	}
	ep := &db.SessionEndpoint{
		ID:             "ep-" + conversationID,
		ConversationID: conversationID,
		VMInstanceID:   vmInstanceID,
		ContainerName:  containerName,
		Port:           port,
		Status:         "running",
		LastActivity:   time.Now(),
	}
	m.endpoints[conversationID] = ep
	return ep, nil
}

func (m *mockDBRepo) UpdateSessionEndpointLastActivity(_ context.Context, _ string) error {
	return nil
}

func (m *mockDBRepo) GetVMInstance(_ context.Context, id string) (*db.VMInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	vm, ok := m.vms[id]
	if !ok {
		return nil, errors.New("vm not found")
	}
	return vm, nil
}

func (m *mockDBRepo) GetAvailableVMInstance(_ context.Context, _ int) (*db.VMInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.availableVMErr != nil {
		return nil, m.availableVMErr
	}
	for _, vm := range m.vms {
		return vm, nil
	}
	return nil, errors.New("no available VM")
}

func (m *mockDBRepo) CreateVMInstance(_ context.Context, gceInstanceName, zone, internalIP string) (*db.VMInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createVMErr != nil {
		return nil, m.createVMErr
	}
	vm := &db.VMInstance{
		ID:              "vm-" + gceInstanceName,
		GCEInstanceName: gceInstanceName,
		Zone:            zone,
		InternalIP:      internalIP,
		Status:          "running",
	}
	m.vms[vm.ID] = vm
	m.vmByName[gceInstanceName] = vm
	return vm, nil
}

func (m *mockDBRepo) UpdateVMInstanceIP(_ context.Context, id, internalIP string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if vm, ok := m.vms[id]; ok {
		vm.InternalIP = internalIP
	}
	return nil
}

func (m *mockDBRepo) UpdateVMInstanceStatus(_ context.Context, id, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if vm, ok := m.vms[id]; ok {
		vm.Status = status
	}
	return nil
}

func (m *mockDBRepo) IncrementVMActiveContainers(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if vm, ok := m.vms[id]; ok {
		vm.ActiveContainers++
	}
	return nil
}

func (m *mockDBRepo) DecrementVMActiveContainers(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if vm, ok := m.vms[id]; ok {
		if vm.ActiveContainers > 0 {
			vm.ActiveContainers--
		}
	}
	return nil
}

func (m *mockDBRepo) ListIdleSessionEndpoints(_ context.Context, _ time.Duration) ([]*db.SessionEndpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*db.SessionEndpoint
	for _, ep := range m.endpoints {
		result = append(result, ep)
	}
	return result, nil
}

func (m *mockDBRepo) DeleteSessionEndpoint(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, ep := range m.endpoints {
		if ep.ID == id {
			delete(m.endpoints, k)
			return nil
		}
	}
	return nil
}

func (m *mockDBRepo) ListIdleVMInstances(_ context.Context, _ time.Duration) ([]*db.VMInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*db.VMInstance
	for _, vm := range m.vms {
		result = append(result, vm)
	}
	return result, nil
}

func (m *mockDBRepo) DeleteVMInstance(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if vm, ok := m.vms[id]; ok {
		delete(m.vmByName, vm.GCEInstanceName)
		delete(m.vms, id)
	}
	return nil
}

func (m *mockDBRepo) GetMaxPortOnVM(ctx context.Context, vmID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.maxPortOnVMFn != nil {
		return m.maxPortOnVMFn(ctx, vmID)
	}
	return m.maxPortOnVM, nil
}

// --- helpers ---

// fakeAgentServer starts a fake cc-remote-agent HTTP server that returns the given NDJSON events.
func fakeAgentServer(t *testing.T, events []remoteclient.StreamEvent) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/status":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(remoteclient.AuthStatus{LoggedIn: true})
		case "/execute":
			w.Header().Set("Content-Type", "application/x-ndjson")
			w.WriteHeader(http.StatusOK)
			for _, ev := range events {
				line, _ := json.Marshal(ev)
				_, _ = w.Write(append(line, '\n'))
			}
		default:
			http.NotFound(w, r)
		}
	}))
}

// noopContainerManagerFactory returns a ContainerManager that always succeeds and reports IsReady=true.
func noopContainerManagerFactory() func(string) (dockerhost.ContainerManager, error) {
	return func(_ string) (dockerhost.ContainerManager, error) {
		return &dockerhost.MockContainerManager{
			IsReadyFunc: func(_ context.Context) bool { return true },
		}, nil
	}
}

func shortTimeoutConfig() dockergce.DockerGCEConfig {
	return dockergce.DockerGCEConfig{
		ProjectID:               "test-project",
		Zone:                    "us-central1-a",
		MachineType:             "e2-medium",
		AgentImage:              "cc-remote-agent:test",
		AgentPort:               0, // will be replaced by httptest server port
		IdleTimeout:             time.Minute,
		MaxContainers:           1,
		VMReadyTimeout:          200 * time.Millisecond,
		AgentReadyTimeout:       200 * time.Millisecond,
		PollInterval:            10 * time.Millisecond,
		ContainerManagerFactory: noopContainerManagerFactory(),
	}
}

// --- tests ---

func TestDockerGCEProvider_Execute_NewSession(t *testing.T) {
	resultEvent := remoteclient.StreamEvent{
		Type:      "result",
		SessionID: "sess-gce-001",
		Result:    "success",
	}
	srv := fakeAgentServer(t, []remoteclient.StreamEvent{resultEvent})
	defer srv.Close()

	repo := newMockDBRepo()
	repo.availableVMErr = errors.New("no VM") // force VM creation

	cfg := shortTimeoutConfig()
	cfg.AgentPort = 9091

	mockGCEClient := &customMockGCEClient{
		createFn: func(_ context.Context, req *gce.CreateInstanceRequest) (*gce.Instance, error) {
			return &gce.Instance{
				Name:      req.Name,
				Status:    "RUNNING",
				NetworkIP: "127.0.0.1",
			}, nil
		},
		getFn: func(_ context.Context, _, _, name string) (*gce.Instance, error) {
			return &gce.Instance{
				Name:      name,
				Status:    "RUNNING",
				NetworkIP: "127.0.0.1",
			}, nil
		},
	}

	p := dockergce.NewDockerGCEProviderWithClientFactory(cfg, mockGCEClient, repo, func(_ string) *remoteclient.Client {
		return remoteclient.NewClient(srv.URL)
	})

	req := remoteclient.Request{ConversationID: "conv-new-001", Prompt: "hello"}
	var received []remoteclient.StreamEvent
	sessionID, err := p.Execute(context.Background(), req, func(e remoteclient.StreamEvent) {
		received = append(received, e)
	})

	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if sessionID != "sess-gce-001" {
		t.Errorf("sessionID = %q, want %q", sessionID, "sess-gce-001")
	}
	if len(received) == 0 {
		t.Error("Execute: no events received")
	}
}

func TestDockerGCEProvider_Execute_ExistingSession(t *testing.T) {
	resultEvent := remoteclient.StreamEvent{
		Type:      "result",
		SessionID: "sess-gce-existing",
		Result:    "success",
	}
	srv := fakeAgentServer(t, []remoteclient.StreamEvent{resultEvent})
	defer srv.Close()

	repo := newMockDBRepo()
	// Pre-populate an existing VM and endpoint
	vm := &db.VMInstance{
		ID:              "vm-existing",
		GCEInstanceName: "cc-tunnel-existing",
		Zone:            "us-central1-a",
		InternalIP:      "10.0.0.2",
		Status:          "running",
	}
	repo.vms[vm.ID] = vm
	ep := &db.SessionEndpoint{
		ID:             "ep-existing",
		ConversationID: "conv-existing",
		VMInstanceID:   vm.ID,
		ContainerName:  "session-conv-existing",
		Port:           9091,
		Status:         "running",
	}
	repo.endpoints["conv-existing"] = ep

	cfg := shortTimeoutConfig()
	cfg.AgentPort = 9091

	p := dockergce.NewDockerGCEProviderWithClientFactory(cfg, gce.NewMockGCEClient(), repo, func(_ string) *remoteclient.Client {
		return remoteclient.NewClient(srv.URL)
	})

	req := remoteclient.Request{ConversationID: "conv-existing", Prompt: "hello again"}
	var received []remoteclient.StreamEvent
	sessionID, err := p.Execute(context.Background(), req, func(e remoteclient.StreamEvent) {
		received = append(received, e)
	})

	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if sessionID != "sess-gce-existing" {
		t.Errorf("sessionID = %q, want %q", sessionID, "sess-gce-existing")
	}
	if len(received) == 0 {
		t.Error("Execute: no events received")
	}
}

func TestDockerGCEProvider_GetOrCreateEndpoint_Concurrent(t *testing.T) {
	// AF002: concurrent calls with same conversationID should create only one VM
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/status":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(remoteclient.AuthStatus{LoggedIn: true})
		case "/execute":
			w.Header().Set("Content-Type", "application/x-ndjson")
			event := remoteclient.StreamEvent{Type: "result", SessionID: "sess-concurrent"}
			line, _ := json.Marshal(event)
			_, _ = w.Write(append(line, '\n'))
		}
	}))
	defer srv.Close()

	var createCount int
	var mu sync.Mutex

	mockGCEClient := &customMockGCEClient{
		createFn: func(_ context.Context, req *gce.CreateInstanceRequest) (*gce.Instance, error) {
			mu.Lock()
			createCount++
			mu.Unlock()
			return &gce.Instance{Name: req.Name, Status: "RUNNING", NetworkIP: "10.0.1.1"}, nil
		},
		getFn: func(_ context.Context, _, _, name string) (*gce.Instance, error) {
			return &gce.Instance{Name: name, Status: "RUNNING", NetworkIP: "10.0.1.1"}, nil
		},
	}

	repo := newMockDBRepo()
	repo.availableVMErr = errors.New("no VM") // force VM creation path

	cfg := shortTimeoutConfig()
	p := dockergce.NewDockerGCEProviderWithClientFactory(cfg, mockGCEClient, repo, func(_ string) *remoteclient.Client {
		return remoteclient.NewClient(srv.URL)
	})

	const goroutines = 10
	convID := "conv-concurrent-001"
	var wg sync.WaitGroup
	errs := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = p.Execute(context.Background(), remoteclient.Request{ConversationID: convID, Prompt: "hi"}, func(remoteclient.StreamEvent) {})
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: Execute error: %v", i, err)
		}
	}

	mu.Lock()
	if createCount != 1 {
		t.Errorf("GCE CreateInstance called %d times, want 1 (singleflight dedup)", createCount)
	}
	mu.Unlock()
}

func TestDockerGCEProvider_WaitForVMReady_Timeout(t *testing.T) {
	// MockGCEClient always returns STAGING → stage1 should timeout
	mockGCEClient := &customMockGCEClient{
		createFn: func(_ context.Context, req *gce.CreateInstanceRequest) (*gce.Instance, error) {
			return &gce.Instance{Name: req.Name, Status: "STAGING", NetworkIP: ""}, nil
		},
		getFn: func(_ context.Context, _, _, name string) (*gce.Instance, error) {
			return &gce.Instance{Name: name, Status: "STAGING", NetworkIP: ""}, nil
		},
	}

	repo := newMockDBRepo()
	repo.availableVMErr = errors.New("no VM")

	cfg := dockergce.DockerGCEConfig{
		ProjectID:         "test-project",
		Zone:              "us-central1-a",
		AgentImage:        "cc-remote-agent:test",
		AgentPort:         9091,
		MaxContainers:     1,
		IdleTimeout:       time.Minute,
		VMReadyTimeout:    50 * time.Millisecond, // very short
		AgentReadyTimeout: 50 * time.Millisecond,
		PollInterval:      5 * time.Millisecond,
		// No ContainerManagerFactory needed: stage1 times out before stage2
	}

	p := dockergce.NewDockerGCEProvider(cfg, mockGCEClient, repo)

	_, err := p.Execute(context.Background(), remoteclient.Request{ConversationID: "conv-timeout"}, func(remoteclient.StreamEvent) {})
	if err == nil {
		t.Fatal("WaitForVMReady: expected timeout error, got nil")
	}
}

func TestDockerGCEProvider_CleanupOrphans(t *testing.T) {
	var stopCalls, removeCalls []string
	var mu sync.Mutex

	deletedEndpoints := make(map[string]bool)
	deletedVMs := make(map[string]bool)

	factory := func(_ string) (dockerhost.ContainerManager, error) {
		return &dockerhost.MockContainerManager{
			IsReadyFunc: func(_ context.Context) bool { return true },
			StopContainerFunc: func(_ context.Context, name string) error {
				mu.Lock()
				stopCalls = append(stopCalls, name)
				mu.Unlock()
				return nil
			},
			RemoveContainerFunc: func(_ context.Context, name string) error {
				mu.Lock()
				removeCalls = append(removeCalls, name)
				mu.Unlock()
				return nil
			},
		}, nil
	}

	repo := &cleanupMockDBRepo{
		endpointToReturn: []*db.SessionEndpoint{
			{ID: "ep-idle-1", VMInstanceID: "vm-1", ContainerName: "session-conv1"},
			{ID: "ep-idle-2", VMInstanceID: "vm-2", ContainerName: "session-conv2"},
		},
		vmsToReturn: []*db.VMInstance{
			{ID: "vm-1", GCEInstanceName: "cc-tunnel-vm1", Zone: "us-central1-a", InternalIP: "10.0.0.1"},
			{ID: "vm-2", GCEInstanceName: "cc-tunnel-vm2", Zone: "us-central1-a", InternalIP: "10.0.0.2"},
		},
		deletedEndpoints: deletedEndpoints,
		deletedVMs:       deletedVMs,
	}

	mockGCEClient := gce.NewMockGCEClient()

	cfg := dockergce.DockerGCEConfig{
		ProjectID:               "test-project",
		Zone:                    "us-central1-a",
		IdleTimeout:             time.Minute,
		ContainerManagerFactory: factory,
	}

	p := dockergce.NewDockerGCEProvider(cfg, mockGCEClient, repo)

	if err := p.CleanupOrphans(context.Background()); err != nil {
		t.Fatalf("CleanupOrphans: unexpected error: %v", err)
	}

	if len(deletedEndpoints) != 2 {
		t.Errorf("deleted %d endpoints, want 2", len(deletedEndpoints))
	}
	if len(deletedVMs) != 2 {
		t.Errorf("deleted %d VMs, want 2", len(deletedVMs))
	}
	mu.Lock()
	defer mu.Unlock()
	if len(stopCalls) != 2 {
		t.Errorf("StopContainer called %d times, want 2", len(stopCalls))
	}
	if len(removeCalls) != 2 {
		t.Errorf("RemoveContainer called %d times, want 2", len(removeCalls))
	}
}

// TestGetOrCreateEndpoint_NewVM: VMなし → 新規GCE起動 → コンテナ起動
func TestGetOrCreateEndpoint_NewVM(t *testing.T) {
	srv := fakeAgentServer(t, []remoteclient.StreamEvent{
		{Type: "result", SessionID: "sess-newvm", Result: "success"},
	})
	defer srv.Close()

	var runCalls []struct {
		image, name string
		hostPort    int
	}
	var runMu sync.Mutex

	repo := newMockDBRepo()
	repo.availableVMErr = errors.New("no VM")
	repo.maxPortOnVM = 0 // no containers yet → next port = portRangeStart (9091)

	cfg := shortTimeoutConfig()
	cfg.AgentPort = 9091
	cfg.PortRangeStart = 9091
	cfg.ContainerManagerFactory = func(_ string) (dockerhost.ContainerManager, error) {
		return &dockerhost.MockContainerManager{
			IsReadyFunc: func(_ context.Context) bool { return true },
			RunAgentContainerFunc: func(_ context.Context, image, name string, hostPort, _ int) error {
				runMu.Lock()
				runCalls = append(runCalls, struct {
					image, name string
					hostPort    int
				}{image, name, hostPort})
				runMu.Unlock()
				return nil
			},
		}, nil
	}

	mockGCEClient := &customMockGCEClient{
		createFn: func(_ context.Context, req *gce.CreateInstanceRequest) (*gce.Instance, error) {
			return &gce.Instance{Name: req.Name, Status: "RUNNING", NetworkIP: "10.1.1.1"}, nil
		},
		getFn: func(_ context.Context, _, _, name string) (*gce.Instance, error) {
			return &gce.Instance{Name: name, Status: "RUNNING", NetworkIP: "10.1.1.1"}, nil
		},
	}

	p := dockergce.NewDockerGCEProviderWithClientFactory(cfg, mockGCEClient, repo, func(_ string) *remoteclient.Client {
		return remoteclient.NewClient(srv.URL)
	})

	req := remoteclient.Request{ConversationID: "conv-newvm", Prompt: "hi"}
	sessionID, err := p.Execute(context.Background(), req, func(remoteclient.StreamEvent) {})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if sessionID != "sess-newvm" {
		t.Errorf("sessionID = %q, want %q", sessionID, "sess-newvm")
	}

	runMu.Lock()
	defer runMu.Unlock()
	if len(runCalls) == 0 {
		t.Fatal("RunAgentContainer was not called")
	}
	if runCalls[0].hostPort != 9091 {
		t.Errorf("hostPort = %d, want 9091", runCalls[0].hostPort)
	}
	if runCalls[0].name != "session-conv-newvm" {
		t.Errorf("containerName = %q, want %q", runCalls[0].name, "session-conv-newvm")
	}
}

// TestGetOrCreateEndpoint_ExistingVM: 空きVM あり → 同VM上に追加コンテナ（ポート+1）
func TestGetOrCreateEndpoint_ExistingVM(t *testing.T) {
	srv := fakeAgentServer(t, []remoteclient.StreamEvent{
		{Type: "result", SessionID: "sess-existing-vm", Result: "success"},
	})
	defer srv.Close()

	var runCalls []struct{ hostPort int }
	var runMu sync.Mutex

	repo := newMockDBRepo()
	// Existing VM with 1 container on port 9091
	existingVM := &db.VMInstance{
		ID:               "vm-abc",
		GCEInstanceName:  "cc-tunnel-abc",
		Zone:             "us-central1-a",
		InternalIP:       "10.2.2.2",
		Status:           "running",
		ActiveContainers: 1,
	}
	repo.vms[existingVM.ID] = existingVM
	repo.maxPortOnVM = 9091 // 1 container already on 9091 → next = 9092

	cfg := shortTimeoutConfig()
	cfg.AgentPort = 9091
	cfg.PortRangeStart = 9091
	cfg.MaxContainers = 10
	cfg.ContainerManagerFactory = func(_ string) (dockerhost.ContainerManager, error) {
		return &dockerhost.MockContainerManager{
			IsReadyFunc: func(_ context.Context) bool { return true },
			RunAgentContainerFunc: func(_ context.Context, _, _ string, hostPort, _ int) error {
				runMu.Lock()
				runCalls = append(runCalls, struct{ hostPort int }{hostPort})
				runMu.Unlock()
				return nil
			},
		}, nil
	}

	p := dockergce.NewDockerGCEProviderWithClientFactory(cfg, gce.NewMockGCEClient(), repo, func(_ string) *remoteclient.Client {
		return remoteclient.NewClient(srv.URL)
	})

	req := remoteclient.Request{ConversationID: "conv-second", Prompt: "hi"}
	_, err := p.Execute(context.Background(), req, func(remoteclient.StreamEvent) {})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	runMu.Lock()
	defer runMu.Unlock()
	if len(runCalls) == 0 {
		t.Fatal("RunAgentContainer was not called")
	}
	if runCalls[0].hostPort != 9092 {
		t.Errorf("hostPort = %d, want 9092 (second container on same VM)", runCalls[0].hostPort)
	}
}

// TestGetOrCreateEndpoint_ScaleOut: active=MaxContainers → 新VM起動
func TestGetOrCreateEndpoint_ScaleOut(t *testing.T) {
	srv := fakeAgentServer(t, []remoteclient.StreamEvent{
		{Type: "result", SessionID: "sess-scaleout", Result: "success"},
	})
	defer srv.Close()

	var gceCreateCount int32

	repo := newMockDBRepo()
	// Force no available VM (all VMs full)
	repo.availableVMErr = errors.New("no available VM")

	cfg := shortTimeoutConfig()
	cfg.AgentPort = 9091
	cfg.MaxContainers = 2
	cfg.ContainerManagerFactory = noopContainerManagerFactory()

	mockGCEClient := &customMockGCEClient{
		createFn: func(_ context.Context, req *gce.CreateInstanceRequest) (*gce.Instance, error) {
			atomic.AddInt32(&gceCreateCount, 1)
			return &gce.Instance{Name: req.Name, Status: "RUNNING", NetworkIP: "10.3.3.3"}, nil
		},
		getFn: func(_ context.Context, _, _, name string) (*gce.Instance, error) {
			return &gce.Instance{Name: name, Status: "RUNNING", NetworkIP: "10.3.3.3"}, nil
		},
	}

	p := dockergce.NewDockerGCEProviderWithClientFactory(cfg, mockGCEClient, repo, func(_ string) *remoteclient.Client {
		return remoteclient.NewClient(srv.URL)
	})

	req := remoteclient.Request{ConversationID: "conv-scaleout", Prompt: "hi"}
	_, err := p.Execute(context.Background(), req, func(remoteclient.StreamEvent) {})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if n := atomic.LoadInt32(&gceCreateCount); n != 1 {
		t.Errorf("GCE CreateInstance called %d times, want 1", n)
	}
}

// TestCleanupOrphans_StopsContainer: アイドルエンドポイントのコンテナが停止・削除される
func TestCleanupOrphans_StopsContainer(t *testing.T) {
	var stopped, removed []string
	var mu sync.Mutex

	repo := &cleanupMockDBRepo{
		endpointToReturn: []*db.SessionEndpoint{
			{ID: "ep-1", VMInstanceID: "vm-x", ContainerName: "session-abc"},
		},
		vmsToReturn: []*db.VMInstance{
			{ID: "vm-x", GCEInstanceName: "cc-tunnel-x", Zone: "us-central1-a", InternalIP: "10.9.9.9"},
		},
		deletedEndpoints: make(map[string]bool),
		deletedVMs:       make(map[string]bool),
	}

	factory := func(_ string) (dockerhost.ContainerManager, error) {
		return &dockerhost.MockContainerManager{
			StopContainerFunc: func(_ context.Context, name string) error {
				mu.Lock()
				stopped = append(stopped, name)
				mu.Unlock()
				return nil
			},
			RemoveContainerFunc: func(_ context.Context, name string) error {
				mu.Lock()
				removed = append(removed, name)
				mu.Unlock()
				return nil
			},
		}, nil
	}

	cfg := dockergce.DockerGCEConfig{
		ProjectID:               "test-project",
		Zone:                    "us-central1-a",
		IdleTimeout:             time.Minute,
		ContainerManagerFactory: factory,
	}

	p := dockergce.NewDockerGCEProvider(cfg, gce.NewMockGCEClient(), repo)
	if err := p.CleanupOrphans(context.Background()); err != nil {
		t.Fatalf("CleanupOrphans: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(stopped) != 1 || stopped[0] != "session-abc" {
		t.Errorf("StopContainer calls = %v, want [session-abc]", stopped)
	}
	if len(removed) != 1 || removed[0] != "session-abc" {
		t.Errorf("RemoveContainer calls = %v, want [session-abc]", removed)
	}
}

// TestGetOrCreateEndpoint_AgentReadyFail_OrphanCleanup: RunAgentContainer 成功後に
// waitForAgentReady が失敗した場合、コンテナが Stop/Remove される（orphan fix）
func TestGetOrCreateEndpoint_AgentReadyFail_OrphanCleanup(t *testing.T) {
	// Agent server that never becomes ready
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	var stopped, removed []string
	var mu sync.Mutex

	factory := func(_ string) (dockerhost.ContainerManager, error) {
		return &dockerhost.MockContainerManager{
			IsReadyFunc: func(_ context.Context) bool { return true },
			RunAgentContainerFunc: func(_ context.Context, _, _ string, _, _ int) error {
				return nil
			},
			StopContainerFunc: func(_ context.Context, name string) error {
				mu.Lock()
				stopped = append(stopped, name)
				mu.Unlock()
				return nil
			},
			RemoveContainerFunc: func(_ context.Context, name string) error {
				mu.Lock()
				removed = append(removed, name)
				mu.Unlock()
				return nil
			},
		}, nil
	}

	repo := newMockDBRepo()
	repo.availableVMErr = errors.New("no VM")

	cfg := shortTimeoutConfig()
	cfg.AgentPort = 9091
	cfg.AgentReadyTimeout = 50 * time.Millisecond
	cfg.ContainerManagerFactory = factory

	mockGCEClient := &customMockGCEClient{
		createFn: func(_ context.Context, req *gce.CreateInstanceRequest) (*gce.Instance, error) {
			return &gce.Instance{Name: req.Name, Status: "RUNNING", NetworkIP: "127.0.0.1"}, nil
		},
		getFn: func(_ context.Context, _, _, name string) (*gce.Instance, error) {
			return &gce.Instance{Name: name, Status: "RUNNING", NetworkIP: "127.0.0.1"}, nil
		},
	}

	p := dockergce.NewDockerGCEProviderWithClientFactory(cfg, mockGCEClient, repo, func(_ string) *remoteclient.Client {
		return remoteclient.NewClient(srv.URL)
	})

	_, err := p.Execute(context.Background(), remoteclient.Request{ConversationID: "conv-orphan"}, func(remoteclient.StreamEvent) {})
	if err == nil {
		t.Fatal("expected error when agent not ready, got nil")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(stopped) == 0 {
		t.Error("expected StopContainer to be called for orphan cleanup")
	}
	if len(removed) == 0 {
		t.Error("expected RemoveContainer to be called for orphan cleanup")
	}
}

// TestGetOrCreateEndpoint_PortCollisionRetry: CreateSessionEndpoint がポート衝突で失敗した場合、
// コンテナを停止・削除して別ポートで再試行し成功する
func TestGetOrCreateEndpoint_PortCollisionRetry(t *testing.T) {
	srv := fakeAgentServer(t, []remoteclient.StreamEvent{
		{Type: "result", SessionID: "sess-retry", Result: "success"},
	})
	defer srv.Close()

	var stopped, removed []string
	var runPorts []int
	var mu sync.Mutex

	factory := func(_ string) (dockerhost.ContainerManager, error) {
		return &dockerhost.MockContainerManager{
			IsReadyFunc: func(_ context.Context) bool { return true },
			RunAgentContainerFunc: func(_ context.Context, _, _ string, hostPort, _ int) error {
				mu.Lock()
				runPorts = append(runPorts, hostPort)
				mu.Unlock()
				return nil
			},
			StopContainerFunc: func(_ context.Context, name string) error {
				mu.Lock()
				stopped = append(stopped, name)
				mu.Unlock()
				return nil
			},
			RemoveContainerFunc: func(_ context.Context, name string) error {
				mu.Lock()
				removed = append(removed, name)
				mu.Unlock()
				return nil
			},
		}, nil
	}

	repo := newMockDBRepo()
	repo.availableVMErr = errors.New("no VM")

	// GetMaxPortOnVM: first call returns 0 (port=9091), second call returns 9091 (port=9092)
	var maxPortCallCount int32
	repo.maxPortOnVMFn = func(_ context.Context, _ string) (int, error) {
		n := int(atomic.AddInt32(&maxPortCallCount, 1))
		if n == 1 {
			return 0, nil // → port 9091 (will collide)
		}
		return 9091, nil // → port 9092 (succeeds)
	}

	// CreateSessionEndpoint: fail on first call (port collision), succeed on second
	var createCallCount int32
	repo.createEndpointFn = func(_ context.Context, conversationID, vmInstanceID, containerName string, port int) (*db.SessionEndpoint, error) {
		n := int(atomic.AddInt32(&createCallCount, 1))
		if n == 1 {
			return nil, errors.New("unique constraint violation: session_endpoints_vm_port_unique")
		}
		ep := &db.SessionEndpoint{
			ID:             "ep-" + conversationID,
			ConversationID: conversationID,
			VMInstanceID:   vmInstanceID,
			ContainerName:  containerName,
			Port:           port,
			Status:         "running",
			LastActivity:   time.Now(),
		}
		return ep, nil
	}

	cfg := shortTimeoutConfig()
	cfg.AgentPort = 9091
	cfg.PortRangeStart = 9091
	cfg.ContainerManagerFactory = factory
	cfg.PollInterval = 5 * time.Millisecond // short backoff for retry

	mockGCEClient := &customMockGCEClient{
		createFn: func(_ context.Context, req *gce.CreateInstanceRequest) (*gce.Instance, error) {
			return &gce.Instance{Name: req.Name, Status: "RUNNING", NetworkIP: "10.1.2.3"}, nil
		},
		getFn: func(_ context.Context, _, _, name string) (*gce.Instance, error) {
			return &gce.Instance{Name: name, Status: "RUNNING", NetworkIP: "10.1.2.3"}, nil
		},
	}

	p := dockergce.NewDockerGCEProviderWithClientFactory(cfg, mockGCEClient, repo, func(_ string) *remoteclient.Client {
		return remoteclient.NewClient(srv.URL)
	})

	_, err := p.Execute(context.Background(), remoteclient.Request{ConversationID: "conv-retry"}, func(remoteclient.StreamEvent) {})
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// First container (port 9091) should have been stopped and removed
	if len(stopped) == 0 {
		t.Error("expected StopContainer to be called after port collision")
	}
	if len(removed) == 0 {
		t.Error("expected RemoveContainer to be called after port collision")
	}

	// Two RunAgentContainer calls: first on 9091, second on 9092
	if len(runPorts) < 2 {
		t.Fatalf("expected 2 RunAgentContainer calls, got %d", len(runPorts))
	}
	if runPorts[0] != 9091 {
		t.Errorf("first RunAgentContainer port = %d, want 9091", runPorts[0])
	}
	if runPorts[1] != 9092 {
		t.Errorf("second RunAgentContainer port = %d, want 9092", runPorts[1])
	}
}

// TestVMScaler_Started: IdleCheckInterval > 0 のとき VMScaler が CleanupOrphans を呼ぶ
func TestVMScaler_Started(t *testing.T) {
	var cleanupCount int32

	repo := &countingCleanupRepo{}

	factory := func(_ string) (dockerhost.ContainerManager, error) {
		return &dockerhost.MockContainerManager{}, nil
	}

	cfg := dockergce.DockerGCEConfig{
		ProjectID:               "test-project",
		Zone:                    "us-central1-a",
		IdleTimeout:             time.Minute,
		IdleCheckInterval:       20 * time.Millisecond, // very short for test
		ContainerManagerFactory: factory,
	}

	_ = cleanupCount
	p := dockergce.NewDockerGCEProvider(cfg, gce.NewMockGCEClient(), repo)
	defer func() { _ = p.Close() }()

	// Wait long enough for VMScaler to tick at least once
	time.Sleep(100 * time.Millisecond)

	n := atomic.LoadInt32(&repo.callCount)
	if n == 0 {
		t.Error("CleanupOrphans was never called by VMScaler/IdleChecker")
	}
}

// countingCleanupRepo is a minimal dbRepository that counts CleanupOrphans calls.
type countingCleanupRepo struct {
	callCount int32
}

func (r *countingCleanupRepo) GetSessionEndpointByConversationID(_ context.Context, _ string) (*db.SessionEndpoint, error) {
	return nil, errors.New("not found")
}
func (r *countingCleanupRepo) CreateSessionEndpoint(_ context.Context, _, _, _ string, _ int) (*db.SessionEndpoint, error) {
	return nil, errors.New("not implemented")
}
func (r *countingCleanupRepo) UpdateSessionEndpointLastActivity(_ context.Context, _ string) error {
	return nil
}
func (r *countingCleanupRepo) GetVMInstance(_ context.Context, _ string) (*db.VMInstance, error) {
	return nil, errors.New("not found")
}
func (r *countingCleanupRepo) GetAvailableVMInstance(_ context.Context, _ int) (*db.VMInstance, error) {
	return nil, errors.New("no VM")
}
func (r *countingCleanupRepo) CreateVMInstance(_ context.Context, _, _, _ string) (*db.VMInstance, error) {
	return nil, errors.New("not implemented")
}
func (r *countingCleanupRepo) UpdateVMInstanceIP(_ context.Context, _, _ string) error { return nil }
func (r *countingCleanupRepo) UpdateVMInstanceStatus(_ context.Context, _, _ string) error {
	return nil
}
func (r *countingCleanupRepo) IncrementVMActiveContainers(_ context.Context, _ string) error {
	return nil
}
func (r *countingCleanupRepo) DecrementVMActiveContainers(_ context.Context, _ string) error {
	return nil
}
func (r *countingCleanupRepo) ListIdleSessionEndpoints(_ context.Context, _ time.Duration) ([]*db.SessionEndpoint, error) {
	atomic.AddInt32(&r.callCount, 1)
	return nil, nil
}
func (r *countingCleanupRepo) DeleteSessionEndpoint(_ context.Context, _ string) error { return nil }
func (r *countingCleanupRepo) ListIdleVMInstances(_ context.Context, _ time.Duration) ([]*db.VMInstance, error) {
	return nil, nil
}
func (r *countingCleanupRepo) DeleteVMInstance(_ context.Context, _ string) error { return nil }
func (r *countingCleanupRepo) GetMaxPortOnVM(_ context.Context, _ string) (int, error) {
	return 0, nil
}

// TestCreateGCEVM_NetworkTag verifies that createGCEVM passes the "cc-tunnel-agent" network tag.
func TestCreateGCEVM_NetworkTag(t *testing.T) {
	srv := fakeAgentServer(t, []remoteclient.StreamEvent{
		{Type: "result", SessionID: "sess-tag", Result: "success"},
	})
	defer srv.Close()

	var capturedReq *gce.CreateInstanceRequest
	mockGCEClient := &customMockGCEClient{
		createFn: func(_ context.Context, req *gce.CreateInstanceRequest) (*gce.Instance, error) {
			capturedReq = req
			return &gce.Instance{Name: req.Name, Status: "RUNNING", NetworkIP: "10.0.0.1"}, nil
		},
		getFn: func(_ context.Context, _, _, name string) (*gce.Instance, error) {
			return &gce.Instance{Name: name, Status: "RUNNING", NetworkIP: "10.0.0.1"}, nil
		},
	}

	repo := newMockDBRepo()
	repo.availableVMErr = errors.New("no VM")

	cfg := shortTimeoutConfig()
	cfg.AgentPort = 9091

	p := dockergce.NewDockerGCEProviderWithClientFactory(cfg, mockGCEClient, repo, func(_ string) *remoteclient.Client {
		return remoteclient.NewClient(srv.URL)
	})

	_, err := p.Execute(context.Background(), remoteclient.Request{ConversationID: "conv-tag", Prompt: "hi"}, func(remoteclient.StreamEvent) {})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if capturedReq == nil {
		t.Fatal("CreateInstance was not called")
	}
	found := false
	for _, tag := range capturedReq.Tags {
		if tag == "cc-tunnel-agent" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Tags = %v, want to contain \"cc-tunnel-agent\"", capturedReq.Tags)
	}
}

// --- custom mocks for specific test behavior ---

type customMockGCEClient struct {
	createFn func(ctx context.Context, req *gce.CreateInstanceRequest) (*gce.Instance, error)
	getFn    func(ctx context.Context, project, zone, name string) (*gce.Instance, error)
}

var _ gce.GCEClient = (*customMockGCEClient)(nil)

func (m *customMockGCEClient) CreateInstance(ctx context.Context, req *gce.CreateInstanceRequest) (*gce.Instance, error) {
	return m.createFn(ctx, req)
}

func (m *customMockGCEClient) GetInstance(ctx context.Context, project, zone, name string) (*gce.Instance, error) {
	return m.getFn(ctx, project, zone, name)
}

func (m *customMockGCEClient) DeleteInstance(_ context.Context, _, _, _ string) error {
	return nil
}

func (m *customMockGCEClient) ListInstances(_ context.Context, _, _ string) ([]*gce.Instance, error) {
	return nil, nil
}

// cleanupMockDBRepo is a minimal mock for CleanupOrphans tests.
type cleanupMockDBRepo struct {
	endpointToReturn []*db.SessionEndpoint
	vmsToReturn      []*db.VMInstance
	deletedEndpoints map[string]bool
	deletedVMs       map[string]bool
}

func (m *cleanupMockDBRepo) GetSessionEndpointByConversationID(_ context.Context, _ string) (*db.SessionEndpoint, error) {
	return nil, errors.New("not found")
}
func (m *cleanupMockDBRepo) CreateSessionEndpoint(_ context.Context, _, _, _ string, _ int) (*db.SessionEndpoint, error) {
	return nil, errors.New("not implemented")
}
func (m *cleanupMockDBRepo) UpdateSessionEndpointLastActivity(_ context.Context, _ string) error {
	return nil
}
func (m *cleanupMockDBRepo) GetVMInstance(_ context.Context, id string) (*db.VMInstance, error) {
	for _, vm := range m.vmsToReturn {
		if vm.ID == id {
			return vm, nil
		}
	}
	return nil, errors.New("not found")
}
func (m *cleanupMockDBRepo) GetAvailableVMInstance(_ context.Context, _ int) (*db.VMInstance, error) {
	return nil, errors.New("no VM")
}
func (m *cleanupMockDBRepo) CreateVMInstance(_ context.Context, _, _, _ string) (*db.VMInstance, error) {
	return nil, errors.New("not implemented")
}
func (m *cleanupMockDBRepo) UpdateVMInstanceIP(_ context.Context, _, _ string) error { return nil }
func (m *cleanupMockDBRepo) UpdateVMInstanceStatus(_ context.Context, _, _ string) error {
	return nil
}
func (m *cleanupMockDBRepo) IncrementVMActiveContainers(_ context.Context, _ string) error {
	return nil
}
func (m *cleanupMockDBRepo) DecrementVMActiveContainers(_ context.Context, _ string) error {
	return nil
}
func (m *cleanupMockDBRepo) ListIdleSessionEndpoints(_ context.Context, _ time.Duration) ([]*db.SessionEndpoint, error) {
	return m.endpointToReturn, nil
}
func (m *cleanupMockDBRepo) DeleteSessionEndpoint(_ context.Context, id string) error {
	m.deletedEndpoints[id] = true
	return nil
}
func (m *cleanupMockDBRepo) ListIdleVMInstances(_ context.Context, _ time.Duration) ([]*db.VMInstance, error) {
	return m.vmsToReturn, nil
}
func (m *cleanupMockDBRepo) DeleteVMInstance(_ context.Context, id string) error {
	m.deletedVMs[id] = true
	return nil
}
func (m *cleanupMockDBRepo) GetMaxPortOnVM(_ context.Context, _ string) (int, error) {
	return 0, nil
}
