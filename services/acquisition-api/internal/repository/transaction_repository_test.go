package repository

import (
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"go.uber.org/zap"
)

func TestCreateTransactionPersistsTenantID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	tenantID := "22222222-2222-2222-2222-222222222222"
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	status := domain.StatusPending
	tx := &domain.AcquisitionTransaction{
		ID:             uuid.New(),
		CorrelationID:  uuid.New(),
		TenantID:       &tenantID,
		CampaignSlug:   "daily",
		MSISDN:         "233561914461",
		Status:         status,
		ConsentChecked: true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	mock.ExpectExec("INSERT INTO acquisition_transactions").
		WithArgs(
			tx.ID,
			tx.CorrelationID,
			sql.NullString{String: tenantID, Valid: true},
			tx.CampaignSlug,
			tx.MSISDN,
			tx.Status,
			sql.NullString{},
			sql.NullString{},
			sql.NullString{},
			sql.NullString{},
			sql.NullString{},
			sql.NullString{},
			sql.NullString{},
			tx.ConsentRequired,
			tx.ConsentChecked,
			sql.NullString{},
			sql.NullTime{},
			sql.NullString{},
			sql.NullInt64{},
			sql.NullInt64{},
			sql.NullInt64{},
			sql.NullString{},
			sql.NullString{},
			sql.NullString{},
			sql.NullString{},
			sql.NullString{},
			sql.NullString{},
			tx.CreatedAt,
			tx.UpdatedAt,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewTransactionRepository(db, zap.NewNop())
	if err := repo.Create(tx); err != nil {
		t.Fatalf("create transaction: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
