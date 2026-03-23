package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lib/pq"
	cached "github.com/seidu626/subscription-manager/common/cache"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"go.uber.org/zap"
)

type SubscriptionRepository struct {
	db     *sql.DB
	logger *zap.Logger
	redis  cached.RedisClient
	ctx    context.Context

	adminActionSchemaOnce sync.Once
	adminActionSchemaErr  error
}

type notificationRow struct {
	ID              int
	PartnerRole     int
	ExternalTxID    string
	ProductID       int
	PricepointID    int
	MCC             string
	MNC             string
	MSISDN          string
	LargeAccount    string
	TransactionUUID string
	EntryChannel    string
	MessageType     string
	Message         string
	MnoDeliveryCode sql.NullString
	Tags            []string
	CreatedAt       time.Time
	Type            string
}

// NewSubscriptionRepository creates a new repository with proper context handling
func NewSubscriptionRepository(db *sql.DB, logger *zap.Logger, client cached.RedisClient) *SubscriptionRepository {
	return &SubscriptionRepository{
		db:     db,
		logger: logger,
		redis:  client,
		ctx:    context.Background(),
	}
}

// getContextWithTimeout returns a context with timeout for database operations
func (r *SubscriptionRepository) getContextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = 30 * time.Second // Default timeout
	}
	return context.WithTimeout(r.ctx, timeout)
}

// GenerateCacheKey generates a unique cache key for query filters
func (r *SubscriptionRepository) GenerateCacheKey(startDate, endDate time.Time, productId int, shortcode, userIdentifier, entryChannel string, page, pageSize int) string {
	return fmt.Sprintf("notifications:%s:%s:%d:%s:%s:%s:%d:%d", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"), productId, shortcode, userIdentifier, entryChannel, page, pageSize)
}

// FetchActiveMsisdnsMissingSomeProducts finds active MSISDNs that are missing at least one of the specified product IDs with optional offset/limit windowing.
func (r *SubscriptionRepository) FetchActiveMsisdnsMissingSomeProducts(productIds []int, offset int, limit int) ([]string, error) {
	if len(productIds) == 0 {
		return []string{}, nil
	}
	// Normalize negative values
	if offset < 0 {
		offset = 0
	}

	base := `
		-- Find MSISDNs that are active but missing at least one of the specified products
		-- This query returns users who are missing SOME of the specified products
		-- Using a simpler approach that avoids type casting issues
		SELECT DISTINCT s.user_identifier
		FROM subscriptions s
		WHERE (s.status IS NULL OR LOWER(s.status) = 'active')
		AND (
			-- Check if user is missing any of the specified products
			SELECT COUNT(*) < $1
			FROM (
				SELECT DISTINCT product_id
				FROM subscriptions s2
				WHERE s2.user_identifier = s.user_identifier
				AND s2.product_id = ANY($2)
				AND (s2.status IS NULL OR LOWER(s2.status) = 'active')
			) user_products
		)
		ORDER BY s.id`

	// Build final query and args based on provided offset/limit
	query := base

	// Add total product count and product IDs array as parameters
	args := []interface{}{len(productIds), pq.Array(productIds)}

	// Apply OFFSET if provided
	if offset > 0 {
		query += "\n\t\tOFFSET $" + strconv.Itoa(len(args)+1)
		args = append(args, offset)
	}
	// Apply LIMIT only when limit > 0
	if limit > 0 {
		if offset > 0 {
			query += " LIMIT $" + strconv.Itoa(len(args)+1)
			args = append(args, limit)
		} else {
			query += "\n\t\tLIMIT $" + strconv.Itoa(len(args)+1)
			args = append(args, limit)
		}
	}

	// Log the query and parameters for debugging
	log.Printf("FetchActiveMsisdnsMissingSomeProducts - Query: %s", query)
	log.Printf("FetchActiveMsisdnsMissingSomeProducts - Args: %+v", args)
	log.Printf("FetchActiveMsisdnsMissingSomeProducts - Product IDs: %+v", productIds)
	log.Printf("FetchActiveMsisdnsMissingSomeProducts - Offset: %d, Limit: %d", offset, limit)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch active msisdns missing some products: %w", err)
	}
	defer rows.Close()

	var msisdns []string
	for rows.Next() {
		var msisdn string
		if err := rows.Scan(&msisdn); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}
		msisdns = append(msisdns, msisdn)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	log.Printf("FetchActiveMsisdnsMissingSomeProducts - Result count: %d", len(msisdns))
	return msisdns, nil
}

// FetchActiveMsisdnsWithProductsWindow finds active MSISDNs that have any of the specified product IDs with optional offset/limit windowing.
func (r *SubscriptionRepository) FetchActiveMsisdnsWithProductsWindow(productIds []int, offset int, limit int) ([]string, error) {
	if len(productIds) == 0 {
		return []string{}, nil
	}
	if offset < 0 {
		offset = 0
	}

	base := `
		SELECT DISTINCT s.user_identifier, s.id
		FROM subscriptions s
		WHERE (s.status IS NULL OR LOWER(s.status) = 'active')
		AND s.product_id = ANY($1)
		ORDER BY s.id`

	query := base
	args := []interface{}{pq.Array(productIds)}

	if offset > 0 {
		query += "\n\t\tOFFSET $2"
		args = append(args, offset)
	}
	if limit > 0 {
		if offset > 0 {
			query += " LIMIT $3"
			args = append(args, limit)
		} else {
			query += "\n\t\tLIMIT $2"
			args = append(args, limit)
		}
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch active msisdns with products: %w", err)
	}
	defer rows.Close()

	var msisdns []string
	for rows.Next() {
		var msisdn string
		var id int
		if err := rows.Scan(&msisdn, &id); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}
		msisdns = append(msisdns, msisdn)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return msisdns, nil
}

