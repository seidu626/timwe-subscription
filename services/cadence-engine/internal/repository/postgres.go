package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/seidu626/subscription-manager/cadence-engine/internal/domain"
	"go.uber.org/zap"
)

type CadenceRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

var ErrTenantNotFound = errors.New("tenant not found")

func NewCadenceRepository(db *sql.DB, logger *zap.Logger) *CadenceRepository {
	return &CadenceRepository{db: db, logger: logger}
}

func (r *CadenceRepository) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return r.db.BeginTx(ctx, nil)
}

func (r *CadenceRepository) TenantIDByKey(ctx context.Context, tenantKey string) (string, error) {
	tenantKey = strings.TrimSpace(tenantKey)
	if tenantKey == "" {
		return "", fmt.Errorf("tenant_key is required")
	}

	var tenantID string
	err := r.db.QueryRowContext(ctx, `
		SELECT id::text
		FROM tenants
		WHERE tenant_key = $1 AND status = 'ACTIVE'
		LIMIT 1
	`, tenantKey).Scan(&tenantID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrTenantNotFound
	}
	if err != nil {
		return "", err
	}
	return tenantID, nil
}

func (r *CadenceRepository) ClaimDueStatesTx(ctx context.Context, tx *sql.Tx, limit int) ([]domain.DueState, error) {
	query := `
		WITH due AS (
			SELECT sms.subscription_id, sms.tenant_id::text, sms.channel_id::text,
			       sms.series_id, sms.cursor_seq, sms.next_send_at
			FROM subscription_message_state sms
			JOIN subscriptions s ON s.id = sms.subscription_id
			JOIN product_message_series pms ON pms.id = sms.series_id
			WHERE sms.status = 'ACTIVE'
			  AND sms.next_send_at <= NOW()
			  AND (sms.inflight_until IS NULL OR sms.inflight_until < NOW())
			  AND s.status = 'active'
			  AND s.renewal_status = 'active'
			  AND sms.tenant_id = s.tenant_id
			  AND pms.tenant_id = s.tenant_id
			  AND (sms.channel_id IS NULL OR s.channel_id IS NULL OR sms.channel_id = s.channel_id)
			  AND (pms.channel_id IS NULL OR s.channel_id IS NULL OR pms.channel_id = s.channel_id)
			ORDER BY sms.next_send_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		SELECT subscription_id, tenant_id, channel_id, series_id, cursor_seq, next_send_at FROM due;
	`

	rows, err := tx.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.DueState
	for rows.Next() {
		var row domain.DueState
		var tenantID, channelID sql.NullString
		if err := rows.Scan(&row.SubscriptionID, &tenantID, &channelID, &row.SeriesID, &row.CursorSeq, &row.NextSendAt); err != nil {
			return nil, err
		}
		row.TenantID = nullStringPtr(tenantID)
		row.ChannelID = nullStringPtr(channelID)
		results = append(results, row)
	}
	return results, rows.Err()
}

func (r *CadenceRepository) GetSeriesTx(ctx context.Context, tx *sql.Tx, seriesID int64) (*domain.MessageSeries, error) {
	query := `
		SELECT id, tenant_id::text, channel_id::text, partner_role_id, product_id, name, mode, content_version, is_active, created_at
		FROM product_message_series
		WHERE id = $1
	`
	row := tx.QueryRowContext(ctx, query, seriesID)
	series := &domain.MessageSeries{}
	var tenantID, channelID sql.NullString
	if err := row.Scan(
		&series.ID,
		&tenantID,
		&channelID,
		&series.PartnerRoleID,
		&series.ProductID,
		&series.Name,
		&series.Mode,
		&series.ContentVersion,
		&series.IsActive,
		&series.CreatedAt,
	); err != nil {
		return nil, err
	}
	series.TenantID = nullStringPtr(tenantID)
	series.ChannelID = nullStringPtr(channelID)
	return series, nil
}

