package dockergce

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// VMScaler は定期的にアイドル GCE VM を削除する goroutine（設計書 §5.2）。
type VMScaler struct {
	cleaner  idleCleaner
	interval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewVMScaler creates a new VMScaler backed by provider.
func NewVMScaler(provider *DockerGCEProvider, interval time.Duration) *VMScaler {
	return &VMScaler{
		cleaner:  provider,
		interval: interval,
		stopCh:   make(chan struct{}),
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
			if err := vs.cleaner.CleanupOrphans(ctx); err != nil {
				slog.Error("VMScaler: CleanupOrphans failed", "err", err)
			}
		case <-vs.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}
