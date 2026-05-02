package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

// mockRunner is a test double for DockerRunner.
type mockRunner struct {
	createFn  func(ctx context.Context, opts ContainerCreateOpts) (string, error)
	startFn   func(ctx context.Context, containerID string) error
	stopFn    func(ctx context.Context, containerID string) error
	removeFn  func(ctx context.Context, containerID string) error
	inspectFn func(ctx context.Context, containerID string) (*ContainerInfo, error)
	listFn    func(ctx context.Context, namePrefix string, all bool) ([]ContainerSummary, error)
}

func (m *mockRunner) ContainerCreate(ctx context.Context, opts ContainerCreateOpts) (string, error) {
	if m.createFn != nil {
		return m.createFn(ctx, opts)
	}
	return "default-cid", nil
}

func (m *mockRunner) ContainerStart(ctx context.Context, containerID string) error {
	if m.startFn != nil {
		return m.startFn(ctx, containerID)
	}
	return nil
}

func (m *mockRunner) ContainerStop(ctx context.Context, containerID string) error {
	if m.stopFn != nil {
		return m.stopFn(ctx, containerID)
	}
	return nil
}

func (m *mockRunner) ContainerRemove(ctx context.Context, containerID string) error {
	if m.removeFn != nil {
		return m.removeFn(ctx, containerID)
	}
	return nil
}

func (m *mockRunner) ContainerInspect(ctx context.Context, containerID string) (*ContainerInfo, error) {
	if m.inspectFn != nil {
		return m.inspectFn(ctx, containerID)
	}
	return &ContainerInfo{ID: containerID, State: "running"}, nil
}

func (m *mockRunner) ContainerList(ctx context.Context, namePrefix string, all bool) ([]ContainerSummary, error) {
	if m.listFn != nil {
		return m.listFn(ctx, namePrefix, all)
	}
	return nil, nil
}

