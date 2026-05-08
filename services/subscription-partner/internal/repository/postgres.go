package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	cached "github.com/seidu626/subscription-manager/common/cache"
	"github.com/seidu626/subscription-manager/subscription/internal/domain"
	"log"
	"strconv"
	"strings"
	"time"
)

type SubscriptionRepository struct {
	db    *sql.DB
	redis cached.RedisClient
	ctx   context.Context
}

func NewSubscriptionRepository(db *sql.DB, client cached.RedisClient) *SubscriptionRepository {
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
	shortcode = strings.TrimSpace(shortcode)
	userIdentifier = strings.TrimSpace(userIdentifier)
	entryChannel = strings.TrimSpace(entryChannel)

	cacheKey := r.GenerateCacheKey(startDate, endDate, productId, shortcode, userIdentifier, entryChannel, page, pageSize)

	log.Printf("Fetching notifications from cache: %s", cacheKey)
	// Check if cached data exists
	cachedData, err := r.redis.Get(r.ctx, cacheKey)
	if err == nil {
		var listResponse *domain.ListResponse
		if err := json.Unmarshal([]byte(cachedData), &listResponse); err == nil {
			return listResponse, nil
		}
	} else if !errors.Is(err, redis.Nil) {
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

	// startDate/endDate filter by record creation time for list semantics.
	query, countQuery, args, argIndex = applySubscriptionDateFilters(query, countQuery, args, argIndex, startDate, endDate)
	if productId > 0 {
		query += fmt.Sprintf(" AND product_id = $%d", argIndex)
		countQuery += fmt.Sprintf(" AND product_id = $%d", argIndex)
		args = append(args, productId)
		argIndex++
	}
	if shortcode != "" {
		query += fmt.Sprintf(" AND COALESCE(mcc, '') ILIKE $%d", argIndex)
		countQuery += fmt.Sprintf(" AND COALESCE(mcc, '') ILIKE $%d", argIndex)
		args = append(args, "%"+shortcode+"%")
		argIndex++
	}
	if userIdentifier != "" {
		query += fmt.Sprintf(" AND user_identifier ILIKE $%d", argIndex)
		countQuery += fmt.Sprintf(" AND user_identifier ILIKE $%d", argIndex)
		args = append(args, "%"+userIdentifier+"%")
		argIndex++
	}
	if entryChannel != "" {
		query += fmt.Sprintf(" AND COALESCE(entry_channel, '') ILIKE $%d", argIndex)
		countQuery += fmt.Sprintf(" AND COALESCE(entry_channel, '') ILIKE $%d", argIndex)
		args = append(args, "%"+entryChannel+"%")
		argIndex++
	}

	// Get total records count
	var totalRecords int
	err = r.db.QueryRow(countQuery, args...).Scan(&totalRecords)
	if err != nil {
		return nil, fmt.Errorf("count subscriptions: %w", err)
	}

	// Add pagination support
	offset := (page - 1) * pageSize
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, pageSize, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query subscriptions: %w", err)
	}

	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Printf("Error closing rows: %v", err)
		}
	}(rows)

	var subscriptions []*domain.Subscription
	for rows.Next() {
		subscription, scanErr := scanAndMapSubscription(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan subscription row: %w", scanErr)
		}
		subscriptions = append(subscriptions, subscription)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subscription rows: %w", err)
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
		_ = r.redis.Set(r.ctx, cacheKey, data, 30*time.Minute)
	}

	return listResponse, nil
}

