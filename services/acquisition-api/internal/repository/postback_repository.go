package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"go.uber.org/zap"
)

// PostbackRepository handles postback data access
type PostbackRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewPostbackRepository creates a new postback repository
func NewPostbackRepository(db *sql.DB, logger *zap.Logger) *PostbackRepository {
	return &PostbackRepository{
		db:     db,
		logger: logger,
	}
}

// CreateOutbox creates a new postback in the outbox
func (r *PostbackRepository) CreateOutbox(outbox *domain.PostbackOutbox) error {
	query := `
		INSERT INTO postback_outbox (
			id, tenant_id, channel_id, transaction_id, event, provider,
			url_template_rendered, http_method, headers, body, failure_reason,
			attempt_count, max_attempts,
			next_retry_at, status, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14, $15, $16, $17
		)
	`

	var body sql.NullString
	var nextRetryAt sql.NullTime
	var tenantID, channelID, failureReason sql.NullString
	if outbox.TenantID != nil && *outbox.TenantID != "" {
		tenantID.String = *outbox.TenantID
		tenantID.Valid = true
	}
	if outbox.ChannelID != nil && *outbox.ChannelID != "" {
		channelID.String = *outbox.ChannelID
		channelID.Valid = true
	}
	if outbox.Body != nil {
		body.String = *outbox.Body
		body.Valid = true
	}
	if outbox.FailureReason != nil && *outbox.FailureReason != "" {
		failureReason.String = *outbox.FailureReason
		failureReason.Valid = true
	}
	if outbox.NextRetryAt != nil {
		nextRetryAt.Time = *outbox.NextRetryAt
		nextRetryAt.Valid = true
	}

	_, err := r.db.Exec(query,
		outbox.ID, tenantID, channelID, outbox.TransactionID, outbox.Event,
		outbox.Provider, outbox.URLTemplateRendered, outbox.HTTPMethod,
		outbox.Headers, body, failureReason, outbox.AttemptCount,
		outbox.MaxAttempts, nextRetryAt, outbox.Status, outbox.CreatedAt,
		outbox.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create postback outbox: %w", err)
	}

	return nil
}

// GetPendingPostbacks retrieves postbacks ready for retry (legacy, non-concurrent safe)
// Use ClaimPendingPostbacks for production workloads with multiple dispatcher replicas
func (r *PostbackRepository) GetPendingPostbacks(limit int) ([]*domain.PostbackOutbox, error) {
	query := `
		SELECT id, tenant_id::text, channel_id::text, transaction_id, event,
		       provider, url_template_rendered, http_method, headers, body,
		       failure_reason, attempt_count, max_attempts, next_retry_at,
		       status, created_at, updated_at
		FROM postback_outbox
		WHERE status IN ('PENDING', 'PROCESSING')
		  AND (next_retry_at IS NULL OR next_retry_at <= CURRENT_TIMESTAMP)
		ORDER BY created_at ASC
		LIMIT $1
	`

	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending postbacks: %w", err)
	}
	defer rows.Close()

	var postbacks []*domain.PostbackOutbox
	for rows.Next() {
		pb, err := r.scanPostbackOutbox(rows)
		if err != nil {
			r.logger.Error("Failed to scan postback", zap.Error(err))
			continue
		}
		postbacks = append(postbacks, pb)
	}

	return postbacks, nil
}