// CreateSubscription inserts a new subscription record into the database.
func (r *SubscriptionRepository) CreateSubscription(request *domain.SubscriptionRequest) error {
	query := `
        INSERT INTO subscriptions (
            partner_role_id, user_identifier, user_identifier_type, product_id, mcc, mnc, entry_channel,
            large_account, sub_keyword, tracking_id, client_ip, campaign_url, transaction_id, start_date
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7,
            $8, $9, $10, $11, $12, $13, NOW()
        )
        ON CONFLICT (partner_role_id, user_identifier, product_id) DO UPDATE SET
            user_identifier_type = EXCLUDED.user_identifier_type,
            mcc = EXCLUDED.mcc,
            mnc = EXCLUDED.mnc,
            entry_channel = EXCLUDED.entry_channel,
            large_account = EXCLUDED.large_account,
            sub_keyword = EXCLUDED.sub_keyword,
            tracking_id = EXCLUDED.tracking_id,
            client_ip = EXCLUDED.client_ip,
            campaign_url = EXCLUDED.campaign_url,
            transaction_id = EXCLUDED.transaction_id,
            start_date = COALESCE(subscriptions.start_date, EXCLUDED.start_date),
			status = EXCLUDED.status
    `
	_, err := r.db.Exec(query, request.PartnerRoleId, request.UserIdentifier, request.UserIdentifierType, request.ProductId, request.Mcc, request.Mnc, request.EntryChannel, request.LargeAccount, request.SubKeyword, request.TrackingId, request.ClientIp, request.CampaignUrl, request.TransactionId)
	if err != nil {
		return fmt.Errorf("failed to create subscription: %w", err)
	}
	return nil
}

// CheckSubscriptionExists checks if a subscription already exists for the given MSISDN and product
func (r *SubscriptionRepository) CheckSubscriptionExists(msisdn string, productId int) (bool, error) {
	query := `
        SELECT COUNT(*) 
        FROM subscriptions 
        WHERE user_identifier = $1 AND product_id = $2 AND status = 'active'
    `
	var count int
	err := r.db.QueryRow(query, msisdn, productId).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check subscription existence: %w", err)
	}
	return count > 0, nil
}

// HasAnySubscription checks if any subscriptions exist for the given MSISDN regardless of product
// This is used for INVALID_MSISDN cleanup where we want to remove ALL subscriptions for an invalid MSISDN
func (r *SubscriptionRepository) HasAnySubscription(msisdn string) (bool, error) {
	query := `
        SELECT COUNT(*) 
        FROM subscriptions 
        WHERE user_identifier = $1
    `
	var count int
	err := r.db.QueryRow(query, msisdn).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check for any subscriptions: %w", err)
	}
	return count > 0, nil
}

// CheckRenewalNotificationExists checks if a renewal notification was sent to the MSISDN in the current month
func (r *SubscriptionRepository) CheckRenewalNotificationExists(msisdn string, productId int) (bool, error) {
	// Get the first day of current month
	now := time.Now()
	firstDayOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	query := `
        SELECT COUNT(*) 
        FROM notifications 
        WHERE msisdn = $1 
        AND product_id = $2 
        AND (type = 'USER_RENEWED' OR type = 'CHARGE')
        AND created_at >= $3
    `
	var count int
	err := r.db.QueryRow(query, msisdn, productId, firstDayOfMonth).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check renewal notification existence: %w", err)
	}
	return count > 0, nil
}

func (r *SubscriptionRepository) CreateNotification(notification *domain.NotificationRequest) error {
	query := `
        INSERT INTO notifications (
            partner_role, external_tx_id, product_id, pricepoint_id, mcc, mnc, msisdn, large_account, transaction_uuid,
            entry_channel, message_type, message, mno_delivery_code, tags, type
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9,
            $10, $11, $12, $13, $14, $15
        )
    `
	_, err := r.db.Exec(
		query,
		notification.PartnerRole,
		notification.ExternalTxID,
		notification.ProductID,
		notification.PricepointID,
		notification.MCC,
		notification.MNC,
		notification.MSISDN,
		notification.LargeAccount,
		notification.TransactionUUID,
		notification.EntryChannel,
		notification.MessageType,
		notification.Message,
		notification.MnoDeliveryCode,
		pq.Array(notification.Tags),
		notification.Type,
	)
	if err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}
	return nil
}

// CreateInvalidMSISDNLog creates a log entry for invalid MSISDN responses
func (r *SubscriptionRepository) CreateInvalidMSISDNLog(log *domain.InvalidMSISDNLog) error {
	ctx, cancel := r.getContextWithTimeout(30 * time.Second)
	defer cancel()

	query := `
        INSERT INTO invalid_msisdn_logs (
            msisdn, product_id, pricepoint_id, partner_role_id, entry_channel,
            request_id, response_code, response_message, subscription_result,
            subscription_error, external_tx_id, transaction_id
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
        )
        ON CONFLICT (msisdn) DO UPDATE SET
            product_id = EXCLUDED.product_id,
            pricepoint_id = EXCLUDED.pricepoint_id,
            partner_role_id = EXCLUDED.partner_role_id,
            entry_channel = EXCLUDED.entry_channel,
            request_id = EXCLUDED.request_id,
            response_code = EXCLUDED.response_code,
            response_message = EXCLUDED.response_message,
            subscription_result = EXCLUDED.subscription_result,
            subscription_error = EXCLUDED.subscription_error,
            external_tx_id = EXCLUDED.external_tx_id,
            transaction_id = EXCLUDED.transaction_id
        `

	_, err := r.db.ExecContext(ctx, query,
		log.MSISDN,
		log.ProductID,
		log.PricepointID,
		log.PartnerRoleID,
		log.EntryChannel,
		log.RequestID,
		log.ResponseCode,
		log.ResponseMessage,
		log.SubscriptionResult,
		log.SubscriptionError,
		log.ExternalTxID,
		log.TransactionID,
	)
	if err != nil {
		return fmt.Errorf("failed to save invalid MSISDN log: %w", err)
	}

	r.logger.Info("Invalid MSISDN log saved successfully",
		zap.String("msisdn", log.MSISDN),
		zap.String("responseCode", log.ResponseCode),
		zap.String("subscriptionResult", log.SubscriptionResult))

	return nil
}

