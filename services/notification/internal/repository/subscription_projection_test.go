package repository

import (
	"context"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/seidu626/subscription-manager/notification/internal/domain"
)

const (
	activateSQL   = `UPDATE subscriptions SET status = 'active', renewal_status = 'active', end_date = NULL, last_successful_payment = NOW\(\), last_charge_notification_at = NOW\(\), total_charge_notifications = COALESCE\(total_charge_notifications, 0\) \+ 1, consecutive_payment_failures = 0 WHERE tenant_id = \$1 AND user_identifier = \$2 AND product_id = \$3$`
	deactivateSQL = `UPDATE subscriptions SET status = 'inactive', end_date = NOW\(\) WHERE tenant_id = \$1 AND user_identifier = \$2 AND product_id = \$3 AND status IS DISTINCT FROM 'inactive'$`
	insertPaidSQL = `INSERT INTO subscriptions \(.*\) SELECT .*'active', 'active', NOW\(\), NOW\(\), NOW\(\), 1 WHERE EXISTS \( SELECT 1 FROM products p .*\) ON CONFLICT DO NOTHING$`
)

func projectionCallback(eventType, mnoDeliveryCode string) *domain.NotificationRequest {
	tenantID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	return &domain.NotificationRequest{
		TenantID: &tenantID, ProductID: 32535, MSISDN: "233241234567", PartnerRole: 2117,
		MCC: "620", MNC: "03", EntryChannel: "SMS", LargeAccount: "6555",
		ExternalTxID: "tx-1", TransactionUUID: "10d62b17-9b86-11f1-b26e-0050568d15cc",
		Type: eventType, MnoDeliveryCode: mnoDeliveryCode,
	}
}

