package dockergce

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// mockIdleCleaner implements idleCleaner for testing.
type mockIdleCleaner struct {
	count atomic.Int64
}

func (m *mockIdleCleaner) CleanupOrphans(_ context.Context) error {
	m.count.Add(1)
	return nil
}

func TestIdleChecker_Start_CallsCleanupOrphans(t *testing.T) {
	mock := &mockIdleCleaner{}
	ic := &IdleChecker{
		cleaner:  mock,
		interval: 10 * time.Millisecond,
		stopCh:   make(chan struct{}),
	}

	ic.Start(context.Background())
	time.Sleep(80 * time.Millisecond)
	ic.Stop()

	if mock.count.Load() == 0 {
		t.Error("CleanupOrphans was not called after Start")
	}
}

func TestIdleChecker_Stop_StopsLoop(t *testing.T) {
	mock := &mockIdleCleaner{}
	ic := &IdleChecker{
		cleaner:  mock,
		interval: 10 * time.Millisecond,
		stopCh:   make(chan struct{}),
	}

	ic.Start(context.Background())
	time.Sleep(50 * time.Millisecond)
	ic.Stop()

	countAfterStop := mock.count.Load()

	time.Sleep(50 * time.Millisecond)

	if got := mock.count.Load(); got != countAfterStop {
		t.Errorf("CleanupOrphans was called after Stop: count before=%d after=%d", countAfterStop, got)
	}
}

func TestIdleChecker_ContextCancellation(t *testing.T) {
	mock := &mockIdleCleaner{}
	ic := &IdleChecker{
		cleaner:  mock,
		interval: 10 * time.Millisecond,
		stopCh:   make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	ic.Start(ctx)

	time.Sleep(50 * time.Millisecond)
	cancel()

	waitDone := make(chan struct{})
	go func() {
		ic.wg.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
		// goroutine stopped as expected
	case <-time.After(500 * time.Millisecond):
		t.Error("IdleChecker goroutine did not stop after context cancellation")
	}
}