// FetchNotificationsWindow returns notifications of a given type created since `since`,
// after a specific id (cursor), limited by `limit`. Results ordered by id asc.
func (r *SubscriptionRepository) FetchNotificationsWindow(ntype string, since time.Time, afterId int64, limit int) ([]NotificationRow, error) {
	if limit <= 0 {
		limit = 1000
	}
	args := []interface{}{ntype, since}
	query := `
		SELECT id, msisdn, product_id, entry_channel, created_at, type
		FROM notifications
		WHERE type = $1 AND created_at >= $2`
	if afterId > 0 {
		query += " AND id > $3"
		args = append(args, afterId)
	}
	query += " ORDER BY id ASC LIMIT $" + fmt.Sprint(len(args)+1)
	args = append(args, limit)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch notifications window: %w", err)
	}
	defer rows.Close()

	var out []NotificationRow
	for rows.Next() {
		var nr NotificationRow
		if err := rows.Scan(&nr.ID, &nr.MSISDN, &nr.ProductID, &nr.EntryChannel, &nr.CreatedAt, &nr.Type); err != nil {
			return nil, err
		}
		out = append(out, nr)
	}
	return out, rows.Err()
}

// FetchChargeSuccessNotifications returns CHARGE notifications with transaction_uuid,
// paginated by id cursor, ordered ascending.
func (r *SubscriptionRepository) FetchChargeSuccessNotifications(since time.Time, afterID int64, limit int) ([]ChargeSuccessNotificationRow, error) {
	if limit <= 0 {
		limit = 1000
	}
	args := []interface{}{"CHARGE", since}
	query := `
		SELECT id, msisdn, product_id, COALESCE(transaction_uuid, ''), created_at
		FROM notifications
		WHERE type = $1 AND created_at >= $2`
	if afterID > 0 {
		query += " AND id > $3"
		args = append(args, afterID)
	}
	query += " ORDER BY id ASC LIMIT $" + fmt.Sprint(len(args)+1)
	args = append(args, limit)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch charge success notifications: %w", err)
	}
	defer rows.Close()

	var out []ChargeSuccessNotificationRow
	for rows.Next() {
		var nr ChargeSuccessNotificationRow
		if err := rows.Scan(&nr.ID, &nr.MSISDN, &nr.ProductID, &nr.TransactionUUID, &nr.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, nr)
	}
	return out, rows.Err()
}

// FetchUnprocessedOptoutNotifications fetches USER_OPTOUT notifications that haven't been processed yet
// This prevents re-processing of already handled opt-outs by checking against a processing tracking table
func (r *SubscriptionRepository) FetchUnprocessedOptoutNotifications(since time.Time, afterId int64, limit int) ([]NotificationRow, error) {
	if limit <= 0 {
		limit = 1000
	}

	// Enhanced query to avoid duplicates by checking if the opt-out has already been processed
	// We use a LEFT JOIN with a processing tracking table to identify unprocessed notifications
	args := []interface{}{since}
	query := `
		WITH processed_optouts AS (
			-- Check if this opt-out has already been processed by looking at subscription status changes
			SELECT DISTINCT
				n.msisdn,
				n.product_id,
				n.created_at
			FROM notifications n
			INNER JOIN subscriptions s ON n.msisdn = s.user_identifier 
				AND n.product_id = s.product_id
			WHERE n.type = 'USER_OPTOUT'
				AND s.status = 'inactive'
				AND s.start_date >= n.created_at
				AND s.start_date <= n.created_at + INTERVAL '1 hour'
		)
		SELECT n.id, n.msisdn, n.product_id, n.entry_channel, n.created_at, n.type
		FROM notifications n
		LEFT JOIN processed_optouts p ON n.msisdn = p.msisdn 
			AND n.product_id = p.product_id 
			AND n.created_at = p.created_at
		WHERE n.type = 'USER_OPTOUT' 
			AND n.created_at >= $1
			AND p.msisdn IS NULL` // Only unprocessed opt-outs

	if afterId > 0 {
		query += " AND n.id > $2"
		args = append(args, afterId)
	}

	query += " ORDER BY n.id ASC LIMIT $" + fmt.Sprint(len(args)+1)
	args = append(args, limit)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch unprocessed optout notifications: %w", err)
	}
	defer rows.Close()

	var out []NotificationRow
	for rows.Next() {
		var nr NotificationRow
		if err := rows.Scan(&nr.ID, &nr.MSISDN, &nr.ProductID, &nr.EntryChannel, &nr.CreatedAt, &nr.Type); err != nil {
			return nil, err
		}
		out = append(out, nr)
	}
	return out, rows.Err()
}

// GetSubscriptionByMSISDNAndProduct retrieves a subscription by MSISDN and product ID
func (r *SubscriptionRepository) GetSubscriptionByMSISDNAndProduct(msisdn string, productID int) (*domain.Subscription, error) {
	query := `
		SELECT 
			id, partner_role_id, user_identifier, user_identifier_type, product_id,
			mcc, mnc, entry_channel, large_account, sub_keyword, tracking_id,
			client_ip, campaign_url, transaction_auth_code, status, cancel_reason,
			cancel_source, start_date, end_date, created_at
		FROM subscriptions
		WHERE user_identifier = $1 AND product_id = $2
		LIMIT 1`

	var sub domain.Subscription
	err := r.db.QueryRow(query, msisdn, productID).Scan(
		&sub.Id, &sub.PartnerRoleId, &sub.UserIdentifier, &sub.UserIdentifierType, &sub.ProductId,
		&sub.Mcc, &sub.Mnc, &sub.EntryChannel, &sub.LargeAccount, &sub.SubKeyword, &sub.TrackingId,
		&sub.ClientIp, &sub.CampaignUrl, &sub.TransactionAuthCode, &sub.Status, &sub.CancelReason,
		&sub.CancelSource, &sub.StartDate, &sub.EndDate, &sub.CreatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("subscription not found for msisdn %s and product %d", msisdn, productID)
		}
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}

	return &sub, nil
}