// Only a DELIVERED CHARGE/USER_RENEWED proves payment, and USER_OPTOUT ends
// the subscription; everything else must leave subscriptions untouched.
func TestProcessNotification_ProjectsPaidEvidenceOntoSubscription(t *testing.T) {
	cases := []struct {
		name     string
		callback *domain.NotificationRequest
		update   string // "" means no subscriptions UPDATE may run
		rows     int64
	}{
		{"DELIVERED CHARGE activates", projectionCallback(domain.ChargeEvent, "DELIVERED"), activateSQL, 1},
		{"DELIVERED USER_RENEWED activates", projectionCallback(domain.UserRenewedEvent, "DELIVERED"), activateSQL, 1},
		{"padded lowercase delivered is accepted", projectionCallback(domain.ChargeEvent, " delivered "), activateSQL, 1},
		{"USER_OPTOUT deactivates", projectionCallback(domain.UserOptoutEvent, ""), deactivateSQL, 1},
		{"NO_BALANCE CHARGE is not payment", projectionCallback(domain.ChargeEvent, "NO_BALANCE"), "", 0},
		{"NO_BALANCE USER_RENEWED is not payment", projectionCallback(domain.UserRenewedEvent, "NO_BALANCE"), "", 0},
		{"CHARGE without a delivery code is not payment", projectionCallback(domain.ChargeEvent, ""), "", 0},
		{"MO is ignored", projectionCallback("MO", "DELIVERED"), "", 0},
		{"MT_DN is ignored", projectionCallback("MT_DN", "DELIVERED"), "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, mock := newNotificationRepoWithMock(t)
			mock.ExpectBegin()
			expectInsertNotification(mock, true)
			if tc.callback.Type != "MO" && tc.callback.Type != "MT_DN" {
				expectNoTenantEventBinding(mock)
			}
			if tc.update != "" {
				mock.ExpectExec(tc.update).
					WithArgs(*tc.callback.TenantID, tc.callback.MSISDN, tc.callback.ProductID).
					WillReturnResult(sqlmock.NewResult(0, tc.rows))
			}
			mock.ExpectCommit()

			if err := repo.ProcessNotification(context.Background(), tc.callback); err != nil {
				t.Fatalf("ProcessNotification: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TIMWE bills users who have no row here. A payment creates the row, because
// the charge is the only proof of subscription TIMWE gives us.
func TestProcessNotification_PaymentWithoutSubscriptionCreatesIt(t *testing.T) {
	channelID := "e5cb0f18-ae69-40d6-8ea9-796607a4ec6b"
	cases := []struct {
		name      string
		mutate    func(*domain.NotificationRequest)
		channel   any   // expected channel arg; nil lets the INSERT copy a sibling row's channel
		inserted  int64 // -1 means no INSERT may run
		insertErr error
		retry     bool // the UPDATE runs again after losing the INSERT race
	}{
		{name: "missing row is created", inserted: 1},
		{name: "callback channel is kept", mutate: func(n *domain.NotificationRequest) { n.ChannelID = &channelID }, channel: channelID, inserted: 1},
		{name: "row created concurrently, or product not owned, falls back to the UPDATE", inserted: 0, retry: true},
		{name: "no partner role creates nothing", mutate: func(n *domain.NotificationRequest) { n.PartnerRole = 0 }, inserted: -1},
		{name: "insert error rolls back", insertErr: errors.New("connection reset")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			callback := projectionCallback(domain.UserRenewedEvent, "DELIVERED")
			if tc.mutate != nil {
				tc.mutate(callback)
			}
			repo, mock := newNotificationRepoWithMock(t)
			mock.ExpectBegin()
			expectInsertNotification(mock, true)
			expectNoTenantEventBinding(mock)
			mock.ExpectExec(activateSQL).
				WithArgs(*callback.TenantID, callback.MSISDN, callback.ProductID).
				WillReturnResult(sqlmock.NewResult(0, 0))
			if tc.inserted >= 0 {
				insert := mock.ExpectExec(insertPaidSQL).
					WithArgs(*callback.TenantID, tc.channel, 2117, callback.MSISDN, callback.ProductID, "620", "03", "SMS", "6555")
				if tc.insertErr != nil {
					insert.WillReturnError(tc.insertErr)
				} else {
					insert.WillReturnResult(sqlmock.NewResult(0, tc.inserted))
				}
			}
			if tc.retry {
				mock.ExpectExec(activateSQL).
					WithArgs(*callback.TenantID, callback.MSISDN, callback.ProductID).
					WillReturnResult(sqlmock.NewResult(0, 1))
			}
			if tc.insertErr != nil {
				mock.ExpectRollback()
			} else {
				mock.ExpectCommit()
			}

			err := repo.ProcessNotification(context.Background(), callback)
			if (err != nil) != (tc.insertErr != nil) {
				t.Fatalf("ProcessNotification error = %v, want error %v", err, tc.insertErr != nil)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestProcessNotification_ProjectionSkipped(t *testing.T) {
	t.Run("replayed callback projects nothing", func(t *testing.T) {
		repo, mock := newNotificationRepoWithMock(t)
		mock.ExpectBegin()
		expectInsertNotification(mock, false)
		mock.ExpectRollback()

		if err := repo.ProcessNotification(context.Background(), projectionCallback(domain.ChargeEvent, "DELIVERED")); err != nil {
			t.Fatalf("ProcessNotification: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	blank := "  "
	incomplete := map[string]func(*domain.NotificationRequest){
		"nil tenant":     func(n *domain.NotificationRequest) { n.TenantID = nil },
		"blank tenant":   func(n *domain.NotificationRequest) { n.TenantID = &blank },
		"empty msisdn":   func(n *domain.NotificationRequest) { n.MSISDN = "" },
		"zero productId": func(n *domain.NotificationRequest) { n.ProductID = 0 },
	}
	for name, mutate := range incomplete {
		for _, eventType := range []string{domain.ChargeEvent, domain.UserOptoutEvent} {
			t.Run(name+" "+eventType, func(t *testing.T) {
				callback := projectionCallback(eventType, "DELIVERED")
				mutate(callback)
				repo, mock := newNotificationRepoWithMock(t)
				mock.ExpectBegin()
				expectInsertNotification(mock, true)
				if callback.TenantID != nil {
					expectNoTenantEventBinding(mock)
				}
				mock.ExpectCommit()

				if err := repo.ProcessNotification(context.Background(), callback); err != nil {
					t.Fatalf("ProcessNotification: %v", err)
				}
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// A failed projection must roll the stored callback back so TIMWE's retry
// re-delivers it, rather than committing a callback that was never applied.
func TestProcessNotification_ProjectionErrorRollsBack(t *testing.T) {
	cases := map[string]struct {
		callback *domain.NotificationRequest
		update   string
	}{
		"activate":   {projectionCallback(domain.ChargeEvent, "DELIVERED"), activateSQL},
		"deactivate": {projectionCallback(domain.UserOptoutEvent, ""), deactivateSQL},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo, mock := newNotificationRepoWithMock(t)
			mock.ExpectBegin()
			expectInsertNotification(mock, true)
			expectNoTenantEventBinding(mock)
			mock.ExpectExec(tc.update).WillReturnError(errors.New("connection reset"))
			mock.ExpectRollback()

			err := repo.ProcessNotification(context.Background(), tc.callback)
			if err == nil {
				t.Fatal("expected projection error")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
