package dockergce_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/cmclient"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/db"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/gce"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider/dockergce"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// setupMultiContainerTestDB starts a postgres:16-alpine testcontainer and creates the minimal
// schema required by DockerGCEProvider (conversations, vm_instances, session_endpoints).
func setupMultiContainerTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate postgres container: %v", err)
		}
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to create pgx pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	_, err = pool.Exec(ctx, `
		CREATE TABLE conversations (
			id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
			title         TEXT        NOT NULL DEFAULT '',
			model         TEXT        NOT NULL DEFAULT 'claude-sonnet-4-6',
			system_prompt TEXT,
			status        TEXT        NOT NULL DEFAULT 'idle'
			              CHECK (status IN ('idle', 'running', 'completed')),
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE vm_instances (
			id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
			gce_instance_name   TEXT        NOT NULL UNIQUE,
			zone                TEXT        NOT NULL,
			internal_ip         TEXT        NOT NULL,
			status              TEXT        NOT NULL DEFAULT 'provisioning'
			                    CHECK (status IN ('provisioning', 'running', 'terminated')),
			active_containers   INTEGER     NOT NULL DEFAULT 0,
			idle_since          TIMESTAMPTZ,
			created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE session_endpoints (
			id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
			conversation_id UUID        NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
			vm_instance_id  UUID        NOT NULL REFERENCES vm_instances(id),
			container_name  TEXT        NOT NULL,
			port            INTEGER     NOT NULL,
			status          TEXT        NOT NULL DEFAULT 'provisioning'
			                CHECK (status IN ('provisioning', 'running', 'terminated')),
			last_activity   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (conversation_id),
			CONSTRAINT session_endpoints_vm_port_unique UNIQUE (vm_instance_id, port)
		);
	`)
	if err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}

	return pool
}

// insertConversation inserts a new conversation row and returns its UUID string.
func insertConversation(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	err := pool.QueryRow(ctx, `INSERT INTO conversations DEFAULT VALUES RETURNING id`).Scan(&id)
	if err != nil {
		t.Fatalf("insertConversation: %v", err)
	}
	return id
}