func (r *CadenceRepository) GetScheduleRuleTx(ctx context.Context, tx *sql.Tx, seriesID int64) (*domain.ScheduleRule, error) {
	query := `
		SELECT series_id, rule_kind, preferred_time, COALESCE(days_of_week, 0), COALESCE(n_days, 0),
		       send_start_time, send_end_time, timezone, max_per_day, catchup_mode
		FROM message_schedule_rules
		WHERE series_id = $1
	`
	row := tx.QueryRowContext(ctx, query, seriesID)
	rule := &domain.ScheduleRule{}
	if err := row.Scan(
		&rule.SeriesID,
		&rule.RuleKind,
		&rule.PreferredTime,
		&rule.DaysOfWeek,
		&rule.NDays,
		&rule.SendStartTime,
		&rule.SendEndTime,
		&rule.Timezone,
		&rule.MaxPerDay,
		&rule.CatchupMode,
	); err != nil {
		return nil, err
	}
	return rule, nil
}

func (r *CadenceRepository) GetSubscriptionTx(ctx context.Context, tx *sql.Tx, subscriptionID int64) (*domain.Subscription, error) {
	query := `
		SELECT id, tenant_id::text, channel_id::text, partner_role_id, product_id, user_identifier, user_identifier_type,
		       COALESCE(entry_channel, ''), start_date
		FROM subscriptions
		WHERE id = $1
	`
	row := tx.QueryRowContext(ctx, query, subscriptionID)
	sub := &domain.Subscription{}
	var tenantID, channelID sql.NullString
	if err := row.Scan(
		&sub.ID,
		&tenantID,
		&channelID,
		&sub.PartnerRoleID,
		&sub.ProductID,
		&sub.UserIdentifier,
		&sub.UserIdentifierType,
		&sub.EntryChannel,
		&sub.StartDate,
	); err != nil {
		return nil, err
	}
	sub.TenantID = nullStringPtr(tenantID)
	sub.ChannelID = nullStringPtr(channelID)
	return sub, nil
}

func (r *CadenceRepository) GetSequentialContentItemTx(ctx context.Context, tx *sql.Tx, seriesID int64, contentVersion int, seqNo int) (*domain.ContentItem, error) {
	query := `
		SELECT id, tenant_id::text, channel_id::text, series_id, content_version, seq_no, message_text, is_active, created_at
		FROM message_content_items
		WHERE series_id = $1 AND content_version = $2 AND seq_no = $3 AND is_active = TRUE
	`
	row := tx.QueryRowContext(ctx, query, seriesID, contentVersion, seqNo)
	item := &domain.ContentItem{}
	var tenantID, channelID sql.NullString
	if err := row.Scan(&item.ID, &tenantID, &channelID, &item.SeriesID, &item.ContentVersion, &item.SeqNo, &item.MessageText, &item.IsActive, &item.CreatedAt); err != nil {
		return nil, err
	}
	item.TenantID = nullStringPtr(tenantID)
	item.ChannelID = nullStringPtr(channelID)
	return item, nil
}

func (r *CadenceRepository) GetPoolContentItemTx(ctx context.Context, tx *sql.Tx, seriesID int64, contentVersion int) (*domain.ContentItem, error) {
	query := `
		SELECT id, tenant_id::text, channel_id::text, series_id, content_version, COALESCE(seq_no, 0), message_text, is_active, created_at
		FROM message_content_items
		WHERE series_id = $1 AND content_version = $2 AND is_active = TRUE
		ORDER BY RANDOM()
		LIMIT 1
	`
	row := tx.QueryRowContext(ctx, query, seriesID, contentVersion)
	item := &domain.ContentItem{}
	var tenantID, channelID sql.NullString
	if err := row.Scan(&item.ID, &tenantID, &channelID, &item.SeriesID, &item.ContentVersion, &item.SeqNo, &item.MessageText, &item.IsActive, &item.CreatedAt); err != nil {
		return nil, err
	}
	item.TenantID = nullStringPtr(tenantID)
	item.ChannelID = nullStringPtr(channelID)
	return item, nil
}

// ---- Admin repository helpers (series / rules / content) ----