// GetLastOptinNotificationTime retrieves the timestamp of the last USER_OPTIN notification
// for a specific MSISDN and product ID
func (r *SubscriptionRepository) GetLastOptinNotificationTime(msisdn string, productID int) (*time.Time, error) {
	query := `
		SELECT MAX(created_at) as last_optin_time
		FROM notifications
		WHERE msisdn = $1 AND product_id = $2 AND type = 'USER_OPTIN'`

	var lastOptinTime *time.Time
	err := r.db.QueryRow(query, msisdn, productID).Scan(&lastOptinTime)

	if err != nil {
		if err == sql.ErrNoRows {
			// No opt-in notifications found
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get last opt-in time: %w", err)
	}

	return lastOptinTime, nil
}

// FetchGhostSubscriptions fetches subscriptions that exist in the database but have no opt-in notifications
// These are "ghost" subscriptions that were created but never received confirmation from TIMWE
func (r *SubscriptionRepository) FetchGhostSubscriptions(cutoff time.Time, afterId int64, limit int) ([]NotificationRow, error) {
	if limit <= 0 {
		limit = 1000
	}

	// Query to find subscriptions that exist but have no opt-in notifications
	args := []interface{}{cutoff}
	query := `
		WITH subscriptions_without_optins AS (
			SELECT DISTINCT
				s.id,
				s.user_identifier as msisdn,
				s.product_id,
				s.entry_channel,
				s.created_at,
				'SUBSCRIPTION' as type
			FROM subscriptions s
			LEFT JOIN (
				SELECT DISTINCT
					n.msisdn,
					n.product_id
				FROM notifications n
				WHERE n.type = 'USER_OPTIN'
					AND n.created_at >= $1
			) optins ON s.user_identifier = optins.msisdn 
				AND s.product_id = optins.product_id
			WHERE (s.status = 'active' OR s.status IS NULL)
				AND s.created_at < $1  -- Subscription was created before cutoff
				AND optins.msisdn IS NULL  -- No opt-in notification found
		)
		SELECT id, msisdn, product_id, entry_channel, created_at, type
		FROM subscriptions_without_optins`

	if afterId > 0 {
		query += " WHERE id > $2"
		args = append(args, afterId)
	}

	query += " ORDER BY id ASC LIMIT $" + fmt.Sprint(len(args)+1)
	args = append(args, limit)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ghost subscriptions: %w", err)
	}
	defer rows.Close()

	var out []NotificationRow
	for rows.Next() {
		var nr NotificationRow
		if err := rows.Scan(&nr.ID, &nr.MSISDN, &nr.ProductID, &nr.EntryChannel, &nr.CreatedAt, &nr.Type); err != nil {
			return nil, err
		}
		out = append(out, nr)
	}
	return out, rows.Err()
}

// UpsertSubscriptionStatus ensures a row exists and updates status
func (r *SubscriptionRepository) UpsertSubscriptionStatus(msisdn string, productId int, status string) error {
	if msisdn == "" || productId <= 0 {
		return errors.New("invalid args")
	}
	q := `
		INSERT INTO subscriptions (partner_role_id, user_identifier, user_identifier_type, product_id, status, start_date)
		VALUES (COALESCE((SELECT partner_role_id FROM subscriptions WHERE user_identifier=$1 AND product_id=$2 LIMIT 1), 0), $1,'MSISDN',$2,$3, COALESCE((SELECT start_date FROM subscriptions WHERE user_identifier=$1 AND product_id=$2 LIMIT 1), NOW()))
		ON CONFLICT (partner_role_id, user_identifier, product_id)
		DO UPDATE SET status = EXCLUDED.status`
	_, err := r.db.Exec(q, msisdn, productId, status)
	return err
}

// FetchSubscriptionsNeedingRenewal finds subscriptions that haven't had successful charges recently
// and need renewal processing. This replaces the old RENEWAL notification-based approach.
func (r *SubscriptionRepository) FetchSubscriptionsNeedingRenewal(cutoff time.Time, afterId int64, limit int) ([]NotificationRow, error) {
	if limit <= 0 {
		limit = 1000
	}

	args := []interface{}{cutoff}
	query := `
		WITH subscriptions_needing_renewal AS (
			SELECT DISTINCT
				s.id,
				s.user_identifier as msisdn,
				s.product_id,
				s.entry_channel,
				s.created_at,
				'SUBSCRIPTION' as type
			FROM subscriptions s
			LEFT JOIN (
				SELECT 
					n.msisdn,
					n.product_id,
					MAX(n.created_at) as last_charge
				FROM notifications n
				WHERE n.type IN ('CHARGE', 'USER_RENEWED')
				GROUP BY n.msisdn, n.product_id
			) charges ON s.user_identifier = charges.msisdn AND s.product_id = charges.product_id
			WHERE (s.status = 'active' OR s.status IS NULL)
			  AND s.created_at < NOW() - INTERVAL '1 day'
			  AND (charges.last_charge IS NULL OR charges.last_charge < $1)
		)
		SELECT id, msisdn, product_id, entry_channel, created_at, type
		FROM subscriptions_needing_renewal`

	if afterId > 0 {
		query += " WHERE id > $2"
		args = append(args, afterId)
	}

	query += " ORDER BY id ASC LIMIT $" + fmt.Sprint(len(args)+1)
	args = append(args, limit)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch subscriptions needing renewal: %w", err)
	}
	defer rows.Close()

	var out []NotificationRow
	for rows.Next() {
		var nr NotificationRow
		if err := rows.Scan(&nr.ID, &nr.MSISDN, &nr.ProductID, &nr.EntryChannel, &nr.CreatedAt, &nr.Type); err != nil {
			return nil, err
		}
		out = append(out, nr)
	}
	return out, rows.Err()
}

