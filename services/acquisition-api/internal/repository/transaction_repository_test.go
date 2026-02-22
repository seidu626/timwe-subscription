package repository

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/zap"
)

func TestCheckThrottle_ExcludesFailedAndCancelledStatuses(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewTransactionRepository(db, zap.NewNop())
	throttles := map[string]interface{}{"per_msisdn_per_day": float64(1)}

	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*status NOT IN \('FAILED', 'CANCELLED'\).*created_at >= CURRENT_DATE`).
		WithArgs("gh-campaign", "233571111111").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	throttled, err := repo.CheckThrottle("gh-campaign", "233571111111", "", throttles)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if throttled {
		t.Fatal("expected request not to be throttled")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