func (r *CadenceRepository) ListSeries(ctx context.Context, tenantID, channelID string, partnerRoleID *int, productID *int, onlyActive *bool, limit int) ([]domain.MessageSeries, error) {
	tenantID = strings.TrimSpace(tenantID)
	channelID = strings.TrimSpace(channelID)
	if tenantID == "" {
		return nil, fmt.Errorf("tenant_id is required")
	}
	where := []string{"tenant_id::text = $1"}
	args := make([]any, 0, 6)
	args = append(args, tenantID)
	argN := 2

	if channelID != "" {
		where = append(where, fmt.Sprintf("channel_id::text = $%d", argN))
		args = append(args, channelID)
		argN++
	}

	if partnerRoleID != nil {
		where = append(where, fmt.Sprintf("partner_role_id = $%d", argN))
		args = append(args, *partnerRoleID)
		argN++
	}
	if productID != nil {
		where = append(where, fmt.Sprintf("product_id = $%d", argN))
		args = append(args, *productID)
		argN++
	}
	if onlyActive != nil {
		where = append(where, fmt.Sprintf("is_active = $%d", argN))
		args = append(args, *onlyActive)
		argN++
	}

	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	whereSQL := strings.Join(where, " AND ")
	query := fmt.Sprintf(`
		SELECT id, tenant_id::text, channel_id::text, partner_role_id, product_id, name, mode, content_version, is_active, created_at
		FROM product_message_series
		WHERE %s
		ORDER BY created_at DESC
		LIMIT %d
	`, whereSQL, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []domain.MessageSeries
	for rows.Next() {
		var s domain.MessageSeries
		var rowTenantID, rowChannelID sql.NullString
		if err := rows.Scan(&s.ID, &rowTenantID, &rowChannelID, &s.PartnerRoleID, &s.ProductID, &s.Name, &s.Mode, &s.ContentVersion, &s.IsActive, &s.CreatedAt); err != nil {
			return nil, err
		}
		s.TenantID = nullStringPtr(rowTenantID)
		s.ChannelID = nullStringPtr(rowChannelID)
		res = append(res, s)
	}
	return res, rows.Err()
}

func (r *CadenceRepository) UpsertSeries(ctx context.Context, tenantID, channelID string, partnerRoleID int, productID int, name string, mode string, contentVersion int, isActive bool) (*domain.MessageSeries, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, fmt.Errorf("tenant_id is required")
	}
	if contentVersion <= 0 {
		contentVersion = 1
	}
	if mode == "" {
		mode = "SEQUENTIAL"
	}
	query := `
		INSERT INTO product_message_series (tenant_id, channel_id, partner_role_id, product_id, name, mode, content_version, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (tenant_id, partner_role_id, product_id, name) WHERE tenant_id IS NOT NULL
		DO UPDATE SET mode = EXCLUDED.mode,
		              channel_id = EXCLUDED.channel_id,
		              content_version = EXCLUDED.content_version,
		              is_active = EXCLUDED.is_active
		RETURNING id, tenant_id::text, channel_id::text, partner_role_id, product_id, name, mode, content_version, is_active, created_at
	`
	row := r.db.QueryRowContext(ctx, query, tenantID, nullStringFromString(channelID), partnerRoleID, productID, name, mode, contentVersion, isActive)
	var s domain.MessageSeries
	var rowTenantID, rowChannelID sql.NullString
	if err := row.Scan(&s.ID, &rowTenantID, &rowChannelID, &s.PartnerRoleID, &s.ProductID, &s.Name, &s.Mode, &s.ContentVersion, &s.IsActive, &s.CreatedAt); err != nil {
		return nil, err
	}
	s.TenantID = nullStringPtr(rowTenantID)
	s.ChannelID = nullStringPtr(rowChannelID)
	return &s, nil
}

func (r *CadenceRepository) GetSeries(ctx context.Context, seriesID int64) (*domain.MessageSeries, error) {
	query := `
		SELECT id, tenant_id::text, channel_id::text, partner_role_id, product_id, name, mode, content_version, is_active, created_at
		FROM product_message_series
		WHERE id = $1
	`
	row := r.db.QueryRowContext(ctx, query, seriesID)
	var s domain.MessageSeries
	var tenantID, channelID sql.NullString
	if err := row.Scan(&s.ID, &tenantID, &channelID, &s.PartnerRoleID, &s.ProductID, &s.Name, &s.Mode, &s.ContentVersion, &s.IsActive, &s.CreatedAt); err != nil {
		return nil, err
	}
	s.TenantID = nullStringPtr(tenantID)
	s.ChannelID = nullStringPtr(channelID)
	return &s, nil
}

