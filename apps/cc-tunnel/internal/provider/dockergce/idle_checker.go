package dockergce

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// idleCleaner abstracts the CleanupOrphans operation.
// *DockerGCEProvider satisfies this interface.
type idleCleaner interface {
	CleanupOrphans(ctx context.Context) error
}

// IdleChecker は定期的に CleanupOrphans を呼んでアイドル VM/コンテナを削除する goroutine。
type IdleChecker struct {
	cleaner  idleCleaner
	interval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewIdleChecker creates a new IdleChecker backed by provider.
func NewIdleChecker(provider *DockerGCEProvider, interval time.Duration) *IdleChecker {
	return &IdleChecker{
		cleaner:  provider,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start starts the cleanup loop in a goroutine.
func (c *IdleChecker) Start(ctx context.Context) {
	c.wg.Add(1)
	go c.loop(ctx)
}

// Stop stops the cleanup loop and waits for the goroutine to finish.
func (c *IdleChecker) Stop() {
	close(c.stopCh)
	c.wg.Wait()
}

func (c *IdleChecker) loop(ctx context.Context) {
	defer c.wg.Done()
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := c.cleaner.CleanupOrphans(ctx); err != nil {
				slog.Error("IdleChecker: CleanupOrphans failed", "err", err)
			}
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}
