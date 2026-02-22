package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	cached "github.com/seidu626/subscription-manager/common/cache"
	"github.com/seidu626/subscription-manager/notification/internal/domain"
	"log"
	"strings"
	"time"
)

type NotificationRepository struct {
	db    *sql.DB
	redis cached.RedisClient
	ctx   context.Context
}

func NewNotificationRepository(db *sql.DB, client cached.RedisClient) *NotificationRepository {
	return &NotificationRepository{
		db:    db,
		redis: client,
		ctx:   context.Background(),
	}
}

// GenerateCacheKey generates a unique cache key for query filters
func (r *NotificationRepository) GenerateCacheKey(startDate, endDate time.Time, partnerRole, msisdn, channel, notificationType string, page, pageSize int) string {
	return fmt.Sprintf("notifications:%s:%s:%s:%s:%s:%s:%d:%d", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"), partnerRole, msisdn, channel, notificationType, page, pageSize)
}

// FetchNotifications retrieves notifications with filtering, pagination, and caching support.
func (r *NotificationRepository) FetchNotifications(startDate, endDate time.Time, partnerRole, msisdn, entryChannel, notificationType string, page, pageSize int) (*domain.ListResponse, error) {
	partnerRole = strings.TrimSpace(partnerRole)
	msisdn = strings.TrimSpace(msisdn)
	entryChannel = strings.TrimSpace(entryChannel)
	notificationType = strings.TrimSpace(notificationType)

	cacheKey := r.GenerateCacheKey(startDate, endDate, partnerRole, msisdn, entryChannel, notificationType, page, pageSize)

	// Try fetching cached response
	log.Printf("Fetching notifications from cache: %s", cacheKey)
	cachedData, err := r.redis.Get(r.ctx, cacheKey)
	if err == nil {
		var listResponse *domain.ListResponse
		if jsonErr := json.Unmarshal([]byte(cachedData), &listResponse); jsonErr == nil {
			return listResponse, nil
		}
	} else if !errors.Is(err, redis.Nil) {
		log.Printf("Cache miss or error: %+v", err)
	}

	// Build main and count queries with filtering options
	query := `
        SELECT 
            id, partner_role, msisdn, product_id, entry_channel, pricepoint_id, type, created_at
        FROM notifications WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM notifications WHERE 1=1`
	args := []interface{}{}
	argIndex := 1

	// Apply filters if provided
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
	if partnerRole != "" {
		query += fmt.Sprintf(" AND CAST(partner_role AS TEXT) LIKE $%d", argIndex)
		countQuery += fmt.Sprintf(" AND CAST(partner_role AS TEXT) LIKE $%d", argIndex)
		args = append(args, "%"+partnerRole+"%")
		argIndex++
	}
	if msisdn != "" {
		query += fmt.Sprintf(" AND msisdn LIKE $%d", argIndex)
		countQuery += fmt.Sprintf(" AND msisdn LIKE $%d", argIndex)
		args = append(args, "%"+msisdn+"%")
		argIndex++
	}
	if entryChannel != "" {
		query += fmt.Sprintf(" AND COALESCE(entry_channel, '') ILIKE $%d", argIndex)
		countQuery += fmt.Sprintf(" AND COALESCE(entry_channel, '') ILIKE $%d", argIndex)
		args = append(args, "%"+entryChannel+"%")
		argIndex++
	}
	if notificationType != "" {
		query += fmt.Sprintf(" AND COALESCE(type, '') ILIKE $%d", argIndex)
		countQuery += fmt.Sprintf(" AND COALESCE(type, '') ILIKE $%d", argIndex)
		args = append(args, "%"+notificationType+"%")
		argIndex++
	}

	// Get total count for pagination
	var totalRecords int
	if err := r.db.QueryRow(countQuery, args...).Scan(&totalRecords); err != nil {
		return nil, fmt.Errorf("count notifications: %w", err)
	}

	// Add pagination support
	offset := (page - 1) * pageSize
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, pageSize, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query notifications: %w", err)
	}
	defer rows.Close()

	// Scan results into notifications
	var notifications []*domain.Notification
	for rows.Next() {
		notification, scanErr := scanAndMapNotification(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan notification row: %w", scanErr)
		}
		notifications = append(notifications, notification)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notification rows: %w", err)
	}

	totalPages := (totalRecords + pageSize - 1) / pageSize // to round up
	hasNextPage := page < totalPages
	hasPrevPage := page > 1

	listResponse := &domain.ListResponse{
		Data:        notifications,
		TotalCount:  totalRecords,
		Page:        page,
		PageSize:    pageSize,
		TotalPages:  totalPages,
		HasNextPage: hasNextPage,
		HasPrevPage: hasPrevPage,
	}

	// Cache the response data for 10 minutes
	data, jsonErr := json.Marshal(listResponse)
	if jsonErr == nil {
		_ = r.redis.Set(r.ctx, cacheKey, data, 10*time.Minute)
	}

	return listResponse, nil
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanAndMapNotification(scanner rowScanner) (*domain.Notification, error) {
	var id int
	var partnerRole int
	var msisdn string
	var productID int
	var entryChannel sql.NullString
	var pricepointID int
	var notificationType sql.NullString
	var createdAt time.Time

	if err := scanner.Scan(
		&id,
		&partnerRole,
		&msisdn,
		&productID,
		&entryChannel,
		&pricepointID,
		&notificationType,
		&createdAt,
	); err != nil {
		return nil, err
	}

	return &domain.Notification{
		ID:           id,
		PartnerRole:  partnerRole,
		MSISDN:       msisdn,
		ProductID:    productID,
		EntryChannel: nullStringToString(entryChannel),
		PricepointID: pricepointID,
		Type:         nullStringPtr(notificationType),
		CreatedAt:    createdAt,
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

func (r *NotificationRepository) Save(notification *domain.NotificationRequest) error {
	query := `
        INSERT INTO notifications (
            partner_role, external_tx_id, product_id, pricepoint_id, mcc, mnc, msisdn,
            large_account, transaction_uuid, mno_delivery_code, entry_channel, message_type,
            message, tags, type
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
        )`
	_, err := r.db.Exec(query,
		notification.PartnerRole,
		notification.ExternalTxID,
		notification.ProductID,
		notification.PricepointID,
		notification.MCC,
		notification.MNC,
		notification.MSISDN,
		notification.LargeAccount,
		notification.TransactionUUID,
		notification.MnoDeliveryCode,
		notification.EntryChannel,
		notification.MessageType,
		notification.Message,
		pq.Array(notification.Tags),
		notification.Type,
	)
	if err != nil {
		return fmt.Errorf("failed to save notification: %w", err)
	}
	return nil
}
