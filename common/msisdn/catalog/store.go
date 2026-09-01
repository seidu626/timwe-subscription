package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) List(ctx context.Context, filter ListFilter) ([]Record, int64, error) {
	if strings.TrimSpace(filter.TenantID) == "" {
		return nil, 0, fmt.Errorf("tenant_id is required")
	}
	if filter.Limit <= 0 {
		filter.Limit = 25
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	where := []string{"tenant_id = $1"}
	args := []any{filter.TenantID}
	addEqual := func(column, value string) {
		if value = strings.TrimSpace(value); value != "" {
			args = append(args, strings.ToUpper(value))
			where = append(where, fmt.Sprintf("%s = $%d", column, len(args)))
		}
	}
	if q := strings.TrimSpace(filter.Query); q != "" {
		args = append(args, q+"%")
		where = append(where, fmt.Sprintf("msisdn LIKE $%d", len(args)))
	}
	addEqual("region", filter.Region)
	addEqual("telco", filter.Telco)
	addEqual("verification_status", filter.VerificationStatus)
	if filter.DND != nil {
		args = append(args, *filter.DND)
		where = append(where, fmt.Sprintf("dnd = $%d", len(args)))
	}
	if filter.Invalid != nil {
		args = append(args, *filter.Invalid)
		where = append(where, fmt.Sprintf("invalid = $%d", len(args)))
	}

	whereSQL := strings.Join(where, " AND ")
	var total int64
	countTable := "msisdn_catalog"
	countColumn := "COUNT(*)"
	if strings.TrimSpace(filter.Query) == "" {
		countTable = "msisdn_catalog_pool_counts"
		countColumn = "COALESCE(SUM(record_count), 0)"
	}
	if err := s.db.QueryRowContext(ctx, "SELECT "+countColumn+" FROM "+countTable+" WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count msisdn catalog: %w", err)
	}

	args = append(args, filter.Limit, filter.Offset)
	query := fmt.Sprintf(`
		SELECT id, tenant_id::text, msisdn, region, telco, verification_status,
		       dnd, invalid, invalid_reason, source, generation_batch_id,
		       verification_provider, verification_reference, verification_attempts,
		       verified_at, last_verification_at, last_error, created_at, updated_at
		FROM msisdn_catalog
		WHERE %s
		ORDER BY id DESC
		LIMIT $%d OFFSET $%d`, whereSQL, len(args)-1, len(args))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list msisdn catalog: %w", err)
	}
	defer rows.Close()

	items := make([]Record, 0, filter.Limit)
	for rows.Next() {
		record, scanErr := scanRecord(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate msisdn catalog: %w", err)
	}
	return items, total, nil
}

