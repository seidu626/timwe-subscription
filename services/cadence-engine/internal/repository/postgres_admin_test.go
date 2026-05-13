package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/seidu626/subscription-manager/cadence-engine/internal/domain"
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
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), int64(123), 1, 7, "hello", true).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(999)))

	id, err := repo.UpsertContentItemTx(context.Background(), tx, "tenant-1", "channel-1", 123, 1, 7, "hello", true)
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
			"id", "tenant_id", "channel_id", "partner_role_id", "product_id", "name", "mode", "content_version", "is_active", "created_at",
		}).AddRow(int64(1), "tenant-1", "channel-1", 1, 10, "News", "SEQUENTIAL", 1, true, time.Now().UTC()))

	series, err := repo.ListSeries(context.Background(), "tenant-1", "", nil, nil, nil, 0)
	if err != nil {
		t.Fatalf("ListSeries: %v", err)
	}
	if len(series) != 1 || series[0].TenantID == nil || *series[0].TenantID != "tenant-1" {
		t.Fatalf("expected tenant-scoped series, got %#v", series)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestListSeriesRequiresTenant(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := NewCadenceRepository(db, zap.NewNop())
	if _, err := repo.ListSeries(context.Background(), "", "", nil, nil, nil, 0); err == nil {
		t.Fatal("expected tenant_id required error")
	}
}

func TestTenantIDByKeyReturnsActiveTenant(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := NewCadenceRepository(db, zap.NewNop())
	mock.ExpectQuery("FROM tenants").
		WithArgs("nrg").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("66d39a9a-f1ef-4721-a31c-5bb966d25c3d"))

	tenantID, err := repo.TenantIDByKey(context.Background(), " nrg ")
	if err != nil {
		t.Fatalf("TenantIDByKey: %v", err)
	}
	if tenantID != "66d39a9a-f1ef-4721-a31c-5bb966d25c3d" {
		t.Fatalf("tenantID = %q", tenantID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestTenantIDByKeyReturnsNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := NewCadenceRepository(db, zap.NewNop())
	mock.ExpectQuery("FROM tenants").
		WithArgs("missing").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, err = repo.TenantIDByKey(context.Background(), "missing")
	if !errors.Is(err, ErrTenantNotFound) {
		t.Fatalf("expected ErrTenantNotFound, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestInsertOutboxTxReturnsFalseForDuplicateIdempotencyKey(t *testing.T) {
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

	tenantID := "tenant-1"
	channelID := "channel-1"
	mock.ExpectExec("INSERT INTO message_outbox").
		WithArgs("job-1", "tenant-1:channel-1:2117:42:7:1:1", sqlmock.AnyArg(), sqlmock.AnyArg(), int64(42), int64(7), int64(99), sqlmock.AnyArg(), "PENDING", 0).
		WillReturnResult(sqlmock.NewResult(0, 0))

	inserted, err := repo.InsertOutboxTx(context.Background(), tx, domain.OutboxJob{
		JobID:          "job-1",
		IdempotencyKey: "tenant-1:channel-1:2117:42:7:1:1",
		TenantID:       &tenantID,
		ChannelID:      &channelID,
		SubscriptionID: int64(42),
		SeriesID:       int64(7),
		ContentItemID:  int64(99),
		PlannedSendAt:  time.Now().UTC(),
		Status:         "PENDING",
		Attempt:        0,
	})
	if err != nil {
		t.Fatalf("InsertOutboxTx: %v", err)
	}
	if inserted {
		t.Fatal("expected duplicate idempotency key to return inserted=false")
	}

	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestClaimDueStatesTxOnlyClaimsActiveTenantCompatibleStates(t *testing.T) {
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

	now := time.Now().UTC()
	mock.ExpectQuery("sms.status = 'ACTIVE'").
		WithArgs(10).
		WillReturnRows(sqlmock.NewRows([]string{
			"subscription_id", "tenant_id", "channel_id", "series_id", "cursor_seq", "next_send_at",
		}).AddRow(int64(42), "tenant-1", "channel-1", int64(7), 1, now))

	states, err := repo.ClaimDueStatesTx(context.Background(), tx, 10)
	if err != nil {
		t.Fatalf("ClaimDueStatesTx: %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("expected one state, got %d", len(states))
	}
	if states[0].TenantID == nil || *states[0].TenantID != "tenant-1" {
		t.Fatalf("expected tenant on due state, got %#v", states[0].TenantID)
	}
	if states[0].ChannelID == nil || *states[0].ChannelID != "channel-1" {
		t.Fatalf("expected channel on due state, got %#v", states[0].ChannelID)
	}

	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}
