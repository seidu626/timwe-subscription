package repository

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"go.uber.org/zap"
)

func TestCreateTenantWithActivityLogCommitsTenantAndAudit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewAdminManagementRepository(db, zap.NewNop())
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	input := &domain.TenantCreateInput{
		TenantKey:      "tenant-a",
		Name:           "Tenant A",
		Status:         domain.TenantStatusActive,
		DefaultCountry: "GH",
		Metadata:       []byte(`{"tier":"gold"}`),
	}
	entry := &domain.AdminActivityLog{
		ID:        "11111111-1111-1111-1111-111111111111",
		Action:    "create",
		AfterJSON: []byte(`{"tenant_key":"tenant-a"}`),
		CreatedAt: now,
	}

	mock.ExpectBegin()
	expectTenantInsert(mock, now).WillReturnRows(tenantRows(now))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO admin_activity_logs")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	tenant, err := repo.CreateTenantWithActivityLog(input, entry)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if tenant.ID != "22222222-2222-2222-2222-222222222222" || tenant.TenantKey != "tenant-a" {
		t.Fatalf("unexpected tenant: %#v", tenant)
	}
	if entry.EntityType != "tenant" || entry.EntityID != tenant.ID {
		t.Fatalf("activity log not bound to tenant: %#v", entry)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCreateTenantWithActivityLogRollsBackOnAuditFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewAdminManagementRepository(db, zap.NewNop())
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	expectTenantInsert(mock, now).WillReturnRows(tenantRows(now))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO admin_activity_logs")).
		WillReturnError(errors.New("audit unavailable"))
	mock.ExpectRollback()

	_, err = repo.CreateTenantWithActivityLog(&domain.TenantCreateInput{
		TenantKey:      "tenant-a",
		Name:           "Tenant A",
		Status:         domain.TenantStatusActive,
		DefaultCountry: "GH",
		Metadata:       []byte(`{}`),
	}, &domain.AdminActivityLog{ID: "11111111-1111-1111-1111-111111111111", Action: "create"})
	if err == nil || !regexp.MustCompile(`failed to create activity log`).MatchString(err.Error()) {
		t.Fatalf("expected audit failure, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCreateTenantWithActivityLogMapsDuplicateKeyConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewAdminManagementRepository(db, zap.NewNop())
	mock.ExpectBegin()
	expectTenantInsert(mock, time.Now()).WillReturnError(&pq.Error{Code: "23505"})
	mock.ExpectRollback()

	_, err = repo.CreateTenantWithActivityLog(&domain.TenantCreateInput{
		TenantKey:      "tenant-a",
		Name:           "Tenant A",
		Status:         domain.TenantStatusActive,
		DefaultCountry: "GH",
		Metadata:       []byte(`{}`),
	}, nil)
	if !errors.Is(err, ErrAdminConflict) {
		t.Fatalf("expected ErrAdminConflict, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestListProductsRequiresTenantScope(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewAdminManagementRepository(db, zap.NewNop())
	_, _, err = repo.ListProducts(&domain.ProductListFilter{})
	if err == nil || !regexp.MustCompile(`tenant_id is required`).MatchString(err.Error()) {
		t.Fatalf("expected tenant_id error, got %v", err)
	}
}

func TestUpsertUserbaseUsesTenantScopedKey(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewAdminManagementRepository(db, zap.NewNop())
	tenantID := "22222222-2222-2222-2222-222222222222"
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO userbase (tenant_id, msisdn, type)")).
		WithArgs(tenantID, "233201234567", "ALLOWLISTED").
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "msisdn", "type"}).
			AddRow(7, tenantID, "233201234567", "ALLOWLISTED"))

	record, err := repo.UpsertUserbase(tenantID, "233201234567", "ALLOWLISTED")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if record.TenantID != tenantID || record.MSISDN != "233201234567" {
		t.Fatalf("unexpected record: %#v", record)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func expectTenantInsert(mock sqlmock.Sqlmock, _ time.Time) *sqlmock.ExpectedQuery {
	return mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO tenants")).
		WithArgs(
			sqlmock.AnyArg(),
			"tenant-a",
			"Tenant A",
			domain.TenantStatusActive,
			"GH",
			sqlmock.AnyArg(),
		)
}

func tenantRows(now time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "tenant_key", "name", "status", "default_country", "metadata_json", "created_at", "updated_at"}).
		AddRow("22222222-2222-2222-2222-222222222222", "tenant-a", "Tenant A", domain.TenantStatusActive, "GH", []byte(`{"tier":"gold"}`), now, now)
}
