package dockergce

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// vmReconciler abstracts the ReconcileVMs operation so VMScaler can be
// unit-tested without a full DockerGCEProvider.
type vmReconciler interface {
	ReconcileVMs(ctx context.Context) error
}

// VMScaler は定期的に container-manager の実測値を確認し、
// cc-remote-agent コンテナ数がゼロのまま ZeroAgentsTimeout を超えた
// GCE VM を削除する goroutine（設計書 §5.2）。
type VMScaler struct {
	reconciler vmReconciler
	interval   time.Duration
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

// NewVMScaler creates a new VMScaler backed by provider.
func NewVMScaler(provider *DockerGCEProvider, interval time.Duration) *VMScaler {
	return &VMScaler{
		reconciler: provider,
		interval:   interval,
		stopCh:     make(chan struct{}),
	}
}

// Start starts the scale-down loop in a goroutine.
func (vs *VMScaler) Start(ctx context.Context) {
	vs.wg.Add(1)
	go vs.run(ctx)
}

// Stop stops the scale-down loop and waits for the goroutine to finish.
func (vs *VMScaler) Stop() {
	close(vs.stopCh)
	vs.wg.Wait()
}

func (vs *VMScaler) run(ctx context.Context) {
	defer vs.wg.Done()
	ticker := time.NewTicker(vs.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := vs.reconciler.ReconcileVMs(ctx); err != nil {
				slog.Error("VMScaler: ReconcileVMs failed", "err", err)
			}
		case <-vs.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}
