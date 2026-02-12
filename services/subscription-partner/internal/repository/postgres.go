package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/redis/go-redis/v9"
	"github.com/seidu626/subscription-manager/subscription/internal/domain"
	"log"
	"time"
)

type SubscriptionRepository struct {
	db    *sql.DB
	redis *redis.Client
	ctx   context.Context
}

func NewSubscriptionRepository(db *sql.DB, client *redis.Client) *SubscriptionRepository {
	return &SubscriptionRepository{
		db:    db,
		redis: client,
		ctx:   context.Background(),
	}
}

// GenerateCacheKey generates a unique cache key for query filters
func (r *SubscriptionRepository) GenerateCacheKey(startDate, endDate time.Time, productId int, shortcode, userIdentifier, entryChannel string, page, pageSize int) string {
	return fmt.Sprintf("notifications:%s:%s:%d:%s:%s:%s:%d:%d", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"), productId, shortcode, userIdentifier, entryChannel, page, pageSize)
}

func (r *SubscriptionRepository) FetchSubscriptions(startDate, endDate time.Time, productId int, shortcode, userIdentifier, entryChannel string, page, pageSize int) (*domain.ListResponse, error) {
	cacheKey := r.GenerateCacheKey(startDate, endDate, productId, shortcode, userIdentifier, entryChannel, page, pageSize)

	log.Printf("Fetching notifications from cache: %s", cacheKey)
	// Check if cached data exists
	cachedData, err := r.redis.Get(r.ctx, cacheKey).Result()
	if err == nil {
		var listResponse *domain.ListResponse
		if err := json.Unmarshal([]byte(cachedData), &listResponse); err == nil {
			return listResponse, nil
		}
	} else {
		log.Printf("Failed to find cached data: %+v", err.Error())
	}

	query := `
        SELECT 
            id,
            user_identifier,
            user_identifier_type,
            product_id,
            partner_role_id,
            campaign_url,
            entry_channel,
            sub_keyword,
            tracking_id,
            large_account,
            mcc,
            mnc,
            status,
            cancel_reason,
            cancel_source,
            start_date,
            end_date,
            transaction_auth_code,
            client_ip,
            created_at
        FROM subscriptions WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM subscriptions WHERE 1=1`
	var args []interface{}
	argIndex := 1 // PostgreSQL placeholders start with $1

	// Add filtering conditions dynamically based on provided filters
	if !startDate.IsZero() {
		query += fmt.Sprintf(" AND start_date >= $%d", argIndex)
		countQuery += fmt.Sprintf(" AND start_date >= $%d", argIndex)
		args = append(args, startDate)
		argIndex++
	}
	if !endDate.IsZero() {
		query += fmt.Sprintf(" AND end_date <= $%d", argIndex)
		countQuery += fmt.Sprintf(" AND end_date <= $%d", argIndex)
		args = append(args, endDate)
		argIndex++
	}
	if productId > 0 {
		query += fmt.Sprintf(" AND product_id = $%d", argIndex)
		countQuery += fmt.Sprintf(" AND product_id = $%d", argIndex)
		args = append(args, productId)
		argIndex++
	}
	if shortcode != "" {
		query += fmt.Sprintf(" AND mcc = $%d", argIndex)
		countQuery += fmt.Sprintf(" AND mcc = $%d", argIndex)
		args = append(args, shortcode)
		argIndex++
	}
	if userIdentifier != "" {
		query += fmt.Sprintf(" AND user_identifier = $%d", argIndex)
		countQuery += fmt.Sprintf(" AND user_identifier = $%d", argIndex)
		args = append(args, userIdentifier)
		argIndex++
	}
	if entryChannel != "" {
		query += fmt.Sprintf(" AND entry_channel = $%d", argIndex)
		countQuery += fmt.Sprintf(" AND entry_channel = $%d", argIndex)
		args = append(args, entryChannel)
		argIndex++
	}

	// Get total records count
	var totalRecords int
	err = r.db.QueryRow(countQuery, args...).Scan(&totalRecords)
	if err != nil {
		return nil, err
	}

	// Add pagination support
	offset := (page - 1) * pageSize
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, pageSize, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}

	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Printf("Error closing rows: %v", err)
		}
	}(rows)

	var subscriptions []*domain.Subscription
	for rows.Next() {
		subscription := &domain.Subscription{}
		if err := rows.Scan(
			&subscription.Id,
			&subscription.UserIdentifier,
			&subscription.UserIdentifierType,
			&subscription.ProductId,
			&subscription.PartnerRoleId,
			&subscription.CampaignUrl,
			&subscription.EntryChannel,
			&subscription.SubKeyword,
			&subscription.TrackingId,
			&subscription.LargeAccount,
			&subscription.Mcc,
			&subscription.Mnc,
			&subscription.Status,
			&subscription.CancelReason,
			&subscription.CancelSource,
			&subscription.StartDate,
			&subscription.EndDate,
			&subscription.TransactionAuthCode,
			&subscription.ClientIp,
			&subscription.CreatedAt,
		); err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, subscription)
	}

	totalPages := (totalRecords + pageSize - 1) / pageSize // to round up
	hasNextPage := page < totalPages
	hasPrevPage := page > 1

	listResponse := &domain.ListResponse{
		Data:        subscriptions,
		TotalCount:  totalRecords,
		Page:        page,
		PageSize:    pageSize,
		TotalPages:  totalPages,
		HasNextPage: hasNextPage,
		HasPrevPage: hasPrevPage,
	}

	data, err := json.Marshal(listResponse)
	if err == nil {
		r.redis.Set(r.ctx, cacheKey, data, 30*time.Minute)
	}

	return listResponse, nil
}

