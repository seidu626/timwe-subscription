package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/seidu626/subscription-manager/notification/internal/domain"
)

// ownerRepoStub counts product-owner lookups and full-path stores.
type ownerRepoStub struct {
	serviceRepoStub
	mu         sync.Mutex
	lookups    int
	owner      string
	ownerErr   error
	fullStored []*domain.NotificationRequest
}

func (s *ownerRepoStub) TenantIDByProductID(context.Context, int) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lookups++
	return s.owner, s.ownerErr
}

func (s *ownerRepoStub) ProcessNotification(_ context.Context, n *domain.NotificationRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fullStored = append(s.fullStored, n)
	return nil
}

// attemptWriterStub records each InsertChargeAttempts call.
type attemptWriterStub struct {
	mu      sync.Mutex
	batches [][]*domain.NotificationRequest
	fail    func(batch []*domain.NotificationRequest) error
	block   bool
}

func (w *attemptWriterStub) InsertChargeAttempts(ctx context.Context, batch []*domain.NotificationRequest) error {
	if w.block {
		<-ctx.Done()
		return errors.New("pq: canceling statement due to user request")
	}
	w.mu.Lock()
	w.batches = append(w.batches, append([]*domain.NotificationRequest(nil), batch...))
	w.mu.Unlock()
	if w.fail != nil {
		return w.fail(batch)
	}
	return nil
}

func (w *attemptWriterStub) calls() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.batches)
}

func failedCharge(msisdn string) *domain.NotificationRequest {
	return &domain.NotificationRequest{Type: domain.ChargeEvent, MnoDeliveryCode: "NO_BALANCE", MSISDN: msisdn, ProductID: 33261}
}

func TestTenantIDByProductID_ReusesAnswersButNotFailures(t *testing.T) {
	t.Run("owner is cached until the TTL passes", func(t *testing.T) {
		repo := &ownerRepoStub{owner: "tenant-a"}
		svc := NewNotificationService(repo)
		now := time.Unix(1_700_000_000, 0)
		svc.owners.now = func() time.Time { return now }

		for i := 0; i < 3; i++ {
			got, err := svc.TenantIDByProductID(context.Background(), 33261)
			if err != nil || got != "tenant-a" {
				t.Fatalf("lookup %d = %q, %v", i, got, err)
			}
		}
		if repo.lookups != 1 {
			t.Fatalf("repo lookups = %d; want 1", repo.lookups)
		}
		now = now.Add(productOwnerCacheTTL)
		if _, err := svc.TenantIDByProductID(context.Background(), 33261); err != nil {
			t.Fatal(err)
		}
		if repo.lookups != 2 {
			t.Fatalf("repo lookups after TTL = %d; want 2", repo.lookups)
		}
	})

	t.Run("no unique owner is cached", func(t *testing.T) {
		repo := &ownerRepoStub{ownerErr: fmt.Errorf("product 8509: %w", domain.ErrProductOwnerNotUnique)}
		svc := NewNotificationService(repo)
		for i := 0; i < 2; i++ {
			if _, err := svc.TenantIDByProductID(context.Background(), 8509); !errors.Is(err, domain.ErrProductOwnerNotUnique) {
				t.Fatalf("lookup %d err = %v; want ErrProductOwnerNotUnique", i, err)
			}
		}
		if repo.lookups != 1 {
			t.Fatalf("repo lookups = %d; want 1", repo.lookups)
		}
	})

	t.Run("a failed lookup is retried", func(t *testing.T) {
		repo := &ownerRepoStub{ownerErr: errors.New("pq: sorry, too many clients already")}
		svc := NewNotificationService(repo)
		for i := 0; i < 2; i++ {
			if _, err := svc.TenantIDByProductID(context.Background(), 33261); err == nil {
				t.Fatalf("lookup %d: expected the repo error", i)
			}
		}
		if repo.lookups != 2 {
			t.Fatalf("repo lookups = %d; want 2", repo.lookups)
		}
	})
}

