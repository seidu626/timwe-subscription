package repository

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/zap"
)

// A status write must only update existing rows. sqlmock rejects any
// statement it does not expect, so an INSERT here fails the test.
func TestUpsertSubscriptionStatusIsUpdateOnly(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create mock database: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(`^\s*UPDATE subscriptions SET status = \$3\s+WHERE `+regexp.QuoteMeta("user_identifier = $1 AND product_id = $2 AND status IS DISTINCT FROM $3")+`$`).
		WithArgs("233241234567", 32535, "inactive").
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := NewSubscriptionRepository(db, zap.NewNop(), nil)
	if err := repo.UpsertSubscriptionStatus("233241234567", 32535, "inactive"); err != nil {
		t.Fatalf("UpsertSubscriptionStatus: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestUpsertSubscriptionStatusRejectsInvalidArgs(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create mock database: %v", err)
	}
	defer db.Close()

	repo := NewSubscriptionRepository(db, zap.NewNop(), nil)
	if err := repo.UpsertSubscriptionStatus("", 32535, "inactive"); err == nil {
		t.Fatal("expected an error for an empty msisdn")
	}
	if err := repo.UpsertSubscriptionStatus("233241234567", 0, "inactive"); err == nil {
		t.Fatal("expected an error for a non-positive product id")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("invalid args must not reach the database: %v", err)
	}
}
