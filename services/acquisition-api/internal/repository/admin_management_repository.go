package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"go.uber.org/zap"
)

var (
	// ErrAdminNotFound is returned when an admin-managed resource does not exist.
	ErrAdminNotFound = errors.New("admin resource not found")
)

// AdminManagementRepository handles products/userbase/admin activity data access.
type AdminManagementRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

func NewAdminManagementRepository(db *sql.DB, logger *zap.Logger) *AdminManagementRepository {
	return &AdminManagementRepository{db: db, logger: logger}
}

func (r *AdminManagementRepository) ListProducts(filter *domain.ProductListFilter) ([]*domain.AdminProduct, int, error) {
	if filter == nil {
		filter = &domain.ProductListFilter{Limit: 20}
	}
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	where := []string{"1=1"}
	args := []any{}
	argN := 1

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
		SELECT id, product_id, name, price_point_id, price_point_value, short_code, created_at
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
		if err := rows.Scan(&p.ID, &p.ProductID, &p.Name, &p.PricePointID, &p.PricePointValue, &p.ShortCode, &p.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("failed to scan product: %w", err)
		}
		products = append(products, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to iterate products: %w", err)
	}

	return products, total, nil
}

func (r *AdminManagementRepository) GetProductByID(id int) (*domain.AdminProduct, error) {
	query := `
		SELECT id, product_id, name, price_point_id, price_point_value, short_code, created_at
		FROM products
		WHERE id = $1
	`
	var p domain.AdminProduct
	if err := r.db.QueryRow(query, id).Scan(&p.ID, &p.ProductID, &p.Name, &p.PricePointID, &p.PricePointValue, &p.ShortCode, &p.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAdminNotFound
		}
		return nil, fmt.Errorf("failed to get product: %w", err)
	}
	return &p, nil
}

func (r *AdminManagementRepository) CreateProduct(product *domain.AdminProduct) (*domain.AdminProduct, error) {
	query := `
		INSERT INTO products (product_id, name, price_point_id, price_point_value, short_code)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`
	out := *product
	if err := r.db.QueryRow(
		query,
		product.ProductID,
		product.Name,
		product.PricePointID,
		product.PricePointValue,
		product.ShortCode,
	).Scan(&out.ID, &out.CreatedAt); err != nil {
		return nil, fmt.Errorf("failed to create product: %w", err)
	}
	return &out, nil
}

func (r *AdminManagementRepository) UpdateProduct(id int, product *domain.AdminProduct) (*domain.AdminProduct, error) {
	query := `
		UPDATE products
		SET product_id = $1, name = $2, price_point_id = $3, price_point_value = $4, short_code = $5
		WHERE id = $6
		RETURNING id, product_id, name, price_point_id, price_point_value, short_code, created_at
	`
	var out domain.AdminProduct
	if err := r.db.QueryRow(
		query,
		product.ProductID,
		product.Name,
		product.PricePointID,
		product.PricePointValue,
		product.ShortCode,
		id,
	).Scan(&out.ID, &out.ProductID, &out.Name, &out.PricePointID, &out.PricePointValue, &out.ShortCode, &out.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAdminNotFound
		}
		return nil, fmt.Errorf("failed to update product: %w", err)
	}
	return &out, nil
}

