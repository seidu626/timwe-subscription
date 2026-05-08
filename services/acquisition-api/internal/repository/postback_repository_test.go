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

func TestCreateOutboxPersistsTenantChannelAndFailureReason(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	tenantID := "11111111-1111-1111-1111-111111111111"
	channelID := "22222222-2222-2222-2222-222222222222"
	reason := "missing click_id"
	body := `{"kind":"diagnostic"}`
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	outbox := &domain.PostbackOutbox{
		ID:                  uuid.New(),
		TenantID:            &tenantID,
		ChannelID:           &channelID,
		TransactionID:       uuid.New(),
		Event:               domain.PostbackEventConversion,
		Provider:            "mobplus",
		URLTemplateRendered: "skipped://postback",
		HTTPMethod:          "GET",
		Headers:             "{}",
		Body:                &body,
		FailureReason:       &reason,
		AttemptCount:        0,
		MaxAttempts:         0,
		Status:              domain.PostbackStatusFailed,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	mock.ExpectExec("INSERT INTO postback_outbox").
		WithArgs(
			outbox.ID,
			sql.NullString{String: tenantID, Valid: true},
			sql.NullString{String: channelID, Valid: true},
			outbox.TransactionID,
			outbox.Event,
			outbox.Provider,
			outbox.URLTemplateRendered,
			outbox.HTTPMethod,
			outbox.Headers,
			sql.NullString{String: body, Valid: true},
			sql.NullString{String: reason, Valid: true},
			outbox.AttemptCount,
			outbox.MaxAttempts,
			sql.NullTime{},
			outbox.Status,
			outbox.CreatedAt,
			outbox.UpdatedAt,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewPostbackRepository(db, zap.NewNop())
	if err := repo.CreateOutbox(outbox); err != nil {
		t.Fatalf("create outbox: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestResetForRetryForTenantScopesUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	tenantID := "11111111-1111-1111-1111-111111111111"
	id := uuid.New()
	mock.ExpectExec("UPDATE postback_outbox").
		WithArgs(tenantID, id).
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := NewPostbackRepository(db, zap.NewNop())
	if err := repo.ResetForRetryForTenant(tenantID, id); err == nil {
		t.Fatal("expected not found for cross-tenant or missing postback")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
