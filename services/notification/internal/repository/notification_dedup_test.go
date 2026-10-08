package repository

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/seidu626/subscription-manager/notification/internal/domain"
)

// TIMWE re-posts a callback until it gets a 2xx. A replay that hits the
// (tenant_id, type, transaction_uuid) unique index must be a no-op success,
// not a 500 that keeps the retry loop going or a second opt-in SMS.
func TestProcessNotification_ReplayedCallbackIsNoOp(t *testing.T) {
	tenantID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	optin := &domain.NotificationRequest{
		TenantID: &tenantID, ProductID: 32535, MSISDN: "233241234567",
		ExternalTxID: "tx-1", TransactionUUID: "10d62b17-9b86-11f1-b26e-0050568d15cc", Type: domain.UserOptinEvent,
	}

	t.Run("duplicate insert returns success without the opt-in SMS", func(t *testing.T) {
		repo, mock := newNotificationRepoWithMock(t)
		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO notifications .* ON CONFLICT DO NOTHING`).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectRollback()

		if err := repo.ProcessNotification(context.Background(), optin); err != nil {
			t.Fatalf("ProcessNotification: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("first delivery still enqueues the opt-in SMS", func(t *testing.T) {
		repo, mock := newNotificationRepoWithMock(t)
		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO notifications .* ON CONFLICT DO NOTHING`).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery(`FROM tenant_product_sms_templates`).WillReturnRows(sqlmock.NewRows([]string{"id", "template"}))
		mock.ExpectCommit()

		if err := repo.ProcessNotification(context.Background(), optin); err != nil {
			t.Fatalf("ProcessNotification: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}