// FindAndRemoveSubscription finds and removes/deactivates a subscription for the given MSISDN and product
func (r *SubscriptionRepository) FindAndRemoveSubscription(msisdn string, productId int) error {
	// First check if subscription exists
	query := `
        SELECT COUNT(*) 
        FROM subscriptions 
        WHERE user_identifier = $1 AND product_id = $2 AND status = 'active'
    `
	var count int
	err := r.db.QueryRow(query, msisdn, productId).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check subscription existence for removal: %w", err)
	}

	// If no active subscription found, nothing to remove
	if count == 0 {
		r.logger.Info("No active subscription found to remove",
			zap.String("msisdn", msisdn),
			zap.Int("productId", productId))
		return nil
	}

	// Update subscription status to 'inactive' and set end_date
	updateQuery := `
        UPDATE subscriptions 
        SET status = 'inactive', end_date = NOW() 
        WHERE user_identifier = $1 AND product_id = $2 AND status = 'active'
    `
	result, err := r.db.Exec(updateQuery, msisdn, productId)
	if err != nil {
		return fmt.Errorf("failed to deactivate subscription: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		r.logger.Warn("No subscription rows were updated during deactivation",
			zap.String("msisdn", msisdn),
			zap.Int("productId", productId))
	} else {
		r.logger.Info("Successfully deactivated subscription",
			zap.String("msisdn", msisdn),
			zap.Int("productId", productId),
			zap.Int64("rowsAffected", rowsAffected))
	}

	return nil
}

// DeleteSubscriptionRecord completely removes a subscription record for the given MSISDN
func (r *SubscriptionRepository) DeleteSubscriptionRecord(msisdn string) error {
	ctx, cancel := r.getContextWithTimeout(30 * time.Second)
	defer cancel()

	// First check if subscription exists
	query := `
        SELECT COUNT(*) 
        FROM subscriptions 
        WHERE user_identifier = $1
    `
	var count int
	err := r.db.QueryRowContext(ctx, query, msisdn).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check subscription existence for deletion: %w", err)
	}

	// If no subscription found, nothing to delete
	if count == 0 {
		r.logger.Info("No subscription found to delete",
			zap.String("msisdn", msisdn))
		return nil
	}

	// Delete the subscription record completely
	deleteQuery := `
        DELETE FROM subscriptions 
        WHERE user_identifier = $1
    `
	result, err := r.db.ExecContext(ctx, deleteQuery, msisdn)
	if err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		r.logger.Warn("No subscription rows were deleted",
			zap.String("msisdn", msisdn))
	} else {
		r.logger.Info("Successfully deleted subscription record",
			zap.String("msisdn", msisdn),
			zap.Int64("rowsAffected", rowsAffected))
	}

	return nil
}

// GetTotalSubscriptionsCount returns the total count of active subscriptions
func (r *SubscriptionRepository) GetTotalSubscriptionsCount() (int64, error) {
	query := `
		SELECT COUNT(*) 
		FROM subscriptions 
		WHERE status IS NULL OR LOWER(status) = 'active'`

	var count int64
	err := r.db.QueryRowContext(r.ctx, query).Scan(&count)
	if err != nil {
		r.logger.Error("Failed to get total subscriptions count", zap.Error(err))
		return 0, fmt.Errorf("failed to get total subscriptions count: %w", err)
	}

	return count, nil
}

// GetDB returns the database connection (for use by trackers and other components)
func (r *SubscriptionRepository) GetDB() *sql.DB {
	return r.db
}

func (r *SubscriptionRepository) ensureAdminActionAuditSchema() error {
	r.adminActionSchemaOnce.Do(func() {
		ddl := []string{
			`CREATE TABLE IF NOT EXISTS admin_subscription_action_logs (
				id UUID PRIMARY KEY,
				operation VARCHAR(20) NOT NULL,
				msisdn VARCHAR(50) NOT NULL,
				product_id INTEGER NOT NULL,
				partner_role_id INTEGER NOT NULL,
				external_tx_id VARCHAR(255),
				admin_request_id VARCHAR(255),
				request_method VARCHAR(10) NOT NULL,
				request_url TEXT NOT NULL,
				request_headers JSONB NOT NULL DEFAULT '{}'::jsonb,
				request_body JSONB,
				request_timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
				response_status_code INTEGER NOT NULL DEFAULT 0,
				response_headers JSONB NOT NULL DEFAULT '{}'::jsonb,
				response_body JSONB,
				response_timestamp TIMESTAMP WITH TIME ZONE,
				service_result JSONB,
				error_payload JSONB,
				duration_ms BIGINT NOT NULL DEFAULT 0,
				created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS idx_admin_subscription_action_logs_operation_created_at
				ON admin_subscription_action_logs (operation, created_at DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_admin_subscription_action_logs_msisdn_created_at
				ON admin_subscription_action_logs (msisdn, created_at DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_admin_subscription_action_logs_external_tx_id
				ON admin_subscription_action_logs (external_tx_id)`,
			`CREATE INDEX IF NOT EXISTS idx_admin_subscription_action_logs_admin_request_id
				ON admin_subscription_action_logs (admin_request_id)`,
		}

		for _, q := range ddl {
			if _, err := r.db.ExecContext(r.ctx, q); err != nil {
				r.adminActionSchemaErr = fmt.Errorf("failed to apply admin action audit schema: %w", err)
				return
			}
		}
	})

	return r.adminActionSchemaErr
}

func marshalJSONOrNull(value interface{}) ([]byte, error) {
	if value == nil {
		return []byte("null"), nil
	}
	out, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return []byte("null"), nil
	}
	return out, nil
}

