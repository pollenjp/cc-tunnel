package dockergce

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/cmclient"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/db"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/gce"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider"
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
	// GetMaxPortOnVM returns the maximum host port in use on the given VM (0 if none).
	GetMaxPortOnVM(ctx context.Context, vmID string) (int, error)
}

// DockerGCEConfig は DockerGCEProvider の設定
type DockerGCEConfig struct {
	ProjectID        string
	Zone             string
	MachineType      string // デフォルト: "e2-medium"
	VMImage          string // GCE source image (Packer で焼いた Docker 入り Ubuntu)
	VMServiceAccount string // VM にアタッチする SA のメールアドレス（空なら GCE default compute SA）
	// VMSubnetwork は VM をぶら下げる subnet (Private Google Access 有効を想定)。
	// 形式は "projects/<proj>/regions/<region>/subnetworks/<name>" など。
	// 空なら network 既定 subnet が使われる（外部 IP 無し VM は Artifact
	// Registry に到達できないため本番では明示必須）。
	VMSubnetwork  string
	AgentImage    string // Artifact Registry の cc-remote-agent イメージ URL
	AgentPort     int    // cc-remote-agent の listen ポート（デフォルト: 9091）
	IdleTimeout   time.Duration
	MaxContainers int // デフォルト: 10

	// IdleCheckInterval は IdleChecker / VMScaler が CleanupOrphans を呼ぶ間隔（0 = 無効）
	IdleCheckInterval time.Duration

	// VM 起動待機のタイムアウト設定（0 はデフォルト値を使用）
	VMReadyTimeout               time.Duration // デフォルト: 3分
	ContainerManagerReadyTimeout time.Duration // デフォルト: 5分 (cold boot + dockerd + container-manager systemd unit + 9090 listen までの猶予)
	AgentReadyTimeout            time.Duration // デフォルト: 1分 (cc-remote-agent /auth/status 応答までの猶予)
	PollInterval                 time.Duration // デフォルト: 5秒

	// DockerPingTimeout は既存 VM 再利用時の container-manager 健全性チェックのタイムアウト。
	// (デフォルト: 3秒)
	DockerPingTimeout time.Duration

	// Multi-container settings
	ContainerNamePrefix string // default "session"
	PortRangeStart      int    // default 9091
	// ContainerManagerPort is the host port that the container-manager service
	// listens on inside each VM. Default 9090. cc-tunnel reaches it as
	// http://<vm_internal_ip>:<port>; the channel is unauthenticated and
	// must be reachable only from the cc-tunnel VPC connector subnet.
	ContainerManagerPort int

	// ContainerManagerFactory builds a cmclient.ContainerManager for the given
	// base URL (e.g. "http://10.0.0.2:9090"). If nil, cmclient.NewClient is used.
	ContainerManagerFactory func(baseURL string) (cmclient.ContainerManager, error)
}

// DockerGCEProvider implements ExecutionProvider using GCE VMs with Docker.
type DockerGCEProvider struct {
	config          DockerGCEConfig
	gce             gce.GCEClient
	db              dbRepository
	sf              singleflight.Group
	newClient       func(baseURL string) *remoteclient.Client
	newDockerClient func(baseURL string) (cmclient.ContainerManager, error)
	idleChecker     *IdleChecker
	vmScaler        *VMScaler
}

var _ interface {
	Execute(ctx context.Context, req remoteclient.Request, onEvent func(remoteclient.StreamEvent)) (string, error)
} = (*DockerGCEProvider)(nil) // コンパイル時インターフェース確認

