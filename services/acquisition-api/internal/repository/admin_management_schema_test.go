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

func writeTempMigration(t *testing.T, sql string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "migration.sql")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(sql)), 0o600); err != nil {
		t.Fatalf("failed to write temp migration: %v", err)
	}
	return path
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
