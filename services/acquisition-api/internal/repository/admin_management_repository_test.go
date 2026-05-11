package repository

import (
	"database/sql"
	"encoding/json"
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

func TestListTenantsReturnsCatalogRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewAdminManagementRepository(db, zap.NewNop())
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM tenants WHERE")).
		WithArgs(domain.TenantStatusActive, "%nrg%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, tenant_key, name, status, default_country, metadata_json, created_at, updated_at")).
		WithArgs(domain.TenantStatusActive, "%nrg%", 25, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_key", "name", "status", "default_country", "metadata_json", "created_at", "updated_at"}).
			AddRow("22222222-2222-2222-2222-222222222222", "nrg", "NRG", domain.TenantStatusActive, "GH", []byte(`{"kind":"canonical-default"}`), now, now))

	tenants, total, err := repo.ListTenants(&domain.TenantListFilter{
		Limit:  25,
		Status: domain.TenantStatusActive,
		Query:  "nrg",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 1 || len(tenants) != 1 || tenants[0].TenantKey != "nrg" {
		t.Fatalf("unexpected tenant list: total=%d tenants=%#v", total, tenants)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestUpdateTenantWithActivityLogMutatesCatalogAndAudits(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewAdminManagementRepository(db, zap.NewNop())
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	tenantID := "22222222-2222-2222-2222-222222222222"
	name := "NRG Prime"
	status := domain.TenantStatusActive
	country := "GH"
	metadata := json.RawMessage(`{"tier":"gold"}`)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("UPDATE tenants")).
		WithArgs(tenantID, name, status, country, `{"tier":"gold"}`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_key", "name", "status", "default_country", "metadata_json", "created_at", "updated_at"}).
			AddRow(tenantID, "nrg", "NRG Prime", domain.TenantStatusActive, "GH", []byte(`{"tier":"gold"}`), now, now))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO admin_activity_logs")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	updated, err := repo.UpdateTenantWithActivityLog(tenantID, &domain.TenantUpdateInput{
		Name:           &name,
		Status:         &status,
		DefaultCountry: &country,
		Metadata:       &metadata,
	}, &domain.AdminActivityLog{ID: "11111111-1111-1111-1111-111111111111", Action: "update"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.Name != "NRG Prime" || updated.TenantKey != "nrg" {
		t.Fatalf("unexpected tenant: %#v", updated)
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

func TestListChannelsRequiresTenantScope(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewAdminManagementRepository(db, zap.NewNop())
	_, _, err = repo.ListChannels(&domain.ChannelListFilter{})
	if err == nil || !regexp.MustCompile(`tenant_id is required`).MatchString(err.Error()) {
		t.Fatalf("expected tenant_id error, got %v", err)
	}
}

func TestCreateChannelWithActivityLogMapsDuplicateKeyConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewAdminManagementRepository(db, zap.NewNop())
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO tenant_channels")).
		WillReturnError(&pq.Error{Code: "23505"})
	mock.ExpectRollback()

	_, err = repo.CreateChannelWithActivityLog(&domain.AdminChannel{
		ID:           "33333333-3333-3333-3333-333333333333",
		TenantID:     "22222222-2222-2222-2222-222222222222",
		ChannelKey:   "timwe-gh-airteltigo",
		Provider:     "timwe",
		Country:      "GH",
		Capabilities: []string{"confirm", "mt", "optin"},
		Status:       domain.ChannelStatusActive,
	}, nil)
	if !errors.Is(err, ErrAdminConflict) {
		t.Fatalf("expected ErrAdminConflict, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSetChannelStatusWithActivityLogUsesTenantScopedUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewAdminManagementRepository(db, zap.NewNop())
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("UPDATE tenant_channels")).
		WithArgs("22222222-2222-2222-2222-222222222222", "33333333-3333-3333-3333-333333333333", domain.ChannelStatusInactive).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	_, err = repo.SetChannelStatusWithActivityLog(
		"22222222-2222-2222-2222-222222222222",
		"33333333-3333-3333-3333-333333333333",
		domain.ChannelStatusInactive,
		nil,
	)
	if !errors.Is(err, ErrAdminNotFound) {
		t.Fatalf("expected ErrAdminNotFound, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRotateChannelCredentialWithActivityLogInactivatesOldAndInsertsNextVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewAdminManagementRepository(db, zap.NewNop())
	tenantID := "22222222-2222-2222-2222-222222222222"
	channelID := "33333333-3333-3333-3333-333333333333"
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status")).
		WithArgs(tenantID, channelID).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(domain.ChannelStatusActive))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, tenant_id, channel_id, purpose, version, status, secret_ref, secret_ref_display,")).
		WithArgs(tenantID, channelID, "provider_api", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(MAX(version), 0) + 1")).
		WithArgs(tenantID, channelID, "provider_api").
		WillReturnRows(sqlmock.NewRows([]string{"next_version"}).AddRow(2))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE tenant_channel_credentials")).
		WithArgs(tenantID, channelID, "provider_api", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO tenant_channel_credentials")).
		WithArgs(sqlmock.AnyArg(), tenantID, channelID, "provider_api", 2, "vault://tenant/channel/provider", "vault://[REDACTED]", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "channel_id", "purpose", "version", "status", "secret_ref", "secret_ref_display", "secret_fingerprint", "created_by", "created_at", "updated_at", "activated_at", "deactivated_at"}).
			AddRow("44444444-4444-4444-4444-444444444444", tenantID, channelID, "provider_api", 2, domain.ChannelCredentialStatusActive, "vault://tenant/channel/provider", "vault://[REDACTED]", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", nil, now, now, now, nil))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO admin_activity_logs")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	created, err := repo.RotateChannelCredentialWithActivityLog(&domain.AdminChannelCredential{
		TenantID:          tenantID,
		ChannelID:         channelID,
		Purpose:           "provider_api",
		SecretRef:         "vault://tenant/channel/provider",
		SecretRefDisplay:  "vault://[REDACTED]",
		SecretFingerprint: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}, &domain.AdminActivityLog{ID: "55555555-5555-5555-5555-555555555555", Action: "bind"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if created.Version != 2 || created.SecretRefDisplay != "vault://[REDACTED]" {
		t.Fatalf("unexpected credential: %#v", created)
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
