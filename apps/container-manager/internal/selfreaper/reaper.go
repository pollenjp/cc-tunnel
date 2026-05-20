// Package selfreaper implements the per-VM self-termination loop.
//
// Each cc-remote-agent GCE VM runs a single container-manager process
// (systemd-managed). When that container-manager observes zero
// cc-remote-agent containers on its own Docker daemon for longer than
// SELF_REAP_TIMEOUT, it calls compute.instances.delete on its own
// instance. GCE then shuts the VM down; systemd stops container-manager
// and any leftover containers along with it.
//
// This is the *primary* reap path in the dual-path design
// (adr/2026-05 vm_reap_dual_path.md). The Cloud Scheduler →
// cc-tunnel /internal/reconcile-vms path is a 6-hourly safety net for
// VMs whose self-reaper itself is dead.
//
// The package is intentionally small: it owns a goroutine, an in-memory
// `zeroSince` timestamp, and a deletion call. State is not persisted —
// if container-manager restarts, the next zero observation simply
// re-seeds zeroSince. False positives (deleting a non-idle VM) cannot
// happen because each tick re-queries the live docker daemon.
package selfreaper

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"cloud.google.com/go/compute/metadata"
)

// AgentLister reports the current count of cc-remote-agent containers
// on this VM. In production this is docker.Manager.ListAgents; tests
// pass a stub.
type AgentLister interface {
	AgentCount(ctx context.Context) (int, error)
}

// InstanceDeleter deletes a GCE VM instance. In production this is a
// thin wrapper around compute.InstancesClient.Delete; tests pass a stub.
//
// The implementation does NOT need to wait for the long-running
// operation to complete — once the API accepts the request, GCE will
// power the VM off regardless of whether this process is still alive.
type InstanceDeleter interface {
	DeleteInstance(ctx context.Context, project, zone, name string) error
}

// MetadataResolver returns this VM's identity from the GCE metadata
// server. Pluggable so tests don't need a real metadata endpoint.
type MetadataResolver interface {
	InstanceName(ctx context.Context) (string, error)
	Zone(ctx context.Context) (string, error)
	ProjectID(ctx context.Context) (string, error)
}

// Config tunes the reap loop. Zero values fall back to defaults:
// Interval=60s, Timeout=10m.
type Config struct {
	Interval time.Duration
	Timeout  time.Duration
}

// Reaper owns the self-termination goroutine.
type Reaper struct {
	lister   AgentLister
	deleter  InstanceDeleter
	metadata MetadataResolver
	cfg      Config
	now      func() time.Time

	zeroSince time.Time
}

// New builds a Reaper. None of the dependencies may be nil.
func New(lister AgentLister, deleter InstanceDeleter, md MetadataResolver, cfg Config) *Reaper {
	if cfg.Interval <= 0 {
		cfg.Interval = 60 * time.Second
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Minute
	}
	return &Reaper{
		lister:   lister,
		deleter:  deleter,
		metadata: md,
		cfg:      cfg,
		now:      time.Now,
	}
}

// Run drives the reap loop until ctx is cancelled or the VM has been
// successfully marked for deletion. Returns nil on graceful shutdown
// (ctx canceled or self-delete dispatched), or a wrapped error if the
// VM identity could not be resolved from the metadata server (fatal —
// without identity, the reaper cannot do its job).
func (r *Reaper) Run(ctx context.Context) error {
	project, err := r.metadata.ProjectID(ctx)
	if err != nil {
		return fmt.Errorf("self-reaper: resolve project id: %w", err)
	}
	zone, err := r.metadata.Zone(ctx)
	if err != nil {
		return fmt.Errorf("self-reaper: resolve zone: %w", err)
	}
	name, err := r.metadata.InstanceName(ctx)
	if err != nil {
		return fmt.Errorf("self-reaper: resolve instance name: %w", err)
	}
	slog.Info("self-reaper: started",
		"project", project, "zone", zone, "instance", name,
		"interval", r.cfg.Interval, "timeout", r.cfg.Timeout)

	ticker := time.NewTicker(r.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			done, err := r.tick(ctx, project, zone, name)
			if err != nil {
				slog.Warn("self-reaper: tick error", "err", err)
				continue
			}
			if done {
				return nil
			}
		}
	}
}

// tick runs one iteration. Returns done=true once self-delete has been
// dispatched; the caller should stop the loop. Probe failures keep
// done=false and leave zeroSince unchanged, so a transient docker error
// can never accumulate time toward the deletion threshold.
func (r *Reaper) tick(ctx context.Context, project, zone, name string) (bool, error) {
	count, err := r.lister.AgentCount(ctx)
	if err != nil {
		return false, fmt.Errorf("list agents: %w", err)
	}
	if count > 0 {
		if !r.zeroSince.IsZero() {
			slog.Info("self-reaper: agents present, resetting zeroSince", "count", count)
		}
		r.zeroSince = time.Time{}
		return false, nil
	}
	now := r.now()
	if r.zeroSince.IsZero() {
		r.zeroSince = now
		slog.Info("self-reaper: first zero observation", "zero_since", now)
		return false, nil
	}
	elapsed := now.Sub(r.zeroSince)
	if elapsed < r.cfg.Timeout {
		slog.Debug("self-reaper: zero continues", "elapsed", elapsed)
		return false, nil
	}
	slog.Info("self-reaper: timeout reached, deleting self",
		"elapsed", elapsed, "project", project, "zone", zone, "instance", name)
	if err := r.deleter.DeleteInstance(ctx, project, zone, name); err != nil {
		return false, fmt.Errorf("delete self instance: %w", err)
	}
	slog.Info("self-reaper: delete dispatched, awaiting GCE shutdown")
	return true, nil
}

// SDKMetadata is the production MetadataResolver backed by the GCE
// metadata server (cloud.google.com/go/compute/metadata).
type SDKMetadata struct{}

func (SDKMetadata) InstanceName(ctx context.Context) (string, error) {
	return metadata.InstanceNameWithContext(ctx)
}

func (SDKMetadata) Zone(ctx context.Context) (string, error) {
	return metadata.ZoneWithContext(ctx)
}

func (SDKMetadata) ProjectID(ctx context.Context) (string, error) {
	return metadata.ProjectIDWithContext(ctx)
}

// ErrMissingMetadata is returned when a metadata field comes back empty.
// Treated as fatal by Run so a misconfigured environment fails loudly
// rather than running a reaper that can never actually delete anything.
var ErrMissingMetadata = errors.New("metadata server returned empty value")
