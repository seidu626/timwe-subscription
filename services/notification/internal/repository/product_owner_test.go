package repository

import (
	"context"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
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
