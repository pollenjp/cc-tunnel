package selfreaper_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pollenjp/cc-tunnel/apps/container-manager/internal/selfreaper"
)

type stubLister struct {
	count int
	err   error
}

func (s *stubLister) AgentCount(_ context.Context) (int, error) { return s.count, s.err }

type stubDeleter struct {
	called atomic.Int32
	err    error
	last   struct{ project, zone, name string }
}

func (s *stubDeleter) DeleteInstance(_ context.Context, p, z, n string) error {
	s.called.Add(1)
	s.last.project, s.last.zone, s.last.name = p, z, n
	return s.err
}

type stubMeta struct {
	name, zone, project string
	err                 error
}

func (s stubMeta) InstanceName(_ context.Context) (string, error) { return s.name, s.err }
func (s stubMeta) Zone(_ context.Context) (string, error)         { return s.zone, s.err }
func (s stubMeta) ProjectID(_ context.Context) (string, error)    { return s.project, s.err }

// tickHarness is a Reaper exposed for whitebox-style timing tests.
// We exercise the public Run loop with short timeouts in the e2e
// integration test, but the deterministic logic is easier to test via
// repeated New/Run invocations driven by injected time and a short
// interval that we cancel after the assertion.
func runFor(t *testing.T, r *selfreaper.Reaper, timeout time.Duration) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return r.Run(ctx)
}

func newReaper(lister selfreaper.AgentLister, deleter selfreaper.InstanceDeleter, md selfreaper.MetadataResolver) *selfreaper.Reaper {
	return selfreaper.New(lister, deleter, md, selfreaper.Config{
		Interval: 10 * time.Millisecond,
		Timeout:  30 * time.Millisecond,
	})
}

func goodMeta() selfreaper.MetadataResolver {
	return stubMeta{name: "vm-1", zone: "us-central1-a", project: "proj"}
}

func TestReaper_DeletesAfterZeroPersists(t *testing.T) {
	lister := &stubLister{count: 0}
	deleter := &stubDeleter{}
	r := newReaper(lister, deleter, goodMeta())

	// Run long enough for: first tick seeds zeroSince, third+ tick
	// (>= 30ms after seed) dispatches delete and Run returns nil.
	if err := runFor(t, r, 500*time.Millisecond); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if got := deleter.called.Load(); got != 1 {
		t.Fatalf("DeleteInstance called %d times, want 1", got)
	}
	if deleter.last.project != "proj" || deleter.last.zone != "us-central1-a" || deleter.last.name != "vm-1" {
		t.Fatalf("delete target = %+v, want proj/us-central1-a/vm-1", deleter.last)
	}
}

func TestReaper_AgentPresent_NeverDeletes(t *testing.T) {
	lister := &stubLister{count: 1}
	deleter := &stubDeleter{}
	r := newReaper(lister, deleter, goodMeta())

	if err := runFor(t, r, 200*time.Millisecond); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if got := deleter.called.Load(); got != 0 {
		t.Fatalf("DeleteInstance called %d times while agents present, want 0", got)
	}
}

func TestReaper_AgentReappears_ResetsZeroSince(t *testing.T) {
	// Start at zero, switch to non-zero before the threshold elapses,
	// and confirm no deletion fires within the run window.
	lister := &stubLister{count: 0}
	deleter := &stubDeleter{}
	r := newReaper(lister, deleter, goodMeta())

	go func() {
		time.Sleep(15 * time.Millisecond) // before 30ms threshold
		lister.count = 1
	}()
	if err := runFor(t, r, 200*time.Millisecond); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if got := deleter.called.Load(); got != 0 {
		t.Fatalf("DeleteInstance called %d times after reset, want 0", got)
	}
}

func TestReaper_ProbeFailure_DoesNotDelete(t *testing.T) {
	lister := &stubLister{count: 0, err: errors.New("docker down")}
	deleter := &stubDeleter{}
	r := newReaper(lister, deleter, goodMeta())

	if err := runFor(t, r, 200*time.Millisecond); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if got := deleter.called.Load(); got != 0 {
		t.Fatalf("DeleteInstance called %d times despite probe failure, want 0", got)
	}
}

func TestReaper_MetadataFailureIsFatal(t *testing.T) {
	lister := &stubLister{count: 0}
	deleter := &stubDeleter{}
	r := newReaper(lister, deleter, stubMeta{err: errors.New("no metadata server")})

	err := runFor(t, r, 100*time.Millisecond)
	if err == nil {
		t.Fatalf("Run returned nil; want fatal metadata error")
	}
	if got := deleter.called.Load(); got != 0 {
		t.Fatalf("DeleteInstance must not be called when metadata is missing")
	}
}

func TestReaper_DeleteErrorKeepsLooping(t *testing.T) {
	// If the GCE API rejects the delete (e.g. permission denied), the
	// reaper should log and keep retrying rather than crash the
	// container-manager process. Verified by ensuring Run does NOT
	// return until ctx timeout, and the delete is attempted more than
	// once.
	lister := &stubLister{count: 0}
	deleter := &stubDeleter{err: errors.New("permission denied")}
	r := newReaper(lister, deleter, goodMeta())

	err := runFor(t, r, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if got := deleter.called.Load(); got < 2 {
		t.Fatalf("DeleteInstance attempted %d times; want >=2 retries on transient error", got)
	}
}
