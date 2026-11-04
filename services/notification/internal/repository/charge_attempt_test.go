package repository

import (
	"context"
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/seidu626/subscription-manager/notification/internal/domain"
)

func anyNotificationArgs(rows int) []driver.Value {
	args := make([]driver.Value, rows*notificationInsertColumnCount)
	for i := range args {
		args[i] = sqlmock.AnyArg()
	}
	return args
}

// TIMWE's daily sweep is almost all NO_BALANCE: those rows go in one statement,
// with the same replay guard as the single-row path.
func TestInsertChargeAttempts_OneStatementPerBatch(t *testing.T) {
	repo, mock := newNotificationRepoWithMock(t)
	first := projectionCallback(domain.ChargeEvent, "NO_BALANCE")
	second := projectionCallback(domain.UserRenewedEvent, "no_balance")
	second.MSISDN = "233249999999"

	args := anyNotificationArgs(2)
	args[8] = first.MSISDN
	args[11] = "NO_BALANCE"
	args[notificationInsertColumnCount+8] = second.MSISDN
	args[notificationInsertColumnCount+16] = domain.UserRenewedEvent
	mock.ExpectExec(`^INSERT INTO notifications \(\s+tenant_id, .* type\s+\) VALUES \(\$1, \$2, .*\$17\), \(\$18, .*\$34\) ON CONFLICT DO NOTHING$`).
		WithArgs(args...).
		WillReturnResult(sqlmock.NewResult(0, 2))

	if err := repo.InsertChargeAttempts(context.Background(), []*domain.NotificationRequest{first, second}); err != nil {
		t.Fatalf("InsertChargeAttempts: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A DELIVERED charge projects a payment, and an empty code proves nothing; neither may skip the full path.
func TestInsertChargeAttempts_RejectsRowsThatNeedTheFullPath(t *testing.T) {
	for _, code := range []string{"DELIVERED", " delivered ", ""} {
		repo, mock := newNotificationRepoWithMock(t)
		rows := []*domain.NotificationRequest{
			projectionCallback(domain.ChargeEvent, "NO_BALANCE"),
			projectionCallback(domain.ChargeEvent, code),
		}
		if err := repo.InsertChargeAttempts(context.Background(), rows); err == nil {
			t.Fatalf("code %q: expected an error", code)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("code %q: %v", code, err)
		}
	}
}

// A NO_BALANCE charge used to be queued as CHARGE_SUCCESS. Even with a tenant
// binding in place, the full path now stores the row and nothing else.
func TestProcessNotification_FailedChargeQueuesNoTenantEvent(t *testing.T) {
	for _, eventType := range []string{domain.ChargeEvent, domain.UserRenewedEvent} {
		repo, mock := newNotificationRepoWithMock(t)
		mock.ExpectBegin()
		expectInsertNotification(mock, true)
		mock.ExpectCommit()

		if err := repo.ProcessNotification(context.Background(), projectionCallback(eventType, "NO_BALANCE")); err != nil {
			t.Fatalf("%s: ProcessNotification: %v", eventType, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("%s: %v", eventType, err)
		}
	}
}

// Events already queued for uncollected charges are closed at send time, never posted as CHARGE_SUCCESS.
func TestDispatchTenantEvent_SkipsUndeliveredCharge(t *testing.T) {
	url, hits := stubWebhook(t)
	bindTestTenantEvents(t, url, false)
	repo, mock := newNotificationRepoWithMock(t)
	mock.ExpectQuery(`WITH due AS .* UPDATE tenant_event_outbox o\s+SET status = 'sending'.* RETURNING .*mno_delivery_code`).
		WillReturnRows(sqlmock.NewRows([]string{"event_id", "tenant_id", "payload", "attempts", "delivery_code"}).
			AddRow(optoutTestEvent, optoutTestTenant, []byte(`{"type":"CHARGE_SUCCESS","msisdn":"233260000000"}`), 1, "NO_BALANCE"))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE tenant_event_outbox SET status = 'skipped'`)).
		WithArgs(optoutTestTenant, optoutTestEvent, tenantEventNotDelivered).
		WillReturnResult(sqlmock.NewResult(0, 1))

	claimed, err := repo.dispatchOneTenantEvent(context.Background())
	if err != nil || !claimed {
		t.Fatalf("dispatchOneTenantEvent = %v, %v; want claimed, nil", claimed, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("webhook called %d times; want 0", n)
	}
}

// A claim that cannot finish is cancelled, freeing its pool connection, and reported as a timeout.
func TestDispatchTenantEvent_ClaimIsBounded(t *testing.T) {
	previous := tenantEventClaimTimeout
	tenantEventClaimTimeout = 20 * time.Millisecond
	t.Cleanup(func() { tenantEventClaimTimeout = previous })

	repo, mock := newNotificationRepoWithMock(t)
	mock.ExpectQuery(`WITH due AS`).
		WillDelayFor(time.Second).
		WillReturnRows(sqlmock.NewRows([]string{"event_id", "tenant_id", "payload", "attempts", "delivery_code"}))

	start := time.Now()
	claimed, err := repo.dispatchOneTenantEvent(context.Background())
	if claimed || !errors.Is(err, errTenantEventClaimTimeout) {
		t.Fatalf("dispatchOneTenantEvent = %v, %v; want unclaimed with a claim timeout", claimed, err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("claim returned after %s; want about %s", elapsed, tenantEventClaimTimeout)
	}
}
