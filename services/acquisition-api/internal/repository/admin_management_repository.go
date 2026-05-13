package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"go.uber.org/zap"
)

var (
	// ErrAdminNotFound is returned when an admin-managed resource does not exist.
	ErrAdminNotFound = errors.New("admin resource not found")
	// ErrAdminConflict is returned when an admin-managed resource violates uniqueness.
	ErrAdminConflict = errors.New("admin resource conflict")
	// ErrAdminInvalidState is returned when a valid resource is not in a state that allows a mutation.
	ErrAdminInvalidState = errors.New("admin resource invalid state")
)

// AdminManagementRepository handles products/userbase/admin activity data access.
type AdminManagementRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

func NewAdminManagementRepository(db *sql.DB, logger *zap.Logger) *AdminManagementRepository {
	return &AdminManagementRepository{db: db, logger: logger}
}

func (r *AdminManagementRepository) CreateTenantWithActivityLog(input *domain.TenantCreateInput, entry *domain.AdminActivityLog) (*domain.AdminTenant, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin tenant create tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	tenantID := uuid.NewString()
	query := `
		INSERT INTO tenants (id, tenant_key, name, status, default_country, metadata_json)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, tenant_key, name, status, default_country, metadata_json, created_at, updated_at
	`
	tenant, err := scanTenant(tx.QueryRow(
		query,
		tenantID,
		input.TenantKey,
		input.Name,
		input.Status,
		input.DefaultCountry,
		tenantMetadataJSON(input.Metadata),
	))
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrAdminConflict
		}
		return nil, fmt.Errorf("failed to create tenant: %w", err)
	}

	if entry != nil {
		entry.EntityType = "tenant"
		entry.EntityID = tenant.ID
		if err := createActivityLog(tx, entry); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit tenant create tx: %w", err)
	}
	committed = true
	return tenant, nil
}

func (r *AdminManagementRepository) GetTenantByID(id string) (*domain.AdminTenant, error) {
	query := `
		SELECT id, tenant_key, name, status, default_country, metadata_json, created_at, updated_at
		FROM tenants
		WHERE id = $1
	`
	tenant, err := scanTenant(r.db.QueryRow(query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAdminNotFound
		}
		return nil, fmt.Errorf("failed to get tenant by id: %w", err)
	}
	return tenant, nil
}

func (r *AdminManagementRepository) GetTenantByKey(key string) (*domain.AdminTenant, error) {
	query := `
		SELECT id, tenant_key, name, status, default_country, metadata_json, created_at, updated_at
		FROM tenants
		WHERE tenant_key = $1
	`
	tenant, err := scanTenant(r.db.QueryRow(query, key))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAdminNotFound
		}
		return nil, fmt.Errorf("failed to get tenant by key: %w", err)
	}
	return tenant, nil
}

func (r *AdminManagementRepository) ListTenants(filter *domain.TenantListFilter) ([]*domain.AdminTenant, int, error) {
	if filter == nil {
		filter = &domain.TenantListFilter{Limit: 20}
	}
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	where := []string{"1 = 1"}
	args := []any{}
	argN := 1
	if status := domain.TenantStatus(strings.ToUpper(strings.TrimSpace(string(filter.Status)))); status != "" {
		where = append(where, fmt.Sprintf("status = $%d", argN))
		args = append(args, status)
		argN++
	}
	if q := strings.TrimSpace(filter.Query); q != "" {
		where = append(where, fmt.Sprintf("(LOWER(tenant_key) LIKE LOWER($%d) OR LOWER(name) LIKE LOWER($%d))", argN, argN))
		args = append(args, "%"+q+"%")
		argN++
	}
	whereSQL := strings.Join(where, " AND ")

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM tenants WHERE %s`, whereSQL)
	var total int
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count tenants: %w", err)
	}

	args = append(args, filter.Limit, filter.Offset)
	query := fmt.Sprintf(`
		SELECT id, tenant_key, name, status, default_country, metadata_json, created_at, updated_at
		FROM tenants
		WHERE %s
		ORDER BY created_at DESC, tenant_key ASC
		LIMIT $%d OFFSET $%d
	`, whereSQL, argN, argN+1)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list tenants: %w", err)
	}
	defer rows.Close()

	tenants := make([]*domain.AdminTenant, 0)
	for rows.Next() {
		tenant, err := scanTenant(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan tenant: %w", err)
		}
		tenants = append(tenants, tenant)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to iterate tenants: %w", err)
	}
	return tenants, total, nil
}

func (r *AdminManagementRepository) UpdateTenantWithActivityLog(id string, input *domain.TenantUpdateInput, entry *domain.AdminActivityLog) (*domain.AdminTenant, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin tenant update tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	var metadata any
	if input.Metadata != nil {
		metadata = tenantMetadataJSON(*input.Metadata)
	}
	query := `
		UPDATE tenants
		SET
			name = COALESCE($2, name),
			status = COALESCE($3, status),
			default_country = COALESCE($4, default_country),
			metadata_json = COALESCE($5, metadata_json),
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, tenant_key, name, status, default_country, metadata_json, created_at, updated_at
	`
	tenant, err := scanTenant(tx.QueryRow(
		query,
		id,
		nullableStringPtr(input.Name),
		nullableTenantStatusPtr(input.Status),
		nullableStringPtr(input.DefaultCountry),
		metadata,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAdminNotFound
		}
		return nil, fmt.Errorf("failed to update tenant: %w", err)
	}

	if entry != nil {
		entry.EntityType = "tenant"
		entry.EntityID = tenant.ID
		entry.TenantID = tenant.ID
		if len(entry.AfterJSON) == 0 {
			if after, marshalErr := json.Marshal(tenant); marshalErr == nil {
				entry.AfterJSON = after
			}
		}
		if err := createActivityLog(tx, entry); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit tenant update tx: %w", err)
	}
	committed = true
	return tenant, nil
}

func (r *AdminManagementRepository) ListTenantMembers(filter *domain.TenantMemberListFilter) ([]*domain.AdminTenantMember, int, error) {
	if filter == nil {
		filter = &domain.TenantMemberListFilter{Limit: 20}
	}
	tenantID := strings.TrimSpace(filter.TenantID)
	if tenantID == "" {
		return nil, 0, fmt.Errorf("tenant_id is required")
	}
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	where := []string{"tenant_id = $1"}
	args := []any{tenantID}
	argN := 2
	if status := strings.TrimSpace(string(filter.Status)); status != "" {
		where = append(where, fmt.Sprintf("status = $%d", argN))
		args = append(args, status)
		argN++
	}
	if q := strings.TrimSpace(filter.Query); q != "" {
		where = append(where, fmt.Sprintf("(auth0_subject ILIKE $%d OR email ILIKE $%d)", argN, argN))
		args = append(args, "%"+q+"%")
		argN++
	}
	whereSQL := strings.Join(where, " AND ")

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM tenant_admin_memberships WHERE %s`, whereSQL)
	var total int
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count tenant members: %w", err)
	}

	args = append(args, filter.Limit, filter.Offset)
	query := fmt.Sprintf(`
		SELECT id, tenant_id, auth0_subject, email, role, status, created_by, created_at, updated_at
		FROM tenant_admin_memberships
		WHERE %s
		ORDER BY created_at DESC, auth0_subject ASC
		LIMIT $%d OFFSET $%d
	`, whereSQL, argN, argN+1)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list tenant members: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.AdminTenantMember, 0)
	for rows.Next() {
		member, err := scanTenantMember(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, member)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to iterate tenant members: %w", err)
	}
	return out, total, nil
}

