package repository

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/zap"
)

func TestEnsureSchema_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewAdminManagementRepository(db, zap.NewNop())

	migrationSQL := `
CREATE TABLE IF NOT EXISTS admin_activity_logs (id UUID PRIMARY KEY);
CREATE TABLE IF NOT EXISTS userbase_import_jobs (id UUID PRIMARY KEY);
CREATE TABLE IF NOT EXISTS userbase_import_errors (id BIGSERIAL PRIMARY KEY);
CREATE TABLE IF NOT EXISTS tenants (id UUID PRIMARY KEY);
CREATE TABLE IF NOT EXISTS tenant_channels (id UUID PRIMARY KEY);
CREATE TABLE IF NOT EXISTS tenant_channel_credentials (id UUID PRIMARY KEY);
`
	file := writeTempMigration(t, migrationSQL)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(strings.TrimSpace(migrationSQL))).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	expectRelationExists(mock, "public.tenants")
	expectRelationExists(mock, "public.admin_activity_logs")
	expectRelationExists(mock, "public.userbase_import_jobs")
	expectRelationExists(mock, "public.userbase_import_errors")
	expectRelationExists(mock, "public.tenant_channels")
	expectRelationExists(mock, "public.tenant_channel_credentials")

	if err := repo.EnsureSchema(context.Background(), file); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestEnsureSchema_MissingRelationFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewAdminManagementRepository(db, zap.NewNop())

	migrationSQL := `
CREATE TABLE IF NOT EXISTS admin_activity_logs (id UUID PRIMARY KEY);
CREATE TABLE IF NOT EXISTS userbase_import_jobs (id UUID PRIMARY KEY);
CREATE TABLE IF NOT EXISTS userbase_import_errors (id BIGSERIAL PRIMARY KEY);
CREATE TABLE IF NOT EXISTS tenants (id UUID PRIMARY KEY);
CREATE TABLE IF NOT EXISTS tenant_channels (id UUID PRIMARY KEY);
CREATE TABLE IF NOT EXISTS tenant_channel_credentials (id UUID PRIMARY KEY);
`
	file := writeTempMigration(t, migrationSQL)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(strings.TrimSpace(migrationSQL))).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	expectRelationExists(mock, "public.tenants")
	expectRelationExists(mock, "public.admin_activity_logs")
	expectRelationMissing(mock, "public.userbase_import_jobs")
	expectRelationExists(mock, "public.userbase_import_errors")
	expectRelationExists(mock, "public.tenant_channels")
	expectRelationExists(mock, "public.tenant_channel_credentials")

	err = repo.EnsureSchema(context.Background(), file)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "missing relations") {
		t.Fatalf("expected missing relations error, got %v", err)
	}
	if !strings.Contains(err.Error(), "public.userbase_import_jobs") {
		t.Fatalf("expected missing relation name, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestEnsureSchema_FileReadFails(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewAdminManagementRepository(db, zap.NewNop())
	err = repo.EnsureSchema(context.Background(), filepath.Join(t.TempDir(), "missing.sql"))
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "failed to read admin management schema migration") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdminManagementMigrationAddsTenantScopedAdminTables(t *testing.T) {
	migrationPath := filepath.Join("..", "..", "migrations", "add_admin_management_tables.sql")
	body, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("failed to read migration: %v", err)
	}
	sql := string(body)

	required := []string{
		"ALTER TABLE products",
		"ALTER TABLE userbase",
		"ALTER TABLE userbase_import_jobs",
		"ALTER TABLE userbase_import_errors",
		"ALTER TABLE admin_activity_logs",
		"ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_products_tenant_product_id",
		"ON products (tenant_id, product_id)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_userbase_tenant_msisdn",
		"ON userbase (tenant_id, msisdn)",
	}
	for _, want := range required {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration missing %q", want)
		}
	}
}

func TestTenantChannelsMigrationDefinesCapabilityCatalog(t *testing.T) {
	migrationPath := filepath.Join("..", "..", "migrations", "add_tenant_channels.sql")
	body, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("failed to read migration: %v", err)
	}
	sql := string(body)

	required := []string{
		"CREATE TABLE IF NOT EXISTS tenant_channels",
		"tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT",
		"capabilities TEXT[] NOT NULL",
		"CONSTRAINT chk_tenant_channels_capabilities_allowed",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_channels_tenant_key",
		"ON tenant_channels (tenant_id, channel_key)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_channels_tenant_provider_scope",
		"COALESCE(operator, '')",
	}
	for _, want := range required {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration missing %q", want)
		}
	}
}

