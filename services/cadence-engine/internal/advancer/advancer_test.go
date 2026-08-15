package advancer

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/seidu626/subscription-manager/cadence-engine/internal/repository"
	"go.uber.org/zap"
)

func emptyOutboxRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"job_id", "tenant_id", "channel_id", "subscription_id", "series_id", "planned_send_at", "sent_at"})
}

func subscriptionRows(id int64, startDate time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "tenant_id", "channel_id", "partner_role_id", "product_id",
		"user_identifier", "user_identifier_type", "entry_channel", "start_date",
	}).AddRow(id, "tenant-1", "channel-1", 1, 10, "user-1", "MSISDN", "", startDate)
}

func dailyRuleRows(seriesID int64, preferred time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"series_id", "rule_kind", "preferred_time", "days_of_week", "n_days",
		"send_start_time", "send_end_time", "timezone", "max_per_day", "catchup_mode",
	}).AddRow(
		seriesID, "DAILY", preferred, 0, 0,
		time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2000, 1, 1, 23, 59, 0, 0, time.UTC),
		"UTC", 1, "SKIP",
	)
}

// afterArg matches a time.Time argument that falls strictly after `after`,
// used to assert a FAILED reschedule always lands in the future.
type afterArg struct{ after time.Time }

func (a afterArg) Match(v driver.Value) bool {
	t, ok := v.(time.Time)
	return ok && t.After(a.after)
}

func TestProcessBatch_FailedJobReschedulesToFutureSlotWithoutCursorAdvance(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := repository.NewCadenceRepository(db, zap.NewNop())
	a := NewAdvancer(repo, zap.NewNop(), AdvancerConfig{BatchSize: 10, PollInterval: time.Second})

	startDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	preferred := time.Date(2000, 1, 1, 8, 0, 0, 0, time.UTC)
	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectQuery("status = 'SENT'").WillReturnRows(emptyOutboxRows())
	mock.ExpectQuery("status = 'FAILED'").WillReturnRows(
		emptyOutboxRows().AddRow("job-2", "tenant-1", "channel-1", int64(42), int64(7),
			now.AddDate(0, 0, -1), nil),
	)
	mock.ExpectQuery("FROM subscriptions").WithArgs(int64(42)).WillReturnRows(subscriptionRows(42, startDate))
	mock.ExpectQuery("FROM message_schedule_rules").WithArgs(int64(7)).WillReturnRows(dailyRuleRows(7, preferred))
	// RescheduleStateTx must NOT touch cursor_seq: only next_send_at plus the
	// inflight columns are set, and the next_send_at must be in the future.
	mock.ExpectExec("SET next_send_at = \\$1").
		WithArgs(afterArg{after: now}, int64(42), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE message_outbox").WithArgs("job-2").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := a.processBatch(context.Background()); err != nil {
		t.Fatalf("processBatch: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestProcessBatch_FailedJobMissingRuleMarksProcessedWithoutWedging(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := repository.NewCadenceRepository(db, zap.NewNop())
	a := NewAdvancer(repo, zap.NewNop(), AdvancerConfig{BatchSize: 10, PollInterval: time.Second})

	startDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery("status = 'SENT'").WillReturnRows(emptyOutboxRows())
	mock.ExpectQuery("status = 'FAILED'").WillReturnRows(
		emptyOutboxRows().AddRow("job-3", "tenant-1", "channel-1", int64(55), int64(9),
			time.Date(2026, 1, 9, 8, 0, 0, 0, time.UTC), nil),
	)
	mock.ExpectQuery("FROM subscriptions").WithArgs(int64(55)).WillReturnRows(subscriptionRows(55, startDate))
	mock.ExpectQuery("FROM message_schedule_rules").WithArgs(int64(9)).WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("UPDATE message_outbox").WithArgs("job-3").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := a.processBatch(context.Background()); err != nil {
		t.Fatalf("processBatch: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestProcessBatch_SentJobStillAdvancesCursor(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := repository.NewCadenceRepository(db, zap.NewNop())
	a := NewAdvancer(repo, zap.NewNop(), AdvancerConfig{BatchSize: 10, PollInterval: time.Second})

	startDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	preferred := time.Date(2000, 1, 1, 8, 0, 0, 0, time.UTC)
	sentAt := time.Date(2026, 1, 9, 8, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery("status = 'SENT'").WillReturnRows(
		emptyOutboxRows().AddRow("job-1", "tenant-1", "channel-1", int64(42), int64(7),
			sentAt, sentAt),
	)
	mock.ExpectQuery("FROM subscriptions").WithArgs(int64(42)).WillReturnRows(subscriptionRows(42, startDate))
	mock.ExpectQuery("FROM message_schedule_rules").WithArgs(int64(7)).WillReturnRows(dailyRuleRows(7, preferred))
	mock.ExpectExec("cursor_seq = cursor_seq \\+ 1").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), int64(42), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE message_outbox").WithArgs("job-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("status = 'FAILED'").WillReturnRows(emptyOutboxRows())
	mock.ExpectCommit()

	if err := a.processBatch(context.Background()); err != nil {
		t.Fatalf("processBatch: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}