func (r *AdminManagementRepository) UpsertTenantMemberWithActivityLog(input *domain.TenantMemberInput, entry *domain.AdminActivityLog) (*domain.AdminTenantMember, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin tenant member tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	query := `
		INSERT INTO tenant_admin_memberships (tenant_id, auth0_subject, email, role, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (tenant_id, auth0_subject)
		DO UPDATE SET
			email = EXCLUDED.email,
			role = EXCLUDED.role,
			status = EXCLUDED.status,
			updated_at = NOW()
		RETURNING id, tenant_id, auth0_subject, email, role, status, created_by, created_at, updated_at
	`
	member, err := scanTenantMember(tx.QueryRow(
		query,
		input.TenantID,
		input.Auth0Subject,
		nullableStringPtr(input.Email),
		input.Role,
		input.Status,
		input.CreatedBy,
	))
	if err != nil {
		return nil, fmt.Errorf("failed to upsert tenant member: %w", err)
	}

	if entry != nil {
		entry.TenantID = input.TenantID
		entry.EntityType = "tenant_member"
		entry.EntityID = member.Auth0Subject
		if err := createActivityLog(tx, entry); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit tenant member tx: %w", err)
	}
	committed = true
	return member, nil
}

func (r *AdminManagementRepository) DeactivateTenantMemberWithActivityLog(tenantID, auth0Subject string, entry *domain.AdminActivityLog) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin tenant member deactivate tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	res, err := tx.Exec(`
		UPDATE tenant_admin_memberships
		SET status = 'INACTIVE', updated_at = NOW()
		WHERE tenant_id = $1 AND auth0_subject = $2
	`, tenantID, auth0Subject)
	if err != nil {
		return fmt.Errorf("failed to deactivate tenant member: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrAdminNotFound
	}

	if entry != nil {
		entry.TenantID = tenantID
		entry.EntityType = "tenant_member"
		entry.EntityID = auth0Subject
		if err := createActivityLog(tx, entry); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit tenant member deactivate tx: %w", err)
	}
	committed = true
	return nil
}

func (r *AdminManagementRepository) ListActiveTenantsForMember(auth0Subject, email string) ([]*domain.AdminTenant, error) {
	auth0Subject = strings.TrimSpace(auth0Subject)
	email = strings.ToLower(strings.TrimSpace(email))
	if auth0Subject == "" && email == "" {
		return []*domain.AdminTenant{}, nil
	}

	where := []string{"m.status = 'ACTIVE'", "t.status = 'ACTIVE'"}
	args := []any{}
	principalFilters := []string{}
	if auth0Subject != "" {
		args = append(args, auth0Subject)
		principalFilters = append(principalFilters, fmt.Sprintf("m.auth0_subject = $%d", len(args)))
	}
	if email != "" {
		args = append(args, email)
		principalFilters = append(principalFilters, fmt.Sprintf("LOWER(m.email) = $%d", len(args)))
	}
	if len(principalFilters) > 0 {
		where = append(where, "("+strings.Join(principalFilters, " OR ")+")")
	}

	query := fmt.Sprintf(`
		SELECT DISTINCT t.id, t.tenant_key, t.name, t.status, t.default_country, t.metadata_json, t.created_at, t.updated_at
		FROM tenant_admin_memberships m
		JOIN tenants t ON t.id = m.tenant_id
		WHERE %s
		ORDER BY t.tenant_key ASC
	`, strings.Join(where, " AND "))

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list active member tenants: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.AdminTenant, 0)
	for rows.Next() {
		tenant, err := scanTenant(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan member tenant: %w", err)
		}
		out = append(out, tenant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate member tenants: %w", err)
	}
	return out, nil
}

