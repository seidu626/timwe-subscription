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

func TestFindByTenantAndTimweTransactionIDScopesLookup(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	tenantID := "11111111-1111-1111-1111-111111111111"
	timweTxID := "timwe-123"
	txID := uuid.New()
	correlationID := uuid.New()
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows(transactionSelectColumns()).
		AddRow(
			txID,
			correlationID,
			"daily",
			"233241234567",
			domain.StatusSubscribed,
			nil,
			nil,
			"mobplus",
			"click-1",
			[]byte(`{"provider":"mobplus","click_id":"click-1"}`),
			nil,
			nil,
			false,
			true,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			timweTxID,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			false,
			now,
			now,
		)
	mock.ExpectQuery("WHERE tenant_id = \\$1 AND timwe_transaction_id = \\$2").
		WithArgs(tenantID, timweTxID).
		WillReturnRows(rows)

	repo := NewTransactionRepository(db, zap.NewNop())
	tx, err := repo.FindByTenantAndTimweTransactionID(tenantID, timweTxID)
	if err != nil {
		t.Fatalf("find transaction: %v", err)
	}
	if tx.TenantID == nil || *tx.TenantID != tenantID {
		t.Fatalf("expected tenant id %q, got %#v", tenantID, tx.TenantID)
	}
	if tx.TimweTransactionID == nil || *tx.TimweTransactionID != timweTxID {
		t.Fatalf("expected timwe transaction id %q, got %#v", timweTxID, tx.TimweTransactionID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestTenantIDByKeyResolvesActiveTenant(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	tenantID := "11111111-1111-1111-1111-111111111111"
	mock.ExpectQuery("FROM tenants").
		WithArgs("nrg").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(tenantID))

	repo := NewTransactionRepository(db, zap.NewNop())
	got, err := repo.TenantIDByKey(" nrg ")
	if err != nil {
		t.Fatalf("TenantIDByKey: %v", err)
	}
	if got != tenantID {
		t.Fatalf("expected tenant id %q, got %q", tenantID, got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestListTransactionsScopesByTenantID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	tenantID := "11111111-1111-1111-1111-111111111111"
	txID := uuid.New()
	correlationID := uuid.New()
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM acquisition_transactions WHERE 1=1 AND tenant_id = \\$1::uuid").
		WithArgs(tenantID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	rows := sqlmock.NewRows(transactionSelectColumns()).
		AddRow(
			txID,
			correlationID,
			"daily",
			"233241234567",
			domain.StatusSubscribed,
			nil,
			nil,
			"mobplus",
			"click-1",
			[]byte(`{"provider":"mobplus","click_id":"click-1"}`),
			nil,
			nil,
			false,
			true,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			"timwe-123",
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			false,
			now,
			now,
		)
	mock.ExpectQuery("FROM acquisition_transactions\\s+WHERE 1=1 AND tenant_id = \\$1::uuid\\s+ORDER BY created_at DESC").
		WithArgs(tenantID, 20, 0).
		WillReturnRows(rows)

	repo := NewTransactionRepository(db, zap.NewNop())
	result, err := repo.ListTransactions(&TransactionListFilter{TenantID: tenantID, Limit: 20})
	if err != nil {
		t.Fatalf("list transactions: %v", err)
	}
	if result.TotalCount != 1 || len(result.Transactions) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func transactionSelectColumns() []string {
	return []string{
		"id",
		"correlation_id",
		"campaign_slug",
		"msisdn",
		"status",
		"next_action",
		"next_action_payload",
		"ad_provider",
		"click_id",
		"attribution_data",
		"ip_address",
		"user_agent",
		"consent_required",
		"consent_checked",
		"consent_version",
		"consent_timestamp",
		"landing_version_hash",
		"offer_product_id",
		"pricepoint_id",
		"partner_role_id",
		"timwe_transaction_id",
		"transaction_auth_code",
		"timwe_status",
		"he_source",
		"he_msisdn",
		"he_operator",
		"charged_at",
		"charge_payout",
		"conversion_postback_sent",
		"created_at",
		"updated_at",
	}
}
