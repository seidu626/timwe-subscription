package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/zap"
)

// The claim query must compose series message text from message_content_items
// at send time, so admin edits to published content reach every not-yet-sent
// job. Direct opt-in confirmations have no content item and fall back to the
// text stored on message_outbox. LINK items get the URL appended in SQL.
func TestClaimPendingJobsResolvesTextAtSendTime(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := NewOutboxRepository(db, zap.NewNop())

	planned := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`ci\.message_text`).
		WithArgs(10).
		WillReturnRows(sqlmock.NewRows([]string{
			"job_id", "tenant_id", "channel_id", "subscription_id", "content_item_id",
			"attempt", "planned_send_at", "partner_role_id", "product_id",
			"user_identifier", "entry_channel", "delivery_channel", "message_text",
		}).
			AddRow("job-1", "tenant-1", nil, int64(42), int64(99), 1, planned,
				2117, 8509, "233241234567", "MO", "SMS", "Edited tip text").
			AddRow("job-2", "tenant-1", nil, int64(43), int64(100), 1, planned,
				2117, 8509, "233241234568", "MO", "SMS", "Read more https://x.co/a"))

	jobs, err := repo.ClaimPendingJobs(context.Background(), 10)
	if err != nil {
		t.Fatalf("ClaimPendingJobs: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if jobs[0].MessageText != "Edited tip text" {
		t.Errorf("job 1 text = %q", jobs[0].MessageText)
	}
	if jobs[1].MessageText != "Read more https://x.co/a" {
		t.Errorf("job 2 text = %q", jobs[1].MessageText)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

// Guard both message sources: series jobs use the editable content item while
// direct opt-in confirmations fall back to their outbox text.
func TestClaimPendingJobsFallsBackToDirectOutboxText(t *testing.T) {
	directFallbackMatcher := sqlmock.QueryMatcherFunc(func(expected, actual string) error {
		if !strings.Contains(actual, "ci.message_text") {
			return errors.New("claim query does not read message_content_items text")
		}
		normalized := strings.Join(strings.Fields(actual), " ")
		if !strings.Contains(normalized, "COALESCE(ci.message_text, mo.message_text)") {
			return errors.New("claim query does not fall back to direct outbox text")
		}
		return nil
	})
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(directFallbackMatcher))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := NewOutboxRepository(db, zap.NewNop())

	mock.ExpectQuery("claim").
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows([]string{
			"job_id", "tenant_id", "channel_id", "subscription_id", "content_item_id",
			"attempt", "planned_send_at", "partner_role_id", "product_id",
			"user_identifier", "entry_channel", "delivery_channel", "message_text",
		}))

	if _, err := repo.ClaimPendingJobs(context.Background(), 5); err != nil {
		t.Fatalf("ClaimPendingJobs: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}
