package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/zap"
)

func TestUpsertContentItemTx_ReturnsID(t *testing.T) {
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

	mock.ExpectQuery("INSERT INTO message_content_items").
		WithArgs(int64(123), 1, 7, "hello", true).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(999)))

	id, err := repo.UpsertContentItemTx(context.Background(), tx, 123, 1, 7, "hello", true)
	if err != nil {
		t.Fatalf("UpsertContentItemTx: %v", err)
	}
	if id != 999 {
		t.Fatalf("expected id 999, got %d", id)
	}

	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestDeactivateMissingContentItemsTx_WithKeepList(t *testing.T) {
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

	mock.ExpectExec("UPDATE message_content_items").
		WithArgs(int64(5), 2, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 3))

	n, err := repo.DeactivateMissingContentItemsTx(context.Background(), tx, 5, 2, []int{1, 2, 3})
	if err != nil {
		t.Fatalf("DeactivateMissingContentItemsTx: %v", err)
	}
	if n != 3 {
		t.Fatalf("expected 3 rows affected, got %d", n)
	}

	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestListSeries_DefaultLimitClamp(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := NewCadenceRepository(db, zap.NewNop())

	// limit should clamp to 200 when <=0
	mock.ExpectQuery("FROM product_message_series").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "partner_role_id", "product_id", "name", "mode", "content_version", "is_active", "created_at",
		}).AddRow(int64(1), 1, 10, "News", "SEQUENTIAL", 1, true, time.Now().UTC()))

	_, err = repo.ListSeries(context.Background(), nil, nil, nil, 0)
	if err != nil {
		t.Fatalf("ListSeries: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

