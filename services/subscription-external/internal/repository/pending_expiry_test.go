package repository

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
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
// creation, and stay batched and non-blocking.
func TestExpireStalePendingQueryGuards(t *testing.T) {
	q := strings.Join(strings.Fields(expireStalePendingQuery), " ")
	for _, want := range []string{
		"WHERE s.status IN ('pending', 'preactive') AND s.tenant_id IS NOT NULL",
		"s.updated_at < (NOW() AT TIME ZONE 'UTC') - make_interval(secs => $1)",
		"NOT EXISTS ( SELECT 1 FROM notifications n WHERE n.tenant_id = s.tenant_id AND n.msisdn = s.user_identifier AND n.product_id = s.product_id",
		"n.type IN ('CHARGE', 'USER_RENEWED')",
		"upper(trim(n.mno_delivery_code)) = 'DELIVERED'",
		"n.created_at >= (s.created_at AT TIME ZONE 'UTC') - interval '5 minutes'",
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