// NewDockerGCEProvider creates a new DockerGCEProvider with the given config, GCE client, and DB repository.
// Uses remoteclient.NewClient as the default remoteclient factory.
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
		cfg.MaxContainers = 10
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = 15 * time.Minute
	}
	if cfg.VMReadyTimeout == 0 {
		cfg.VMReadyTimeout = 3 * time.Minute
	}
	if cfg.ContainerManagerReadyTimeout == 0 {
		cfg.ContainerManagerReadyTimeout = 5 * time.Minute
	}
	if cfg.AgentReadyTimeout == 0 {
		cfg.AgentReadyTimeout = time.Minute
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 5 * time.Second
	}
	if cfg.DockerPingTimeout == 0 {
		cfg.DockerPingTimeout = 3 * time.Second
	}
	if cfg.ContainerNamePrefix == "" {
		cfg.ContainerNamePrefix = "session"
	}
	if cfg.PortRangeStart == 0 {
		cfg.PortRangeStart = 9091
	}
	if cfg.ContainerManagerPort == 0 {
		cfg.ContainerManagerPort = 9090
	}

	dockerClientFactory := cfg.ContainerManagerFactory
	if dockerClientFactory == nil {
		dockerClientFactory = func(baseURL string) (cmclient.ContainerManager, error) {
			return cmclient.NewClient(baseURL)
		}
	}

	p := &DockerGCEProvider{
		config:          cfg,
		gce:             gceClient,
		db:              repo,
		newClient:       clientFactory,
		newDockerClient: dockerClientFactory,
	}

	if cfg.IdleCheckInterval > 0 {
		ic := NewIdleChecker(p, cfg.IdleCheckInterval)
		ic.Start(context.Background())
		p.idleChecker = ic

		vs := NewVMScaler(p, cfg.IdleCheckInterval)
		vs.Start(context.Background())
		p.vmScaler = vs
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

		// Find available VM (with container-manager health check) or create a new one.
		// 既存 VM の container-manager が応答しないと ContainerCreate が 30 秒タイムアウトするため、
		// 短い ping で先に弾いて DB 上 unhealthy にマークし、別 VM か新規作成にフォールバックする。
		vm, err := p.pickHealthyAvailableVM(ctx)
		if err != nil {
			// No healthy available VM: create a new GCE VM
			vm, err = p.createGCEVM(ctx)
			if err != nil {
				return nil, fmt.Errorf("create GCE VM: %w", err)
			}

			// Wait for VM (Stage 1: GCE RUNNING + IP, Stage 2: Docker daemon reachable)
			networkIP, err := p.waitForVMReady(ctx, vm)
			if err != nil {
				return nil, fmt.Errorf("wait for VM ready: %w", err)
			}
			vm.InternalIP = networkIP
		}

		// Unique container name per conversation (stable across retries)
		containerName := fmt.Sprintf("%s-%s", p.config.ContainerNamePrefix, conversationID)

		// container-manager API URL on the VM. Unauthenticated by design;
		// VPC + firewall restricts access to the cc-tunnel VPC connector.
		cmURL := fmt.Sprintf("http://%s:%d", vm.InternalIP, p.config.ContainerManagerPort)

		// Retry loop for port collision (UNIQUE constraint on vm_instance_id + port).
		var ep *db.SessionEndpoint
		for attempt := 0; attempt < 3; attempt++ {
			// Re-query max port each attempt so collisions resolved by other goroutines are visible.
			maxPort, err := p.db.GetMaxPortOnVM(ctx, vm.ID)
			if err != nil {
				return nil, fmt.Errorf("get max port on VM: %w", err)
			}
			hostPort := maxPort + 1
			if hostPort < p.config.PortRangeStart {
				hostPort = p.config.PortRangeStart
			}

			dcm, err := p.newDockerClient(cmURL)
			if err != nil {
				return nil, fmt.Errorf("create container-manager client for %s: %w", cmURL, err)
			}
			if err := dcm.RunAgent(ctx, cmclient.RunAgentRequest{
				Image:         p.config.AgentImage,
				Name:          containerName,
				HostPort:      hostPort,
				ContainerPort: p.config.AgentPort,
				// cc-remote-agent reads PORT from env and defaults to 9090
				// when unset (apps/cc-remote-agent/cmd/cc-remote-agent/main.go),
				// so without this the container would listen on 9090 while
				// container-manager binds host:p.config.AgentPort -> container:p.config.AgentPort.
				Env: []string{fmt.Sprintf("PORT=%d", p.config.AgentPort)},
				Labels: map[string]string{
					"component":       "cc-remote-agent",
					"conversation_id": conversationID,
					"vm_instance_id":  vm.ID,
				},
			}); err != nil {
				return nil, fmt.Errorf("run agent container on %s: %w", vm.InternalIP, err)
			}

			// Orphan fix: if the agent fails to become ready, stop and remove the container.
			if err := p.waitForAgentReady(ctx, vm.InternalIP, hostPort); err != nil {
				_ = dcm.StopContainer(ctx, containerName)
				_ = dcm.RemoveContainer(ctx, containerName)
				return nil, fmt.Errorf("wait for agent ready: %w", err)
			}

			// Record session endpoint in DB.
			ep, err = p.db.CreateSessionEndpoint(ctx, conversationID, vm.ID, containerName, hostPort)
			if err == nil {
				break
			}
			// Port collision or transient error: cleanup container and retry.
			_ = dcm.StopContainer(ctx, containerName)
			_ = dcm.RemoveContainer(ctx, containerName)
			if attempt < 2 {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(p.config.PollInterval / 2):
				}
				continue
			}
			return nil, fmt.Errorf("create session endpoint after %d attempts: %w", attempt+1, err)
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

// pickHealthyAvailableVM returns an available VM whose container-manager responds to a
// short ping. Unhealthy VMs are demoted to status="unhealthy" in the DB so subsequent calls
// to GetAvailableVMInstance skip them. Returns the same error shape as GetAvailableVMInstance
// when nothing usable is left, so the caller falls through to createGCEVM.
func (p *DockerGCEProvider) pickHealthyAvailableVM(ctx context.Context) (*db.VMInstance, error) {
	const maxAttempts = 5
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		vm, err := p.db.GetAvailableVMInstance(ctx, p.config.MaxContainers)
		if err != nil {
			return nil, err
		}

		cmURL := fmt.Sprintf("http://%s:%d", vm.InternalIP, p.config.ContainerManagerPort)
		dcm, err := p.newDockerClient(cmURL)
		if err != nil {
			lastErr = fmt.Errorf("create container-manager client for %s: %w", cmURL, err)
			if markErr := p.db.UpdateVMInstanceStatus(ctx, vm.ID, "unhealthy"); markErr != nil {
				slog.Warn("failed to mark VM unhealthy", "err", markErr, "vm_id", vm.ID)
			}
			continue
		}

		pingCtx, cancel := context.WithTimeout(ctx, p.config.DockerPingTimeout)
		ready := dcm.IsReady(pingCtx)
		cancel()
		if ready {
			return vm, nil
		}

		slog.Warn("reusable VM has unreachable container-manager; marking unhealthy",
			"vm_id", vm.ID, "internal_ip", vm.InternalIP, "container_manager_url", cmURL)
		if markErr := p.db.UpdateVMInstanceStatus(ctx, vm.ID, "unhealthy"); markErr != nil {
			slog.Warn("failed to mark VM unhealthy", "err", markErr, "vm_id", vm.ID)
			// Avoid infinite loop if the DB update keeps failing: the same VM would be returned again.
			return nil, fmt.Errorf("mark VM %s unhealthy: %w", vm.ID, markErr)
		}
		lastErr = fmt.Errorf("container-manager at %s unreachable", cmURL)
	}
	if lastErr != nil {
		return nil, lastErr
	}
	// Should not reach here; defensively trigger the createGCEVM fallback.
	return nil, fmt.Errorf("no healthy available VM after %d attempts", maxAttempts)
}

