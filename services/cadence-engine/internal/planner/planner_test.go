package planner

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/seidu626/subscription-manager/cadence-engine/internal/domain"
	"github.com/seidu626/subscription-manager/cadence-engine/internal/repository"
	"go.uber.org/zap"
)

// Message text snapshotting was removed from the planner: the notification
// sender now composes text (including the LINK URL suffix) from
// message_content_items at send time so post-publish edits reach pending jobs.
// That composition is covered by TestClaimPendingJobsResolvesTextAtSendTime in
// services/notification/internal/repository.

func TestOutboxIdempotencyKey_DistinctPerOccurrenceSamePerClaim(t *testing.T) {
	tenant := "t1"
	channel := "c1"
	sub := &domain.Subscription{ID: 42, PartnerRoleID: 7}
	series := &domain.MessageSeries{ID: 3, ContentVersion: 1}
	slot := time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)
	state := domain.DueState{SubscriptionID: 42, CursorSeq: 5, NextSendAt: slot}

	first := outboxIdempotencyKey(&tenant, &channel, sub, series, state)
	reclaimed := outboxIdempotencyKey(&tenant, &channel, sub, series, state)
	if first != reclaimed {
		t.Fatalf("same occurrence must dedupe: %q != %q", first, reclaimed)
	}

	// A FAILED occurrence reschedules the state to a later slot with the same
	// cursor; the retry must not collide with the terminal job's key.
	state.NextSendAt = slot.Add(24 * time.Hour)
	retry := outboxIdempotencyKey(&tenant, &channel, sub, series, state)
	if retry == first {
		t.Fatalf("rescheduled occurrence must get a fresh key, got %q twice", first)
	}
}

func newMockPlanner(t *testing.T) (*Planner, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := repository.NewCadenceRepository(db, zap.NewNop())
	return NewPlanner(repo, zap.NewNop(), PlannerConfig{BatchSize: 500, InflightDuration: 10 * time.Minute}), mock
}

// With every tenant paused the planner must not claim any state: no outbox
// job may be created for a paused tenant. sqlmock fails on the unexpected
// claim query if it runs.
func TestProcessBatchPlansNothingWhenAllTenantsPaused(t *testing.T) {
	p, mock := newMockPlanner(t)
	mock.ExpectBegin()
	mock.ExpectQuery("sms_sending_paused").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectRollback()

	if err := p.processBatch(context.Background()); err != nil {
		t.Fatalf("processBatch: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestProcessBatchClaimsOnlySendingTenants(t *testing.T) {
	p, mock := newMockPlanner(t)
	mock.ExpectBegin()
	mock.ExpectQuery("sms_sending_paused").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("tenant-live"))
	mock.ExpectQuery("FROM subscription_message_state").
		WithArgs(500, `{"tenant-live"}`).
		WillReturnRows(sqlmock.NewRows([]string{
			"subscription_id", "tenant_id", "channel_id", "series_id", "cursor_seq", "next_send_at",
		}))
	mock.ExpectRollback()

	if err := p.processBatch(context.Background()); err != nil {
		t.Fatalf("processBatch: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}