func (r *CadenceRepository) GetSeriesForTenant(ctx context.Context, tenantID string, seriesID int64) (*domain.MessageSeries, error) {
	query := `
		SELECT id, tenant_id::text, channel_id::text, partner_role_id, product_id, name, mode, content_version, is_active, created_at
		FROM product_message_series
		WHERE tenant_id::text = $1 AND id = $2
	`
	row := r.db.QueryRowContext(ctx, query, strings.TrimSpace(tenantID), seriesID)
	var s domain.MessageSeries
	var rowTenantID, rowChannelID sql.NullString
	if err := row.Scan(&s.ID, &rowTenantID, &rowChannelID, &s.PartnerRoleID, &s.ProductID, &s.Name, &s.Mode, &s.ContentVersion, &s.IsActive, &s.CreatedAt); err != nil {
		return nil, err
	}
	s.TenantID = nullStringPtr(rowTenantID)
	s.ChannelID = nullStringPtr(rowChannelID)
	return &s, nil
}

func (r *CadenceRepository) GetSeriesByKey(ctx context.Context, tenantID string, partnerRoleID int, productID int, name string) (*domain.MessageSeries, error) {
	query := `
		SELECT id, tenant_id::text, channel_id::text, partner_role_id, product_id, name, mode, content_version, is_active, created_at
		FROM product_message_series
		WHERE tenant_id::text = $1 AND partner_role_id = $2 AND product_id = $3 AND name = $4
	`
	row := r.db.QueryRowContext(ctx, query, strings.TrimSpace(tenantID), partnerRoleID, productID, name)
	var s domain.MessageSeries
	var rowTenantID, rowChannelID sql.NullString
	if err := row.Scan(&s.ID, &rowTenantID, &rowChannelID, &s.PartnerRoleID, &s.ProductID, &s.Name, &s.Mode, &s.ContentVersion, &s.IsActive, &s.CreatedAt); err != nil {
		return nil, err
	}
	s.TenantID = nullStringPtr(rowTenantID)
	s.ChannelID = nullStringPtr(rowChannelID)
	return &s, nil
}