func TestTenantChannelCredentialsMigrationStoresReferencesOnly(t *testing.T) {
	migrationPath := filepath.Join("..", "..", "migrations", "add_tenant_channel_credentials.sql")
	body, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("failed to read migration: %v", err)
	}
	sql := string(body)

	required := []string{
		"CREATE TABLE IF NOT EXISTS tenant_channel_credentials",
		"FOREIGN KEY (tenant_id, channel_id)",
		"REFERENCES tenant_channels (tenant_id, id)",
		"secret_ref TEXT NOT NULL",
		"secret_ref_display TEXT NOT NULL",
		"secret_fingerprint TEXT NOT NULL",
		"WHERE status = 'ACTIVE'",
		"idx_tenant_channel_credentials_fingerprint",
	}
	for _, want := range required {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration missing %q", want)
		}
	}
	forbidden := []string{"secret_value", "password", "api_key", "token_value"}
	for _, bad := range forbidden {
		if strings.Contains(strings.ToLower(sql), bad) {
			t.Fatalf("migration contains plaintext-like column %q", bad)
		}
	}
}

func TestTenantAcquisitionFlowMigrationDropsLegacyCampaignSlugForeignKeys(t *testing.T) {
	migrationPath := filepath.Join("..", "..", "migrations", "add_tenant_zz_acquisition_flow.sql")
	body, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("failed to read migration: %v", err)
	}
	sql := string(body)

	required := []string{
		"constraint_record.contype = 'f'",
		"referenced_table.relname = 'campaigns'",
		"referenced_column.attname = 'slug'",
		"referenced_column.attnum = ANY (constraint_record.confkey)",
		"ALTER TABLE %s DROP CONSTRAINT IF EXISTS %I",
		"DROP CONSTRAINT IF EXISTS campaigns_slug_key",
		"idx_acq_trans_tenant_campaign_msisdn",
	}
	for _, want := range required {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration missing %q", want)
		}
	}

	if strings.Contains(strings.ToUpper(sql), "CASCADE") {
		t.Fatalf("migration must not use CASCADE when dropping legacy slug constraints")
	}
}

func TestDefaultAdminManagementSchemaPathsIncludePostbackTenantRouting(t *testing.T) {
	createPath := "migrations/create_postback_tables.sql"
	routingPath := "migrations/add_tenant_postback_routing.sql"

	createIndex := indexOfString(defaultAdminManagementSchemaPaths, createPath)
	if createIndex < 0 {
		t.Fatalf("default schema bootstrap missing %q", createPath)
	}
	routingIndex := indexOfString(defaultAdminManagementSchemaPaths, routingPath)
	if routingIndex < 0 {
		t.Fatalf("default schema bootstrap missing %q", routingPath)
	}
	if createIndex > routingIndex {
		t.Fatalf("postback table migration must run before tenant routing migration: %v", defaultAdminManagementSchemaPaths)
	}
}

func TestTenantPostbackRoutingMigrationAddsColumnsUsedByRepository(t *testing.T) {
	migrationPath := filepath.Join("..", "..", "migrations", "add_tenant_postback_routing.sql")
	body, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("failed to read migration: %v", err)
	}
	sql := string(body)

	required := []string{
		"ALTER TABLE postback_outbox",
		"ADD COLUMN IF NOT EXISTS tenant_id UUID",
		"ADD COLUMN IF NOT EXISTS channel_id UUID",
		"ADD COLUMN IF NOT EXISTS failure_reason TEXT",
		"idx_postback_outbox_tenant_status_retry",
		"idx_postback_outbox_tenant_transaction",
		"idx_postback_outbox_tenant_channel",
	}
	for _, want := range required {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration missing %q", want)
		}
	}
}

func writeTempMigration(t *testing.T, sql string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "migration.sql")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(sql)), 0o600); err != nil {
		t.Fatalf("failed to write temp migration: %v", err)
	}
	return path
}

func indexOfString(values []string, target string) int {
	for i, value := range values {
		if value == target {
			return i
		}
	}
	return -1
}

func expectRelationExists(mock sqlmock.Sqlmock, relation string) {
	mock.ExpectQuery(`SELECT to_regclass\(\$1\)`).
		WithArgs(relation).
		WillReturnRows(sqlmock.NewRows([]string{"to_regclass"}).AddRow(relation))
}

func expectRelationMissing(mock sqlmock.Sqlmock, relation string) {
	mock.ExpectQuery(`SELECT to_regclass\(\$1\)`).
		WithArgs(relation).
		WillReturnRows(sqlmock.NewRows([]string{"to_regclass"}).AddRow(nil))
}
