package dockergce_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/db"
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

func (m *mockDBRepo) CreateSessionEndpoint(_ context.Context, conversationID, vmInstanceID, containerName string, port int) (*db.SessionEndpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
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

func shortTimeoutConfig(agentHost string) dockergce.DockerGCEConfig {
	return dockergce.DockerGCEConfig{
		ProjectID:         "test-project",
		Zone:              "us-central1-a",
		MachineType:       "e2-medium",
		AgentImage:        "cc-remote-agent:test",
		AgentPort:         0, // will be replaced by httptest server port
		IdleTimeout:       time.Minute,
		MaxContainers:     1,
		VMReadyTimeout:    200 * time.Millisecond,
		AgentReadyTimeout: 200 * time.Millisecond,
		PollInterval:      10 * time.Millisecond,
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

	// MockGCEClient: returns VM with the httptest server's host as IP
	mockGCE := gce.NewMockGCEClient()
	// Override CreateInstance to return the httptest server address as networkIP
	// We can't easily set the port to match httptest, so we'll use a custom newClient factory
	_ = mockGCE

	// Build a real GCE mock where GetInstance also returns RUNNING
	// Use default mock behavior (returns NetworkIP: "10.0.0.1")
	repo := newMockDBRepo()
	repo.availableVMErr = errors.New("no VM") // force VM creation

	// We need the provider to contact srv for both health check and execute.
	// Use a custom newClient that always points to srv.URL regardless of agentURL.
	cfg := shortTimeoutConfig(srv.URL)
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
		ContainerName:  "cc-remote-agent",
		Port:           9091,
		Status:         "running",
	}
	repo.endpoints["conv-existing"] = ep

	cfg := shortTimeoutConfig(srv.URL)
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

	cfg := shortTimeoutConfig(srv.URL)
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
	}

	p := dockergce.NewDockerGCEProvider(cfg, mockGCEClient, repo)

	_, err := p.Execute(context.Background(), remoteclient.Request{ConversationID: "conv-timeout"}, func(remoteclient.StreamEvent) {})
	if err == nil {
		t.Fatal("WaitForVMReady: expected timeout error, got nil")
	}
}

func TestDockerGCEProvider_CleanupOrphans(t *testing.T) {
	deletedEndpoints := make(map[string]bool)
	deletedVMs := make(map[string]bool)

	repo := &cleanupMockDBRepo{
		endpointToReturn: []*db.SessionEndpoint{
			{ID: "ep-idle-1", VMInstanceID: "vm-1"},
			{ID: "ep-idle-2", VMInstanceID: "vm-2"},
		},
		vmsToReturn: []*db.VMInstance{
			{ID: "vm-1", GCEInstanceName: "cc-tunnel-vm1", Zone: "us-central1-a"},
			{ID: "vm-2", GCEInstanceName: "cc-tunnel-vm2", Zone: "us-central1-a"},
		},
		deletedEndpoints: deletedEndpoints,
		deletedVMs:       deletedVMs,
	}

	mockGCEClient := gce.NewMockGCEClient()

	cfg := dockergce.DockerGCEConfig{
		ProjectID:   "test-project",
		Zone:        "us-central1-a",
		IdleTimeout: time.Minute,
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
func (m *cleanupMockDBRepo) GetVMInstance(_ context.Context, _ string) (*db.VMInstance, error) {
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