func (r *CadenceRepository) PatchSeries(ctx context.Context, seriesID int64, isActive *bool, mode *string, contentVersion *int) error {
	sets := make([]string, 0, 3)
	args := make([]any, 0, 4)
	argN := 1

	if isActive != nil {
		sets = append(sets, fmt.Sprintf("is_active = $%d", argN))
		args = append(args, *isActive)
		argN++
	}
	if mode != nil && strings.TrimSpace(*mode) != "" {
		sets = append(sets, fmt.Sprintf("mode = $%d", argN))
		args = append(args, strings.TrimSpace(*mode))
		argN++
	}
	if contentVersion != nil && *contentVersion > 0 {
		sets = append(sets, fmt.Sprintf("content_version = $%d", argN))
		args = append(args, *contentVersion)
		argN++
	}
	if len(sets) == 0 {
		return nil
	}

	args = append(args, seriesID)
	query := fmt.Sprintf(`UPDATE product_message_series SET %s WHERE id = $%d`, strings.Join(sets, ", "), argN)
	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

func (r *CadenceRepository) GetScheduleRule(ctx context.Context, seriesID int64) (*domain.ScheduleRule, error) {
	query := `
		SELECT series_id, rule_kind, preferred_time, COALESCE(days_of_week, 0), COALESCE(n_days, 0),
		       send_start_time, send_end_time, timezone, max_per_day, catchup_mode
		FROM message_schedule_rules
		WHERE series_id = $1
	`
	row := r.db.QueryRowContext(ctx, query, seriesID)
	rule := &domain.ScheduleRule{}
	if err := row.Scan(
		&rule.SeriesID,
		&rule.RuleKind,
		&rule.PreferredTime,
		&rule.DaysOfWeek,
		&rule.NDays,
		&rule.SendStartTime,
		&rule.SendEndTime,
		&rule.Timezone,
		&rule.MaxPerDay,
		&rule.CatchupMode,
	); err != nil {
		return nil, err
	}
	return rule, nil
}

func (r *CadenceRepository) UpsertScheduleRule(ctx context.Context, rule domain.ScheduleRule) error {
	query := `
		INSERT INTO message_schedule_rules (
			series_id, rule_kind, preferred_time, days_of_week, n_days,
			send_start_time, send_end_time, timezone, max_per_day, catchup_mode
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (series_id) DO UPDATE SET
			rule_kind = EXCLUDED.rule_kind,
			preferred_time = EXCLUDED.preferred_time,
			days_of_week = EXCLUDED.days_of_week,
			n_days = EXCLUDED.n_days,
			send_start_time = EXCLUDED.send_start_time,
			send_end_time = EXCLUDED.send_end_time,
			timezone = EXCLUDED.timezone,
			max_per_day = EXCLUDED.max_per_day,
			catchup_mode = EXCLUDED.catchup_mode
	`
	_, err := r.db.ExecContext(ctx, query,
		rule.SeriesID,
		rule.RuleKind,
		rule.PreferredTime,
		sql.NullInt64{Int64: int64(rule.DaysOfWeek), Valid: rule.DaysOfWeek != 0},
		sql.NullInt64{Int64: int64(rule.NDays), Valid: rule.NDays != 0},
		rule.SendStartTime,
		rule.SendEndTime,
		rule.Timezone,
		rule.MaxPerDay,
		rule.CatchupMode,
	)
	return err
}

func (r *CadenceRepository) ListContentItems(ctx context.Context, seriesID int64, contentVersion *int, onlyActive *bool, limit int) ([]domain.ContentItem, error) {
	where := []string{"series_id = $1"}
	args := make([]any, 0, 4)
	args = append(args, seriesID)
	argN := 2

	if contentVersion != nil && *contentVersion > 0 {
		where = append(where, fmt.Sprintf("content_version = $%d", argN))
		args = append(args, *contentVersion)
		argN++
	}
	if onlyActive != nil {
		where = append(where, fmt.Sprintf("is_active = $%d", argN))
		args = append(args, *onlyActive)
		argN++
	}

	if limit <= 0 || limit > 2000 {
		limit = 500
	}
	query := fmt.Sprintf(`
		SELECT id, tenant_id::text, channel_id::text, series_id, content_version, COALESCE(seq_no, 0), message_text, is_active, created_at
		FROM message_content_items
		WHERE %s
		ORDER BY content_version DESC, COALESCE(seq_no, 0) ASC, created_at ASC
		LIMIT %d
	`, strings.Join(where, " AND "), limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []domain.ContentItem
	for rows.Next() {
		var c domain.ContentItem
		var tenantID, channelID sql.NullString
		if err := rows.Scan(&c.ID, &tenantID, &channelID, &c.SeriesID, &c.ContentVersion, &c.SeqNo, &c.MessageText, &c.IsActive, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.TenantID = nullStringPtr(tenantID)
		c.ChannelID = nullStringPtr(channelID)
		res = append(res, c)
	}
	return res, rows.Err()
}

func (r *CadenceRepository) UpsertContentItemTx(ctx context.Context, tx *sql.Tx, tenantID, channelID string, seriesID int64, contentVersion int, seqNo int, messageText string, isActive bool) (int64, error) {
	if contentVersion <= 0 {
		return 0, fmt.Errorf("content_version must be > 0")
	}
	if seqNo <= 0 {
		return 0, fmt.Errorf("seq_no must be > 0")
	}
	query := `
		INSERT INTO message_content_items (tenant_id, channel_id, series_id, content_version, seq_no, message_text, is_active)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (series_id, content_version, seq_no) DO UPDATE SET
			tenant_id = EXCLUDED.tenant_id,
			channel_id = EXCLUDED.channel_id,
			message_text = EXCLUDED.message_text,
			is_active = EXCLUDED.is_active
		RETURNING id
	`
	var id int64
	if err := tx.QueryRowContext(ctx, query, nullStringFromString(tenantID), nullStringFromString(channelID), seriesID, contentVersion, seqNo, messageText, isActive).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *CadenceRepository) DeactivateMissingContentItemsTx(ctx context.Context, tx *sql.Tx, seriesID int64, contentVersion int, keepSeqNos []int) (int64, error) {
	if contentVersion <= 0 {
		return 0, fmt.Errorf("content_version must be > 0")
	}
	if len(keepSeqNos) == 0 {
		res, err := tx.ExecContext(ctx, `
			UPDATE message_content_items
			SET is_active = FALSE
			WHERE series_id = $1 AND content_version = $2
		`, seriesID, contentVersion)
		if err != nil {
			return 0, err
		}
		n, _ := res.RowsAffected()
		return n, nil
	}

	res, err := tx.ExecContext(ctx, `
		UPDATE message_content_items
		SET is_active = FALSE
		WHERE series_id = $1
		  AND content_version = $2
		  AND (seq_no IS NULL OR NOT (seq_no = ANY($3)))
	`, seriesID, contentVersion, pq.Array(keepSeqNos))
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (r *CadenceRepository) InsertOutboxTx(ctx context.Context, tx *sql.Tx, job domain.OutboxJob) (bool, error) {
	query := `
		INSERT INTO message_outbox (
			job_id, idempotency_key, tenant_id, channel_id, subscription_id, series_id, content_item_id,
			planned_send_at, status, attempt
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (idempotency_key) DO NOTHING
	`
	res, err := tx.ExecContext(ctx, query,
		job.JobID,
		job.IdempotencyKey,
		nullStringPtrValue(job.TenantID),
		nullStringPtrValue(job.ChannelID),
		job.SubscriptionID,
		job.SeriesID,
		job.ContentItemID,
		job.PlannedSendAt,
		job.Status,
		job.Attempt,
	)
	if err != nil {
		return false, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (r *CadenceRepository) UpdateInflightTx(ctx context.Context, tx *sql.Tx, subscriptionID int64, seriesID int64, jobID *string, inflightUntil time.Time) error {
	query := `
		UPDATE subscription_message_state
		SET inflight_job_id = $1, inflight_until = $2
		WHERE subscription_id = $3 AND series_id = $4
	`
	_, err := tx.ExecContext(ctx, query, jobID, inflightUntil, subscriptionID, seriesID)
	return err
}

func (r *CadenceRepository) ClearInflightTx(ctx context.Context, tx *sql.Tx, subscriptionID int64, seriesID int64) error {
	query := `
		UPDATE subscription_message_state
		SET inflight_job_id = NULL, inflight_until = NULL
		WHERE subscription_id = $1 AND series_id = $2
	`
	_, err := tx.ExecContext(ctx, query, subscriptionID, seriesID)
	return err
}

func (r *CadenceRepository) StopStateTx(ctx context.Context, tx *sql.Tx, subscriptionID int64, seriesID int64, reason string) error {
	query := `
		UPDATE subscription_message_state
		SET status = 'STOPPED', inflight_job_id = NULL, inflight_until = NULL
		WHERE subscription_id = $1 AND series_id = $2
	`
	if _, err := tx.ExecContext(ctx, query, subscriptionID, seriesID); err != nil {
		return fmt.Errorf("stop state (%s): %w", reason, err)
	}
	return nil
}

func (r *CadenceRepository) AdvanceStateTx(ctx context.Context, tx *sql.Tx, subscriptionID int64, seriesID int64, nextSendAt time.Time, sentAt time.Time) error {
	query := `
		UPDATE subscription_message_state
		SET cursor_seq = cursor_seq + 1,
		    last_sent_at = $1,
		    next_send_at = $2,
		    inflight_job_id = NULL,
		    inflight_until = NULL
		WHERE subscription_id = $3 AND series_id = $4
	`
	_, err := tx.ExecContext(ctx, query, sentAt, nextSendAt, subscriptionID, seriesID)
	return err
}

func (r *CadenceRepository) ListMissingStates(ctx context.Context, limit int) ([]domain.MissingState, error) {
	query := `
		SELECT s.id, pms.tenant_id::text, pms.channel_id::text, pms.id, s.start_date,
		       msr.rule_kind, msr.preferred_time, COALESCE(msr.days_of_week, 0), COALESCE(msr.n_days, 0),
		       msr.send_start_time, msr.send_end_time, msr.timezone, msr.max_per_day, msr.catchup_mode
		FROM subscriptions s
		JOIN product_message_series pms
			ON pms.partner_role_id = s.partner_role_id
		   AND pms.product_id = s.product_id
		   AND pms.tenant_id = s.tenant_id
		   AND (pms.channel_id IS NULL OR s.channel_id IS NULL OR pms.channel_id = s.channel_id)
		   AND pms.is_active = TRUE
		JOIN message_schedule_rules msr
			ON msr.series_id = pms.id
		LEFT JOIN subscription_message_state sms
			ON sms.subscription_id = s.id AND sms.series_id = pms.id
		WHERE s.status = 'active'
		  AND s.renewal_status = 'active'
		  AND sms.subscription_id IS NULL
		LIMIT $1
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.MissingState
	for rows.Next() {
		var item domain.MissingState
		var tenantID, channelID sql.NullString
		if err := rows.Scan(
			&item.SubscriptionID,
			&tenantID,
			&channelID,
			&item.SeriesID,
			&item.StartDate,
			&item.Rule.RuleKind,
			&item.Rule.PreferredTime,
			&item.Rule.DaysOfWeek,
			&item.Rule.NDays,
			&item.Rule.SendStartTime,
			&item.Rule.SendEndTime,
			&item.Rule.Timezone,
			&item.Rule.MaxPerDay,
			&item.Rule.CatchupMode,
		); err != nil {
			return nil, err
		}
		item.TenantID = nullStringPtr(tenantID)
		item.ChannelID = nullStringPtr(channelID)
		item.Rule.SeriesID = item.SeriesID
		results = append(results, item)
	}
	return results, rows.Err()
}

func (r *CadenceRepository) InsertState(ctx context.Context, tenantID, channelID *string, subscriptionID int64, seriesID int64, nextSendAt time.Time) error {
	query := `
		INSERT INTO subscription_message_state (tenant_id, channel_id, subscription_id, series_id, status, cursor_seq, next_send_at)
		VALUES ($1, $2, $3, $4, 'ACTIVE', 1, $5)
		ON CONFLICT DO NOTHING
	`
	_, err := r.db.ExecContext(ctx, query, nullStringPtrValue(tenantID), nullStringPtrValue(channelID), subscriptionID, seriesID, nextSendAt)
	return err
}

func (r *CadenceRepository) ClaimSentOutboxTx(ctx context.Context, tx *sql.Tx, limit int) ([]domain.OutboxJob, error) {
	query := `
		SELECT job_id, tenant_id::text, channel_id::text, subscription_id, series_id, planned_send_at, sent_at
		FROM message_outbox
		WHERE status = 'SENT' AND processed_at IS NULL
		ORDER BY planned_send_at
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`
	rows, err := tx.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []domain.OutboxJob
	for rows.Next() {
		var job domain.OutboxJob
		var sentAt sql.NullTime
		var tenantID, channelID sql.NullString
		if err := rows.Scan(&job.JobID, &tenantID, &channelID, &job.SubscriptionID, &job.SeriesID, &job.PlannedSendAt, &sentAt); err != nil {
			return nil, err
		}
		job.TenantID = nullStringPtr(tenantID)
		job.ChannelID = nullStringPtr(channelID)
		if sentAt.Valid {
			job.SentAt = &sentAt.Time
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (r *CadenceRepository) MarkOutboxProcessedTx(ctx context.Context, tx *sql.Tx, jobID string) error {
	query := `
		UPDATE message_outbox
		SET processed_at = NOW()
		WHERE job_id = $1
	`
	_, err := tx.ExecContext(ctx, query, jobID)
	return err
}

func nullStringPtr(val sql.NullString) *string {
	if !val.Valid || strings.TrimSpace(val.String) == "" {
		return nil
	}
	s := strings.TrimSpace(val.String)
	return &s
}

func nullStringFromString(value string) sql.NullString {
	value = strings.TrimSpace(value)
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func nullStringPtrValue(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return nullStringFromString(*value)
}