// createGCEVM creates a new GCE VM with the cc-remote-agent startup script and records it in the DB.
func (p *DockerGCEProvider) createGCEVM(ctx context.Context) (*db.VMInstance, error) {
	vmName := "cc-tunnel-" + uuid.New().String()[:8]

	inst, err := p.gce.CreateInstance(ctx, &gce.CreateInstanceRequest{
		Project:             p.config.ProjectID,
		Zone:                p.config.Zone,
		Name:                vmName,
		MachineType:         p.config.MachineType,
		SourceImage:         p.config.VMImage,
		ServiceAccountEmail: p.config.VMServiceAccount,
		Subnetwork:          p.config.VMSubnetwork,
		StartupScript:       p.buildStartupScript(),
		Labels:              map[string]string{"managed-by": "cc-tunnel"},
		Tags:                []string{"cc-tunnel-agent"},
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

// waitForVMReady waits for the GCE VM to be RUNNING (Stage 1) and Docker daemon to be
// reachable (Stage 2). Returns the VM's internal IP.
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

	// Update IP and status in DB (non-fatal)
	_ = p.db.UpdateVMInstanceIP(ctx, vm.ID, networkIP)
	_ = p.db.UpdateVMInstanceStatus(ctx, vm.ID, "running")

	// Stage 2: Wait for the container-manager HTTP API to become reachable.
	cmURL := fmt.Sprintf("http://%s:%d", networkIP, p.config.ContainerManagerPort)
	dcm, err := p.newDockerClient(cmURL)
	if err != nil {
		return "", fmt.Errorf("create container-manager client for %s: %w", cmURL, err)
	}

	stage2Ctx, cancel2 := context.WithTimeout(ctx, p.config.ContainerManagerReadyTimeout)
	defer cancel2()

	for {
		if dcm.IsReady(stage2Ctx) {
			return networkIP, nil
		}
		select {
		case <-stage2Ctx.Done():
			return "", fmt.Errorf("timeout waiting for container-manager at %s: %w", cmURL, stage2Ctx.Err())
		case <-time.After(p.config.PollInterval):
		}
	}
}

// waitForAgentReady polls the cc-remote-agent health endpoint until it responds.
func (p *DockerGCEProvider) waitForAgentReady(ctx context.Context, vmIP string, port int) error {
	agentURL := fmt.Sprintf("http://%s:%d", vmIP, port)
	client := p.newClient(agentURL)

	agentCtx, cancel := context.WithTimeout(ctx, p.config.AgentReadyTimeout)
	defer cancel()

	for {
		if _, err := client.GetAuthStatus(agentCtx); err == nil {
			return nil
		}
		select {
		case <-agentCtx.Done():
			return fmt.Errorf("timeout waiting for cc-remote-agent at %s: %w", agentURL, agentCtx.Err())
		case <-time.After(p.config.PollInterval):
		}
	}
}

// buildStartupScript generates the startup script run on each VM boot.
// Docker daemon と container-manager コンテナの起動は Packer で焼いたカスタム
// イメージの systemd unit が担当するため、ここでは追加の処理を行わない。
// cc-remote-agent イメージの pull は container-manager がリクエスト時に行う
// (VM の SA 経由で Artifact Registry に認証する)。
func (p *DockerGCEProvider) buildStartupScript() string {
	return "#!/bin/bash\n# managed by Packer image; nothing to do at boot.\n"
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

// GetSessionClient returns the remoteclient.Client for an existing per-session
// GCE container. Returns provider.ErrSessionNotFound if no session exists.
func (p *DockerGCEProvider) GetSessionClient(ctx context.Context, conversationID string) (*remoteclient.Client, error) {
	ep, err := p.db.GetSessionEndpointByConversationID(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("%w: conversation %s: %v", provider.ErrSessionNotFound, conversationID, err)
	}
	vm, err := p.db.GetVMInstance(ctx, ep.VMInstanceID)
	if err != nil {
		return nil, fmt.Errorf("get VM instance for conversation %s: %w", conversationID, err)
	}
	agentURL := fmt.Sprintf("http://%s:%d", vm.InternalIP, ep.Port)
	return p.newClient(agentURL), nil
}

// Close stops the IdleChecker, VMScaler, and releases resources held by the provider.
func (p *DockerGCEProvider) Close() error {
	if p.idleChecker != nil {
		p.idleChecker.Stop()
	}
	if p.vmScaler != nil {
		p.vmScaler.Stop()
	}
	return nil
}

// CleanupOrphans removes idle session endpoints (stopping their containers) and stops idle GCE VMs.
func (p *DockerGCEProvider) CleanupOrphans(ctx context.Context) error {
	// Clean up idle session endpoints
	endpoints, err := p.db.ListIdleSessionEndpoints(ctx, p.config.IdleTimeout)
	if err != nil {
		return fmt.Errorf("list idle session endpoints: %w", err)
	}
	for _, ep := range endpoints {
		// Stop and remove the container via container-manager (non-fatal on failure)
		vm, vmErr := p.db.GetVMInstance(ctx, ep.VMInstanceID)
		if vmErr == nil && vm.InternalIP != "" {
			cmURL := fmt.Sprintf("http://%s:%d", vm.InternalIP, p.config.ContainerManagerPort)
			if dcm, dcErr := p.newDockerClient(cmURL); dcErr == nil {
				_ = dcm.StopContainer(ctx, ep.ContainerName)
				_ = dcm.RemoveContainer(ctx, ep.ContainerName)
			}
		}
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
