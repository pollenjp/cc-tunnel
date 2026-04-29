package dockergce

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/db"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/gce"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

// dbRepository abstracts the database operations needed by DockerGCEProvider.
type dbRepository interface {
	GetSessionEndpointByConversationID(ctx context.Context, conversationID string) (*db.SessionEndpoint, error)
	CreateSessionEndpoint(ctx context.Context, conversationID, vmInstanceID, containerName string, port int) (*db.SessionEndpoint, error)
	UpdateSessionEndpointLastActivity(ctx context.Context, conversationID string) error
	GetVMInstance(ctx context.Context, id string) (*db.VMInstance, error)
	GetAvailableVMInstance(ctx context.Context, maxContainers int) (*db.VMInstance, error)
	CreateVMInstance(ctx context.Context, gceInstanceName, zone, internalIP string) (*db.VMInstance, error)
	UpdateVMInstanceIP(ctx context.Context, id, internalIP string) error
	UpdateVMInstanceStatus(ctx context.Context, id, status string) error
	IncrementVMActiveContainers(ctx context.Context, id string) error
	DecrementVMActiveContainers(ctx context.Context, id string) error
	ListIdleSessionEndpoints(ctx context.Context, idleThreshold time.Duration) ([]*db.SessionEndpoint, error)
	DeleteSessionEndpoint(ctx context.Context, id string) error
	ListIdleVMInstances(ctx context.Context, idleThreshold time.Duration) ([]*db.VMInstance, error)
	DeleteVMInstance(ctx context.Context, id string) error
}

// DockerGCEConfig は DockerGCEProvider の設定
type DockerGCEConfig struct {
	ProjectID   string
	Zone        string
	MachineType string // デフォルト: "e2-medium"
	AgentImage  string // Artifact Registry の cc-remote-agent イメージ URL
	AgentPort   int    // cc-remote-agent の listen ポート（デフォルト: 9091）
	IdleTimeout time.Duration
	MaxContainers int

	// IdleCheckInterval は IdleChecker が CleanupOrphans を呼ぶ間隔（0 = IdleChecker 無効）
	IdleCheckInterval time.Duration // デフォルト: 無効（0）。60s を推奨。

	// VM 起動待機のタイムアウト設定（0 はデフォルト値を使用）
	VMReadyTimeout    time.Duration // デフォルト: 3分
	AgentReadyTimeout time.Duration // デフォルト: 1分
	PollInterval      time.Duration // デフォルト: 5秒
}

// DockerGCEProvider implements ExecutionProvider using GCE VMs with Docker.
type DockerGCEProvider struct {
	config      DockerGCEConfig
	gce         gce.GCEClient
	db          dbRepository
	sf          singleflight.Group
	newClient   func(baseURL string) *remoteclient.Client
	idleChecker *IdleChecker
}

var _ interface {
	Execute(ctx context.Context, req remoteclient.Request, onEvent func(remoteclient.StreamEvent)) (string, error)
} = (*DockerGCEProvider)(nil) // コンパイル時インターフェース確認

// NewDockerGCEProvider creates a new DockerGCEProvider with the given config, GCE client, and DB repository.
// Uses remoteclient.NewClient as the default client factory.
func NewDockerGCEProvider(cfg DockerGCEConfig, gceClient gce.GCEClient, repo dbRepository) *DockerGCEProvider {
	return NewDockerGCEProviderWithClientFactory(cfg, gceClient, repo, remoteclient.NewClient)
}

// NewDockerGCEProviderWithClientFactory creates a DockerGCEProvider with a custom remoteclient factory.
// Useful for testing where a mock HTTP server needs to be injected.
func NewDockerGCEProviderWithClientFactory(cfg DockerGCEConfig, gceClient gce.GCEClient, repo dbRepository, clientFactory func(string) *remoteclient.Client) *DockerGCEProvider {
	if cfg.MachineType == "" {
		cfg.MachineType = "e2-medium"
	}
	if cfg.AgentPort == 0 {
		cfg.AgentPort = 9091
	}
	if cfg.MaxContainers == 0 {
		cfg.MaxContainers = 1
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = 15 * time.Minute
	}
	if cfg.VMReadyTimeout == 0 {
		cfg.VMReadyTimeout = 3 * time.Minute
	}
	if cfg.AgentReadyTimeout == 0 {
		cfg.AgentReadyTimeout = time.Minute
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 5 * time.Second
	}
	p := &DockerGCEProvider{
		config:    cfg,
		gce:       gceClient,
		db:        repo,
		newClient: clientFactory,
	}

	if cfg.IdleCheckInterval > 0 {
		ic := NewIdleChecker(p, cfg.IdleCheckInterval)
		ic.Start(context.Background())
		p.idleChecker = ic
	}

	return p
}

