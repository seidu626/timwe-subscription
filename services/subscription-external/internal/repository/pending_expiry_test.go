package repository

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func TestExpireStalePendingSubscriptionsQueryShape(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create mock database: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE subscriptions s\n\t\tSET status = 'inactive', end_date = NOW()")).
		WithArgs(float64(7*24*60*60), 5000).
		WillReturnResult(sqlmock.NewResult(0, 42))

	repo := NewSubscriptionRepository(db, zap.NewNop(), nil)
	expired, err := repo.ExpireStalePendingSubscriptions(context.Background(), 7*24*time.Hour, 5000)
	if err != nil {
		t.Fatalf("ExpireStalePendingSubscriptions: %v", err)
	}
	if expired != 42 {
		t.Fatalf("expected 42 expired rows, got %d", expired)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// The query must only touch pending/preactive tenant rows past the window,
// spare rows with a DELIVERED charge for the same tenant+msisdn+product since
// creation that no USER_OPTOUT followed, and stay batched and non-blocking.
func TestExpireStalePendingQueryGuards(t *testing.T) {
	q := strings.Join(strings.Fields(expireStalePendingQuery), " ")
	for _, want := range []string{
		"WHERE s.status IN ('pending', 'preactive') AND s.tenant_id IS NOT NULL",
		"s.updated_at < (NOW() AT TIME ZONE 'UTC') - make_interval(secs => $1)",
		"NOT EXISTS ( SELECT 1 FROM notifications n WHERE n.tenant_id = s.tenant_id AND n.msisdn = s.user_identifier AND n.product_id = s.product_id",
		"n.type IN ('CHARGE', 'USER_RENEWED')",
		"upper(trim(n.mno_delivery_code)) = 'DELIVERED'",
		"n.created_at >= (s.created_at AT TIME ZONE 'UTC') - interval '5 minutes'",
		"AND NOT EXISTS ( SELECT 1 FROM notifications o WHERE o.tenant_id = s.tenant_id AND o.msisdn = s.user_identifier AND o.product_id = s.product_id AND o.type = 'USER_OPTOUT' AND o.created_at > n.created_at )",
		"ORDER BY s.id LIMIT $2 FOR UPDATE OF s SKIP LOCKED",
		"SET status = 'inactive', end_date = NOW() FROM stale WHERE s.id = stale.id AND s.status IN ('pending', 'preactive')",
	} {
		if !strings.Contains(q, want) {
			t.Errorf("expiry query is missing %q", want)
		}
	}
	if strings.Contains(strings.ToUpper(q), "DELETE") {
		t.Error("expiry query must never delete rows")
	}
}

func TestExpireStalePendingSubscriptionsRejectsInvalidArgs(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create mock database: %v", err)
	}
	defer db.Close()

	repo := NewSubscriptionRepository(db, zap.NewNop(), nil)
	if _, err := repo.ExpireStalePendingSubscriptions(context.Background(), 0, 5000); err == nil {
		t.Fatal("expected an error for a zero window")
	}
	if _, err := repo.ExpireStalePendingSubscriptions(context.Background(), time.Hour, 0); err == nil {
		t.Fatal("expected an error for a zero limit")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("invalid args must not reach the database: %v", err)
	}
}

func TestExpireStalePendingSubscriptionsReturnsExecError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create mock database: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("WITH stale AS")).
		WithArgs(float64(3600), 10).
		WillReturnError(errors.New("canceling statement due to lock timeout"))

	repo := NewSubscriptionRepository(db, zap.NewNop(), nil)
	if _, err := repo.ExpireStalePendingSubscriptions(context.Background(), time.Hour, 10); err == nil {
		t.Fatal("expected the exec error to be returned")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// Runs the real expiry statement against PostgreSQL. A payment counts as
// evidence only when no USER_OPTOUT follows it, so a former payer who opted
// out and opted in again expires like any unpaid row.
func TestExpireStalePendingSubscriptionsPostgres(t *testing.T) {
	db := openIsolatedRepositoryTestDB(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	if _, err := db.ExecContext(ctx, `INSERT INTO tenants (id) VALUES ($1)`, tenantID); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE subscriptions (
			id SERIAL PRIMARY KEY,
			tenant_id UUID,
			user_identifier VARCHAR(255) NOT NULL,
			product_id INTEGER NOT NULL,
			status VARCHAR(50),
			end_date TIMESTAMP,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		);
		CREATE TABLE notifications (
			id SERIAL PRIMARY KEY,
			tenant_id UUID,
			msisdn VARCHAR(255),
			product_id INTEGER,
			type VARCHAR(50),
			mno_delivery_code VARCHAR(50),
			created_at TIMESTAMPTZ NOT NULL
		)`); err != nil {
		t.Fatalf("create tables: %v", err)
	}
	// Every row was created 30 days ago and last written 10 days ago (re-opt-in).
	rows := []struct {
		msisdn string
		status string
		fresh  bool
	}{
		{"233000000001", "pending", false},   // paid, no opt-out: kept
		{"233000000002", "pending", false},   // paid, opted out, re-opted: expires
		{"233000000003", "preactive", false}, // never paid: expires
		{"233000000004", "pending", false},   // opted out, then paid: kept
		{"233000000005", "pending", true},    // inside the window: kept
		{"233000000006", "active", false},    // not pending: untouched
	}
	for _, r := range rows {
		updated := "NOW() AT TIME ZONE 'UTC' - interval '10 days'"
		if r.fresh {
			updated = "NOW() AT TIME ZONE 'UTC' - interval '1 hour'"
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO subscriptions (tenant_id, user_identifier, product_id, status, created_at, updated_at)
			VALUES ($1, $2, 32535, $3, NOW() AT TIME ZONE 'UTC' - interval '30 days', `+updated+`)`, tenantID, r.msisdn, r.status); err != nil {
			t.Fatalf("seed subscription %s: %v", r.msisdn, err)
		}
	}
	events := []struct {
		msisdn, typ, code, ago string
	}{
		{"233000000001", "CHARGE", "DELIVERED", "20 days"},
		{"233000000002", "CHARGE", "DELIVERED", "20 days"},
		{"233000000002", "USER_OPTOUT", "", "15 days"},
		{"233000000003", "CHARGE", "NO_BALANCE", "20 days"},
		{"233000000004", "USER_OPTOUT", "", "20 days"},
		{"233000000004", "USER_RENEWED", " delivered ", "15 days"},
	}
	for _, e := range events {
		if _, err := db.ExecContext(ctx, `INSERT INTO notifications (tenant_id, msisdn, product_id, type, mno_delivery_code, created_at)
			VALUES ($1, $2, 32535, $3, $4, NOW() - $5::interval)`, tenantID, e.msisdn, e.typ, e.code, e.ago); err != nil {
			t.Fatalf("seed notification %s %s: %v", e.msisdn, e.typ, err)
		}
	}

	repo := NewSubscriptionRepository(db, zap.NewNop(), nil)
	expired, err := repo.ExpireStalePendingSubscriptions(ctx, 7*24*time.Hour, 100)
	if err != nil {
		t.Fatalf("ExpireStalePendingSubscriptions: %v", err)
	}
	if expired != 2 {
		t.Errorf("expired %d rows, want 2", expired)
	}
	want := map[string]string{
		"233000000001": "pending",
		"233000000002": "inactive",
		"233000000003": "inactive",
		"233000000004": "pending",
		"233000000005": "pending",
		"233000000006": "active",
	}
	for msisdn, wantStatus := range want {
		var got string
		var ended bool
		if err := db.QueryRowContext(ctx, `SELECT status, end_date IS NOT NULL FROM subscriptions WHERE user_identifier = $1`, msisdn).Scan(&got, &ended); err != nil {
			t.Fatalf("read %s: %v", msisdn, err)
		}
		if got != wantStatus || ended != (wantStatus == "inactive") {
			t.Errorf("%s = (%q, end_date set %v), want %q", msisdn, got, ended, wantStatus)
		}
	}
}