// ClaimPendingPostbacks atomically claims postbacks for processing using FOR UPDATE SKIP LOCKED.
// This is safe for horizontal scaling with multiple dispatcher replicas.
// Returns claimed postbacks that are now marked as PROCESSING.
func (r *PostbackRepository) ClaimPendingPostbacks(limit int) ([]*domain.PostbackOutbox, error) {
	// Start transaction
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Select and lock rows atomically using FOR UPDATE SKIP LOCKED
	// This prevents duplicate processing when multiple dispatcher instances run
	selectQuery := `
		SELECT id, tenant_id::text, channel_id::text, transaction_id, event,
		       provider, url_template_rendered, http_method, headers, body,
		       failure_reason, attempt_count, max_attempts, next_retry_at,
		       status, created_at, updated_at
		FROM postback_outbox
		WHERE status = 'PENDING'
		  AND (next_retry_at IS NULL OR next_retry_at <= CURRENT_TIMESTAMP)
		ORDER BY created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`

	rows, err := tx.Query(selectQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to select postbacks: %w", err)
	}

	var postbacks []*domain.PostbackOutbox
	var ids []uuid.UUID

	for rows.Next() {
		pb, err := r.scanPostbackOutboxFromRows(rows)
		if err != nil {
			r.logger.Error("Failed to scan postback", zap.Error(err))
			continue
		}
		postbacks = append(postbacks, pb)
		ids = append(ids, pb.ID)
	}
	rows.Close()

	if len(ids) == 0 {
		tx.Commit()
		return postbacks, nil
	}

	// Convert UUIDs to strings for PostgreSQL array
	idStrings := make([]string, len(ids))
	for i, id := range ids {
		idStrings[i] = id.String()
	}

	// Update status to PROCESSING for all claimed rows
	updateQuery := `
		UPDATE postback_outbox
		SET status = 'PROCESSING', updated_at = CURRENT_TIMESTAMP
		WHERE id = ANY($1::uuid[])
	`

	_, err = tx.Exec(updateQuery, pq.Array(idStrings))
	if err != nil {
		return nil, fmt.Errorf("failed to update postback status: %w", err)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	r.logger.Info("Claimed postbacks for processing", zap.Int("count", len(postbacks)))

	return postbacks, nil
}

// scanPostbackOutboxFromRows scans a postback from sql.Rows (used by both claim methods)
func (r *PostbackRepository) scanPostbackOutboxFromRows(rows *sql.Rows) (*domain.PostbackOutbox, error) {
	var pb domain.PostbackOutbox
	var body sql.NullString
	var tenantID, channelID, failureReason sql.NullString
	var nextRetryAt sql.NullTime

	err := rows.Scan(
		&pb.ID, &tenantID, &channelID, &pb.TransactionID, &pb.Event,
		&pb.Provider, &pb.URLTemplateRendered, &pb.HTTPMethod, &pb.Headers,
		&body, &failureReason, &pb.AttemptCount, &pb.MaxAttempts,
		&nextRetryAt, &pb.Status, &pb.CreatedAt, &pb.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if tenantID.Valid {
		pb.TenantID = &tenantID.String
	}
	if channelID.Valid {
		pb.ChannelID = &channelID.String
	}
	if body.Valid {
		pb.Body = &body.String
	}
	if failureReason.Valid {
		pb.FailureReason = &failureReason.String
	}
	if nextRetryAt.Valid {
		pb.NextRetryAt = &nextRetryAt.Time
	}

	return &pb, nil
}

// UpdateStatus updates the postback status
func (r *PostbackRepository) UpdateStatus(id uuid.UUID, status domain.PostbackStatus, nextRetryAt *time.Time) error {
	query := `
		UPDATE postback_outbox
		SET status = $1, next_retry_at = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $3
	`

	var nextRetry sql.NullTime
	if nextRetryAt != nil {
		nextRetry.Time = *nextRetryAt
		nextRetry.Valid = true
	}

	_, err := r.db.Exec(query, status, nextRetry, id)
	if err != nil {
		return fmt.Errorf("failed to update postback status: %w", err)
	}

	return nil
}

// IncrementAttempt increments the attempt count
func (r *PostbackRepository) IncrementAttempt(id uuid.UUID, nextRetryAt *time.Time) error {
	query := `
		UPDATE postback_outbox
		SET attempt_count = attempt_count + 1, next_retry_at = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`

	var nextRetry sql.NullTime
	if nextRetryAt != nil {
		nextRetry.Time = *nextRetryAt
		nextRetry.Valid = true
	}

	_, err := r.db.Exec(query, nextRetry, id)
	if err != nil {
		return fmt.Errorf("failed to increment attempt: %w", err)
	}

	return nil
}

// CreateAttempt logs a postback attempt
func (r *PostbackRepository) CreateAttempt(attempt *domain.PostbackAttempt) error {
	query := `
		INSERT INTO postback_attempts (
			id, outbox_id, attempt_number, http_status, response_body,
			error_message, duration_ms, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		)
	`

	var httpStatus sql.NullInt64
	var responseBody, errorMessage sql.NullString
	var durationMs sql.NullInt64

	if attempt.HTTPStatus != nil {
		httpStatus.Int64 = int64(*attempt.HTTPStatus)
		httpStatus.Valid = true
	}
	if attempt.ResponseBody != nil {
		responseBody.String = *attempt.ResponseBody
		responseBody.Valid = true
	}
	if attempt.ErrorMessage != nil {
		errorMessage.String = *attempt.ErrorMessage
		errorMessage.Valid = true
	}
	if attempt.DurationMs != nil {
		durationMs.Int64 = int64(*attempt.DurationMs)
		durationMs.Valid = true
	}

	_, err := r.db.Exec(query,
		attempt.ID, attempt.OutboxID, attempt.AttemptNumber, httpStatus,
		responseBody, errorMessage, durationMs, attempt.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create postback attempt: %w", err)
	}

	return nil
}

// scanPostbackOutbox scans a postback from database rows
func (r *PostbackRepository) scanPostbackOutbox(rows *sql.Rows) (*domain.PostbackOutbox, error) {
	var pb domain.PostbackOutbox
	var body sql.NullString
	var tenantID, channelID, failureReason sql.NullString
	var nextRetryAt sql.NullTime

	err := rows.Scan(
		&pb.ID, &tenantID, &channelID, &pb.TransactionID, &pb.Event,
		&pb.Provider, &pb.URLTemplateRendered, &pb.HTTPMethod, &pb.Headers,
		&body, &failureReason, &pb.AttemptCount, &pb.MaxAttempts,
		&nextRetryAt, &pb.Status, &pb.CreatedAt, &pb.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if tenantID.Valid {
		pb.TenantID = &tenantID.String
	}
	if channelID.Valid {
		pb.ChannelID = &channelID.String
	}
	if body.Valid {
		pb.Body = &body.String
	}
	if failureReason.Valid {
		pb.FailureReason = &failureReason.String
	}
	if nextRetryAt.Valid {
		pb.NextRetryAt = &nextRetryAt.Time
	}

	return &pb, nil
}

// ResetStaleProcessing resets PROCESSING records older than the given duration back to PENDING.
// This recovers postbacks stuck due to dispatcher crashes.
func (r *PostbackRepository) ResetStaleProcessing(olderThan time.Duration) (int64, error) {
	query := `
		UPDATE postback_outbox
		SET status = 'PENDING', updated_at = CURRENT_TIMESTAMP
		WHERE status = 'PROCESSING'
		  AND updated_at < CURRENT_TIMESTAMP - $1::interval
	`

	result, err := r.db.Exec(query, olderThan.String())
	if err != nil {
		return 0, fmt.Errorf("failed to reset stale processing postbacks: %w", err)
	}

	return result.RowsAffected()
}

// GetByStatus returns postback outbox records filtered by status with paging metadata.
func (r *PostbackRepository) GetByStatus(status domain.PostbackStatus, limit int, offset int) ([]*domain.PostbackOutbox, int64, error) {
	countQuery := `
		SELECT COUNT(*)
		FROM postback_outbox
		WHERE status = $1
	`

	var totalCount int64
	if err := r.db.QueryRow(countQuery, status).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("failed to count postback outbox by status: %w", err)
	}

	orderDirection := "DESC"
	if status == domain.PostbackStatusDLQ {
		orderDirection = "ASC"
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id::text, channel_id::text, transaction_id, event,
		       provider, url_template_rendered, http_method, headers, body,
		       failure_reason, attempt_count, max_attempts, next_retry_at,
		       status, created_at, updated_at
		FROM postback_outbox
		WHERE status = $1
		ORDER BY created_at %s
		LIMIT $2 OFFSET $3
	`, orderDirection)

	rows, err := r.db.Query(query, status, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query postback outbox by status: %w", err)
	}
	defer rows.Close()

	var outbox []*domain.PostbackOutbox
	for rows.Next() {
		pb, err := r.scanPostbackOutbox(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan postback outbox: %w", err)
		}
		outbox = append(outbox, pb)
	}
	return outbox, totalCount, nil
}

// GetByStatusForTenant returns tenant-owned postback outbox records filtered by status.
func (r *PostbackRepository) GetByStatusForTenant(tenantID string, status domain.PostbackStatus, limit int, offset int) ([]*domain.PostbackOutbox, int64, error) {
	countQuery := `
		SELECT COUNT(*)
		FROM postback_outbox
		WHERE tenant_id = $1::uuid AND status = $2
	`

	var totalCount int64
	if err := r.db.QueryRow(countQuery, tenantID, status).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("failed to count tenant postback outbox by status: %w", err)
	}

	orderDirection := "DESC"
	if status == domain.PostbackStatusDLQ {
		orderDirection = "ASC"
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id::text, channel_id::text, transaction_id, event,
		       provider, url_template_rendered, http_method, headers, body,
		       failure_reason, attempt_count, max_attempts, next_retry_at,
		       status, created_at, updated_at
		FROM postback_outbox
		WHERE tenant_id = $1::uuid AND status = $2
		ORDER BY created_at %s
		LIMIT $3 OFFSET $4
	`, orderDirection)

	rows, err := r.db.Query(query, tenantID, status, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query tenant postback outbox by status: %w", err)
	}
	defer rows.Close()

	var outbox []*domain.PostbackOutbox
	for rows.Next() {
		pb, err := r.scanPostbackOutbox(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan tenant postback outbox: %w", err)
		}
		outbox = append(outbox, pb)
	}
	return outbox, totalCount, nil
}

// ResetForRetry resets a single postback to PENDING with attempt_count=0 and next_retry_at=NULL.
func (r *PostbackRepository) ResetForRetry(id uuid.UUID) error {
	query := `
		UPDATE postback_outbox
		SET status = 'PENDING', attempt_count = 0, next_retry_at = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND status IN ('DLQ', 'FAILED')
	`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to reset postback for retry: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("postback not found or not in DLQ/FAILED status")
	}

	return nil
}

// ResetForRetryForTenant resets a tenant-owned failed or DLQ postback.
func (r *PostbackRepository) ResetForRetryForTenant(tenantID string, id uuid.UUID) error {
	query := `
		UPDATE postback_outbox
		SET status = 'PENDING', attempt_count = 0, next_retry_at = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE tenant_id = $1::uuid AND id = $2 AND status IN ('DLQ', 'FAILED')
	`

	result, err := r.db.Exec(query, tenantID, id)
	if err != nil {
		return fmt.Errorf("failed to reset tenant postback for retry: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("postback not found or not in DLQ/FAILED status")
	}

	return nil
}

// BulkResetDLQ resets a page of DLQ postbacks back to PENDING. Returns count of updated rows.
func (r *PostbackRepository) BulkResetDLQ(limit int, offset int) (int64, error) {
	query := `
		UPDATE postback_outbox
		SET status = 'PENDING', attempt_count = 0, next_retry_at = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE id IN (
			SELECT id FROM postback_outbox
			WHERE status = 'DLQ'
			ORDER BY created_at ASC
			LIMIT $1 OFFSET $2
		)
	`

	result, err := r.db.Exec(query, limit, offset)
	if err != nil {
		return 0, fmt.Errorf("failed to bulk reset DLQ postbacks: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to check rows affected: %w", err)
	}

	return count, nil
}

// BulkResetDLQForTenant resets tenant-owned DLQ postbacks back to PENDING.
func (r *PostbackRepository) BulkResetDLQForTenant(tenantID string, limit int, offset int) (int64, error) {
	query := `
		UPDATE postback_outbox
		SET status = 'PENDING', attempt_count = 0, next_retry_at = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE id IN (
			SELECT id FROM postback_outbox
			WHERE tenant_id = $1::uuid AND status = 'DLQ'
			ORDER BY created_at ASC
			LIMIT $2 OFFSET $3
		)
	`

	result, err := r.db.Exec(query, tenantID, limit, offset)
	if err != nil {
		return 0, fmt.Errorf("failed to bulk reset tenant DLQ postbacks: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to check rows affected: %w", err)
	}

	return count, nil
}

// PostbackStats holds aggregate counts of postback outbox records by status.
type PostbackStats struct {
	Pending    int64 `json:"pending"`
	Processing int64 `json:"processing"`
	Success    int64 `json:"success"`
	Failed     int64 `json:"failed"`
	DLQ        int64 `json:"dlq"`
	Total      int64 `json:"total"`
}

// GetPostbackStats returns aggregate counts by status from the postback outbox.
func (r *PostbackRepository) GetPostbackStats() (*PostbackStats, error) {
	query := `SELECT status, COUNT(*) as count FROM postback_outbox GROUP BY status`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query postback stats: %w", err)
	}
	defer rows.Close()

	stats := &PostbackStats{}
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("failed to scan postback stats: %w", err)
		}
		stats.Total += count
		switch status {
		case "PENDING":
			stats.Pending = count
		case "PROCESSING":
			stats.Processing = count
		case "SUCCESS":
			stats.Success = count
		case "FAILED":
			stats.Failed = count
		case "DLQ":
			stats.DLQ = count
		}
	}

	return stats, nil
}

