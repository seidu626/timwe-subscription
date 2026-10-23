package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
)

type expiryCall struct {
	window time.Duration
	limit  int
}

// fakeExpirer returns results in order and records every call.
type fakeExpirer struct {
	results []int64
	err     error
	calls   []expiryCall
	onCall  func()
}

func (f *fakeExpirer) ExpireStalePendingSubscriptions(_ context.Context, window time.Duration, limit int) (int64, error) {
	f.calls = append(f.calls, expiryCall{window: window, limit: limit})
	if f.onCall != nil {
		f.onCall()
	}
	if f.err != nil {
		return 0, f.err
	}
	if len(f.calls) > len(f.results) {
		return 0, nil
	}
	return f.results[len(f.calls)-1], nil
}

func newTestPendingExpiryWorker(repo PendingExpirer, cfg PendingExpiryConfig) *PendingExpiryWorker {
	w := NewPendingExpiryWorker(repo, zap.NewNop(), cfg)
	w.pause = 0
	return w
}

func TestPendingExpiryRunOnceDrainsFullBatchesThenStops(t *testing.T) {
	repo := &fakeExpirer{results: []int64{3, 3, 1}}
	w := newTestPendingExpiryWorker(repo, PendingExpiryConfig{Enabled: true, Window: 72 * time.Hour, BatchSize: 3})

	if total := w.RunOnce(context.Background()); total != 7 {
		t.Fatalf("expected 7 expired rows, got %d", total)
	}
	if len(repo.calls) != 3 {
		t.Fatalf("expected 3 batches (stop after the short one), got %d", len(repo.calls))
	}
	for _, c := range repo.calls {
		if c.window != 72*time.Hour || c.limit != 3 {
			t.Fatalf("unexpected batch args %+v", c)
		}
	}
}

func TestPendingExpiryRunOnceStopsOnError(t *testing.T) {
	repo := &fakeExpirer{err: errors.New("db down")}
	w := newTestPendingExpiryWorker(repo, PendingExpiryConfig{Enabled: true, BatchSize: 3})

	if total := w.RunOnce(context.Background()); total != 0 {
		t.Fatalf("expected 0 expired rows, got %d", total)
	}
	if len(repo.calls) != 1 {
		t.Fatalf("expected a single attempt after an error, got %d", len(repo.calls))
	}
}

func TestPendingExpiryRunOnceStopsWhenContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	repo := &fakeExpirer{results: []int64{3, 3, 3, 3}, onCall: cancel}
	w := newTestPendingExpiryWorker(repo, PendingExpiryConfig{Enabled: true, BatchSize: 3})

	if total := w.RunOnce(ctx); total != 3 {
		t.Fatalf("expected one full batch before cancellation, got %d", total)
	}
	if len(repo.calls) != 1 {
		t.Fatalf("expected no batch after cancellation, got %d calls", len(repo.calls))
	}
}

func TestPendingExpiryStartDisabledDoesNothing(t *testing.T) {
	repo := &fakeExpirer{}
	w := newTestPendingExpiryWorker(repo, PendingExpiryConfig{Enabled: false})

	done := make(chan struct{})
	go func() {
		w.Start(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("disabled worker must return at once")
	}
	if len(repo.calls) != 0 {
		t.Fatalf("disabled worker must not expire rows, got %d calls", len(repo.calls))
	}
}

func TestPendingExpiryStartRunsAtOnceAndStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	repo := &fakeExpirer{onCall: cancel}
	w := newTestPendingExpiryWorker(repo, PendingExpiryConfig{Enabled: true, Interval: time.Hour})

	done := make(chan struct{})
	go func() {
		w.Start(ctx)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker must stop when its context is cancelled")
	}
	if len(repo.calls) != 1 {
		t.Fatalf("expected one pass at start, got %d calls", len(repo.calls))
	}
}

func TestNewPendingExpiryWorkerAppliesDefaults(t *testing.T) {
	w := NewPendingExpiryWorker(&fakeExpirer{}, zap.NewNop(), PendingExpiryConfig{Enabled: true})
	if w.cfg.Interval != time.Hour || w.cfg.Window != 7*24*time.Hour || w.cfg.BatchSize != 5000 {
		t.Fatalf("unexpected defaults %+v", w.cfg)
	}
}

func TestPendingExpiryConfigFromEnv(t *testing.T) {
	t.Setenv("PENDING_EXPIRY_ENABLED", "")
	t.Setenv("PENDING_EXPIRY_INTERVAL", "")
	t.Setenv("PENDING_EXPIRY_WINDOW", "")
	t.Setenv("PENDING_EXPIRY_BATCH_SIZE", "")
	cfg := PendingExpiryConfigFromEnv(zap.NewNop())
	if !cfg.Enabled || cfg.Interval != time.Hour || cfg.Window != 7*24*time.Hour || cfg.BatchSize != 5000 {
		t.Fatalf("unexpected defaults %+v", cfg)
	}

	t.Setenv("PENDING_EXPIRY_ENABLED", "false")
	t.Setenv("PENDING_EXPIRY_INTERVAL", "15m")
	t.Setenv("PENDING_EXPIRY_WINDOW", "72h")
	t.Setenv("PENDING_EXPIRY_BATCH_SIZE", "1000")
	cfg = PendingExpiryConfigFromEnv(zap.NewNop())
	if cfg.Enabled || cfg.Interval != 15*time.Minute || cfg.Window != 72*time.Hour || cfg.BatchSize != 1000 {
		t.Fatalf("env overrides not applied: %+v", cfg)
	}

	t.Setenv("PENDING_EXPIRY_ENABLED", "maybe")
	t.Setenv("PENDING_EXPIRY_INTERVAL", "-1m")
	t.Setenv("PENDING_EXPIRY_WINDOW", "soon")
	t.Setenv("PENDING_EXPIRY_BATCH_SIZE", "0")
	cfg = PendingExpiryConfigFromEnv(zap.NewNop())
	if !cfg.Enabled || cfg.Interval != time.Hour || cfg.Window != 7*24*time.Hour || cfg.BatchSize != 5000 {
		t.Fatalf("invalid values must fall back to defaults, got %+v", cfg)
	}
}
