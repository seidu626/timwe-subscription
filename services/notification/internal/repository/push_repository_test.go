package repository

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/zap"
)

func newPushRepoWithMock(t *testing.T) (*PushRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewPushRepository(db, zap.NewNop()), mock
}

func TestDeviceToken_ReturnsTokenWhenRegistered(t *testing.T) {
	repo, mock := newPushRepoWithMock(t)
	rows := sqlmock.NewRows([]string{"fcm_token"}).AddRow("device-token-1")
	mock.ExpectQuery("SELECT fcm_token").WithArgs("233241234567").WillReturnRows(rows)

	token, found, err := repo.DeviceToken(context.Background(), "233241234567", "")
	if err != nil {
		t.Fatalf("DeviceToken: %v", err)
	}
	if !found || token != "device-token-1" {
		t.Errorf("got token=%q found=%v, want device-token-1/true", token, found)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestDeviceToken_NotFoundWhenNoDevice(t *testing.T) {
	repo, mock := newPushRepoWithMock(t)
	mock.ExpectQuery("SELECT fcm_token").WithArgs("233241234567").
		WillReturnRows(sqlmock.NewRows([]string{"fcm_token"}))

	token, found, err := repo.DeviceToken(context.Background(), "233241234567", "")
	if err != nil {
		t.Fatalf("DeviceToken: %v", err)
	}
	if found || token != "" {
		t.Errorf("got token=%q found=%v, want empty/false", token, found)
	}
}

func TestDeviceToken_ScopesToTenantWhenTenantIDProvided(t *testing.T) {
	repo, mock := newPushRepoWithMock(t)
	tenantID := "11111111-1111-1111-1111-111111111111"
	rows := sqlmock.NewRows([]string{"fcm_token"}).AddRow("tenant-a-token")
	mock.ExpectQuery("SELECT ad.fcm_token").
		WithArgs("233241234567", tenantID).
		WillReturnRows(rows)

	token, found, err := repo.DeviceToken(context.Background(), "233241234567", tenantID)
	if err != nil {
		t.Fatalf("DeviceToken: %v", err)
	}
	if !found || token != "tenant-a-token" {
		t.Errorf("got token=%q found=%v, want tenant-a-token/true", token, found)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestDeviceToken_NotFoundWhenTenantScopedButNoMatchingDevice(t *testing.T) {
	repo, mock := newPushRepoWithMock(t)
	tenantID := "11111111-1111-1111-1111-111111111111"
	mock.ExpectQuery("SELECT ad.fcm_token").
		WithArgs("233241234567", tenantID).
		WillReturnRows(sqlmock.NewRows([]string{"fcm_token"}))

	// This is the cross-tenant misdelivery scenario: msisdn has a device,
	// but it is registered under a different tenant_key than tenantID
	// resolves to, so the tenant-scoped join returns no row.
	token, found, err := repo.DeviceToken(context.Background(), "233241234567", tenantID)
	if err != nil {
		t.Fatalf("DeviceToken: %v", err)
	}
	if found || token != "" {
		t.Errorf("got token=%q found=%v, want empty/false", token, found)
	}
}

func TestPreferredChannel_ReturnsStoredChannel(t *testing.T) {
	repo, mock := newPushRepoWithMock(t)
	rows := sqlmock.NewRows([]string{"channel"}).AddRow("PUSH")
	mock.ExpectQuery("SELECT anp.channel").
		WithArgs("233241234567", 42, 7).
		WillReturnRows(rows)

	channel, err := repo.PreferredChannel(context.Background(), "233241234567", 7, 42)
	if err != nil {
		t.Fatalf("PreferredChannel: %v", err)
	}
	if channel != "PUSH" {
		t.Errorf("channel = %q, want PUSH", channel)
	}
}

func TestPreferredChannel_EmptyWhenNoPreferenceRow(t *testing.T) {
	repo, mock := newPushRepoWithMock(t)
	mock.ExpectQuery("SELECT anp.channel").
		WithArgs("233241234567", 42, 7).
		WillReturnRows(sqlmock.NewRows([]string{"channel"}))

	channel, err := repo.PreferredChannel(context.Background(), "233241234567", 7, 42)
	if err != nil {
		t.Fatalf("PreferredChannel: %v", err)
	}
	if channel != "" {
		t.Errorf("channel = %q, want empty", channel)
	}
}
