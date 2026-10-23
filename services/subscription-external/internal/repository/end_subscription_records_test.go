package repository

import (
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/zap"
)

// INVALID_MSISDN cleanup must end subscriptions, never delete them: a DELETE
// cascades to message_outbox and loses paid history. sqlmock rejects any
// statement it does not expect, so a DELETE (or the old COUNT) fails the test.
func TestEndSubscriptionRecordsUpdatesInsteadOfDeleting(t *testing.T) {
	for _, affected := range []int64{0, 2} {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("create mock database: %v", err)
		}

		mock.ExpectExec(regexp.QuoteMeta("SET status = 'inactive', end_date = NOW()") + `\s+WHERE user_identifier = \$1\s+AND status IS DISTINCT FROM 'inactive'`).
			WithArgs("233241234567").
			WillReturnResult(sqlmock.NewResult(0, affected))

		repo := NewSubscriptionRepository(db, zap.NewNop(), nil)
		if err := repo.EndSubscriptionRecords("233241234567"); err != nil {
			t.Fatalf("EndSubscriptionRecords (%d rows): %v", affected, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet expectations (%d rows): %v", affected, err)
		}
		db.Close()
	}
}

func TestEndSubscriptionRecordsReturnsUpdateError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create mock database: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE subscriptions")).
		WithArgs("233241234567").
		WillReturnError(errors.New("connection reset"))

	repo := NewSubscriptionRepository(db, zap.NewNop(), nil)
	if err := repo.EndSubscriptionRecords("233241234567"); err == nil {
		t.Fatal("expected the update error to be returned")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
