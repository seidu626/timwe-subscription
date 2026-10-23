package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/zap"
)

// The advancer claims only cadence rows. Transactional opt-in confirmations
// (series_id NULL) share message_outbox; scanning their NULL series_id into
// an int64 failed every advancer batch from 2026-08-20 to 2026-10-23.
func TestClaimOutboxTxSkipsTransactionalRows(t *testing.T) {
	claims := map[string]func(*CadenceRepository, context.Context, *sql.Tx, int) (int, error){
		"SENT": func(r *CadenceRepository, ctx context.Context, tx *sql.Tx, limit int) (int, error) {
			jobs, err := r.ClaimSentOutboxTx(ctx, tx, limit)
			return len(jobs), err
		},
		"FAILED": func(r *CadenceRepository, ctx context.Context, tx *sql.Tx, limit int) (int, error) {
			jobs, err := r.ClaimFailedOutboxTx(ctx, tx, limit)
			return len(jobs), err
		},
	}
	for status, claim := range claims {
		t.Run(status, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("sqlmock.New: %v", err)
			}
			defer db.Close()
			repo := NewCadenceRepository(db, zap.NewNop())

			mock.ExpectBegin()
			tx, err := db.Begin()
			if err != nil {
				t.Fatalf("Begin: %v", err)
			}
			planned := time.Date(2026, 10, 23, 10, 0, 0, 0, time.UTC)
			mock.ExpectQuery(`WHERE status = '` + status + `' AND processed_at IS NULL AND series_id IS NOT NULL`).
				WithArgs(100).
				WillReturnRows(sqlmock.NewRows([]string{
					"job_id", "tenant_id", "channel_id", "subscription_id", "series_id", "planned_send_at", "sent_at",
				}).AddRow("job-1", "tenant-1", "channel-1", int64(42), int64(2), planned, planned))

			n, err := claim(repo, context.Background(), tx, 100)
			if err != nil || n != 1 {
				t.Fatalf("claim %s: n=%d err=%v", status, n, err)
			}
			_ = tx.Rollback()
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("ExpectationsWereMet: %v", err)
			}
		})
	}
}
