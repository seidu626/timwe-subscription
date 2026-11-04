package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/seidu626/subscription-manager/notification/internal/domain"
)

const (
	// chargeAttemptBatchMax caps rows per INSERT (17 placeholders each, far below Postgres' 65535).
	chargeAttemptBatchMax = 200
	// chargeAttemptBatchWait is how long a flush waits for more rows after the first one arrives.
	chargeAttemptBatchWait = 10 * time.Millisecond
	// chargeAttemptFlushTimeout bounds one flush, and separately the per-row retry after a failed flush.
	chargeAttemptFlushTimeout = 5 * time.Second
)

// ChargeAttemptWriter stores failed charge attempts in one statement.
type ChargeAttemptWriter interface {
	InsertChargeAttempts(ctx context.Context, attempts []*domain.NotificationRequest) error
}

// ErrNotificationStoreUnavailable marks a store failure worth a retryable answer: the batcher
// stopped, or a flush ran out of time.
var ErrNotificationStoreUnavailable = errors.New("notification store unavailable")

// errChargeAttemptBatcherStopped is returned when the batcher stopped before storing the row.
var errChargeAttemptBatcherStopped = fmt.Errorf("charge attempt batcher stopped: %w", ErrNotificationStoreUnavailable)

type chargeAttempt struct {
	notification *domain.NotificationRequest
	done         chan error
}

// chargeAttemptBatcher group-commits failed charge attempts. TIMWE's daily
// charge sweep posts one callback per subscriber, almost all NO_BALANCE; each
// caller waits for its row's flush result, so a 200 is only sent once the row is
// stored and a failed flush still reaches TIMWE as a retryable error.
type chargeAttemptBatcher struct {
	writer       ChargeAttemptWriter
	queue        chan chargeAttempt
	stopped      chan struct{}
	maxBatch     int
	maxWait      time.Duration
	flushTimeout time.Duration
}

func newChargeAttemptBatcher(writer ChargeAttemptWriter, maxBatch int, maxWait, flushTimeout time.Duration) *chargeAttemptBatcher {
	return &chargeAttemptBatcher{
		writer:       writer,
		queue:        make(chan chargeAttempt, 4*maxBatch),
		stopped:      make(chan struct{}),
		maxBatch:     maxBatch,
		maxWait:      maxWait,
		flushTimeout: flushTimeout,
	}
}

// submit queues n and waits for its flush. When ctx ends first the row may
// still be written; TIMWE's retry is then dropped by the unique index.
func (b *chargeAttemptBatcher) submit(ctx context.Context, n *domain.NotificationRequest) error {
	attempt := chargeAttempt{notification: n, done: make(chan error, 1)}
	select {
	case b.queue <- attempt:
	case <-b.stopped:
		return errChargeAttemptBatcherStopped
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-attempt.done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-b.stopped:
		select {
		case err := <-attempt.done:
			return err
		default:
			return errChargeAttemptBatcherStopped
		}
	}
}

func (b *chargeAttemptBatcher) run(ctx context.Context) {
	defer close(b.stopped)
	batch := make([]chargeAttempt, 0, b.maxBatch)
	for {
		select {
		case <-ctx.Done():
			return
		case first := <-b.queue:
			batch = append(batch[:0], first)
		}
		linger := time.NewTimer(b.maxWait)
	fill:
		for len(batch) < b.maxBatch {
			select {
			case next := <-b.queue:
				batch = append(batch, next)
			case <-linger.C:
				break fill
			}
		}
		linger.Stop()
		b.flush(batch)
	}
}

// flush writes the batch in one statement. If the database rejected it, each row
// is retried alone, so one bad row does not fail every batch it joins. A timeout
// is not retried row by row: that would add load to a database already too slow.
func (b *chargeAttemptBatcher) flush(batch []chargeAttempt) {
	rows := make([]*domain.NotificationRequest, len(batch))
	for i, attempt := range batch {
		rows[i] = attempt.notification
	}
	ctx, cancel := context.WithTimeout(context.Background(), b.flushTimeout)
	err := b.writer.InsertChargeAttempts(ctx, rows)
	// lib/pq may report a cancelled statement as a server error, so ask the context.
	timedOut := ctx.Err() != nil
	cancel()
	if err != nil && timedOut {
		err = fmt.Errorf("%w: flush of %d rows timed out: %v", ErrNotificationStoreUnavailable, len(batch), err)
	}
	if err == nil || len(batch) == 1 || timedOut {
		if err != nil {
			log.Printf("charge attempt insert failed: %v", err)
		}
		for _, attempt := range batch {
			attempt.done <- err
		}
		return
	}
	log.Printf("charge attempt batch of %d failed, retrying rows one at a time: %v", len(batch), err)
	ctx, cancel = context.WithTimeout(context.Background(), b.flushTimeout)
	defer cancel()
	failed := 0
	for i, attempt := range batch {
		rowErr := b.writer.InsertChargeAttempts(ctx, rows[i:i+1])
		if rowErr != nil {
			failed++
		}
		attempt.done <- rowErr
	}
	if failed > 0 {
		log.Printf("charge attempt retry: %d of %d rows still failed", failed, len(batch))
	}
}