// GetPostbackStatsForTenant returns aggregate counts by status for one tenant.
func (r *PostbackRepository) GetPostbackStatsForTenant(tenantID string) (*PostbackStats, error) {
	query := `SELECT status, COUNT(*) as count FROM postback_outbox WHERE tenant_id = $1::uuid GROUP BY status`

	rows, err := r.db.Query(query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to query tenant postback stats: %w", err)
	}
	defer rows.Close()

	stats := &PostbackStats{}
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("failed to scan tenant postback stats: %w", err)
		}
		stats.Total += count
		switch status {
		case "PENDING":
			stats.Pending = count
		case "PROCESSING":
			stats.Processing = count
		case "SUCCESS":
			stats.Success = count
		case "FAILED":
			stats.Failed = count
		case "DLQ":
			stats.DLQ = count
		}
	}

	return stats, nil
}

// GetOutboxByTransactionID returns all postback outbox records for a transaction.
func (r *PostbackRepository) GetOutboxByTransactionID(transactionID uuid.UUID) ([]*domain.PostbackOutbox, error) {
	query := `
		SELECT id, tenant_id::text, channel_id::text, transaction_id, event,
		       provider, url_template_rendered, http_method, headers, body,
		       failure_reason, attempt_count, max_attempts, next_retry_at,
		       status, created_at, updated_at
		FROM postback_outbox
		WHERE transaction_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, transactionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query postback outbox: %w", err)
	}
	defer rows.Close()

	var outbox []*domain.PostbackOutbox
	for rows.Next() {
		pb, err := r.scanPostbackOutbox(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan postback outbox: %w", err)
		}
		outbox = append(outbox, pb)
	}
	return outbox, nil
}

// GetOutboxByTransactionIDForTenant returns tenant-owned postbacks for a transaction.
func (r *PostbackRepository) GetOutboxByTransactionIDForTenant(tenantID string, transactionID uuid.UUID) ([]*domain.PostbackOutbox, error) {
	query := `
		SELECT id, tenant_id::text, channel_id::text, transaction_id, event,
		       provider, url_template_rendered, http_method, headers, body,
		       failure_reason, attempt_count, max_attempts, next_retry_at,
		       status, created_at, updated_at
		FROM postback_outbox
		WHERE tenant_id = $1::uuid AND transaction_id = $2
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, tenantID, transactionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query tenant postback outbox: %w", err)
	}
	defer rows.Close()

	var outbox []*domain.PostbackOutbox
	for rows.Next() {
		pb, err := r.scanPostbackOutbox(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tenant postback outbox: %w", err)
		}
		outbox = append(outbox, pb)
	}
	return outbox, nil
}

