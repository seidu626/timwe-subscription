package repository

import (
	"context"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"go.uber.org/zap"
)

// An optin result never proves payment, so CreateSubscription must write the
// requested status (default "pending") instead of the column default
// 'active', and must never demote a row that is already active.
func TestCreateSubscriptionWritesPendingAndKeepsActiveOnConflict(t *testing.T) {
	tenantID := uuid.NewString()
	tests := []struct {
		name       string
		tenantID   *string
		status     string
		wantStatus string
		argCount   int
	}{
		{name: "tenant scoped default", tenantID: &tenantID, status: "", wantStatus: "pending", argCount: 16},
		{name: "tenant scoped preactive", tenantID: &tenantID, status: "preactive", wantStatus: "preactive", argCount: 16},
		{name: "legacy default", tenantID: nil, status: "  ", wantStatus: "pending", argCount: 14},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("create mock database: %v", err)
			}
			defer db.Close()

			args := make([]driver.Value, tt.argCount)
			for i := range args {
				args[i] = sqlmock.AnyArg()
			}
			args[tt.argCount-1] = tt.wantStatus
			mock.ExpectExec(regexp.QuoteMeta("status = CASE WHEN subscriptions.status = 'active' THEN subscriptions.status ELSE EXCLUDED.status END")).
				WithArgs(args...).
				WillReturnResult(sqlmock.NewResult(0, 1))

			repo := NewSubscriptionRepository(db, zap.NewNop(), nil)
			if err := repo.CreateSubscription(&domain.SubscriptionRequest{
				TenantID:       tt.tenantID,
				UserIdentifier: "233572503330",
				ProductId:      32535,
				Status:         tt.status,
			}); err != nil {
				t.Fatalf("CreateSubscription: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("unmet expectations: %v", err)
			}
		})
	}
}

func TestUpdateSubscriptionStatusUnlessActiveGuardsActiveRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create mock database: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE subscriptions SET status = $1, end_date = CASE WHEN $1::varchar IN ('pending', 'preactive') THEN NULL ELSE end_date END WHERE user_identifier = $2 AND product_id = $3 AND status IS DISTINCT FROM 'active'")).
		WithArgs("pending", "233572503330", 32535).
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := NewSubscriptionRepository(db, zap.NewNop(), nil)
	if err := repo.UpdateSubscriptionStatusUnlessActive("233572503330", "32535", "pending"); err != nil {
		t.Fatalf("UpdateSubscriptionStatusUnlessActive: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// A CHARGE with mno_delivery_code NO_BALANCE is a failed attempt and must not
// be forwarded to acquisition-api as a conversion.
func TestFetchChargeSuccessNotificationsFiltersDelivered(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create mock database: %v", err)
	}
	defer db.Close()

	since := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("AND UPPER(TRIM(COALESCE(mno_delivery_code, ''))) = 'DELIVERED'")).
		WithArgs("CHARGE", since, int64(10), 50).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "channel_id", "msisdn", "product_id", "transaction_uuid", "created_at"}))

	repo := NewSubscriptionRepository(db, zap.NewNop(), nil)
	if _, err := repo.FetchChargeSuccessNotifications(since, 10, 50); err != nil {
		t.Fatalf("FetchChargeSuccessNotifications: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// Runs the real upsert against PostgreSQL: a re-optin keeps an active (paid)
// row active, moves an inactive row to pending without the end_date of its
// previous term, and inserts new rows pending.
func TestCreateSubscriptionStatusTransitionsPostgres(t *testing.T) {
	db := openIsolatedRepositoryTestDB(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	if _, err := db.ExecContext(ctx, `INSERT INTO tenants (id) VALUES ($1)`, tenantID); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE subscriptions (
			id SERIAL PRIMARY KEY,
			tenant_id UUID REFERENCES tenants(id),
			channel_id UUID,
			partner_role_id INTEGER NOT NULL,
			user_identifier VARCHAR(255) NOT NULL,
			user_identifier_type VARCHAR(50),
			product_id INTEGER NOT NULL,
			mcc VARCHAR(10), mnc VARCHAR(10), entry_channel VARCHAR(50),
			large_account VARCHAR(50), sub_keyword VARCHAR(50), tracking_id VARCHAR(255),
			client_ip VARCHAR(64), campaign_url TEXT, transaction_id VARCHAR(255),
			start_date TIMESTAMP, end_date TIMESTAMP,
			status VARCHAR(50) DEFAULT 'active'
		)`); err != nil {
		t.Fatalf("create subscriptions table: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		CREATE UNIQUE INDEX ux_subscriptions_tenant ON subscriptions (tenant_id, partner_role_id, user_identifier, product_id) WHERE tenant_id IS NOT NULL`); err != nil {
		t.Fatalf("create tenant unique index: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO subscriptions (tenant_id, partner_role_id, user_identifier, product_id, status)
		VALUES ($1, 42, '233000000001', 32535, 'active'), ($1, 42, '233000000002', 32535, 'inactive'),
			($1, 42, '233000000004', 32535, 'inactive'), ($1, 42, '233000000005', 32535, 'inactive')`, tenantID); err != nil {
		t.Fatalf("seed subscriptions: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE subscriptions SET end_date = '2026-10-20 10:00:00'`); err != nil {
		t.Fatalf("seed end dates: %v", err)
	}

	repo := NewSubscriptionRepository(db, zap.NewNop(), nil)
	for _, msisdn := range []string{"233000000001", "233000000002", "233000000003"} {
		if err := repo.CreateSubscription(&domain.SubscriptionRequest{
			TenantID:       &tenantID,
			PartnerRoleId:  42,
			UserIdentifier: msisdn,
			ProductId:      32535,
		}); err != nil {
			t.Fatalf("CreateSubscription(%s): %v", msisdn, err)
		}
	}
	for msisdn, status := range map[string]string{"233000000001": "preactive", "233000000004": "preactive", "233000000005": "inactive"} {
		if err := repo.UpdateSubscriptionStatusUnlessActive(msisdn, "32535", status); err != nil {
			t.Fatalf("UpdateSubscriptionStatusUnlessActive(%s): %v", msisdn, err)
		}
	}

	want := map[string]struct {
		status     string
		endDateSet bool
	}{
		"233000000001": {"active", true},
		"233000000002": {"pending", false},
		"233000000003": {"pending", false},
		"233000000004": {"preactive", false},
		"233000000005": {"inactive", true},
	}
	for msisdn, w := range want {
		var got string
		var endDateSet bool
		if err := db.QueryRowContext(ctx, `SELECT status, end_date IS NOT NULL FROM subscriptions WHERE user_identifier = $1`, msisdn).Scan(&got, &endDateSet); err != nil {
			t.Fatalf("read status for %s: %v", msisdn, err)
		}
		if got != w.status {
			t.Errorf("status for %s = %q, want %q", msisdn, got, w.status)
		}
		if endDateSet != w.endDateSet {
			t.Errorf("end_date set for %s = %v, want %v", msisdn, endDateSet, w.endDateSet)
		}
	}
}
