package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/seidu626/subscription-manager/notification/internal/domain"
	"go.uber.org/zap"
)

type OutboxRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

func NewOutboxRepository(db *sql.DB, logger *zap.Logger) *OutboxRepository {
	return &OutboxRepository{db: db, logger: logger}
}

func (r *OutboxRepository) ClaimPendingJobs(ctx context.Context, limit int) ([]domain.OutboxJob, error) {
	query := `
		WITH claimed AS (
			SELECT job_id
			FROM message_outbox
			WHERE status IN ('PENDING', 'RETRYING')
			  AND planned_send_at <= NOW()
			ORDER BY planned_send_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		),
		updated AS (
			UPDATE message_outbox mo
			SET status = 'RETRYING',
			    attempt = mo.attempt + 1,
			    last_error = NULL
			FROM claimed
			WHERE mo.job_id = claimed.job_id
			RETURNING mo.job_id, mo.subscription_id, mo.content_item_id, mo.attempt, mo.planned_send_at
		)
		SELECT u.job_id, mo.tenant_id::text, mo.channel_id::text, u.subscription_id, u.content_item_id, u.attempt, u.planned_send_at,
		       s.partner_role_id, s.product_id, s.user_identifier, COALESCE(s.entry_channel, ''),
		       ci.message_text
		FROM updated u
		JOIN message_outbox mo ON mo.job_id = u.job_id
		JOIN subscriptions s ON s.id = u.subscription_id
		JOIN message_content_items ci ON ci.id = u.content_item_id
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []domain.OutboxJob
	for rows.Next() {
		var job domain.OutboxJob
		var tenantID, channelID sql.NullString
		if err := rows.Scan(
			&job.JobID,
			&tenantID,
			&channelID,
			&job.SubscriptionID,
			&job.ContentItemID,
			&job.Attempt,
			&job.PlannedSendAt,
			&job.PartnerRoleID,
			&job.ProductID,
			&job.MSISDN,
			&job.EntryChannel,
			&job.MessageText,
		); err != nil {
			return nil, err
		}
		job.TenantID = outboxNullStringPtr(tenantID)
		job.ChannelID = outboxNullStringPtr(channelID)
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (r *OutboxRepository) MarkSent(ctx context.Context, jobID string) error {
	query := `
		UPDATE message_outbox
		SET status = 'SENT',
		    sent_at = NOW(),
		    last_error = NULL
		WHERE job_id = $1
	`
	_, err := r.db.ExecContext(ctx, query, jobID)
	return err
}

func outboxNullStringPtr(val sql.NullString) *string {
	if !val.Valid || val.String == "" {
		return nil
	}
	s := val.String
	return &s
}

func (r *OutboxRepository) ScheduleRetry(ctx context.Context, jobID string, nextTime time.Time, errMsg string) error {
	query := `
		UPDATE message_outbox
		SET status = 'RETRYING',
		    planned_send_at = $2,
		    last_error = $3
		WHERE job_id = $1
	`
	_, err := r.db.ExecContext(ctx, query, jobID, nextTime, errMsg)
	return err
}

func (r *OutboxRepository) MarkFailed(ctx context.Context, jobID string, errMsg string) error {
	query := `
		UPDATE message_outbox
		SET status = 'FAILED',
		    last_error = $2
		WHERE job_id = $1
	`
	_, err := r.db.ExecContext(ctx, query, jobID, errMsg)
	return err
}