// CreateAdminActionLog persists a full request/response audit record for admin-triggered TIMWE calls.
func (r *SubscriptionRepository) CreateAdminActionLog(logEntry *domain.AdminSubscriptionActionLog) error {
	if err := r.ensureAdminActionAuditSchema(); err != nil {
		return err
	}

	requestHeadersJSON, err := marshalJSONOrNull(logEntry.RequestHeaders)
	if err != nil {
		return fmt.Errorf("failed to marshal request headers: %w", err)
	}
	requestBodyJSON, err := marshalJSONOrNull(logEntry.RequestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}
	responseHeadersJSON, err := marshalJSONOrNull(logEntry.ResponseHeaders)
	if err != nil {
		return fmt.Errorf("failed to marshal response headers: %w", err)
	}
	responseBodyJSON, err := marshalJSONOrNull(logEntry.ResponseBody)
	if err != nil {
		return fmt.Errorf("failed to marshal response body: %w", err)
	}
	serviceResultJSON, err := marshalJSONOrNull(logEntry.ServiceResult)
	if err != nil {
		return fmt.Errorf("failed to marshal service result: %w", err)
	}
	errorPayloadJSON, err := marshalJSONOrNull(logEntry.ErrorPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal error payload: %w", err)
	}

	query := `
		INSERT INTO admin_subscription_action_logs (
			id,
			operation,
			msisdn,
			product_id,
			partner_role_id,
			external_tx_id,
			admin_request_id,
			request_method,
			request_url,
			request_headers,
			request_body,
			request_timestamp,
			response_status_code,
			response_headers,
			response_body,
			response_timestamp,
			service_result,
			error_payload,
			duration_ms,
			created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb,
			$11::jsonb, $12, $13, $14::jsonb, $15::jsonb, $16, $17::jsonb,
			$18::jsonb, $19, $20
		)
	`

	_, err = r.db.ExecContext(
		r.ctx,
		query,
		logEntry.ID,
		string(logEntry.Operation),
		logEntry.MSISDN,
		logEntry.ProductID,
		logEntry.PartnerRoleID,
		logEntry.ExternalTxID,
		logEntry.AdminRequestID,
		logEntry.RequestMethod,
		logEntry.RequestURL,
		string(requestHeadersJSON),
		string(requestBodyJSON),
		logEntry.RequestTimestamp,
		logEntry.ResponseStatusCode,
		string(responseHeadersJSON),
		string(responseBodyJSON),
		logEntry.ResponseTimestamp,
		string(serviceResultJSON),
		string(errorPayloadJSON),
		logEntry.DurationMs,
		logEntry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create admin action log: %w", err)
	}
	return nil
}

