package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/seidu626/subscription-manager/postback-dispatcher/internal/domain"
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

// GetPendingPostbacks retrieves postbacks ready for retry (legacy, non-concurrent safe)
func (r *PostbackRepository) GetPendingPostbacks(limit int) ([]*domain.PostbackOutbox, error) {
	query := `
		SELECT id, transaction_id, event, provider, url_template_rendered,
		       http_method, headers, body, attempt_count, max_attempts,
		       next_retry_at, status, created_at, updated_at
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

// ClaimPendingPostbacks atomically claims postbacks for processing using FOR UPDATE SKIP LOCKED
func (r *PostbackRepository) ClaimPendingPostbacks(limit int) ([]*domain.PostbackOutbox, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	selectQuery := `
		SELECT id, transaction_id, event, provider, url_template_rendered,
		       http_method, headers, body, attempt_count, max_attempts,
		       next_retry_at, status, created_at, updated_at
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

	idStrings := make([]string, len(ids))
	for i, id := range ids {
		idStrings[i] = id.String()
	}

	updateQuery := `
		UPDATE postback_outbox
		SET status = 'PROCESSING', updated_at = CURRENT_TIMESTAMP
		WHERE id = ANY($1::uuid[])
	`

	_, err = tx.Exec(updateQuery, pq.Array(idStrings))
	if err != nil {
		return nil, fmt.Errorf("failed to update postback status: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	r.logger.Info("Claimed postbacks for processing", zap.Int("count", len(postbacks)))

	return postbacks, nil
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

func (r *PostbackRepository) scanPostbackOutbox(rows *sql.Rows) (*domain.PostbackOutbox, error) {
	var pb domain.PostbackOutbox
	var body sql.NullString
	var nextRetryAt sql.NullTime

	err := rows.Scan(
		&pb.ID, &pb.TransactionID, &pb.Event, &pb.Provider,
		&pb.URLTemplateRendered, &pb.HTTPMethod, &pb.Headers, &body,
		&pb.AttemptCount, &pb.MaxAttempts, &nextRetryAt, &pb.Status,
		&pb.CreatedAt, &pb.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if body.Valid {
		pb.Body = &body.String
	}
	if nextRetryAt.Valid {
		pb.NextRetryAt = &nextRetryAt.Time
	}

	return &pb, nil
}

func (r *PostbackRepository) scanPostbackOutboxFromRows(rows *sql.Rows) (*domain.PostbackOutbox, error) {
	return r.scanPostbackOutbox(rows)
}
