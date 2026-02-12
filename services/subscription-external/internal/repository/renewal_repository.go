package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"go.uber.org/zap"
)

// RenewalRepositoryInterface defines the interface for renewal operations
type RenewalRepositoryInterface interface {
	// Renewal cycles
	CreateRenewalCycle(ctx context.Context, cycle *domain.RenewalCycle) error
	UpdateRenewalCycle(ctx context.Context, cycle *domain.RenewalCycle) error
	GetRenewalCycle(ctx context.Context, msisdn, productID string) (*domain.RenewalCycle, error)
	GetRenewalCyclesByStatus(ctx context.Context, status string, limit int) ([]*domain.RenewalCycle, error)

	// Churn tracking
	CreateChurnRecord(ctx context.Context, record *domain.ChurnRecord) error
	GetChurnRecords(ctx context.Context, msisdn, productID string) ([]*domain.ChurnRecord, error)
	GetDailyChurnCount(ctx context.Context, date time.Time) (int, error)

	// Priority retry queue
	AddToPriorityRetryQueue(ctx context.Context, item *domain.PriorityRetryQueue) error
	GetPriorityRetryItems(ctx context.Context, limit int) ([]*domain.PriorityRetryQueue, error)
	UpdatePriorityRetryItem(ctx context.Context, item *domain.PriorityRetryQueue) error
	RemoveFromPriorityRetryQueue(ctx context.Context, id int64) error

	// Subscription renewal operations
	GetSubscriptionsNeedingRenewal(ctx context.Context, daysThreshold, limit int) ([]*domain.SubscriptionWithRenewalInfo, error)
	IncrementRenewalAttempt(ctx context.Context, msisdn, productID string) error
	UpdateSubscriptionRenewalStatus(ctx context.Context, msisdn, productID, status string) error
	GetLastSuccessfulPayment(ctx context.Context, msisdn, productID string) (*time.Time, error)
	GetRenewalAttemptsCount(ctx context.Context, msisdn, productID string, since time.Time) (int, error)
	GetLastRenewalAttempt(ctx context.Context, msisdn, productID string) (*time.Time, error)

	// Statistics and monitoring
	GetRenewalStatistics(ctx context.Context, hoursBack int) (*domain.RenewalMetrics, error)
	GetChurnCandidates(ctx context.Context, maxDays, maxAttempts, limit int) ([]*domain.SubscriptionWithRenewalInfo, error)

	// Utility functions
	ChurnSubscription(ctx context.Context, msisdn, productID, reason string) error
	GetSubscriptionWithRenewalInfo(ctx context.Context, msisdn, productID string) (*domain.SubscriptionWithRenewalInfo, error)
}

// RenewalRepository implements RenewalRepositoryInterface
type RenewalRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewRenewalRepository creates a new renewal repository
func NewRenewalRepository(db *sql.DB, logger *zap.Logger) RenewalRepositoryInterface {
	return &RenewalRepository{
		db:     db,
		logger: logger,
	}
}

// CreateRenewalCycle creates a new renewal cycle record
func (r *RenewalRepository) CreateRenewalCycle(ctx context.Context, cycle *domain.RenewalCycle) error {
	query := `
		INSERT INTO renewal_cycles (
			subscription_id, msisdn, product_id, cycle_number,
			opt_out_time, opt_out_status, opt_out_response,
			opt_in_time, opt_in_status, opt_in_response,
			billing_status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id`

	now := time.Now()
	cycle.CreatedAt = now
	cycle.UpdatedAt = now

	r.logger.Debug("Creating renewal cycle",
		zap.String("msisdn", cycle.MSISDN),
		zap.String("product_id", cycle.ProductID),
		zap.Int64("subscription_id", cycle.SubscriptionID),
		zap.Int("cycle_number", cycle.CycleNumber))

	err := r.db.QueryRowContext(ctx, query,
		cycle.SubscriptionID, cycle.MSISDN, cycle.ProductID, cycle.CycleNumber,
		cycle.OptOutTime, cycle.OptOutStatus, cycle.OptOutResponse,
		cycle.OptInTime, cycle.OptInStatus, cycle.OptInResponse,
		cycle.BillingStatus, cycle.CreatedAt, cycle.UpdatedAt,
	).Scan(&cycle.ID)

	if err != nil {
		r.logger.Error("Failed to create renewal cycle",
			zap.String("msisdn", cycle.MSISDN),
			zap.String("product_id", cycle.ProductID),
			zap.Error(err))
		return fmt.Errorf("failed to create renewal cycle: %w", err)
	}

	r.logger.Info("Successfully created renewal cycle",
		zap.String("msisdn", cycle.MSISDN),
		zap.String("product_id", cycle.ProductID),
		zap.Int64("cycle_id", cycle.ID))

	return nil
}