// TestDockerGCEProvider_MultiContainerIntegration verifies multi-container scheduling using a
// real postgres database (via testcontainers) and MockContainerManager:
//
//   - Session 1 → VM1, port 9091
//   - Session 2 → VM1 (same VM, active_containers < MaxContainers), port 9092
//   - Session 3 → VM2 (VM1 full: active_containers == MaxContainers), port 9091
//   - CleanupOrphans → active_containers decrements for idle endpoints
func TestDockerGCEProvider_MultiContainerIntegration(t *testing.T) {
	pool := setupMultiContainerTestDB(t)
	ctx := context.Background()
	repo := db.NewRepository(pool)

	// Fake cc-remote-agent HTTP server (handles /auth/status and /execute).
	srv := fakeAgentServer(t, []remoteclient.StreamEvent{
		{Type: "result", SessionID: "sess-mc", Result: "success"},
	})
	defer srv.Close()

	// Track Docker operations.
	type runCall struct {
		name     string
		hostPort int
	}
	var (
		mu        sync.Mutex
		runCalls  []runCall
		stopCalls []string
	)

	containerManagerFactory := func(_ string) (cmclient.ContainerManager, error) {
		return &cmclient.MockContainerManager{
			IsReadyFunc: func(_ context.Context) bool { return true },
			RunAgentContainerFunc: func(_ context.Context, _, name string, hostPort, _ int) error {
				mu.Lock()
				runCalls = append(runCalls, runCall{name: name, hostPort: hostPort})
				mu.Unlock()
				return nil
			},
			StopContainerFunc: func(_ context.Context, name string) error {
				mu.Lock()
				stopCalls = append(stopCalls, name)
				mu.Unlock()
				return nil
			},
		}, nil
	}

	// GCE mock: first createFn call → VM1 (10.1.1.1), second → VM2 (10.1.2.2).
	// getFn tracks IP by name so waitForVMReady stage1 succeeds.
	vmIPByName := make(map[string]string)
	var vmMu sync.Mutex
	var gceCreateCount int32
	vmIPs := []string{"10.1.1.1", "10.1.2.2"}

	mockGCEClient := &customMockGCEClient{
		createFn: func(_ context.Context, req *gce.CreateInstanceRequest) (*gce.Instance, error) {
			n := int(atomic.AddInt32(&gceCreateCount, 1))
			ip := vmIPs[(n-1)%len(vmIPs)]
			vmMu.Lock()
			vmIPByName[req.Name] = ip
			vmMu.Unlock()
			return &gce.Instance{Name: req.Name, Status: "RUNNING", NetworkIP: ip}, nil
		},
		getFn: func(_ context.Context, _, _, name string) (*gce.Instance, error) {
			vmMu.Lock()
			ip := vmIPByName[name]
			vmMu.Unlock()
			return &gce.Instance{Name: name, Status: "RUNNING", NetworkIP: ip}, nil
		},
	}

	cfg := dockergce.DockerGCEConfig{
		ProjectID:               "test-project",
		Zone:                    "us-central1-a",
		AgentImage:              "cc-remote-agent:test",
		AgentPort:               9091,
		MaxContainers:           2, // 1 VM holds up to 2 containers
		IdleTimeout:             time.Minute,
		VMReadyTimeout:          500 * time.Millisecond,
		AgentReadyTimeout:       500 * time.Millisecond,
		PollInterval:            10 * time.Millisecond,
		PortRangeStart:          9091,
		ContainerManagerFactory: containerManagerFactory,
	}

	p := dockergce.NewDockerGCEProviderWithClientFactory(cfg, mockGCEClient, repo, func(_ string) *remoteclient.Client {
		return remoteclient.NewClient(srv.URL)
	})
	defer func() { _ = p.Close() }()

	// --- Session 1: new VM1, first container at port 9091 ---
	conv1ID := insertConversation(t, ctx, pool)
	if _, err := p.Execute(ctx, remoteclient.Request{ConversationID: conv1ID, Prompt: "s1"}, func(remoteclient.StreamEvent) {}); err != nil {
		t.Fatalf("Execute session 1: %v", err)
	}

	// --- Session 2: reuse VM1 (active_containers=1 < 2), second container at port 9092 ---
	conv2ID := insertConversation(t, ctx, pool)
	if _, err := p.Execute(ctx, remoteclient.Request{ConversationID: conv2ID, Prompt: "s2"}, func(remoteclient.StreamEvent) {}); err != nil {
		t.Fatalf("Execute session 2: %v", err)
	}

	mu.Lock()
	if len(runCalls) < 2 {
		mu.Unlock()
		t.Fatalf("expected ≥2 RunAgentContainer calls after 2 sessions, got %d", len(runCalls))
	}
	port1 := runCalls[0].hostPort
	port2 := runCalls[1].hostPort
	mu.Unlock()

	if port1 != 9091 {
		t.Errorf("session 1 hostPort = %d, want 9091", port1)
	}
	if port2 != 9092 {
		t.Errorf("session 2 hostPort = %d, want 9092 (second container on same VM)", port2)
	}
	if port1 == port2 {
		t.Errorf("sessions 1 and 2 share the same port %d, want different ports", port1)
	}

	// VM1 active_containers must be 2.
	ep1, err := repo.GetSessionEndpointByConversationID(ctx, conv1ID)
	if err != nil {
		t.Fatalf("get ep1: %v", err)
	}
	vm1, err := repo.GetVMInstance(ctx, ep1.VMInstanceID)
	if err != nil {
		t.Fatalf("get vm1: %v", err)
	}
	if vm1.ActiveContainers != 2 {
		t.Errorf("VM1 active_containers = %d after 2 sessions, want 2", vm1.ActiveContainers)
	}

	// GCE CreateInstance called exactly once (VM1 only) so far.
	if n := atomic.LoadInt32(&gceCreateCount); n != 1 {
		t.Errorf("GCE CreateInstance calls = %d after 2 sessions, want 1", n)
	}

	// --- Session 3: VM1 full (MaxContainers=2) → new VM2, port 9091 ---
	conv3ID := insertConversation(t, ctx, pool)
	if _, err := p.Execute(ctx, remoteclient.Request{ConversationID: conv3ID, Prompt: "s3"}, func(remoteclient.StreamEvent) {}); err != nil {
		t.Fatalf("Execute session 3: %v", err)
	}

	if n := atomic.LoadInt32(&gceCreateCount); n != 2 {
		t.Errorf("GCE CreateInstance calls = %d after 3rd session (VM1 full), want 2", n)
	}

	mu.Lock()
	if len(runCalls) < 3 {
		mu.Unlock()
		t.Fatalf("expected ≥3 RunAgentContainer calls after 3 sessions, got %d", len(runCalls))
	}
	port3 := runCalls[2].hostPort
	mu.Unlock()

	if port3 != 9091 {
		t.Errorf("session 3 hostPort = %d, want 9091 (first port on VM2)", port3)
	}

	// VM1 and VM2 must be different VMs.
	ep3, err := repo.GetSessionEndpointByConversationID(ctx, conv3ID)
	if err != nil {
		t.Fatalf("get ep3: %v", err)
	}
	if ep3.VMInstanceID == ep1.VMInstanceID {
		t.Error("session 3 should be on a different VM than sessions 1 and 2")
	}

	// --- Session termination: CleanupOrphans decrements active_containers ---
	// Mark ep1 and ep2 as idle (last_activity far in the past).
	ep2, err := repo.GetSessionEndpointByConversationID(ctx, conv2ID)
	if err != nil {
		t.Fatalf("get ep2: %v", err)
	}
	for _, epID := range []string{ep1.ID, ep2.ID} {
		if _, err := pool.Exec(ctx,
			`UPDATE session_endpoints SET last_activity = NOW() - INTERVAL '2 hours' WHERE id = $1`,
			epID,
		); err != nil {
			t.Fatalf("set endpoint idle: %v", err)
		}
	}

	if err := p.CleanupOrphans(ctx); err != nil {
		t.Fatalf("CleanupOrphans: %v", err)
	}

	// Both ep1 and ep2 containers should have been stopped.
	mu.Lock()
	stopCount := len(stopCalls)
	mu.Unlock()
	if stopCount < 2 {
		t.Errorf("expected ≥2 StopContainer calls after cleanup, got %d", stopCount)
	}

	// VM1 active_containers must now be 0 (or VM1 deleted by CleanupOrphans).
	// ListIdleVMInstances returns VMs with active_containers=0 AND idle_since set;
	// since idle_since is set after decrement, VM1 may already be deleted.
	var vm1ActiveAfter int
	scanErr := pool.QueryRow(ctx,
		`SELECT active_containers FROM vm_instances WHERE id = $1`, vm1.ID,
	).Scan(&vm1ActiveAfter)
	if scanErr != nil {
		// VM1 deleted → active_containers reached 0 and GCE delete was issued. Expected.
		t.Logf("VM1 was deleted by CleanupOrphans (expected when active_containers=0)")
	} else if vm1ActiveAfter != 0 {
		t.Errorf("VM1 active_containers after CleanupOrphans = %d, want 0", vm1ActiveAfter)
	}
}