// newMockAuthServer creates a test HTTP server that serves /auth/status with a logged-in response.
func newMockAuthServer(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/status" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(remoteclient.AuthStatus{LoggedIn: true})
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

// newTestSessionManager creates a SessionManager with mock auth server injected.
// If ts is nil, no client factory override is applied (for tests that don't call GetOrCreate).
func newTestSessionManager(t *testing.T, runner DockerRunner, ts *httptest.Server) *SessionManager {
	t.Helper()
	sm := NewSessionManager(runner, SessionManagerConfig{
		Image:         "test-image:latest",
		ContainerPort: "9091",
		IdleTimeout:   time.Hour, // long enough not to interfere with tests
	})
	if ts != nil {
		sm.newClientFn = func(_ string) *remoteclient.Client {
			return remoteclient.NewClient(ts.URL)
		}
	}
	return sm
}

func TestSessionManager_GetOrCreate_newSession(t *testing.T) {
	var createCount int32
	runner := &mockRunner{
		createFn: func(_ context.Context, _ ContainerCreateOpts) (string, error) {
			atomic.AddInt32(&createCount, 1)
			return "cid1", nil
		},
		startFn: func(_ context.Context, _ string) error {
			return nil
		},
		inspectFn: func(_ context.Context, containerID string) (*ContainerInfo, error) {
			return &ContainerInfo{ID: containerID, State: "running"}, nil
		},
	}
	ts := newMockAuthServer(t)
	sm := newTestSessionManager(t, runner, ts)

	client, err := sm.GetOrCreate(context.Background(), "conv1", nil)
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if n := atomic.LoadInt32(&createCount); n != 1 {
		t.Errorf("expected 1 create call, got %d", n)
	}
}

func TestSessionManager_GetOrCreate_cached(t *testing.T) {
	var createCount int32
	runner := &mockRunner{
		createFn: func(_ context.Context, _ ContainerCreateOpts) (string, error) {
			atomic.AddInt32(&createCount, 1)
			return "cid1", nil
		},
		inspectFn: func(_ context.Context, containerID string) (*ContainerInfo, error) {
			return &ContainerInfo{ID: containerID, State: "running"}, nil
		},
	}
	ts := newMockAuthServer(t)
	sm := newTestSessionManager(t, runner, ts)
	ctx := context.Background()

	client1, err := sm.GetOrCreate(ctx, "conv1", nil)
	if err != nil {
		t.Fatalf("first GetOrCreate: %v", err)
	}
	client2, err := sm.GetOrCreate(ctx, "conv1", nil)
	if err != nil {
		t.Fatalf("second GetOrCreate: %v", err)
	}
	if client1 != client2 {
		t.Error("expected same client on cache hit")
	}
	if n := atomic.LoadInt32(&createCount); n != 1 {
		t.Errorf("expected 1 create call, got %d", n)
	}
}

func TestSessionManager_Stop(t *testing.T) {
	var stopCount, removeCount int32
	runner := &mockRunner{
		createFn: func(_ context.Context, _ ContainerCreateOpts) (string, error) {
			return "cid1", nil
		},
		inspectFn: func(_ context.Context, containerID string) (*ContainerInfo, error) {
			return &ContainerInfo{ID: containerID, State: "running"}, nil
		},
		stopFn: func(_ context.Context, _ string) error {
			atomic.AddInt32(&stopCount, 1)
			return nil
		},
		removeFn: func(_ context.Context, _ string) error {
			atomic.AddInt32(&removeCount, 1)
			return nil
		},
	}
	ts := newMockAuthServer(t)
	sm := newTestSessionManager(t, runner, ts)
	ctx := context.Background()

	if _, err := sm.GetOrCreate(ctx, "conv1", nil); err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}

	if err := sm.Stop(ctx, "conv1"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if n := atomic.LoadInt32(&stopCount); n != 1 {
		t.Errorf("expected 1 stop call, got %d", n)
	}
	if n := atomic.LoadInt32(&removeCount); n != 1 {
		t.Errorf("expected 1 remove call, got %d", n)
	}

	// After Stop, a second GetOrCreate must create a fresh container.
	var createCount2 int32
	runner.createFn = func(_ context.Context, _ ContainerCreateOpts) (string, error) {
		atomic.AddInt32(&createCount2, 1)
		return "cid2", nil
	}
	if _, err := sm.GetOrCreate(ctx, "conv1", nil); err != nil {
		t.Fatalf("second GetOrCreate: %v", err)
	}
	if n := atomic.LoadInt32(&createCount2); n != 1 {
		t.Errorf("expected 1 create after stop, got %d", n)
	}
}

func TestSessionManager_StopAll(t *testing.T) {
	var createCount, stopCount, removeCount int32
	runner := &mockRunner{
		createFn: func(_ context.Context, _ ContainerCreateOpts) (string, error) {
			n := atomic.AddInt32(&createCount, 1)
			return fmt.Sprintf("cid%d", n), nil
		},
		inspectFn: func(_ context.Context, containerID string) (*ContainerInfo, error) {
			return &ContainerInfo{ID: containerID, State: "running"}, nil
		},
		stopFn: func(_ context.Context, _ string) error {
			atomic.AddInt32(&stopCount, 1)
			return nil
		},
		removeFn: func(_ context.Context, _ string) error {
			atomic.AddInt32(&removeCount, 1)
			return nil
		},
	}
	ts := newMockAuthServer(t)
	sm := newTestSessionManager(t, runner, ts)
	ctx := context.Background()

	if _, err := sm.GetOrCreate(ctx, "conv1", nil); err != nil {
		t.Fatalf("GetOrCreate conv1: %v", err)
	}
	if _, err := sm.GetOrCreate(ctx, "conv2", nil); err != nil {
		t.Fatalf("GetOrCreate conv2: %v", err)
	}

	if err := sm.StopAll(ctx); err != nil {
		t.Fatalf("StopAll: %v", err)
	}

	if n := atomic.LoadInt32(&stopCount); n != 2 {
		t.Errorf("expected 2 stop calls, got %d", n)
	}
	if n := atomic.LoadInt32(&removeCount); n != 2 {
		t.Errorf("expected 2 remove calls, got %d", n)
	}
}

func TestSessionManager_compose_mode_unchanged(t *testing.T) {
	ts := newMockAuthServer(t)
	var capturedURL string
	runner := &mockRunner{
		inspectFn: func(_ context.Context, containerID string) (*ContainerInfo, error) {
			return &ContainerInfo{ID: containerID, State: "running"}, nil
		},
	}
	sm := NewSessionManager(runner, SessionManagerConfig{
		Image:         "test-image:latest",
		ContainerPort: "9091",
		IdleTimeout:   time.Hour,
	})
	sm.newClientFn = func(url string) *remoteclient.Client {
		capturedURL = url
		return remoteclient.NewClient(ts.URL)
	}

	if _, err := sm.GetOrCreate(context.Background(), "conv12345678", nil); err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	want := "http://cctunnel-session-conv1234:9091"
	if capturedURL != want {
		t.Errorf("expected containerURL %q, got %q", want, capturedURL)
	}
}

// TestSessionManager_GetOrCreate_noTmpfsMounts verifies that the container is created
// without any tmpfs or volume mounts (using normal container filesystem for isolation).
func TestSessionManager_GetOrCreate_noTmpfsMounts(t *testing.T) {
	var capturedOpts ContainerCreateOpts
	runner := &mockRunner{
		createFn: func(_ context.Context, opts ContainerCreateOpts) (string, error) {
			capturedOpts = opts
			return "cid-notmpfs", nil
		},
		inspectFn: func(_ context.Context, containerID string) (*ContainerInfo, error) {
			return &ContainerInfo{ID: containerID, State: "running"}, nil
		},
	}
	ts := newMockAuthServer(t)
	sm := newTestSessionManager(t, runner, ts)

	if _, err := sm.GetOrCreate(context.Background(), "conv-notmpfs", nil); err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}

	if len(capturedOpts.VolumeMounts) != 0 {
		t.Errorf("expected no volume mounts, got %v", capturedOpts.VolumeMounts)
	}
	if len(capturedOpts.TmpfsMounts) != 0 {
		t.Errorf("expected no tmpfs mounts, got %v", capturedOpts.TmpfsMounts)
	}
}

