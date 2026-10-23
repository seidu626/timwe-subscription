package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/seidu626/subscription-manager/notification/internal/domain"
)

func newNotificationRepoWithMock(t *testing.T) (*NotificationRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &NotificationRepository{db: db, ctx: context.Background()}, mock
}

// expectInsertNotification mirrors ProcessNotification's INSERT ... RETURNING;
// stored=false is a replay that hit the dedup index and returns no row.
func expectInsertNotification(mock sqlmock.Sqlmock, stored bool) {
	rows := sqlmock.NewRows([]string{"id", "created_at"})
	if stored {
		rows.AddRow(int64(1), time.Now())
	}
	mock.ExpectQuery(`INSERT INTO notifications .* ON CONFLICT DO NOTHING RETURNING id, created_at`).WillReturnRows(rows)
}

// expectNoTenantEventBinding: the tenant has no tenant_events credential, so no event is queued.
func expectNoTenantEventBinding(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`FROM tenant_channel_credentials c`).WillReturnRows(sqlmock.NewRows([]string{"channel_id", "secret_ref"}))
}

func TestTenantIDByProductID(t *testing.T) {
	const query = `SELECT DISTINCT p\.tenant_id::text\s+FROM products p\s+JOIN tenants t ON t\.id = p\.tenant_id AND t\.status = 'ACTIVE'\s+WHERE p\.product_id = \$1`

	t.Run("single active owner", func(t *testing.T) {
		repo, mock := newNotificationRepoWithMock(t)
		mock.ExpectQuery(query).WithArgs(32535).
			WillReturnRows(sqlmock.NewRows([]string{"tenant_id"}).AddRow("tenant-careerify"))

		got, err := repo.TenantIDByProductID(context.Background(), 32535)
		if err != nil || got != "tenant-careerify" {
			t.Fatalf("expected tenant-careerify, got %q err=%v", got, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("shared product fails closed", func(t *testing.T) {
		repo, mock := newNotificationRepoWithMock(t)
		mock.ExpectQuery(query).WithArgs(8509).
			WillReturnRows(sqlmock.NewRows([]string{"tenant_id"}).AddRow("tenant-a").AddRow("tenant-b"))

		if _, err := repo.TenantIDByProductID(context.Background(), 8509); !errors.Is(err, ErrProductOwnerNotUnique) {
			t.Fatalf("expected ErrProductOwnerNotUnique, got %v", err)
		}
	})

	t.Run("unknown product fails closed", func(t *testing.T) {
		repo, mock := newNotificationRepoWithMock(t)
		mock.ExpectQuery(query).WithArgs(99999).WillReturnRows(sqlmock.NewRows([]string{"tenant_id"}))

		if _, err := repo.TenantIDByProductID(context.Background(), 99999); !errors.Is(err, ErrProductOwnerNotUnique) {
			t.Fatalf("expected ErrProductOwnerNotUnique, got %v", err)
		}
	})

	t.Run("non-positive product skips the query", func(t *testing.T) {
		repo, mock := newNotificationRepoWithMock(t)
		if _, err := repo.TenantIDByProductID(context.Background(), 0); !errors.Is(err, ErrProductOwnerNotUnique) {
			t.Fatalf("expected ErrProductOwnerNotUnique, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("query error propagates", func(t *testing.T) {
		repo, mock := newNotificationRepoWithMock(t)
		mock.ExpectQuery(query).WithArgs(14392).WillReturnError(errors.New("db down"))

		if _, err := repo.TenantIDByProductID(context.Background(), 14392); err == nil || errors.Is(err, ErrProductOwnerNotUnique) {
			t.Fatalf("expected raw db error, got %v", err)
		}
	})
}

func TestTenantOwnsChannel(t *testing.T) {
	const query = `SELECT EXISTS \(\s+SELECT 1 FROM tenant_channels\s+WHERE id = \$1 AND tenant_id = \$2 AND status = 'ACTIVE'`

	for _, owned := range []bool{true, false} {
		repo, mock := newNotificationRepoWithMock(t)
		mock.ExpectQuery(query).WithArgs("channel-1", "tenant-1").
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(owned))

		got, err := repo.TenantOwnsChannel(context.Background(), " tenant-1 ", "channel-1")
		if err != nil || got != owned {
			t.Fatalf("expected owned=%v, got %v err=%v", owned, got, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("blank ids skip the query", func(t *testing.T) {
		repo, mock := newNotificationRepoWithMock(t)
		if owned, err := repo.TenantOwnsChannel(context.Background(), "tenant-1", " "); owned || err == nil {
			t.Fatalf("expected not owned with error, got %v err=%v", owned, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestProcessNotification_SkipOptinSMSNeverReadsTemplates(t *testing.T) {
	tenantID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	optin := func(skip bool) *domain.NotificationRequest {
		return &domain.NotificationRequest{
			TenantID: &tenantID, ProductID: 32535, MSISDN: "233241234567",
			ExternalTxID: "tx-forged-1", Type: domain.UserOptinEvent, SkipOptinSMS: skip,
		}
	}

	t.Run("inferred tenant: insert only, no template lookup or outbox write", func(t *testing.T) {
		repo, mock := newNotificationRepoWithMock(t)
		mock.ExpectBegin()
		expectInsertNotification(mock, true)
		expectNoTenantEventBinding(mock)
		mock.ExpectCommit()

		if err := repo.ProcessNotification(context.Background(), optin(true)); err != nil {
			t.Fatalf("ProcessNotification: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("explicit tenant: template lookup still runs", func(t *testing.T) {
		repo, mock := newNotificationRepoWithMock(t)
		mock.ExpectBegin()
		expectInsertNotification(mock, true)
		expectNoTenantEventBinding(mock)
		mock.ExpectQuery(`FROM tenant_product_sms_templates`).WillReturnRows(sqlmock.NewRows([]string{"id", "template"}))
		mock.ExpectCommit()

		if err := repo.ProcessNotification(context.Background(), optin(false)); err != nil {
			t.Fatalf("ProcessNotification: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}
