package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
)

var defaultAdminManagementSchemaPaths = []string{
	"migrations/add_admin_management_tables.sql",
	"migrations/add_tenant_channels.sql",
	"migrations/add_tenant_channel_credentials.sql",
	"migrations/add_tenant_z_campaign_binding.sql",
	"migrations/add_tenant_zz_acquisition_flow.sql",
	"migrations/remove_legacy_campaign_slug_index.sql",
	"migrations/create_postback_tables.sql",
	"migrations/add_tenant_postback_routing.sql",
	"migrations/add_tenant_admin_memberships.sql",
}

var requiredAdminManagementRelations = []string{
	"public.tenants",
	"public.admin_activity_logs",
	"public.userbase_import_jobs",
	"public.userbase_import_errors",
	"public.tenant_channels",
	"public.tenant_channel_credentials",
	"public.tenant_admin_memberships",
}

// EnsureSchema ensures admin-management tables/indexes exist using the SQL migration file,
// then verifies required relations are present.
func (r *AdminManagementRepository) EnsureSchema(ctx context.Context, migrationPath string) error {
	paths := defaultAdminManagementSchemaPaths
	if path := strings.TrimSpace(migrationPath); path != "" {
		paths = []string{path}
	}

	statements := make([]string, 0, len(paths))
	for _, path := range paths {
		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read admin management schema migration %q: %w", path, err)
		}
		sqlText := strings.TrimSpace(string(sqlBytes))
		if sqlText == "" {
			return fmt.Errorf("admin management schema migration %q is empty", path)
		}
		statements = append(statements, sqlText)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin admin schema bootstrap tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	for i, sqlText := range statements {
		if _, err := tx.ExecContext(ctx, sqlText); err != nil {
			return fmt.Errorf("failed to execute admin schema migration %q: %w", paths[i], err)
		}
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
