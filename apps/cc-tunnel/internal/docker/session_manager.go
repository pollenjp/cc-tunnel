package docker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/remoteclient"
)

// SessionManagerConfig holds configuration for the SessionManager.
type SessionManagerConfig struct {
	Image         string        // cc-remote-agent イメージ名（例: "cc-remote-agent:latest"）
	Network       string        // compose ネットワーク名（例: "apps_default"）
	VolumeName    string        // claude-sessions ボリューム名
	ContainerPort string        // cc-remote-agent が Listen するポート（例: "9091"）
	IdleTimeout   time.Duration // アイドルタイムアウト（デフォルト: 15分）
	StartTimeout  time.Duration // コンテナ起動タイムアウト（デフォルト: 30秒）
}

// session はひとつの会話セッションに対応するコンテナ情報。
type session struct {
	containerID string
	client      *remoteclient.Client
	lastUsed    time.Time
	idleTimer   *time.Timer
}

// SessionManager manages per-conversation Docker containers.
type SessionManager struct {
	runner      DockerRunner
	config      SessionManagerConfig
	sessions    map[string]*session // convID → *session
	mu          sync.Mutex
	newClientFn func(url string) *remoteclient.Client // injectable for testing
}

// NewSessionManager creates a new SessionManager.
func NewSessionManager(runner DockerRunner, config SessionManagerConfig) *SessionManager {
	if config.IdleTimeout == 0 {
		config.IdleTimeout = 15 * time.Minute
	}
	if config.StartTimeout == 0 {
		config.StartTimeout = 30 * time.Second
	}
	return &SessionManager{
		runner:      runner,
		config:      config,
		sessions:    make(map[string]*session),
		newClientFn: remoteclient.NewClient,
	}
}

// GetOrCreate returns the remoteclient.Client for the given convID,
// creating a new container if one does not exist.
func (sm *SessionManager) GetOrCreate(ctx context.Context, convID string) (*remoteclient.Client, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Cache hit: reuse if container is still running.
	if s, ok := sm.sessions[convID]; ok {
		info, err := sm.runner.ContainerInspect(ctx, s.containerID)
		if err == nil && info.State == "running" {
			s.lastUsed = time.Now()
			s.idleTimer.Reset(sm.config.IdleTimeout)
			return s.client, nil
		}
		// Container is dead — remove stale entry and recreate below.
		delete(sm.sessions, convID)
	}

	// Build container name from first 8 chars of convID.
	suffix := convID
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	containerName := "cctunnel-session-" + suffix

	opts := ContainerCreateOpts{
		Name:    containerName,
		Image:   sm.config.Image,
		Env:     []string{"PORT=" + sm.config.ContainerPort},
		Network: sm.config.Network,
		VolumeMounts: []VolumeMount{
			{Source: sm.config.VolumeName, Target: "/home/user/.claude"},
		},
	}
	containerID, err := sm.runner.ContainerCreate(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("container create: %w", err)
	}

	if err := sm.runner.ContainerStart(ctx, containerID); err != nil {
		return nil, fmt.Errorf("container start: %w", err)
	}

	containerURL := "http://" + containerName + ":" + sm.config.ContainerPort
	client := sm.newClientFn(containerURL)

	// Health check: wait until the container's auth status endpoint responds.
	if err := sm.waitForReady(ctx, client); err != nil {
		_ = sm.runner.ContainerStop(ctx, containerID)
		_ = sm.runner.ContainerRemove(ctx, containerID)
		return nil, fmt.Errorf("container health check: %w", err)
	}

	s := &session{
		containerID: containerID,
		client:      client,
		lastUsed:    time.Now(),
	}
	s.idleTimer = time.AfterFunc(sm.config.IdleTimeout, func() {
		slog.Info("idle timeout, stopping container", "convID", convID)
		if err := sm.Stop(context.Background(), convID); err != nil {
			slog.Error("idle stop failed", "convID", convID, "err", err)
		}
	})

	sm.sessions[convID] = s
	return client, nil
}

// waitForReady polls GetAuthStatus every 500 ms until it succeeds or StartTimeout elapses.
func (sm *SessionManager) waitForReady(ctx context.Context, client *remoteclient.Client) error {
	deadline := time.Now().Add(sm.config.StartTimeout)
	for {
		if _, err := client.GetAuthStatus(ctx); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for container to be ready")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// Stop stops and removes the container for the given convID.
func (sm *SessionManager) Stop(ctx context.Context, convID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	s, ok := sm.sessions[convID]
	if !ok {
		return nil
	}

	if s.idleTimer != nil {
		s.idleTimer.Stop()
	}

	if err := sm.runner.ContainerStop(ctx, s.containerID); err != nil {
		return fmt.Errorf("stop %s: %w", convID, err)
	}
	if err := sm.runner.ContainerRemove(ctx, s.containerID); err != nil {
		return fmt.Errorf("remove %s: %w", convID, err)
	}

	delete(sm.sessions, convID)
	return nil
}

// StopAll stops all managed sessions. Used for graceful shutdown.
func (sm *SessionManager) StopAll(ctx context.Context) error {
	sm.mu.Lock()
	convIDs := make([]string, 0, len(sm.sessions))
	for id := range sm.sessions {
		convIDs = append(convIDs, id)
	}
	sm.mu.Unlock()

	var lastErr error
	for _, convID := range convIDs {
		if err := sm.Stop(ctx, convID); err != nil {
			slog.Error("StopAll: stop failed", "convID", convID, "err", err)
			lastErr = err
		}
	}
	return lastErr
}

// CleanupOrphans removes any stopped "cctunnel-session-*" containers
// that are not in the running state. Called at startup.
func (sm *SessionManager) CleanupOrphans(ctx context.Context) error {
	containers, err := sm.runner.ContainerList(ctx, "cctunnel-session-", true)
	if err != nil {
		return fmt.Errorf("list containers: %w", err)
	}

	for _, c := range containers {
		if c.State != "running" {
			if err := sm.runner.ContainerRemove(ctx, c.ID); err != nil {
				slog.Error("CleanupOrphans: remove failed", "id", c.ID, "err", err)
			}
		}
	}
	return nil
}