func (r *AdminManagementRepository) DeleteProduct(id int) error {
	res, err := r.db.Exec(`DELETE FROM products WHERE id = $1`, id)
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

func (r *AdminManagementRepository) BatchUpsertProducts(products []*domain.AdminProduct) (int, error) {
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
			SET name = $2, price_point_id = $3, price_point_value = $4, short_code = $5
			WHERE product_id = $1
			RETURNING id
		)
		INSERT INTO products (product_id, name, price_point_id, price_point_value, short_code)
		SELECT $1, $2, $3, $4, $5
		WHERE NOT EXISTS (SELECT 1 FROM updated)
	`
	for _, p := range products {
		if _, err = tx.Exec(query, p.ProductID, p.Name, p.PricePointID, p.PricePointValue, p.ShortCode); err != nil {
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

func (r *AdminManagementRepository) ListUserbase(filter *domain.UserbaseListFilter) ([]*domain.UserbaseRecord, int, error) {
	if filter == nil {
		filter = &domain.UserbaseListFilter{Limit: 20}
	}
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	where := []string{"1=1"}
	args := []any{}
	argN := 1
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
		SELECT id, msisdn, type
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
		if err := rows.Scan(&rec.ID, &rec.MSISDN, &rec.Type); err != nil {
			return nil, 0, fmt.Errorf("failed to scan userbase row: %w", err)
		}
		records = append(records, &rec)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to iterate userbase rows: %w", err)
	}

	return records, total, nil
}

func (r *AdminManagementRepository) GetUserbaseByMSISDN(msisdn string) (*domain.UserbaseRecord, error) {
	query := `SELECT id, msisdn, type FROM userbase WHERE msisdn = $1`
	var rec domain.UserbaseRecord
	if err := r.db.QueryRow(query, msisdn).Scan(&rec.ID, &rec.MSISDN, &rec.Type); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAdminNotFound
		}
		return nil, fmt.Errorf("failed to get userbase row: %w", err)
	}
	return &rec, nil
}

func (r *AdminManagementRepository) UpsertUserbase(msisdn, userType string) (*domain.UserbaseRecord, error) {
	query := `
		INSERT INTO userbase (msisdn, type)
		VALUES ($1, $2)
		ON CONFLICT (msisdn)
		DO UPDATE SET type = EXCLUDED.type
		RETURNING id, msisdn, type
	`
	var rec domain.UserbaseRecord
	if err := r.db.QueryRow(query, msisdn, userType).Scan(&rec.ID, &rec.MSISDN, &rec.Type); err != nil {
		return nil, fmt.Errorf("failed to upsert userbase row: %w", err)
	}
	return &rec, nil
}

func (r *AdminManagementRepository) DeleteUserbase(msisdn string) error {
	res, err := r.db.Exec(`DELETE FROM userbase WHERE msisdn = $1`, msisdn)
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

func (r *AdminManagementRepository) CreateUserbaseImportJob(filename string, createdBy *string) (*domain.UserbaseImportJob, error) {
	id := uuid.NewString()
	startedAt := time.Now().UTC()
	query := `
		INSERT INTO userbase_import_jobs (id, filename, status, total_rows, success_rows, failed_rows, started_at, created_by)
		VALUES ($1, $2, $3, 0, 0, 0, $4, $5)
	`
	if _, err := r.db.Exec(query, id, filename, domain.UserbaseImportStatusProcessing, startedAt, createdBy); err != nil {
		return nil, fmt.Errorf("failed to create userbase import job: %w", err)
	}
	return &domain.UserbaseImportJob{
		ID:        id,
		Filename:  filename,
		Status:    domain.UserbaseImportStatusProcessing,
		StartedAt: startedAt,
		CreatedBy: createdBy,
	}, nil
}

func (r *AdminManagementRepository) CompleteUserbaseImportJob(jobID string, status domain.UserbaseImportJobStatus, totalRows, successRows, failedRows int) error {
	query := `
		UPDATE userbase_import_jobs
		SET status = $1, total_rows = $2, success_rows = $3, failed_rows = $4, completed_at = $5
		WHERE id = $6
	`
	now := time.Now().UTC()
	res, err := r.db.Exec(query, status, totalRows, successRows, failedRows, now, jobID)
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

func (r *AdminManagementRepository) InsertUserbaseImportErrors(jobID string, rows []*domain.UserbaseImportError) error {
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
		INSERT INTO userbase_import_errors (job_id, row_number, raw_row, error_message)
		VALUES ($1, $2, $3, $4)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare import-error statement: %w", err)
	}
	defer stmt.Close()

	for _, row := range rows {
		if _, err = stmt.Exec(jobID, row.RowNumber, row.RawRow, row.ErrorMessage); err != nil {
			return fmt.Errorf("failed to insert import error row %d: %w", row.RowNumber, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit import-error tx: %w", err)
	}
	return nil
}

func (r *AdminManagementRepository) ListUserbaseImportJobs(limit, offset int) ([]*domain.UserbaseImportJob, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM userbase_import_jobs`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count userbase import jobs: %w", err)
	}

	rows, err := r.db.Query(`
		SELECT id, filename, status, total_rows, success_rows, failed_rows, started_at, completed_at, created_by
		FROM userbase_import_jobs
		ORDER BY started_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
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
		if err := rows.Scan(&job.ID, &job.Filename, &job.Status, &job.TotalRows, &job.SuccessRows, &job.FailedRows, &job.StartedAt, &completed, &createdBy); err != nil {
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

func (r *AdminManagementRepository) GetUserbaseImportJob(jobID string) (*domain.UserbaseImportJob, error) {
	var (
		job       domain.UserbaseImportJob
		completed sql.NullTime
		createdBy sql.NullString
	)
	query := `
		SELECT id, filename, status, total_rows, success_rows, failed_rows, started_at, completed_at, created_by
		FROM userbase_import_jobs
		WHERE id = $1
	`
	if err := r.db.QueryRow(query, jobID).Scan(&job.ID, &job.Filename, &job.Status, &job.TotalRows, &job.SuccessRows, &job.FailedRows, &job.StartedAt, &completed, &createdBy); err != nil {
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

func (r *AdminManagementRepository) ListUserbaseImportErrors(jobID string, limit, offset int) ([]*domain.UserbaseImportError, int, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM userbase_import_errors WHERE job_id = $1`, jobID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count userbase import errors: %w", err)
	}

	rows, err := r.db.Query(`
		SELECT id, job_id, row_number, raw_row, error_message
		FROM userbase_import_errors
		WHERE job_id = $1
		ORDER BY id ASC
		LIMIT $2 OFFSET $3
	`, jobID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list userbase import errors: %w", err)
	}
	defer rows.Close()

	errorsOut := make([]*domain.UserbaseImportError, 0)
	for rows.Next() {
		var item domain.UserbaseImportError
		if err := rows.Scan(&item.ID, &item.JobID, &item.RowNumber, &item.RawRow, &item.ErrorMessage); err != nil {
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
	query := `
		INSERT INTO admin_activity_logs (
			id, entity_type, entity_id, action, actor, request_id, before_json, after_json, metadata_json, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	if entry.ID == "" {
		entry.ID = uuid.NewString()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.Exec(query,
		entry.ID,
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
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	where := []string{"1=1"}
	args := []any{}
	argN := 1

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
		SELECT id, entity_type, entity_id, action, actor, request_id, before_json, after_json, metadata_json, created_at
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
			item      domain.AdminActivityLog
			actor     sql.NullString
			requestID sql.NullString
			before    sql.NullString
			after     sql.NullString
			metadata  sql.NullString
		)
		if err := rows.Scan(
			&item.ID,
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