func TestProcessNotification_RoutesOnlyFailedChargesToTheBatcher(t *testing.T) {
	repo := &ownerRepoStub{}
	writer := &attemptWriterStub{}
	svc := NewNotificationService(repo)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.StartChargeAttemptBatching(ctx, writer)

	full := []*domain.NotificationRequest{
		{Type: domain.ChargeEvent, MnoDeliveryCode: "DELIVERED"},
		{Type: domain.ChargeEvent, MnoDeliveryCode: ""},
		{Type: domain.UserOptinEvent, MnoDeliveryCode: "NO_BALANCE"},
		{Type: domain.UserOptoutEvent},
	}
	for _, n := range full {
		if err := svc.ProcessNotification(context.Background(), n); err != nil {
			t.Fatal(err)
		}
	}
	for _, n := range []*domain.NotificationRequest{failedCharge("1"), {Type: domain.UserRenewedEvent, MnoDeliveryCode: "INSUFFICIENT_FUNDS"}} {
		if err := svc.ProcessNotification(context.Background(), n); err != nil {
			t.Fatal(err)
		}
	}
	if len(repo.fullStored) != len(full) {
		t.Fatalf("full path stored %d; want %d", len(repo.fullStored), len(full))
	}
	stored := 0
	for _, batch := range writer.batches {
		stored += len(batch)
	}
	if stored != 2 {
		t.Fatalf("batcher stored %d; want 2", stored)
	}
}

func TestChargeAttemptBatcher_GroupsConcurrentCallbacks(t *testing.T) {
	writer := &attemptWriterStub{}
	b := newChargeAttemptBatcher(writer, 50, 50*time.Millisecond, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.run(ctx)

	const callers = 120
	var wg sync.WaitGroup
	errs := make(chan error, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- b.submit(context.Background(), failedCharge(fmt.Sprint(i)))
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("submit: %v", err)
		}
	}
	stored := 0
	for _, batch := range writer.batches {
		if len(batch) > 50 {
			t.Fatalf("batch of %d exceeds the cap of 50", len(batch))
		}
		stored += len(batch)
	}
	if stored != callers {
		t.Fatalf("stored %d rows; want %d", stored, callers)
	}
	if writer.calls() >= callers/2 {
		t.Fatalf("%d inserts for %d callbacks; want them grouped", writer.calls(), callers)
	}
}

// One row the database rejects fails only its own caller, not the whole batch.
func TestChargeAttemptBatcher_IsolatesARejectedRow(t *testing.T) {
	writer := &attemptWriterStub{fail: func(batch []*domain.NotificationRequest) error {
		for _, n := range batch {
			if n.MSISDN == "bad" {
				return errors.New("pq: value too long for type character varying(20)")
			}
		}
		return nil
	}}
	b := newChargeAttemptBatcher(writer, 10, 50*time.Millisecond, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.run(ctx)

	results := make(map[string]error)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, msisdn := range []string{"a", "bad", "c"} {
		wg.Add(1)
		go func(msisdn string) {
			defer wg.Done()
			err := b.submit(context.Background(), failedCharge(msisdn))
			mu.Lock()
			results[msisdn] = err
			mu.Unlock()
		}(msisdn)
	}
	wg.Wait()
	if results["bad"] == nil {
		t.Fatal("rejected row reported success")
	}
	if results["a"] != nil || results["c"] != nil {
		t.Fatalf("good rows failed: a=%v c=%v", results["a"], results["c"])
	}
}

// A flush that runs out of time is a retryable store outage, not a per-row retry storm.
func TestChargeAttemptBatcher_TimedOutFlushIsUnavailable(t *testing.T) {
	writer := &attemptWriterStub{block: true}
	b := newChargeAttemptBatcher(writer, 10, time.Millisecond, 20*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.run(ctx)

	err := b.submit(context.Background(), failedCharge("1"))
	if !errors.Is(err, ErrNotificationStoreUnavailable) {
		t.Fatalf("submit err = %v; want ErrNotificationStoreUnavailable", err)
	}
}

func TestChargeAttemptBatcher_StoppedOrCallerGone(t *testing.T) {
	t.Run("stopped batcher refuses rows", func(t *testing.T) {
		b := newChargeAttemptBatcher(&attemptWriterStub{}, 10, time.Millisecond, time.Second)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		b.run(ctx)
		// Fill the queue so the only ready case is the stopped channel.
		for i := 0; i < cap(b.queue); i++ {
			b.queue <- chargeAttempt{notification: failedCharge("x"), done: make(chan error, 1)}
		}
		if err := b.submit(context.Background(), failedCharge("1")); !errors.Is(err, ErrNotificationStoreUnavailable) {
			t.Fatalf("submit err = %v; want ErrNotificationStoreUnavailable", err)
		}
	})

	t.Run("caller deadline ends the wait", func(t *testing.T) {
		writer := &attemptWriterStub{block: true}
		b := newChargeAttemptBatcher(writer, 10, time.Millisecond, time.Second)
		runCtx, stop := context.WithCancel(context.Background())
		defer stop()
		go b.run(runCtx)

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		if err := b.submit(ctx, failedCharge("1")); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("submit err = %v; want context.DeadlineExceeded", err)
		}
	})
}