// TestSessionManager_GetOrCreate_injectsCredentials verifies that credentials are
// sent to the container via /init when provided.
func TestSessionManager_GetOrCreate_injectsCredentials(t *testing.T) {
	runner := &mockRunner{
		createFn: func(_ context.Context, _ ContainerCreateOpts) (string, error) {
			return "cid-cred", nil
		},
		inspectFn: func(_ context.Context, containerID string) (*ContainerInfo, error) {
			return &ContainerInfo{ID: containerID, State: "running"}, nil
		},
	}

	var receivedCredJSON string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/status":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(remoteclient.AuthStatus{LoggedIn: true})
		case "/init":
			var body struct {
				CredentialsJSON string `json:"credentialsJson"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			receivedCredJSON = body.CredentialsJSON
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"message":"credentials initialized"}`))
		}
	}))
	t.Cleanup(ts.Close)

	sm := NewSessionManager(runner, SessionManagerConfig{
		Image:         "test-image:latest",
		ContainerPort: "9091",
		IdleTimeout:   time.Hour,
	})
	sm.newClientFn = func(_ string) *remoteclient.Client {
		return remoteclient.NewClient(ts.URL)
	}

	creds := []byte(`{"access_token":"tok"}`)
	if _, err := sm.GetOrCreate(context.Background(), "conv-cred", creds); err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}

	if receivedCredJSON != string(creds) {
		t.Errorf("expected credentials %q, got %q", creds, receivedCredJSON)
	}
}

func TestSessionManager_CleanupOrphans(t *testing.T) {
	var removeCount int32
	runner := &mockRunner{
		listFn: func(_ context.Context, _ string, _ bool) ([]ContainerSummary, error) {
			return []ContainerSummary{
				{ID: "cid-running", Name: "cctunnel-session-abc12345", State: "running"},
				{ID: "cid-exited", Name: "cctunnel-session-def67890", State: "exited"},
			}, nil
		},
		removeFn: func(_ context.Context, containerID string) error {
			atomic.AddInt32(&removeCount, 1)
			if containerID != "cid-exited" {
				t.Errorf("unexpected remove of container %s", containerID)
			}
			return nil
		},
	}
	sm := newTestSessionManager(t, runner, nil)

	if err := sm.CleanupOrphans(context.Background()); err != nil {
		t.Fatalf("CleanupOrphans: %v", err)
	}
	if n := atomic.LoadInt32(&removeCount); n != 1 {
		t.Errorf("expected 1 remove call, got %d", n)
	}
}