// GetLatestStatusByTransactionIDs returns the latest postback status for each transaction ID.
// If a transaction has multiple postbacks, the most recently created one wins.
func (r *PostbackRepository) GetLatestStatusByTransactionIDs(txIDs []uuid.UUID) (map[uuid.UUID]domain.PostbackStatus, error) {
	if len(txIDs) == 0 {
		return nil, nil
	}

	query := `
		SELECT DISTINCT ON (transaction_id) transaction_id, status
		FROM postback_outbox
		WHERE transaction_id = ANY($1)
		ORDER BY transaction_id, created_at DESC
	`

	rows, err := r.db.Query(query, pq.Array(txIDs))
	if err != nil {
		return nil, fmt.Errorf("failed to query postback statuses: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]domain.PostbackStatus, len(txIDs))
	for rows.Next() {
		var txID uuid.UUID
		var status domain.PostbackStatus
		if err := rows.Scan(&txID, &status); err != nil {
			return nil, fmt.Errorf("failed to scan postback status: %w", err)
		}
		result[txID] = status
	}
	return result, nil
}

// GetAttemptsByOutboxIDs returns all attempts grouped by outbox_id.
func (r *PostbackRepository) GetAttemptsByOutboxIDs(outboxIDs []uuid.UUID) (map[uuid.UUID][]*domain.PostbackAttempt, error) {
	result := make(map[uuid.UUID][]*domain.PostbackAttempt)
	if len(outboxIDs) == 0 {
		return result, nil
	}

	// Convert UUIDs to strings for PostgreSQL array
	idStrings := make([]string, len(outboxIDs))
	for i, id := range outboxIDs {
		idStrings[i] = id.String()
	}

	query := `
		SELECT id, outbox_id, attempt_number, http_status, response_body,
		       error_message, duration_ms, created_at
		FROM postback_attempts
		WHERE outbox_id = ANY($1::uuid[])
		ORDER BY created_at ASC
	`

	rows, err := r.db.Query(query, pq.Array(idStrings))
	if err != nil {
		return nil, fmt.Errorf("failed to query postback attempts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var a domain.PostbackAttempt
		var httpStatus sql.NullInt64
		var responseBody, errorMessage sql.NullString
		var durationMs sql.NullInt64

		if err := rows.Scan(
			&a.ID, &a.OutboxID, &a.AttemptNumber, &httpStatus, &responseBody,
			&errorMessage, &durationMs, &a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan postback attempt: %w", err)
		}

		if httpStatus.Valid {
			v := int(httpStatus.Int64)
			a.HTTPStatus = &v
		}
		if responseBody.Valid {
			v := responseBody.String
			a.ResponseBody = &v
		}
		if errorMessage.Valid {
			v := errorMessage.String
			a.ErrorMessage = &v
		}
		if durationMs.Valid {
			v := int(durationMs.Int64)
			a.DurationMs = &v
		}

		result[a.OutboxID] = append(result[a.OutboxID], &a)
	}

	return result, nil
}
