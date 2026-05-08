package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
)

const defaultAdminManagementSchemaPath = "migrations/add_admin_management_tables.sql"

var requiredAdminManagementRelations = []string{
	"public.tenants",
	"public.admin_activity_logs",
	"public.userbase_import_jobs",
	"public.userbase_import_errors",
}

// EnsureSchema ensures admin-management tables/indexes exist using the SQL migration file,
// then verifies required relations are present.
func (r *AdminManagementRepository) EnsureSchema(ctx context.Context, migrationPath string) error {
	path := strings.TrimSpace(migrationPath)
	if path == "" {
		path = defaultAdminManagementSchemaPath
	}

	sqlBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read admin management schema migration %q: %w", path, err)
	}
	sqlText := strings.TrimSpace(string(sqlBytes))
	if sqlText == "" {
		return fmt.Errorf("admin management schema migration %q is empty", path)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin admin schema bootstrap tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, sqlText); err != nil {
		return fmt.Errorf("failed to execute admin schema migration %q: %w", path, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit admin schema bootstrap tx: %w", err)
	}

	missing, err := r.findMissingRelations(ctx, requiredAdminManagementRelations)
	if err != nil {
		return fmt.Errorf("failed to verify admin management schema: %w", err)
	}
	if len(missing) > 0 {
		return fmt.Errorf("admin management schema bootstrap incomplete; missing relations: %s", strings.Join(missing, ", "))
	}

	return nil
}

func (r *AdminManagementRepository) findMissingRelations(ctx context.Context, relations []string) ([]string, error) {
	missing := make([]string, 0)

	for _, rel := range relations {
		var regclass sql.NullString
		if err := r.db.QueryRowContext(ctx, `SELECT to_regclass($1)`, rel).Scan(&regclass); err != nil {
			return nil, fmt.Errorf("failed to check relation %q: %w", rel, err)
		}

		if !regclass.Valid || strings.TrimSpace(regclass.String) == "" {
			missing = append(missing, rel)
		}
	}

	return missing, nil
}