func (r *AdminManagementRepository) ListProducts(filter *domain.ProductListFilter) ([]*domain.AdminProduct, int, error) {
	if filter == nil {
		filter = &domain.ProductListFilter{Limit: 20}
	}
	tenantID := strings.TrimSpace(filter.TenantID)
	if tenantID == "" {
		return nil, 0, fmt.Errorf("tenant_id is required")
	}
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	where := []string{"tenant_id = $1"}
	args := []any{tenantID}
	argN := 2

	if q := strings.TrimSpace(filter.Query); q != "" {
		where = append(where, fmt.Sprintf("(LOWER(product_id) LIKE LOWER($%d) OR LOWER(name) LIKE LOWER($%d))", argN, argN))
		args = append(args, "%"+q+"%")
		argN++
	}
	if s := strings.TrimSpace(filter.ShortCode); s != "" {
		where = append(where, fmt.Sprintf("LOWER(short_code) = LOWER($%d)", argN))
		args = append(args, s)
		argN++
	}

	whereSQL := strings.Join(where, " AND ")

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM products WHERE %s`, whereSQL)
	var total int
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count products: %w", err)
	}

	args = append(args, filter.Limit, filter.Offset)
	listQuery := fmt.Sprintf(`
		SELECT id, tenant_id, product_id, name, price_point_id, price_point_value, short_code, created_at
		FROM products
		WHERE %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, argN, argN+1)

	rows, err := r.db.Query(listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list products: %w", err)
	}
	defer rows.Close()

	products := make([]*domain.AdminProduct, 0)
	for rows.Next() {
		var p domain.AdminProduct
		if err := rows.Scan(&p.ID, &p.TenantID, &p.ProductID, &p.Name, &p.PricePointID, &p.PricePointValue, &p.ShortCode, &p.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("failed to scan product: %w", err)
		}
		products = append(products, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to iterate products: %w", err)
	}

	return products, total, nil
}

func (r *AdminManagementRepository) GetProductByID(tenantID string, id int) (*domain.AdminProduct, error) {
	query := `
		SELECT id, tenant_id, product_id, name, price_point_id, price_point_value, short_code, created_at
		FROM products
		WHERE tenant_id = $1 AND id = $2
	`
	var p domain.AdminProduct
	if err := r.db.QueryRow(query, tenantID, id).Scan(&p.ID, &p.TenantID, &p.ProductID, &p.Name, &p.PricePointID, &p.PricePointValue, &p.ShortCode, &p.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAdminNotFound
		}
		return nil, fmt.Errorf("failed to get product: %w", err)
	}
	return &p, nil
}

func (r *AdminManagementRepository) CreateProduct(product *domain.AdminProduct) (*domain.AdminProduct, error) {
	query := `
		INSERT INTO products (tenant_id, product_id, name, price_point_id, price_point_value, short_code)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`
	out := *product
	if err := r.db.QueryRow(
		query,
		product.TenantID,
		product.ProductID,
		product.Name,
		product.PricePointID,
		product.PricePointValue,
		product.ShortCode,
	).Scan(&out.ID, &out.CreatedAt); err != nil {
		if isUniqueViolation(err) {
			return nil, ErrAdminConflict
		}
		return nil, fmt.Errorf("failed to create product: %w", err)
	}
	return &out, nil
}

func (r *AdminManagementRepository) UpdateProduct(tenantID string, id int, product *domain.AdminProduct) (*domain.AdminProduct, error) {
	query := `
		UPDATE products
		SET product_id = $1, name = $2, price_point_id = $3, price_point_value = $4, short_code = $5
		WHERE tenant_id = $6 AND id = $7
		RETURNING id, tenant_id, product_id, name, price_point_id, price_point_value, short_code, created_at
	`
	var out domain.AdminProduct
	if err := r.db.QueryRow(
		query,
		product.ProductID,
		product.Name,
		product.PricePointID,
		product.PricePointValue,
		product.ShortCode,
		tenantID,
		id,
	).Scan(&out.ID, &out.TenantID, &out.ProductID, &out.Name, &out.PricePointID, &out.PricePointValue, &out.ShortCode, &out.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAdminNotFound
		}
		if isUniqueViolation(err) {
			return nil, ErrAdminConflict
		}
		return nil, fmt.Errorf("failed to update product: %w", err)
	}
	return &out, nil
}