func (s *Store) Stats(ctx context.Context, tenantID string) (Stats, error) {
	var out Stats
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(record_count), 0),
		       COALESCE(SUM(record_count) FILTER (WHERE verification_status = 'VERIFIED' AND NOT dnd AND NOT invalid), 0),
		       COALESCE(SUM(record_count) FILTER (WHERE verification_status = 'VERIFIED'), 0),
		       COALESCE(SUM(record_count) FILTER (WHERE verification_status = 'PENDING'), 0),
		       COALESCE(SUM(record_count) FILTER (WHERE dnd), 0),
		       COALESCE(SUM(record_count) FILTER (WHERE invalid), 0),
		       COALESCE(SUM(record_count) FILTER (WHERE verification_status = 'ERROR'), 0)
		FROM msisdn_catalog_pool_counts WHERE tenant_id = $1`, tenantID).
		Scan(&out.Total, &out.Ready, &out.Verified, &out.Pending, &out.DND, &out.Invalid, &out.Errors)
	if err != nil {
		return Stats{}, fmt.Errorf("get msisdn catalog stats: %w", err)
	}
	return out, nil
}

// InsertGenerated persists candidate numbers and returns only newly inserted
// records. Existing records (including DND and invalid entries) are not reused.
func (s *Store) InsertGenerated(ctx context.Context, tenantID, region, telco, batchID string, msisdns []string) ([]Record, error) {
	return s.insertMany(ctx, tenantID, region, telco, SourceBatchGenerated, batchID, false, msisdns)
}

func (s *Store) MarkDND(ctx context.Context, tenantID, region, telco string, msisdns []string) (int, error) {
	if len(msisdns) == 0 {
		return 0, nil
	}
	const chunkSize = 500
	for start := 0; start < len(msisdns); start += chunkSize {
		end := start + chunkSize
		if end > len(msisdns) {
			end = len(msisdns)
		}
		args := make([]any, 0, (end-start)*4)
		values := make([]string, 0, end-start)
		for _, msisdn := range msisdns[start:end] {
			base := len(args)
			args = append(args, tenantID, msisdn, region, telco)
			values = append(values, fmt.Sprintf("($%d,$%d,$%d,$%d,TRUE,'DND_IMPORT')", base+1, base+2, base+3, base+4))
		}
		query := `INSERT INTO msisdn_catalog (tenant_id, msisdn, region, telco, dnd, source) VALUES ` + strings.Join(values, ",") + `
			ON CONFLICT (tenant_id, region, msisdn) DO UPDATE SET dnd = TRUE, updated_at = NOW()`
		if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
			return 0, fmt.Errorf("mark DND: %w", err)
		}
	}
	return len(msisdns), nil
}

func (s *Store) SetFlags(ctx context.Context, tenantID string, id int64, dnd, invalid *bool, reason string) (Record, error) {
	if dnd == nil && invalid == nil {
		return Record{}, fmt.Errorf("dnd or invalid flag is required")
	}
	query := `UPDATE msisdn_catalog SET updated_at = NOW()`
	args := []any{}
	if dnd != nil {
		args = append(args, *dnd)
		query += fmt.Sprintf(", dnd = $%d", len(args))
	}
	if invalid != nil {
		args = append(args, *invalid)
		query += fmt.Sprintf(", invalid = $%d", len(args))
		if *invalid {
			args = append(args, strings.TrimSpace(reason))
			query += fmt.Sprintf(", invalid_reason = NULLIF($%d, ''), verification_status = 'INVALID'", len(args))
		} else {
			query += ", invalid_reason = NULL, verification_status = CASE WHEN verification_status = 'INVALID' THEN 'PENDING' ELSE verification_status END"
		}
	}
	args = append(args, tenantID, id)
	query += fmt.Sprintf(" WHERE tenant_id = $%d AND id = $%d RETURNING ", len(args)-1, len(args)) + recordColumns
	record, err := scanRecord(s.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, sql.ErrNoRows
	}
	return record, err
}

func (s *Store) PendingForVerification(ctx context.Context, tenantID string, ids []int64, limit int) ([]Record, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	args := []any{tenantID}
	where := "tenant_id = $1 AND region = 'GH' AND telco = 'MTN' AND verification_status IN ('PENDING', 'ERROR') AND NOT dnd AND NOT invalid"
	if len(ids) > 0 {
		placeholders := make([]string, 0, len(ids))
		for _, id := range ids {
			args = append(args, id)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
		}
		where += " AND id IN (" + strings.Join(placeholders, ",") + ")"
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, "SELECT "+recordColumns+" FROM msisdn_catalog WHERE "+where+fmt.Sprintf(" ORDER BY id LIMIT $%d", len(args)), args...)
	if err != nil {
		return nil, fmt.Errorf("list verification candidates: %w", err)
	}
	defer rows.Close()
	items := make([]Record, 0, limit)
	for rows.Next() {
		record, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, record)
	}
	return items, rows.Err()
}

func (s *Store) SaveVerification(ctx context.Context, tenantID string, id int64, result VerificationResult) error {
	now := time.Now().UTC()
	dbResult, err := s.db.ExecContext(ctx, `
		UPDATE msisdn_catalog
		SET verification_status = $1,
		    invalid = CASE WHEN $1 = 'INVALID' THEN TRUE ELSE invalid END,
		    invalid_reason = CASE WHEN $1 = 'INVALID' THEN NULLIF($2, '') ELSE invalid_reason END,
		    verification_provider = NULLIF($3, ''),
		    verification_reference = NULLIF($4, ''),
		    verification_attempts = verification_attempts + 1,
		    verified_at = CASE WHEN $1 = 'VERIFIED' THEN $5 ELSE verified_at END,
		    last_verification_at = $5,
		    last_error = CASE WHEN $1 = 'ERROR' THEN NULLIF($2, '') ELSE NULL END,
		    updated_at = $5
		WHERE tenant_id = $6 AND id = $7`, result.Status, result.Reason, result.Provider, result.Reference, now, tenantID, id)
	if err != nil {
		return fmt.Errorf("save MADAPI verification: %w", err)
	}
	affected, err := dbResult.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect MADAPI verification update: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("save MADAPI verification: catalog record not found")
	}
	return nil
}

const recordColumns = `id, tenant_id::text, msisdn, region, telco, verification_status,
	dnd, invalid, invalid_reason, source, generation_batch_id,
	verification_provider, verification_reference, verification_attempts,
	verified_at, last_verification_at, last_error, created_at, updated_at`

type scanner interface{ Scan(dest ...any) error }

func scanRecord(row scanner) (Record, error) {
	var rec Record
	var invalidReason, batchID, provider, reference, lastError sql.NullString
	var verifiedAt, lastVerificationAt sql.NullTime
	err := row.Scan(&rec.ID, &rec.TenantID, &rec.MSISDN, &rec.Region, &rec.Telco, &rec.VerificationStatus,
		&rec.DND, &rec.Invalid, &invalidReason, &rec.Source, &batchID, &provider, &reference,
		&rec.VerificationAttempts, &verifiedAt, &lastVerificationAt, &lastError, &rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		return Record{}, err
	}
	if invalidReason.Valid {
		rec.InvalidReason = &invalidReason.String
	}
	if batchID.Valid {
		rec.GenerationBatchID = &batchID.String
	}
	if provider.Valid {
		rec.VerificationProvider = &provider.String
	}
	if reference.Valid {
		rec.VerificationReference = &reference.String
	}
	if lastError.Valid {
		rec.LastError = &lastError.String
	}
	if verifiedAt.Valid {
		rec.VerifiedAt = &verifiedAt.Time
	}
	if lastVerificationAt.Valid {
		rec.LastVerificationAt = &lastVerificationAt.Time
	}
	return rec, nil
}

func (s *Store) insertMany(ctx context.Context, tenantID, region, telco, source, batchID string, dnd bool, msisdns []string) ([]Record, error) {
	if len(msisdns) == 0 {
		return []Record{}, nil
	}
	const chunkSize = 500
	inserted := make([]Record, 0, len(msisdns))
	for start := 0; start < len(msisdns); start += chunkSize {
		end := start + chunkSize
		if end > len(msisdns) {
			end = len(msisdns)
		}
		args := make([]any, 0, (end-start)*7)
		values := make([]string, 0, end-start)
		for _, msisdn := range msisdns[start:end] {
			base := len(args)
			args = append(args, tenantID, msisdn, region, telco, dnd, source, batchID)
			values = append(values, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,NULLIF($%d,''))", base+1, base+2, base+3, base+4, base+5, base+6, base+7))
		}
		query := `INSERT INTO msisdn_catalog
			(tenant_id, msisdn, region, telco, dnd, source, generation_batch_id)
			VALUES ` + strings.Join(values, ",") + `
			ON CONFLICT (tenant_id, region, msisdn) DO NOTHING
			RETURNING ` + recordColumns
		rows, err := s.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("insert generated catalog records: %w", err)
		}
		for rows.Next() {
			record, scanErr := scanRecord(rows)
			if scanErr != nil {
				rows.Close()
				return nil, scanErr
			}
			inserted = append(inserted, record)
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	return inserted, nil
}
