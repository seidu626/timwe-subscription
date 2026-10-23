package repository

import (
	"context"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/seidu626/subscription-manager/notification/internal/domain"
)

const (
	insertNotificationSQL = `INSERT INTO notifications .* ON CONFLICT DO NOTHING`
	activateSQL           = `UPDATE subscriptions SET status = 'active', renewal_status = 'active', end_date = NULL, last_successful_payment = NOW\(\), last_charge_notification_at = NOW\(\), total_charge_notifications = COALESCE\(total_charge_notifications, 0\) \+ 1, consecutive_payment_failures = 0 WHERE tenant_id = \$1 AND user_identifier = \$2 AND product_id = \$3$`
	deactivateSQL         = `UPDATE subscriptions SET status = 'inactive', end_date = NOW\(\) WHERE tenant_id = \$1 AND user_identifier = \$2 AND product_id = \$3 AND status IS DISTINCT FROM 'inactive'$`
)

func projectionCallback(eventType, mnoDeliveryCode string) *domain.NotificationRequest {
	tenantID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	return &domain.NotificationRequest{
		TenantID: &tenantID, ProductID: 32535, MSISDN: "233241234567",
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
		{"no matching subscription is not an error", projectionCallback(domain.ChargeEvent, "DELIVERED"), activateSQL, 0},
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
			mock.ExpectExec(insertNotificationSQL).WillReturnResult(sqlmock.NewResult(1, 1))
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

func TestProcessNotification_ProjectionSkipped(t *testing.T) {
	t.Run("replayed callback projects nothing", func(t *testing.T) {
		repo, mock := newNotificationRepoWithMock(t)
		mock.ExpectBegin()
		mock.ExpectExec(insertNotificationSQL).WillReturnResult(sqlmock.NewResult(0, 0))
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
				mock.ExpectExec(insertNotificationSQL).WillReturnResult(sqlmock.NewResult(1, 1))
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
			mock.ExpectExec(insertNotificationSQL).WillReturnResult(sqlmock.NewResult(1, 1))
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