// Execute routes the request to the cc-remote-agent on a GCE VM.
func (p *DockerGCEProvider) Execute(ctx context.Context, req remoteclient.Request, onEvent func(remoteclient.StreamEvent)) (string, error) {
	ep, err := p.getOrCreateEndpoint(ctx, req.ConversationID)
	if err != nil {
		return "", fmt.Errorf("get or create endpoint: %w", err)
	}

	vm, err := p.db.GetVMInstance(ctx, ep.VMInstanceID)
	if err != nil {
		return "", fmt.Errorf("get VM instance: %w", err)
	}

	agentURL := fmt.Sprintf("http://%s:%d", vm.InternalIP, ep.Port)
	client := p.newClient(agentURL)

	sessionID, err := client.Execute(ctx, req, onEvent)
	if err != nil {
		return "", fmt.Errorf("remote execute: %w", err)
	}

	// Update last_activity (non-fatal on error)
	_ = p.db.UpdateSessionEndpointLastActivity(ctx, req.ConversationID)

	return sessionID, nil
}

// getOrCreateEndpoint returns an existing session endpoint or creates a new one.
// AF002: singleflight prevents concurrent duplicate creation for the same conversationID.
func (p *DockerGCEProvider) getOrCreateEndpoint(ctx context.Context, conversationID string) (*db.SessionEndpoint, error) {
	// Fast path: endpoint already exists
	if ep, err := p.db.GetSessionEndpointByConversationID(ctx, conversationID); err == nil && ep.Status == "running" {
		return ep, nil
	}

	// Slow path: create endpoint under singleflight (AF002 concurrent dedup)
	v, err, _ := p.sf.Do(conversationID, func() (interface{}, error) {
		// Re-check inside singleflight (idempotent)
		if ep, err := p.db.GetSessionEndpointByConversationID(ctx, conversationID); err == nil && ep.Status == "running" {
			return ep, nil
		}

		// Find available VM or create a new one
		vm, err := p.db.GetAvailableVMInstance(ctx, p.config.MaxContainers)
		if err != nil {
			// No available VM: create a new GCE VM
			vm, err = p.createGCEVM(ctx)
			if err != nil {
				return nil, fmt.Errorf("create GCE VM: %w", err)
			}

			// Wait for VM (Stage 1: RUNNING + IP) and agent (Stage 2: health check)
			networkIP, err := p.waitForVMReady(ctx, vm)
			if err != nil {
				return nil, fmt.Errorf("wait for VM ready: %w", err)
			}
			vm.InternalIP = networkIP
		}

		// Create session endpoint in DB
		ep, err := p.db.CreateSessionEndpoint(ctx, conversationID, vm.ID, "cc-remote-agent", p.config.AgentPort)
		if err != nil {
			return nil, fmt.Errorf("create session endpoint: %w", err)
		}

		if err := p.db.IncrementVMActiveContainers(ctx, vm.ID); err != nil {
			return nil, fmt.Errorf("increment VM active containers: %w", err)
		}

		return ep, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*db.SessionEndpoint), nil
}

// createGCEVM creates a new GCE VM with the cc-remote-agent startup script and records it in the DB.
func (p *DockerGCEProvider) createGCEVM(ctx context.Context) (*db.VMInstance, error) {
	vmName := "cc-tunnel-" + uuid.New().String()[:8]

	inst, err := p.gce.CreateInstance(ctx, &gce.CreateInstanceRequest{
		Project:       p.config.ProjectID,
		Zone:          p.config.Zone,
		Name:          vmName,
		MachineType:   p.config.MachineType,
		StartupScript: p.buildStartupScript(),
		Labels:        map[string]string{"managed-by": "cc-tunnel"},
	})
	if err != nil {
		return nil, fmt.Errorf("create GCE instance: %w", err)
	}

	vm, err := p.db.CreateVMInstance(ctx, inst.Name, p.config.Zone, inst.NetworkIP)
	if err != nil {
		return nil, fmt.Errorf("record VM in DB: %w", err)
	}
	return vm, nil
}

// waitForVMReady waits for the GCE VM to be RUNNING (Stage 1) and cc-remote-agent to be
// healthy (Stage 2: AF004). Returns the VM's internal IP.
func (p *DockerGCEProvider) waitForVMReady(ctx context.Context, vm *db.VMInstance) (string, error) {
	// Stage 1: Wait for GCE API to report RUNNING + internal IP
	stage1Ctx, cancel1 := context.WithTimeout(ctx, p.config.VMReadyTimeout)
	defer cancel1()

	var networkIP string
	for {
		inst, err := p.gce.GetInstance(stage1Ctx, p.config.ProjectID, p.config.Zone, vm.GCEInstanceName)
		if err == nil && inst.Status == "RUNNING" && inst.NetworkIP != "" {
			networkIP = inst.NetworkIP
			break
		}
		select {
		case <-stage1Ctx.Done():
			return "", fmt.Errorf("timeout waiting for VM %q to be RUNNING: %w", vm.GCEInstanceName, stage1Ctx.Err())
		case <-time.After(p.config.PollInterval):
		}
	}

	// Update IP in DB (non-fatal)
	_ = p.db.UpdateVMInstanceIP(ctx, vm.ID, networkIP)

	// Stage 2: Wait for cc-remote-agent health check (AF004)
	agentURL := fmt.Sprintf("http://%s:%d", networkIP, p.config.AgentPort)
	client := p.newClient(agentURL)

	stage2Ctx, cancel2 := context.WithTimeout(ctx, p.config.AgentReadyTimeout)
	defer cancel2()

	for {
		if _, err := client.GetAuthStatus(stage2Ctx); err == nil {
			return networkIP, nil
		}
		select {
		case <-stage2Ctx.Done():
			return "", fmt.Errorf("timeout waiting for cc-remote-agent on %s to be ready: %w", agentURL, stage2Ctx.Err())
		case <-time.After(p.config.PollInterval):
		}
	}
}

// buildStartupScript generates the COS startup script that configures Docker TCP and starts cc-remote-agent.
func (p *DockerGCEProvider) buildStartupScript() string {
	return fmt.Sprintf(`#!/bin/bash
# COS では Docker がプリインストール済み
# Docker daemon に TCP アクセスを追加して cc-remote-agent 管理を可能にする
mkdir -p /etc/docker
echo '{"hosts":["tcp://0.0.0.0:2375","unix:///var/run/docker.sock"]}' > /etc/docker/daemon.json
systemctl restart docker 2>/dev/null || true
sleep 10
# cc-remote-agent コンテナを起動
docker pull %s || true
docker run -d \
  --name cc-remote-agent \
  --restart unless-stopped \
  -p %d:%d \
  %s
`, p.config.AgentImage, p.config.AgentPort, p.config.AgentPort, p.config.AgentImage)
}

// PrepareForRelogin ensures a session endpoint exists for the given conversation
// without running the execute flow, so the frontend can initiate a PTY-based
// re-login flow against the container.
func (p *DockerGCEProvider) PrepareForRelogin(ctx context.Context, conversationID string) error {
	_, err := p.getOrCreateEndpoint(ctx, conversationID)
	return err
}

// PullCredentialsFromSession fetches the credentials.json written by the PTY
// login flow from the GCE session container.
func (p *DockerGCEProvider) PullCredentialsFromSession(ctx context.Context, conversationID string) (string, error) {
	ep, err := p.db.GetSessionEndpointByConversationID(ctx, conversationID)
	if err != nil {
		return "", fmt.Errorf("get session endpoint: %w", err)
	}
	vm, err := p.db.GetVMInstance(ctx, ep.VMInstanceID)
	if err != nil {
		return "", fmt.Errorf("get VM instance: %w", err)
	}
	agentURL := fmt.Sprintf("http://%s:%d", vm.InternalIP, ep.Port)
	client := p.newClient(agentURL)
	return client.FinalizeCredentials(ctx)
}

// Close stops the IdleChecker and releases resources held by the provider.
func (p *DockerGCEProvider) Close() error {
	if p.idleChecker != nil {
		p.idleChecker.Stop()
	}
	return nil
}

// CleanupOrphans removes idle session endpoints and stops idle GCE VMs.
func (p *DockerGCEProvider) CleanupOrphans(ctx context.Context) error {
	// Clean up idle session endpoints
	endpoints, err := p.db.ListIdleSessionEndpoints(ctx, p.config.IdleTimeout)
	if err != nil {
		return fmt.Errorf("list idle session endpoints: %w", err)
	}
	for _, ep := range endpoints {
		_ = p.db.DecrementVMActiveContainers(ctx, ep.VMInstanceID)
		if err := p.db.DeleteSessionEndpoint(ctx, ep.ID); err != nil {
			return fmt.Errorf("delete session endpoint %s: %w", ep.ID, err)
		}
	}

	// Clean up idle VMs
	vms, err := p.db.ListIdleVMInstances(ctx, p.config.IdleTimeout)
	if err != nil {
		return fmt.Errorf("list idle VM instances: %w", err)
	}
	for _, vm := range vms {
		_ = p.gce.DeleteInstance(ctx, p.config.ProjectID, p.config.Zone, vm.GCEInstanceName)
		if err := p.db.DeleteVMInstance(ctx, vm.ID); err != nil {
			return fmt.Errorf("delete VM instance %s: %w", vm.ID, err)
		}
	}

	return nil
}