// UpdateRenewalCycle updates an existing renewal cycle
func (r *RenewalRepository) UpdateRenewalCycle(ctx context.Context, cycle *domain.RenewalCycle) error {
	// Validate cycle ID
	if cycle.ID <= 0 {
		return fmt.Errorf("invalid renewal cycle ID: %d", cycle.ID)
	}

	query := `
		UPDATE renewal_cycles SET
			opt_out_time = $1, opt_out_status = $2, opt_out_response = $3,
			opt_in_time = $4, opt_in_status = $5, opt_in_response = $6,
			billing_status = $7, updated_at = $8
		WHERE id = $9`

	cycle.UpdatedAt = time.Now()

	result, err := r.db.ExecContext(ctx, query,
		cycle.OptOutTime, cycle.OptOutStatus, cycle.OptOutResponse,
		cycle.OptInTime, cycle.OptInStatus, cycle.OptInResponse,
		cycle.BillingStatus, cycle.UpdatedAt, cycle.ID,
	)

	if err != nil {
		r.logger.Error("Failed to update renewal cycle",
			zap.Int64("cycle_id", cycle.ID),
			zap.Error(err))
		return fmt.Errorf("failed to update renewal cycle: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("no renewal cycle found with id %d", cycle.ID)
	}

	return nil
}

// GetRenewalCycle retrieves a renewal cycle by MSISDN and product ID
func (r *RenewalRepository) GetRenewalCycle(ctx context.Context, msisdn, productID string) (*domain.RenewalCycle, error) {
	query := `
		SELECT id, subscription_id, msisdn, product_id, cycle_number,
		       opt_out_time, opt_out_status, opt_out_response,
		       opt_in_time, opt_in_status, opt_in_response,
		       billing_status, created_at, updated_at
		FROM renewal_cycles
		WHERE msisdn = $1 AND product_id = $2
		ORDER BY created_at DESC
		LIMIT 1`

	cycle := &domain.RenewalCycle{}
	err := r.db.QueryRowContext(ctx, query, msisdn, productID).Scan(
		&cycle.ID, &cycle.SubscriptionID, &cycle.MSISDN, &cycle.ProductID, &cycle.CycleNumber,
		&cycle.OptOutTime, &cycle.OptOutStatus, &cycle.OptOutResponse,
		&cycle.OptInTime, &cycle.OptInStatus, &cycle.OptInResponse,
		&cycle.BillingStatus, &cycle.CreatedAt, &cycle.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get renewal cycle",
			zap.String("msisdn", msisdn),
			zap.String("product_id", productID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get renewal cycle: %w", err)
	}

	return cycle, nil
}

// GetRenewalCyclesByStatus retrieves renewal cycles by status
func (r *RenewalRepository) GetRenewalCyclesByStatus(ctx context.Context, status string, limit int) ([]*domain.RenewalCycle, error) {
	query := `
		SELECT id, subscription_id, msisdn, product_id, cycle_number,
		       opt_out_time, opt_out_status, opt_out_response,
		       opt_in_time, opt_in_status, opt_in_response,
		       billing_status, created_at, updated_at
		FROM renewal_cycles
		WHERE billing_status = $1
		ORDER BY created_at ASC
		LIMIT $2`

	rows, err := r.db.QueryContext(ctx, query, status, limit)
	if err != nil {
		r.logger.Error("Failed to get renewal cycles by status",
			zap.String("status", status),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get renewal cycles: %w", err)
	}
	defer rows.Close()

	var cycles []*domain.RenewalCycle
	for rows.Next() {
		cycle := &domain.RenewalCycle{}
		err := rows.Scan(
			&cycle.ID, &cycle.SubscriptionID, &cycle.MSISDN, &cycle.ProductID, &cycle.CycleNumber,
			&cycle.OptOutTime, &cycle.OptOutStatus, &cycle.OptOutResponse,
			&cycle.OptInTime, &cycle.OptInStatus, &cycle.OptInResponse,
			&cycle.BillingStatus, &cycle.CreatedAt, &cycle.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan renewal cycle row", zap.Error(err))
			continue
		}
		cycles = append(cycles, cycle)
	}

	return cycles, nil
}

// CreateChurnRecord creates a new churn record
func (r *RenewalRepository) CreateChurnRecord(ctx context.Context, record *domain.ChurnRecord) error {
	query := `
		INSERT INTO churn_tracking (
			subscription_id, msisdn, product_id, last_payment_date,
			hours_without_payment, renewal_attempts, churn_reason, churned_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`

	record.CreatedAt = time.Now()

	err := r.db.QueryRowContext(ctx, query,
		record.SubscriptionID, record.MSISDN, record.ProductID, record.LastPaymentDate,
		record.HoursWithoutPayment, record.TotalRenewalAttempts, record.Reason,
		record.ChurnedAt, record.CreatedAt,
	).Scan(&record.ID)

	if err != nil {
		r.logger.Error("Failed to create churn record",
			zap.String("msisdn", record.MSISDN),
			zap.Error(err))
		return fmt.Errorf("failed to create churn record: %w", err)
	}

	return nil
}

// GetDailyChurnCount gets the count of churned subscriptions for a specific date
func (r *RenewalRepository) GetDailyChurnCount(ctx context.Context, date time.Time) (int, error) {
	query := `
		SELECT COUNT(*) FROM churn_tracking
		WHERE DATE(churned_at) = DATE($1)`

	var count int
	err := r.db.QueryRowContext(ctx, query, date).Scan(&count)
	if err != nil {
		r.logger.Error("Failed to get daily churn count",
			zap.Time("date", date),
			zap.Error(err))
		return 0, fmt.Errorf("failed to get daily churn count: %w", err)
	}

	return count, nil
}

// AddToPriorityRetryQueue adds an item to the priority retry queue
func (r *RenewalRepository) AddToPriorityRetryQueue(ctx context.Context, item *domain.PriorityRetryQueue) error {
	query := `
		INSERT INTO priority_retry_queue (
			msisdn, product_id, reason, priority, retry_count,
			next_retry_at, last_attempt_at, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id`

	now := time.Now()
	item.CreatedAt = now
	item.UpdatedAt = now

	err := r.db.QueryRowContext(ctx, query,
		item.MSISDN, item.ProductID, item.Reason, item.Priority, item.RetryCount,
		item.NextRetryAt, item.LastAttemptAt, item.Status, item.CreatedAt, item.UpdatedAt,
	).Scan(&item.ID)

	if err != nil {
		r.logger.Error("Failed to add to priority retry queue",
			zap.String("msisdn", item.MSISDN),
			zap.Error(err))
		return fmt.Errorf("failed to add to priority retry queue: %w", err)
	}

	return nil
}

// GetPriorityRetryItems gets items from the priority retry queue that are ready for retry
func (r *RenewalRepository) GetPriorityRetryItems(ctx context.Context, limit int) ([]*domain.PriorityRetryQueue, error) {
	query := `
		SELECT id, msisdn, product_id, reason, priority, retry_count,
		       next_retry_at, last_attempt_at, status, created_at, updated_at
		FROM priority_retry_queue
		WHERE status = 'pending' AND next_retry_at <= NOW()
		ORDER BY priority DESC, created_at ASC
		LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		r.logger.Error("Failed to get priority retry items", zap.Error(err))
		return nil, fmt.Errorf("failed to get priority retry items: %w", err)
	}
	defer rows.Close()

	var items []*domain.PriorityRetryQueue
	for rows.Next() {
		item := &domain.PriorityRetryQueue{}
		err := rows.Scan(
			&item.ID, &item.MSISDN, &item.ProductID, &item.Reason, &item.Priority,
			&item.RetryCount, &item.NextRetryAt, &item.LastAttemptAt, &item.Status,
			&item.CreatedAt, &item.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan priority retry item row", zap.Error(err))
			continue
		}
		items = append(items, item)
	}

	return items, nil
}

// GetSubscriptionsNeedingRenewal gets subscriptions that need renewal
func (r *RenewalRepository) GetSubscriptionsNeedingRenewal(ctx context.Context, daysThreshold, limit int) ([]*domain.SubscriptionWithRenewalInfo, error) {
	// Use the database function for better performance
	query := `SELECT * FROM get_subscriptions_needing_renewal($1, $2)`

	rows, err := r.db.QueryContext(ctx, query, daysThreshold, limit)
	if err != nil {
		r.logger.Error("Failed to get subscriptions needing renewal", zap.Error(err))
		return nil, fmt.Errorf("failed to get subscriptions needing renewal: %w", err)
	}
	defer rows.Close()

	var subscriptions []*domain.SubscriptionWithRenewalInfo
	for rows.Next() {
		var subID int64
		var msisdn, productID string
		var lastPayment *time.Time
		var daysSincePayment int

		err := rows.Scan(&subID, &msisdn, &productID, &lastPayment, &daysSincePayment)
		if err != nil {
			r.logger.Error("Failed to scan subscription row", zap.Error(err))
			continue
		}

		// Get full subscription details
		sub, err := r.GetSubscriptionWithRenewalInfo(ctx, msisdn, productID)
		if err != nil {
			r.logger.Error("Failed to get subscription details",
				zap.String("msisdn", msisdn),
				zap.Error(err))
			continue
		}

		subscriptions = append(subscriptions, sub)
	}

	return subscriptions, nil
}

// IncrementRenewalAttempt increments the renewal attempt count for a subscription
func (r *RenewalRepository) IncrementRenewalAttempt(ctx context.Context, msisdn, productID string) error {
	query := `SELECT increment_renewal_attempt($1, $2)`

	_, err := r.db.ExecContext(ctx, query, msisdn, productID)
	if err != nil {
		r.logger.Error("Failed to increment renewal attempt",
			zap.String("msisdn", msisdn),
			zap.String("product_id", productID),
			zap.Error(err))
		return fmt.Errorf("failed to increment renewal attempt: %w", err)
	}

	return nil
}

// UpdateSubscriptionRenewalStatus updates the renewal status of a subscription
func (r *RenewalRepository) UpdateSubscriptionRenewalStatus(ctx context.Context, msisdn, productID, status string) error {
	query := `
		UPDATE subscriptions 
		SET renewal_status = $1, updated_at = NOW()
		WHERE user_identifier = $2 AND product_id = $3`

	result, err := r.db.ExecContext(ctx, query, status, msisdn, productID)
	if err != nil {
		r.logger.Error("Failed to update subscription renewal status",
			zap.String("msisdn", msisdn),
			zap.String("status", status),
			zap.Error(err))
		return fmt.Errorf("failed to update subscription renewal status: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		r.logger.Warn("No subscription found to update renewal status",
			zap.String("msisdn", msisdn),
			zap.String("product_id", productID))
	}

	return nil
}

// GetLastSuccessfulPayment gets the last successful payment date for a subscription
func (r *RenewalRepository) GetLastSuccessfulPayment(ctx context.Context, msisdn, productID string) (*time.Time, error) {
	query := `
		SELECT last_successful_payment 
		FROM subscriptions 
		WHERE user_identifier = $1 AND product_id = $2`

	var lastPayment *time.Time
	err := r.db.QueryRowContext(ctx, query, msisdn, productID).Scan(&lastPayment)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get last successful payment",
			zap.String("msisdn", msisdn),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get last successful payment: %w", err)
	}

	return lastPayment, nil
}

// GetRenewalAttemptsCount gets the count of renewal attempts for a subscription since a given time
func (r *RenewalRepository) GetRenewalAttemptsCount(ctx context.Context, msisdn, productID string, since time.Time) (int, error) {
	query := `
		SELECT COUNT(*) FROM renewal_cycles
		WHERE msisdn = $1 AND product_id = $2 AND created_at >= $3`

	var count int
	err := r.db.QueryRowContext(ctx, query, msisdn, productID, since).Scan(&count)
	if err != nil {
		r.logger.Error("Failed to get renewal attempts count",
			zap.String("msisdn", msisdn),
			zap.Error(err))
		return 0, fmt.Errorf("failed to get renewal attempts count: %w", err)
	}

	return count, nil
}

// GetLastRenewalAttempt gets the last renewal attempt time for a subscription
func (r *RenewalRepository) GetLastRenewalAttempt(ctx context.Context, msisdn, productID string) (*time.Time, error) {
	query := `
		SELECT last_renewal_attempt 
		FROM subscriptions 
		WHERE user_identifier = $1 AND product_id = $2`

	var lastAttempt *time.Time
	err := r.db.QueryRowContext(ctx, query, msisdn, productID).Scan(&lastAttempt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get last renewal attempt",
			zap.String("msisdn", msisdn),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get last renewal attempt: %w", err)
	}

	return lastAttempt, nil
}

// GetRenewalStatistics gets renewal statistics for monitoring
func (r *RenewalRepository) GetRenewalStatistics(ctx context.Context, hoursBack int) (*domain.RenewalMetrics, error) {
	query := `SELECT * FROM get_renewal_statistics($1)`

	rows, err := r.db.QueryContext(ctx, query, hoursBack)
	if err != nil {
		r.logger.Error("Failed to get renewal statistics", zap.Error(err))
		return nil, fmt.Errorf("failed to get renewal statistics: %w", err)
	}
	defer rows.Close()

	metrics := &domain.RenewalMetrics{
		LastRunTime: time.Now(),
	}

	if rows.Next() {
		var totalCycles, successfulOptouts, successfulOptins, failedOptouts, failedOptins, pendingBilling int
		var successRate float64

		err := rows.Scan(&totalCycles, &successfulOptouts, &successfulOptins, &failedOptouts, &failedOptins, &pendingBilling, &successRate)
		if err != nil {
			r.logger.Error("Failed to scan renewal statistics", zap.Error(err))
			return metrics, nil
		}

		metrics.TotalProcessed = int64(totalCycles)
		metrics.SuccessfulRenewals = int64(successfulOptouts + successfulOptins)
		metrics.FailedRenewals = int64(failedOptouts + failedOptins)
		metrics.SuccessRate = successRate
	}

	return metrics, nil
}

// GetChurnCandidates gets subscriptions that are candidates for churning
func (r *RenewalRepository) GetChurnCandidates(ctx context.Context, maxDays, maxAttempts, limit int) ([]*domain.SubscriptionWithRenewalInfo, error) {
	query := `SELECT * FROM get_churn_candidates($1, $2, $3)`

	rows, err := r.db.QueryContext(ctx, query, maxDays, maxAttempts, limit)
	if err != nil {
		r.logger.Error("Failed to get churn candidates", zap.Error(err))
		return nil, fmt.Errorf("failed to get churn candidates: %w", err)
	}
	defer rows.Close()

	var candidates []*domain.SubscriptionWithRenewalInfo
	for rows.Next() {
		var subID int64
		var msisdn, productID string
		var hoursWithoutPayment, renewalAttempts int
		var lastRenewalAttempt *time.Time

		err := rows.Scan(&subID, &msisdn, &productID, &hoursWithoutPayment, &renewalAttempts, &lastRenewalAttempt)
		if err != nil {
			r.logger.Error("Failed to scan churn candidate row", zap.Error(err))
			continue
		}

		// Get full subscription details
		sub, err := r.GetSubscriptionWithRenewalInfo(ctx, msisdn, productID)
		if err != nil {
			r.logger.Error("Failed to get subscription details for churn candidate",
				zap.String("msisdn", msisdn),
				zap.Error(err))
			continue
		}

		candidates = append(candidates, sub)
	}

	return candidates, nil
}

// ChurnSubscription marks a subscription as churned
func (r *RenewalRepository) ChurnSubscription(ctx context.Context, msisdn, productID, reason string) error {
	query := `SELECT churn_subscription($1, $2, $3)`

	_, err := r.db.ExecContext(ctx, query, msisdn, productID, reason)
	if err != nil {
		r.logger.Error("Failed to churn subscription",
			zap.String("msisdn", msisdn),
			zap.String("product_id", productID),
			zap.String("reason", reason),
			zap.Error(err))
		return fmt.Errorf("failed to churn subscription: %w", err)
	}

	return nil
}

// GetSubscriptionWithRenewalInfo gets a subscription with renewal information
func (r *RenewalRepository) GetSubscriptionWithRenewalInfo(ctx context.Context, msisdn, productID string) (*domain.SubscriptionWithRenewalInfo, error) {
	query := `
		SELECT id, partner_role_id, user_identifier, user_identifier_type, product_id,
		       mcc, mnc, entry_channel, large_account, sub_keyword, tracking_id,
		       client_ip, campaign_url, status, cancel_reason, cancel_source,
		       created_at, start_date, end_date, transaction_auth_code,
		       COALESCE(renewal_status, 'active') as renewal_status,
		       last_renewal_attempt, COALESCE(total_renewal_attempts, 0) as total_renewal_attempts,
		       last_successful_payment, COALESCE(consecutive_payment_failures, 0) as consecutive_payment_failures
		FROM subscriptions
		WHERE user_identifier = $1 AND product_id = $2`

	sub := &domain.SubscriptionWithRenewalInfo{
		Subscription: &domain.Subscription{},
	}

	err := r.db.QueryRowContext(ctx, query, msisdn, productID).Scan(
		&sub.Id, &sub.PartnerRoleId, &sub.UserIdentifier, &sub.UserIdentifierType, &sub.ProductId,
		&sub.Mcc, &sub.Mnc, &sub.EntryChannel, &sub.LargeAccount, &sub.SubKeyword, &sub.TrackingId,
		&sub.ClientIp, &sub.CampaignUrl, &sub.Status, &sub.CancelReason, &sub.CancelSource,
		&sub.CreatedAt, &sub.StartDate, &sub.EndDate, &sub.TransactionAuthCode,
		&sub.RenewalStatus, &sub.LastRenewalAttempt, &sub.TotalRenewalAttempts,
		&sub.LastSuccessfulPayment, &sub.ConsecutivePaymentFailures,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get subscription with renewal info",
			zap.String("msisdn", msisdn),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get subscription with renewal info: %w", err)
	}

	return sub, nil
}

// UpdatePriorityRetryItem updates a priority retry queue item
func (r *RenewalRepository) UpdatePriorityRetryItem(ctx context.Context, item *domain.PriorityRetryQueue) error {
	query := `
		UPDATE priority_retry_queue SET
			retry_count = $1, next_retry_at = $2, last_attempt_at = $3,
			status = $4, updated_at = $5
		WHERE id = $6`

	item.UpdatedAt = time.Now()

	result, err := r.db.ExecContext(ctx, query,
		item.RetryCount, item.NextRetryAt, item.LastAttemptAt,
		item.Status, item.UpdatedAt, item.ID,
	)

	if err != nil {
		r.logger.Error("Failed to update priority retry item",
			zap.Int64("item_id", item.ID),
			zap.Error(err))
		return fmt.Errorf("failed to update priority retry item: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("no priority retry item found with id %d", item.ID)
	}

	return nil
}

// RemoveFromPriorityRetryQueue removes an item from the priority retry queue
func (r *RenewalRepository) RemoveFromPriorityRetryQueue(ctx context.Context, id int64) error {
	query := `DELETE FROM priority_retry_queue WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to remove from priority retry queue",
			zap.Int64("item_id", id),
			zap.Error(err))
		return fmt.Errorf("failed to remove from priority retry queue: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("no priority retry item found with id %d", id)
	}

	return nil
}

// GetChurnRecords gets churn records for a subscription
func (r *RenewalRepository) GetChurnRecords(ctx context.Context, msisdn, productID string) ([]*domain.ChurnRecord, error) {
	query := `
		SELECT id, subscription_id, msisdn, product_id, reason, churned_at,
		       last_payment_date, hours_without_payment, renewal_attempts, created_at
		FROM churn_tracking
		WHERE msisdn = $1 AND product_id = $2
		ORDER BY churned_at DESC`

	rows, err := r.db.QueryContext(ctx, query, msisdn, productID)
	if err != nil {
		r.logger.Error("Failed to get churn records",
			zap.String("msisdn", msisdn),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get churn records: %w", err)
	}
	defer rows.Close()

	var records []*domain.ChurnRecord
	for rows.Next() {
		record := &domain.ChurnRecord{}
		err := rows.Scan(
			&record.ID, &record.SubscriptionID, &record.MSISDN, &record.ProductID,
			&record.Reason, &record.ChurnedAt, &record.LastPaymentDate,
			&record.HoursWithoutPayment, &record.TotalRenewalAttempts, &record.CreatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan churn record row", zap.Error(err))
			continue
		}
		records = append(records, record)
	}

	return records, nil
}