func applySubscriptionDateFilters(query, countQuery string, args []interface{}, argIndex int, startDate, endDate time.Time) (string, string, []interface{}, int) {
	if !startDate.IsZero() {
		query += fmt.Sprintf(" AND created_at >= $%d", argIndex)
		countQuery += fmt.Sprintf(" AND created_at >= $%d", argIndex)
		args = append(args, startDate)
		argIndex++
	}
	if !endDate.IsZero() {
		query += fmt.Sprintf(" AND created_at <= $%d", argIndex)
		countQuery += fmt.Sprintf(" AND created_at <= $%d", argIndex)
		args = append(args, endDate)
		argIndex++
	}
	return query, countQuery, args, argIndex
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanAndMapSubscription(scanner rowScanner) (*domain.Subscription, error) {
	var id int
	var userIdentifier string
	var userIdentifierType string
	var productID int
	var partnerRoleID int
	var campaignURL sql.NullString
	var entryChannel sql.NullString
	var subKeyword sql.NullString
	var trackingID sql.NullString
	var largeAccount sql.NullString
	var mcc sql.NullString
	var mnc sql.NullString
	var status sql.NullString
	var cancelReason sql.NullInt64
	var cancelSource sql.NullInt64
	var startDate time.Time
	var endDate sql.NullTime
	var transactionAuthCode sql.NullString
	var clientIP sql.NullString
	var createdAt time.Time

	if err := scanner.Scan(
		&id,
		&userIdentifier,
		&userIdentifierType,
		&productID,
		&partnerRoleID,
		&campaignURL,
		&entryChannel,
		&subKeyword,
		&trackingID,
		&largeAccount,
		&mcc,
		&mnc,
		&status,
		&cancelReason,
		&cancelSource,
		&startDate,
		&endDate,
		&transactionAuthCode,
		&clientIP,
		&createdAt,
	); err != nil {
		return nil, err
	}

	return &domain.Subscription{
		Id:                  id,
		PartnerRoleId:       strconv.Itoa(partnerRoleID),
		UserIdentifier:      userIdentifier,
		UserIdentifierType:  userIdentifierType,
		ProductId:           strconv.Itoa(productID),
		Mcc:                 nullStringToString(mcc),
		Mnc:                 nullStringToString(mnc),
		EntryChannel:        nullStringToString(entryChannel),
		LargeAccount:        nullStringToString(largeAccount),
		SubKeyword:          nullStringToString(subKeyword),
		TrackingId:          nullStringToString(trackingID),
		ClientIp:            nullStringToString(clientIP),
		CampaignUrl:         nullStringToString(campaignURL),
		Status:              nullStringToString(status),
		CancelReason:        nullIntToStringPtr(cancelReason),
		CancelSource:        nullIntToStringPtr(cancelSource),
		CreatedAt:           createdAt.Format(time.RFC3339Nano),
		StartDate:           startDate,
		EndDate:             nullTimePtr(endDate),
		TransactionAuthCode: nullStringPtr(transactionAuthCode),
	}, nil
}

func nullStringToString(val sql.NullString) string {
	if val.Valid {
		return val.String
	}
	return ""
}

func nullStringPtr(val sql.NullString) *string {
	if !val.Valid {
		return nil
	}
	s := val.String
	return &s
}

func nullIntToStringPtr(val sql.NullInt64) *string {
	if !val.Valid {
		return nil
	}
	s := strconv.FormatInt(val.Int64, 10)
	return &s
}

func nullTimePtr(val sql.NullTime) *time.Time {
	if !val.Valid {
		return nil
	}
	t := val.Time
	return &t
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

// CreateNotification inserts an inbound TIMWE notification into the notifications table.
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

// ConfirmSubscription updates a subscription record based on confirmation details.
func (r *SubscriptionRepository) ConfirmSubscription(request *domain.SubscriptionConfirmationRequest) error {
	query := `
        UPDATE subscriptions 
        SET transaction_auth_code = $1
        WHERE partner_role_id = $2 AND user_identifier = $3 AND product_id = $4
    `
	result, err := r.db.Exec(query, request.TransactionAuthCode, request.PartnerRoleId, request.UserIdentifier, request.ProductId)
	if err != nil {
		return fmt.Errorf("failed to confirm subscription: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read confirm result: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("subscription not found for confirmation")
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