func (r *AdminManagementRepository) DeleteProduct(tenantID string, id int) error {
	res, err := r.db.Exec(`DELETE FROM products WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return fmt.Errorf("failed to delete product: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to inspect delete result: %w", err)
	}
	if affected == 0 {
		return ErrAdminNotFound
	}
	return nil
}

func (r *AdminManagementRepository) BatchUpsertProducts(tenantID string, products []*domain.AdminProduct) (int, error) {
	if len(products) == 0 {
		return 0, nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin batch upsert tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	query := `
		WITH updated AS (
			UPDATE products
			SET name = $3, price_point_id = $4, price_point_value = $5, short_code = $6
			WHERE tenant_id = $1 AND product_id = $2
			RETURNING id
		)
		INSERT INTO products (tenant_id, product_id, name, price_point_id, price_point_value, short_code)
		SELECT $1, $2, $3, $4, $5, $6
		WHERE NOT EXISTS (SELECT 1 FROM updated)
	`
	for _, p := range products {
		if _, err = tx.Exec(query, tenantID, p.ProductID, p.Name, p.PricePointID, p.PricePointValue, p.ShortCode); err != nil {
			return 0, fmt.Errorf("failed to upsert product %s: %w", p.ProductID, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit batch upsert tx: %w", err)
	}
	return len(products), nil
}

func (r *AdminManagementRepository) CountProductDependencies(productID string) (*domain.ProductDependencyCounts, error) {
	counts := &domain.ProductDependencyCounts{}

	if err := r.db.QueryRow(`
		SELECT COUNT(*)
		FROM campaigns
		WHERE CAST(offer_product_id AS TEXT) = $1
	`, productID).Scan(&counts.CampaignCount); err != nil {
		return nil, fmt.Errorf("failed to count campaign dependencies: %w", err)
	}

	if err := r.db.QueryRow(`
		SELECT COUNT(*)
		FROM subscriptions
		WHERE CAST(product_id AS TEXT) = $1
	`, productID).Scan(&counts.SubscriptionCount); err != nil {
		return nil, fmt.Errorf("failed to count subscription dependencies: %w", err)
	}

	return counts, nil
}

func (r *AdminManagementRepository) ListChannels(filter *domain.ChannelListFilter) ([]*domain.AdminChannel, int, error) {
	if filter == nil {
		filter = &domain.ChannelListFilter{Limit: 20}
	}
	tenantID := strings.TrimSpace(filter.TenantID)
	if tenantID == "" {
		return nil, 0, fmt.Errorf("tenant_id is required")
	}
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	where := []string{"tenant_id = $1"}
	args := []any{tenantID}
	argN := 2
	if provider := strings.TrimSpace(filter.Provider); provider != "" {
		where = append(where, fmt.Sprintf("provider = $%d", argN))
		args = append(args, provider)
		argN++
	}
	if country := strings.TrimSpace(filter.Country); country != "" {
		where = append(where, fmt.Sprintf("country = $%d", argN))
		args = append(args, country)
		argN++
	}
	if filter.Enabled != nil {
		status := domain.ChannelStatusInactive
		if *filter.Enabled {
			status = domain.ChannelStatusActive
		}
		where = append(where, fmt.Sprintf("status = $%d", argN))
		args = append(args, status)
		argN++
	}

	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM tenant_channels WHERE %s`, whereSQL), args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count tenant channels: %w", err)
	}

	args = append(args, filter.Limit, filter.Offset)
	rows, err := r.db.Query(fmt.Sprintf(`
		SELECT id, tenant_id, channel_key, provider, country, operator, capabilities, status, created_at, updated_at
		FROM tenant_channels
		WHERE %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, argN, argN+1), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list tenant channels: %w", err)
	}
	defer rows.Close()

	channels := make([]*domain.AdminChannel, 0)
	for rows.Next() {
		channel, err := scanChannel(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan tenant channel: %w", err)
		}
		channels = append(channels, channel)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to iterate tenant channels: %w", err)
	}
	return channels, total, nil
}

func (r *AdminManagementRepository) GetChannelByID(tenantID, id string) (*domain.AdminChannel, error) {
	query := `
		SELECT id, tenant_id, channel_key, provider, country, operator, capabilities, status, created_at, updated_at
		FROM tenant_channels
		WHERE tenant_id = $1 AND id = $2
	`
	channel, err := scanChannel(r.db.QueryRow(query, tenantID, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAdminNotFound
		}
		return nil, fmt.Errorf("failed to get tenant channel: %w", err)
	}
	return channel, nil
}

func (r *AdminManagementRepository) CreateChannelWithActivityLog(channel *domain.AdminChannel, entry *domain.AdminActivityLog) (*domain.AdminChannel, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin channel create tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	channel.ID = uuid.NewString()
	query := `
		INSERT INTO tenant_channels (id, tenant_id, channel_key, provider, country, operator, capabilities, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, tenant_id, channel_key, provider, country, operator, capabilities, status, created_at, updated_at
	`
	created, err := scanChannel(tx.QueryRow(
		query,
		channel.ID,
		channel.TenantID,
		channel.ChannelKey,
		channel.Provider,
		channel.Country,
		nullableStringPtr(channel.Operator),
		pq.StringArray(channel.Capabilities),
		channel.Status,
	))
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrAdminConflict
		}
		return nil, fmt.Errorf("failed to create tenant channel: %w", err)
	}

	if entry != nil {
		entry.TenantID = created.TenantID
		entry.EntityType = "tenant_channel"
		entry.EntityID = created.ID
		if err := createActivityLog(tx, entry); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit channel create tx: %w", err)
	}
	committed = true
	return created, nil
}

func (r *AdminManagementRepository) SetChannelStatusWithActivityLog(tenantID, id string, status domain.ChannelStatus, entry *domain.AdminActivityLog) (*domain.AdminChannel, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin channel status tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	query := `
		UPDATE tenant_channels
		SET status = $3, updated_at = NOW()
		WHERE tenant_id = $1 AND id = $2
		RETURNING id, tenant_id, channel_key, provider, country, operator, capabilities, status, created_at, updated_at
	`
	channel, err := scanChannel(tx.QueryRow(query, tenantID, id, status))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAdminNotFound
		}
		return nil, fmt.Errorf("failed to set tenant channel status: %w", err)
	}

	if entry != nil {
		entry.TenantID = channel.TenantID
		entry.EntityType = "tenant_channel"
		entry.EntityID = channel.ID
		if err := createActivityLog(tx, entry); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit channel status tx: %w", err)
	}
	committed = true
	return channel, nil
}

func (r *AdminManagementRepository) ListChannelCredentials(filter *domain.ChannelCredentialListFilter) ([]*domain.AdminChannelCredential, int, error) {
	if filter == nil {
		filter = &domain.ChannelCredentialListFilter{Limit: 20}
	}
	tenantID := strings.TrimSpace(filter.TenantID)
	channelID := strings.TrimSpace(filter.ChannelID)
	if tenantID == "" || channelID == "" {
		return nil, 0, fmt.Errorf("tenant_id and channel_id are required")
	}
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	where := []string{"tenant_id = $1", "channel_id = $2"}
	args := []any{tenantID, channelID}
	argN := 3
	if purpose := strings.TrimSpace(filter.Purpose); purpose != "" {
		where = append(where, fmt.Sprintf("purpose = $%d", argN))
		args = append(args, purpose)
		argN++
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM tenant_channel_credentials WHERE %s`, whereSQL), args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count channel credentials: %w", err)
	}

	args = append(args, filter.Limit, filter.Offset)
	rows, err := r.db.Query(fmt.Sprintf(`
		SELECT id, tenant_id, channel_id, purpose, version, status, secret_ref, secret_ref_display,
		       secret_fingerprint, created_by, created_at, updated_at, activated_at, deactivated_at
		FROM tenant_channel_credentials
		WHERE %s
		ORDER BY version DESC, created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, argN, argN+1), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list channel credentials: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.AdminChannelCredential, 0)
	for rows.Next() {
		credential, err := scanChannelCredential(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan channel credential: %w", err)
		}
		out = append(out, credential)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to iterate channel credentials: %w", err)
	}
	return out, total, nil
}

func (r *AdminManagementRepository) RotateChannelCredentialWithActivityLog(credential *domain.AdminChannelCredential, entry *domain.AdminActivityLog) (*domain.AdminChannelCredential, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin channel credential rotation tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	var channelStatus domain.ChannelStatus
	if err := tx.QueryRow(`
		SELECT status
		FROM tenant_channels
		WHERE tenant_id = $1 AND id = $2
		FOR UPDATE
	`, credential.TenantID, credential.ChannelID).Scan(&channelStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAdminNotFound
		}
		return nil, fmt.Errorf("failed to lock tenant channel: %w", err)
	}
	if channelStatus != domain.ChannelStatusActive {
		return nil, ErrAdminInvalidState
	}

	existing, err := scanChannelCredential(tx.QueryRow(`
		SELECT id, tenant_id, channel_id, purpose, version, status, secret_ref, secret_ref_display,
		       secret_fingerprint, created_by, created_at, updated_at, activated_at, deactivated_at
		FROM tenant_channel_credentials
		WHERE tenant_id = $1 AND channel_id = $2 AND purpose = $3 AND secret_fingerprint = $4 AND status = 'ACTIVE'
	`, credential.TenantID, credential.ChannelID, credential.Purpose, credential.SecretFingerprint))
	if err == nil {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("failed to commit idempotent credential lookup tx: %w", err)
		}
		committed = true
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check existing channel credential: %w", err)
	}

	var nextVersion int
	if err := tx.QueryRow(`
		SELECT COALESCE(MAX(version), 0) + 1
		FROM tenant_channel_credentials
		WHERE tenant_id = $1 AND channel_id = $2 AND purpose = $3
	`, credential.TenantID, credential.ChannelID, credential.Purpose).Scan(&nextVersion); err != nil {
		return nil, fmt.Errorf("failed to calculate credential version: %w", err)
	}

	now := time.Now().UTC()
	if _, err := tx.Exec(`
		UPDATE tenant_channel_credentials
		SET status = 'INACTIVE', updated_at = $4, deactivated_at = $4
		WHERE tenant_id = $1 AND channel_id = $2 AND purpose = $3 AND status = 'ACTIVE'
	`, credential.TenantID, credential.ChannelID, credential.Purpose, now); err != nil {
		return nil, fmt.Errorf("failed to deactivate old channel credential: %w", err)
	}

	credential.ID = uuid.NewString()
	created, err := scanChannelCredential(tx.QueryRow(`
		INSERT INTO tenant_channel_credentials (
			id, tenant_id, channel_id, purpose, version, status, secret_ref, secret_ref_display,
			secret_fingerprint, created_by, activated_at
		)
		VALUES ($1, $2, $3, $4, $5, 'ACTIVE', $6, $7, $8, $9, $10)
		RETURNING id, tenant_id, channel_id, purpose, version, status, secret_ref, secret_ref_display,
		          secret_fingerprint, created_by, created_at, updated_at, activated_at, deactivated_at
	`, credential.ID, credential.TenantID, credential.ChannelID, credential.Purpose, nextVersion, credential.SecretRef,
		credential.SecretRefDisplay, credential.SecretFingerprint, credential.CreatedBy, now))
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrAdminConflict
		}
		return nil, fmt.Errorf("failed to insert channel credential: %w", err)
	}

	if entry != nil {
		entry.TenantID = created.TenantID
		entry.EntityType = "tenant_channel_credential"
		entry.EntityID = created.ID
		if err := createActivityLog(tx, entry); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit channel credential rotation tx: %w", err)
	}
	committed = true
	return created, nil
}

func (r *AdminManagementRepository) ListUserbase(filter *domain.UserbaseListFilter) ([]*domain.UserbaseRecord, int, error) {
	if filter == nil {
		filter = &domain.UserbaseListFilter{Limit: 20}
	}
	tenantID := strings.TrimSpace(filter.TenantID)
	if tenantID == "" {
		return nil, 0, fmt.Errorf("tenant_id is required")
	}
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	where := []string{"tenant_id = $1"}
	args := []any{tenantID}
	argN := 2
	if msisdn := strings.TrimSpace(filter.MSISDN); msisdn != "" {
		where = append(where, fmt.Sprintf("msisdn LIKE $%d", argN))
		args = append(args, msisdn+"%")
		argN++
	}
	if typ := strings.TrimSpace(filter.Type); typ != "" {
		where = append(where, fmt.Sprintf("LOWER(type) = LOWER($%d)", argN))
		args = append(args, typ)
		argN++
	}

	whereSQL := strings.Join(where, " AND ")
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM userbase WHERE %s`, whereSQL)
	var total int
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count userbase: %w", err)
	}

	args = append(args, filter.Limit, filter.Offset)
	listQuery := fmt.Sprintf(`
		SELECT id, tenant_id, msisdn, type
		FROM userbase
		WHERE %s
		ORDER BY id DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, argN, argN+1)
	rows, err := r.db.Query(listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list userbase: %w", err)
	}
	defer rows.Close()

	records := make([]*domain.UserbaseRecord, 0)
	for rows.Next() {
		var rec domain.UserbaseRecord
		if err := rows.Scan(&rec.ID, &rec.TenantID, &rec.MSISDN, &rec.Type); err != nil {
			return nil, 0, fmt.Errorf("failed to scan userbase row: %w", err)
		}
		records = append(records, &rec)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to iterate userbase rows: %w", err)
	}

	return records, total, nil
}

func (r *AdminManagementRepository) GetUserbaseByMSISDN(tenantID, msisdn string) (*domain.UserbaseRecord, error) {
	query := `SELECT id, tenant_id, msisdn, type FROM userbase WHERE tenant_id = $1 AND msisdn = $2`
	var rec domain.UserbaseRecord
	if err := r.db.QueryRow(query, tenantID, msisdn).Scan(&rec.ID, &rec.TenantID, &rec.MSISDN, &rec.Type); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAdminNotFound
		}
		return nil, fmt.Errorf("failed to get userbase row: %w", err)
	}
	return &rec, nil
}

func (r *AdminManagementRepository) UpsertUserbase(tenantID, msisdn, userType string) (*domain.UserbaseRecord, error) {
	query := `
		INSERT INTO userbase (tenant_id, msisdn, type)
		VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id, msisdn)
		DO UPDATE SET type = EXCLUDED.type
		RETURNING id, tenant_id, msisdn, type
	`
	var rec domain.UserbaseRecord
	if err := r.db.QueryRow(query, tenantID, msisdn, userType).Scan(&rec.ID, &rec.TenantID, &rec.MSISDN, &rec.Type); err != nil {
		return nil, fmt.Errorf("failed to upsert userbase row: %w", err)
	}
	return &rec, nil
}

func (r *AdminManagementRepository) DeleteUserbase(tenantID, msisdn string) error {
	res, err := r.db.Exec(`DELETE FROM userbase WHERE tenant_id = $1 AND msisdn = $2`, tenantID, msisdn)
	if err != nil {
		return fmt.Errorf("failed to delete userbase row: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to inspect delete result: %w", err)
	}
	if affected == 0 {
		return ErrAdminNotFound
	}
	return nil
}

func (r *AdminManagementRepository) CreateUserbaseImportJob(tenantID, filename string, createdBy *string) (*domain.UserbaseImportJob, error) {
	id := uuid.NewString()
	startedAt := time.Now().UTC()
	query := `
		INSERT INTO userbase_import_jobs (id, tenant_id, filename, status, total_rows, success_rows, failed_rows, started_at, created_by)
		VALUES ($1, $2, $3, $4, 0, 0, 0, $5, $6)
	`
	if _, err := r.db.Exec(query, id, tenantID, filename, domain.UserbaseImportStatusProcessing, startedAt, createdBy); err != nil {
		return nil, fmt.Errorf("failed to create userbase import job: %w", err)
	}
	return &domain.UserbaseImportJob{
		ID:        id,
		TenantID:  tenantID,
		Filename:  filename,
		Status:    domain.UserbaseImportStatusProcessing,
		StartedAt: startedAt,
		CreatedBy: createdBy,
	}, nil
}

func (r *AdminManagementRepository) CompleteUserbaseImportJob(tenantID, jobID string, status domain.UserbaseImportJobStatus, totalRows, successRows, failedRows int) error {
	query := `
		UPDATE userbase_import_jobs
		SET status = $1, total_rows = $2, success_rows = $3, failed_rows = $4, completed_at = $5
		WHERE tenant_id = $6 AND id = $7
	`
	now := time.Now().UTC()
	res, err := r.db.Exec(query, status, totalRows, successRows, failedRows, now, tenantID, jobID)
	if err != nil {
		return fmt.Errorf("failed to complete userbase import job: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to inspect update result: %w", err)
	}
	if affected == 0 {
		return ErrAdminNotFound
	}
	return nil
}

func (r *AdminManagementRepository) InsertUserbaseImportErrors(tenantID, jobID string, rows []*domain.UserbaseImportError) error {
	if len(rows) == 0 {
		return nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin import-error tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.Prepare(`
		INSERT INTO userbase_import_errors (tenant_id, job_id, row_number, raw_row, error_message)
		VALUES ($1, $2, $3, $4, $5)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare import-error statement: %w", err)
	}
	defer stmt.Close()

	for _, row := range rows {
		if _, err = stmt.Exec(tenantID, jobID, row.RowNumber, row.RawRow, row.ErrorMessage); err != nil {
			return fmt.Errorf("failed to insert import error row %d: %w", row.RowNumber, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit import-error tx: %w", err)
	}
	return nil
}

func (r *AdminManagementRepository) ListUserbaseImportJobs(tenantID string, limit, offset int) ([]*domain.UserbaseImportJob, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM userbase_import_jobs WHERE tenant_id = $1`, tenantID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count userbase import jobs: %w", err)
	}

	rows, err := r.db.Query(`
		SELECT id, tenant_id, filename, status, total_rows, success_rows, failed_rows, started_at, completed_at, created_by
		FROM userbase_import_jobs
		WHERE tenant_id = $1
		ORDER BY started_at DESC
		LIMIT $2 OFFSET $3
	`, tenantID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list userbase import jobs: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.UserbaseImportJob, 0)
	for rows.Next() {
		var (
			job       domain.UserbaseImportJob
			completed sql.NullTime
			createdBy sql.NullString
		)
		if err := rows.Scan(&job.ID, &job.TenantID, &job.Filename, &job.Status, &job.TotalRows, &job.SuccessRows, &job.FailedRows, &job.StartedAt, &completed, &createdBy); err != nil {
			return nil, 0, fmt.Errorf("failed to scan userbase import job: %w", err)
		}
		if completed.Valid {
			t := completed.Time
			job.CompletedAt = &t
		}
		if createdBy.Valid {
			job.CreatedBy = &createdBy.String
		}
		out = append(out, &job)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to iterate import jobs: %w", err)
	}
	return out, total, nil
}

func (r *AdminManagementRepository) GetUserbaseImportJob(tenantID, jobID string) (*domain.UserbaseImportJob, error) {
	var (
		job       domain.UserbaseImportJob
		completed sql.NullTime
		createdBy sql.NullString
	)
	query := `
		SELECT id, tenant_id, filename, status, total_rows, success_rows, failed_rows, started_at, completed_at, created_by
		FROM userbase_import_jobs
		WHERE tenant_id = $1 AND id = $2
	`
	if err := r.db.QueryRow(query, tenantID, jobID).Scan(&job.ID, &job.TenantID, &job.Filename, &job.Status, &job.TotalRows, &job.SuccessRows, &job.FailedRows, &job.StartedAt, &completed, &createdBy); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAdminNotFound
		}
		return nil, fmt.Errorf("failed to get userbase import job: %w", err)
	}
	if completed.Valid {
		t := completed.Time
		job.CompletedAt = &t
	}
	if createdBy.Valid {
		job.CreatedBy = &createdBy.String
	}
	return &job, nil
}

func (r *AdminManagementRepository) ListUserbaseImportErrors(tenantID, jobID string, limit, offset int) ([]*domain.UserbaseImportError, int, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM userbase_import_errors WHERE tenant_id = $1 AND job_id = $2`, tenantID, jobID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count userbase import errors: %w", err)
	}

	rows, err := r.db.Query(`
		SELECT id, tenant_id, job_id, row_number, raw_row, error_message
		FROM userbase_import_errors
		WHERE tenant_id = $1 AND job_id = $2
		ORDER BY id ASC
		LIMIT $3 OFFSET $4
	`, tenantID, jobID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list userbase import errors: %w", err)
	}
	defer rows.Close()

	errorsOut := make([]*domain.UserbaseImportError, 0)
	for rows.Next() {
		var item domain.UserbaseImportError
		if err := rows.Scan(&item.ID, &item.TenantID, &item.JobID, &item.RowNumber, &item.RawRow, &item.ErrorMessage); err != nil {
			return nil, 0, fmt.Errorf("failed to scan import error row: %w", err)
		}
		errorsOut = append(errorsOut, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to iterate import errors: %w", err)
	}
	return errorsOut, total, nil
}

func (r *AdminManagementRepository) CreateActivityLog(entry *domain.AdminActivityLog) error {
	if err := createActivityLog(r.db, entry); err != nil {
		return err
	}
	return nil
}

type activityLogExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func createActivityLog(exec activityLogExecer, entry *domain.AdminActivityLog) error {
	query := `
		INSERT INTO admin_activity_logs (
			id, tenant_id, entity_type, entity_id, action, actor, request_id, before_json, after_json, metadata_json, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	if entry.ID == "" {
		entry.ID = uuid.NewString()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}
	_, err := exec.Exec(query,
		entry.ID,
		nullableString(entry.TenantID),
		entry.EntityType,
		entry.EntityID,
		entry.Action,
		entry.Actor,
		entry.RequestID,
		nullableJSON(entry.BeforeJSON),
		nullableJSON(entry.AfterJSON),
		nullableJSON(entry.Metadata),
		entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create activity log: %w", err)
	}
	return nil
}

func (r *AdminManagementRepository) ListActivityLogs(filter *domain.AdminActivityLogFilter) ([]*domain.AdminActivityLog, int, error) {
	if filter == nil {
		filter = &domain.AdminActivityLogFilter{Limit: 20}
	}
	tenantID := strings.TrimSpace(filter.TenantID)
	if tenantID == "" {
		return nil, 0, fmt.Errorf("tenant_id is required")
	}
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	where := []string{"tenant_id = $1"}
	args := []any{tenantID}
	argN := 2

	if v := strings.TrimSpace(filter.EntityType); v != "" {
		where = append(where, fmt.Sprintf("entity_type = $%d", argN))
		args = append(args, v)
		argN++
	}
	if v := strings.TrimSpace(filter.Action); v != "" {
		where = append(where, fmt.Sprintf("action = $%d", argN))
		args = append(args, v)
		argN++
	}
	if v := strings.TrimSpace(filter.Actor); v != "" {
		where = append(where, fmt.Sprintf("actor = $%d", argN))
		args = append(args, v)
		argN++
	}
	if filter.From != nil {
		where = append(where, fmt.Sprintf("created_at >= $%d", argN))
		args = append(args, *filter.From)
		argN++
	}
	if filter.To != nil {
		where = append(where, fmt.Sprintf("created_at <= $%d", argN))
		args = append(args, *filter.To)
		argN++
	}

	whereSQL := strings.Join(where, " AND ")
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM admin_activity_logs WHERE %s`, whereSQL)
	var total int
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count activity logs: %w", err)
	}

	args = append(args, filter.Limit, filter.Offset)
	query := fmt.Sprintf(`
		SELECT id, tenant_id, entity_type, entity_id, action, actor, request_id, before_json, after_json, metadata_json, created_at
		FROM admin_activity_logs
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, argN, argN+1)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list activity logs: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.AdminActivityLog, 0)
	for rows.Next() {
		var (
			item         domain.AdminActivityLog
			scanTenantID sql.NullString
			actor        sql.NullString
			requestID    sql.NullString
			before       sql.NullString
			after        sql.NullString
			metadata     sql.NullString
		)
		if err := rows.Scan(
			&item.ID,
			&scanTenantID,
			&item.EntityType,
			&item.EntityID,
			&item.Action,
			&actor,
			&requestID,
			&before,
			&after,
			&metadata,
			&item.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan activity log: %w", err)
		}
		if scanTenantID.Valid {
			item.TenantID = scanTenantID.String
		}
		if actor.Valid {
			item.Actor = &actor.String
		}
		if requestID.Valid {
			item.RequestID = &requestID.String
		}
		if before.Valid {
			item.BeforeJSON = []byte(before.String)
		}
		if after.Valid {
			item.AfterJSON = []byte(after.String)
		}
		if metadata.Valid {
			item.Metadata = []byte(metadata.String)
		}
		out = append(out, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to iterate activity logs: %w", err)
	}

	return out, total, nil
}

func nullableJSON(data []byte) any {
	if len(data) == 0 {
		return nil
	}
	return string(data)
}

func nullableString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func nullableStringPtr(value *string) any {
	if value == nil {
		return nil
	}
	return nullableString(*value)
}

func nullableTenantStatusPtr(value *domain.TenantStatus) any {
	if value == nil {
		return nil
	}
	status := domain.TenantStatus(strings.ToUpper(strings.TrimSpace(string(*value))))
	if status == "" {
		return nil
	}
	return status
}

func tenantMetadataJSON(data []byte) string {
	if len(data) == 0 {
		return "{}"
	}
	return string(data)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTenant(row rowScanner) (*domain.AdminTenant, error) {
	var (
		tenant   domain.AdminTenant
		metadata []byte
	)
	if err := row.Scan(
		&tenant.ID,
		&tenant.TenantKey,
		&tenant.Name,
		&tenant.Status,
		&tenant.DefaultCountry,
		&metadata,
		&tenant.CreatedAt,
		&tenant.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if len(metadata) == 0 {
		metadata = []byte("{}")
	}
	tenant.Metadata = metadata
	return &tenant, nil
}

func scanTenantMember(row rowScanner) (*domain.AdminTenantMember, error) {
	var (
		member    domain.AdminTenantMember
		email     sql.NullString
		createdBy sql.NullString
	)
	if err := row.Scan(
		&member.ID,
		&member.TenantID,
		&member.Auth0Subject,
		&email,
		&member.Role,
		&member.Status,
		&createdBy,
		&member.CreatedAt,
		&member.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan tenant member: %w", err)
	}
	if email.Valid {
		member.Email = &email.String
	}
	if createdBy.Valid {
		member.CreatedBy = &createdBy.String
	}
	return &member, nil
}

func scanChannel(row rowScanner) (*domain.AdminChannel, error) {
	var (
		channel      domain.AdminChannel
		operator     sql.NullString
		capabilities pq.StringArray
	)
	if err := row.Scan(
		&channel.ID,
		&channel.TenantID,
		&channel.ChannelKey,
		&channel.Provider,
		&channel.Country,
		&operator,
		&capabilities,
		&channel.Status,
		&channel.CreatedAt,
		&channel.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if operator.Valid {
		channel.Operator = &operator.String
	}
	channel.Capabilities = []string(capabilities)
	channel.Enabled = channel.Status == domain.ChannelStatusActive
	return &channel, nil
}

func scanChannelCredential(row rowScanner) (*domain.AdminChannelCredential, error) {
	var (
		credential  domain.AdminChannelCredential
		createdBy   sql.NullString
		activated   sql.NullTime
		deactivated sql.NullTime
	)
	if err := row.Scan(
		&credential.ID,
		&credential.TenantID,
		&credential.ChannelID,
		&credential.Purpose,
		&credential.Version,
		&credential.Status,
		&credential.SecretRef,
		&credential.SecretRefDisplay,
		&credential.SecretFingerprint,
		&createdBy,
		&credential.CreatedAt,
		&credential.UpdatedAt,
		&activated,
		&deactivated,
	); err != nil {
		return nil, err
	}
	if createdBy.Valid {
		credential.CreatedBy = &createdBy.String
	}
	if activated.Valid {
		t := activated.Time
		credential.ActivatedAt = &t
	}
	if deactivated.Valid {
		t := deactivated.Time
		credential.DeactivatedAt = &t
	}
	return &credential, nil
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && string(pqErr.Code) == "23505" {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate key value")
}