// ListAdminActionLogs returns paginated audit summaries for admin-triggered TIMWE calls.
func (r *SubscriptionRepository) ListAdminActionLogs(filter domain.AdminActionLogFilter) ([]domain.AdminActionLogSummary, int64, error) {
	if err := r.ensureAdminActionAuditSchema(); err != nil {
		return nil, 0, err
	}

	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}

	var (
		conditions []string
		args       []interface{}
	)
	if filter.Operation != "" {
		args = append(args, string(filter.Operation))
		conditions = append(conditions, fmt.Sprintf("operation = $%d", len(args)))
	}
	if filter.MSISDN != "" {
		args = append(args, filter.MSISDN)
		conditions = append(conditions, fmt.Sprintf("msisdn = $%d", len(args)))
	}
	if filter.ExternalTxID != "" {
		args = append(args, filter.ExternalTxID)
		conditions = append(conditions, fmt.Sprintf("external_tx_id = $%d", len(args)))
	}
	if filter.AdminRequestID != "" {
		args = append(args, filter.AdminRequestID)
		conditions = append(conditions, fmt.Sprintf("admin_request_id = $%d", len(args)))
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := "SELECT COUNT(*) FROM admin_subscription_action_logs" + whereClause
	var totalCount int64
	if err := r.db.QueryRowContext(r.ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("failed to count admin action logs: %w", err)
	}

	offset := (filter.Page - 1) * filter.PageSize
	args = append(args, filter.PageSize, offset)
	limitPos := len(args) - 1
	offsetPos := len(args)

	query := `
		SELECT
			id,
			operation,
			msisdn,
			product_id,
			partner_role_id,
			external_tx_id,
			admin_request_id,
			response_status_code,
			duration_ms,
			created_at,
			error_payload
		FROM admin_subscription_action_logs
	` + whereClause + fmt.Sprintf(`
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, limitPos, offsetPos)

	rows, err := r.db.QueryContext(r.ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list admin action logs: %w", err)
	}
	defer rows.Close()

	summaries := make([]domain.AdminActionLogSummary, 0)
	for rows.Next() {
		var (
			summary      domain.AdminActionLogSummary
			operation    string
			errorPayload []byte
		)
		if err := rows.Scan(
			&summary.ID,
			&operation,
			&summary.MSISDN,
			&summary.ProductID,
			&summary.PartnerRoleID,
			&summary.ExternalTxID,
			&summary.AdminRequestID,
			&summary.ResponseStatusCode,
			&summary.DurationMs,
			&summary.CreatedAt,
			&errorPayload,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan admin action summary: %w", err)
		}
		summary.Operation = domain.AdminActionOperation(operation)

		logEntry := domain.AdminSubscriptionActionLog{ErrorPayload: json.RawMessage(errorPayload)}
		summary.HasError = logEntry.HasError()
		summary.ErrorMessage = logEntry.ErrorMessage()
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed iterating admin action summaries: %w", err)
	}

	return summaries, totalCount, nil
}

// GetAdminActionLogByID returns a full audit record by its ID.
func (r *SubscriptionRepository) GetAdminActionLogByID(id string) (*domain.AdminSubscriptionActionLog, error) {
	if err := r.ensureAdminActionAuditSchema(); err != nil {
		return nil, err
	}

	query := `
		SELECT
			id,
			operation,
			msisdn,
			product_id,
			partner_role_id,
			external_tx_id,
			admin_request_id,
			request_method,
			request_url,
			request_headers,
			request_body,
			request_timestamp,
			response_status_code,
			response_headers,
			response_body,
			response_timestamp,
			service_result,
			error_payload,
			duration_ms,
			created_at
		FROM admin_subscription_action_logs
		WHERE id = $1
	`

	var (
		logEntry     domain.AdminSubscriptionActionLog
		operation    string
		reqHeaders   []byte
		reqBody      []byte
		resHeaders   []byte
		resBody      []byte
		serviceJSON  []byte
		errorJSON    []byte
		responseTime sql.NullTime
	)

	err := r.db.QueryRowContext(r.ctx, query, id).Scan(
		&logEntry.ID,
		&operation,
		&logEntry.MSISDN,
		&logEntry.ProductID,
		&logEntry.PartnerRoleID,
		&logEntry.ExternalTxID,
		&logEntry.AdminRequestID,
		&logEntry.RequestMethod,
		&logEntry.RequestURL,
		&reqHeaders,
		&reqBody,
		&logEntry.RequestTimestamp,
		&logEntry.ResponseStatusCode,
		&resHeaders,
		&resBody,
		&responseTime,
		&serviceJSON,
		&errorJSON,
		&logEntry.DurationMs,
		&logEntry.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get admin action log: %w", err)
	}

	logEntry.Operation = domain.AdminActionOperation(operation)
	logEntry.RequestBody = json.RawMessage(reqBody)
	logEntry.ResponseBody = json.RawMessage(resBody)
	logEntry.ServiceResult = json.RawMessage(serviceJSON)
	logEntry.ErrorPayload = json.RawMessage(errorJSON)

	if responseTime.Valid {
		logEntry.ResponseTimestamp = &responseTime.Time
	}

	if len(reqHeaders) > 0 {
		var headers map[string]string
		if err := json.Unmarshal(reqHeaders, &headers); err == nil {
			logEntry.RequestHeaders = headers
		}
	}
	if len(resHeaders) > 0 {
		var headers map[string]string
		if err := json.Unmarshal(resHeaders, &headers); err == nil {
			logEntry.ResponseHeaders = headers
		}
	}

	return &logEntry, nil
}

// AddToPriorityRetryQueue adds an item to the priority retry queue
func (r *SubscriptionRepository) AddToPriorityRetryQueue(item *domain.PriorityRetryQueue) error {
	query := `
		INSERT INTO priority_retry_queue (
			msisdn, product_id, reason, priority, retry_count,
			next_retry_at, last_attempt_at, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id`

	now := time.Now()
	item.CreatedAt = now
	item.UpdatedAt = now

	err := r.db.QueryRowContext(r.ctx, query,
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

// GetSubscription retrieves a subscription with renewal info
func (r *SubscriptionRepository) GetSubscription(msisdn string, productID string) (*domain.SubscriptionWithRenewalInfo, error) {
	query := `
		SELECT id, user_identifier, product_id, status, created_at
		FROM subscriptions
		WHERE user_identifier = $1 AND product_id = $2`

	var sub domain.SubscriptionWithRenewalInfo
	sub.Subscription = &domain.Subscription{}
	productIDInt, _ := strconv.Atoi(productID)

	err := r.db.QueryRowContext(r.ctx, query, msisdn, productIDInt).Scan(
		&sub.Subscription.Id, &sub.Subscription.UserIdentifier,
		&sub.Subscription.ProductId, &sub.Subscription.Status,
		&sub.Subscription.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}

	return &sub, nil
}

// GetLastSuccessfulPayment returns the last successful payment time
func (r *SubscriptionRepository) GetLastSuccessfulPayment(msisdn string, productID string) (*time.Time, error) {
	query := `
		SELECT created_at FROM notifications
		WHERE msisdn = $1 AND product_id = $2 AND type = 'CHARGE'
		ORDER BY created_at DESC LIMIT 1`

	productIDInt, _ := strconv.Atoi(productID)
	var paymentTime time.Time

	err := r.db.QueryRowContext(r.ctx, query, msisdn, productIDInt).Scan(&paymentTime)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get last payment: %w", err)
	}

	return &paymentTime, nil
}

// GetRenewalAttemptsCount returns the count of renewal attempts since a given time
func (r *SubscriptionRepository) GetRenewalAttemptsCount(msisdn string, productID string, since time.Time) (int, error) {
	query := `
		SELECT COUNT(*) FROM renewal_cycles
		WHERE msisdn = $1 AND product_id = $2 AND created_at >= $3`

	var count int
	err := r.db.QueryRowContext(r.ctx, query, msisdn, productID, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get renewal attempts count: %w", err)
	}

	return count, nil
}

// GetDailyChurnCount returns the count of churns for a given date
func (r *SubscriptionRepository) GetDailyChurnCount(date time.Time) (int, error) {
	query := `
		SELECT COUNT(*) FROM churn_records
		WHERE DATE(churned_at) = DATE($1)`

	var count int
	err := r.db.QueryRowContext(r.ctx, query, date).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get daily churn count: %w", err)
	}

	return count, nil
}

// GetLastRenewalAttempt returns the last renewal attempt time
func (r *SubscriptionRepository) GetLastRenewalAttempt(msisdn string, productID string) (*time.Time, error) {
	query := `
		SELECT created_at FROM renewal_cycles
		WHERE msisdn = $1 AND product_id = $2
		ORDER BY created_at DESC LIMIT 1`

	var attemptTime time.Time
	err := r.db.QueryRowContext(r.ctx, query, msisdn, productID).Scan(&attemptTime)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get last renewal attempt: %w", err)
	}

	return &attemptTime, nil
}

// ChurnSubscription marks a subscription as churned
func (r *SubscriptionRepository) ChurnSubscription(msisdn string, productID string, reason string, churnTime time.Time) error {
	query := `
		UPDATE subscriptions SET status = 'CHURNED', cancel_reason = $1
		WHERE user_identifier = $2 AND product_id = $3`

	productIDInt, _ := strconv.Atoi(productID)
	_, err := r.db.ExecContext(r.ctx, query, reason, msisdn, productIDInt)
	if err != nil {
		return fmt.Errorf("failed to churn subscription: %w", err)
	}

	return nil
}

// CreateChurnRecord creates a new churn record
func (r *SubscriptionRepository) CreateChurnRecord(record *domain.ChurnRecord) error {
	query := `
		INSERT INTO churn_records (msisdn, product_id, reason, churned_at, last_payment_date,
			hours_without_payment, total_renewal_attempts, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.db.ExecContext(r.ctx, query,
		record.MSISDN, record.ProductID, record.Reason, record.ChurnedAt,
		record.LastPaymentDate, record.HoursWithoutPayment,
		record.TotalRenewalAttempts, record.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create churn record: %w", err)
	}

	return nil
}

// SaveRenewalCycle saves a renewal cycle
func (r *SubscriptionRepository) SaveRenewalCycle(cycle *domain.RenewalCycle) error {
	query := `
		INSERT INTO renewal_cycles (msisdn, product_id, opt_out_time, opt_out_status,
			opt_in_time, opt_in_status, billing_status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`

	err := r.db.QueryRowContext(r.ctx, query,
		cycle.MSISDN, cycle.ProductID, cycle.OptOutTime, cycle.OptOutStatus,
		cycle.OptInTime, cycle.OptInStatus, cycle.BillingStatus,
		cycle.CreatedAt, cycle.UpdatedAt,
	).Scan(&cycle.ID)
	if err != nil {
		return fmt.Errorf("failed to save renewal cycle: %w", err)
	}

	return nil
}

// UpdateSubscriptionStatus updates the subscription status
func (r *SubscriptionRepository) UpdateSubscriptionStatus(msisdn string, productID string, status string) error {
	query := `UPDATE subscriptions SET status = $1 WHERE user_identifier = $2 AND product_id = $3`

	productIDInt, _ := strconv.Atoi(productID)
	_, err := r.db.ExecContext(r.ctx, query, status, msisdn, productIDInt)
	if err != nil {
		return fmt.Errorf("failed to update subscription status: %w", err)
	}

	return nil
}

// IncrementRenewalAttempt increments the renewal attempt counter
func (r *SubscriptionRepository) IncrementRenewalAttempt(msisdn string, productID string) error {
	query := `
		UPDATE subscriptions SET total_renewal_attempts = COALESCE(total_renewal_attempts, 0) + 1,
			last_renewal_attempt = NOW()
		WHERE user_identifier = $1 AND product_id = $2`

	productIDInt, _ := strconv.Atoi(productID)
	_, err := r.db.ExecContext(r.ctx, query, msisdn, productIDInt)
	if err != nil {
		return fmt.Errorf("failed to increment renewal attempt: %w", err)
	}

	return nil
}

// GetSubscriptionsNeedingRenewal returns subscriptions that need renewal
func (r *SubscriptionRepository) GetSubscriptionsNeedingRenewal(hoursThreshold int, limit int) ([]*domain.SubscriptionWithRenewalInfo, error) {
	query := `
		SELECT id, user_identifier, product_id, status, created_at
		FROM subscriptions
		WHERE status = 'ACTIVE'
		AND (last_successful_payment IS NULL OR last_successful_payment < NOW() - INTERVAL '1 hour' * $1)
		LIMIT $2`

	rows, err := r.db.QueryContext(r.ctx, query, hoursThreshold, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriptions needing renewal: %w", err)
	}
	defer rows.Close()

	var subs []*domain.SubscriptionWithRenewalInfo
	for rows.Next() {
		sub := &domain.SubscriptionWithRenewalInfo{Subscription: &domain.Subscription{}}
		if err := rows.Scan(&sub.Subscription.Id, &sub.Subscription.UserIdentifier,
			&sub.Subscription.ProductId, &sub.Subscription.Status,
			&sub.Subscription.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan subscription: %w", err)
		}
		subs = append(subs, sub)
	}

	return subs, nil
}

// SaveRenewalMetrics saves renewal metrics
func (r *SubscriptionRepository) SaveRenewalMetrics(metrics *domain.RenewalMetrics) error {
	query := `
		INSERT INTO renewal_metrics (total_processed, successful_renewals, failed_renewals,
			churned_subscriptions, success_rate, average_cycle_time, last_run_time)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := r.db.ExecContext(r.ctx, query,
		metrics.TotalProcessed, metrics.SuccessfulRenewals, metrics.FailedRenewals,
		metrics.ChurnedSubscriptions, metrics.SuccessRate, metrics.AverageCycleTime,
		metrics.LastRunTime,
	)
	if err != nil {
		return fmt.Errorf("failed to save renewal metrics: %w", err)
	}

	return nil
}

// GetDuePriorityRetryItems returns priority retry items that are due
func (r *SubscriptionRepository) GetDuePriorityRetryItems(limit int) ([]*domain.PriorityRetryQueue, error) {
	query := `
		SELECT id, msisdn, product_id, reason, priority, retry_count,
			next_retry_at, last_attempt_at, status, created_at, updated_at
		FROM priority_retry_queue
		WHERE status = 'pending' AND (next_retry_at IS NULL OR next_retry_at <= NOW())
		ORDER BY priority DESC, created_at ASC
		LIMIT $1`

	rows, err := r.db.QueryContext(r.ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get due priority retry items: %w", err)
	}
	defer rows.Close()

	var items []*domain.PriorityRetryQueue
	for rows.Next() {
		item := &domain.PriorityRetryQueue{}
		if err := rows.Scan(&item.ID, &item.MSISDN, &item.ProductID, &item.Reason,
			&item.Priority, &item.RetryCount, &item.NextRetryAt, &item.LastAttemptAt,
			&item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan priority retry item: %w", err)
		}
		items = append(items, item)
	}

	return items, nil
}

// UpdatePriorityRetryItem updates a priority retry item
func (r *SubscriptionRepository) UpdatePriorityRetryItem(item *domain.PriorityRetryQueue) error {
	query := `
		UPDATE priority_retry_queue
		SET retry_count = $1, next_retry_at = $2, last_attempt_at = $3, status = $4, updated_at = $5
		WHERE id = $6`

	item.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(r.ctx, query,
		item.RetryCount, item.NextRetryAt, item.LastAttemptAt, item.Status, item.UpdatedAt, item.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update priority retry item: %w", err)
	}

	return nil
}