// CreateSubscription inserts a new subscription record into the database.
func (r *SubscriptionRepository) CreateSubscription(request *domain.SubscriptionRequest) error {
	query := `
        INSERT INTO subscriptions (partner_role_id, user_identifier, user_identifier_type, product_id, mcc, mnc, entry_channel, large_account, sub_keyword, tracking_id, client_ip, campaign_url)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
    `
	_, err := r.db.Exec(query, request.PartnerRoleId, request.UserIdentifier, request.UserIdentifierType, request.ProductId, request.Mcc, request.Mnc, request.EntryChannel, request.LargeAccount, request.SubKeyword, request.TrackingId, request.ClientIp, request.CampaignUrl)
	if err != nil {
		return fmt.Errorf("failed to create subscription: %w", err)
	}
	return nil
}

// ConfirmSubscription updates a subscription record based on confirmation details.
func (r *SubscriptionRepository) ConfirmSubscription(request *domain.SubscriptionConfirmationRequest) error {
	query := `
        UPDATE subscriptions 
        SET transaction_auth_code = $1
        WHERE partner_role_id = $2 AND user_identifier = $3 AND product_id = $4
    `
	_, err := r.db.Exec(query, request.TransactionAuthCode, request.PartnerRoleId, request.UserIdentifier, request.ProductId)
	if err != nil {
		return fmt.Errorf("failed to confirm subscription: %w", err)
	}
	return nil
}

// OptOutSubscription deletes or deactivates a subscription record.
func (r *SubscriptionRepository) OptOutSubscription(request *domain.UnsubscriptionRequest) error {
	query := `
        UPDATE subscriptions 
        SET status = 'inactive', cancel_reason = $1, cancel_source = $2
        WHERE partner_role_id = $3 AND user_identifier = $4 AND product_id = $5
    `
	_, err := r.db.Exec(query, request.CancelReason, request.CancelSource, request.PartnerRoleId, request.UserIdentifier, request.ProductId)
	if err != nil {
		return fmt.Errorf("failed to opt out subscription: %w", err)
	}
	return nil
}

// GetSubscriptionStatus retrieves the status of a subscription for a given user.
func (r *SubscriptionRepository) GetSubscriptionStatus(request *domain.GetStatusRequest) (*domain.SubscriptionStatus, error) {
	query := `
        SELECT product_id, user_identifier, status, start_date, end_date
        FROM subscriptions
        WHERE partner_role_id = $1 AND user_identifier = $2 AND product_id = $3
    `
	row := r.db.QueryRow(query, request.PartnerRoleId, request.UserIdentifier, request.ProductId)

	var status domain.SubscriptionStatus
	if err := row.Scan(&status.ProductId, &status.UserIdentifier, &status.Status, &status.StartDate, &status.EndDate); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no subscription found for user %s and product %d", request.UserIdentifier, request.ProductId)
		}
		return nil, fmt.Errorf("failed to retrieve subscription status: %w", err)
	}

	return &status, nil
}
