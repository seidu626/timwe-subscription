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

func (r *NotificationRepository) TenantIDByKey(ctx context.Context, tenantKey string) (string, error) {
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
		return "", fmt.Errorf("tenant not found")
	}
	if err != nil {
		return "", err
	}
	return tenantID, nil
}

// MemberTenant is a lightweight active-membership record returned to the
// admin gate. Source of truth is tenant_admin_memberships JOIN tenants.
type MemberTenant struct {
	ID        string
	TenantKey string
}

// ListActiveTenantsForMember returns active tenant memberships for an Auth0
// subject and/or email. Matches acquisition-api's resolver — the membership
// table is authoritative; tenant headers are never trusted on their own.
func (r *NotificationRepository) ListActiveTenantsForMember(auth0Subject, email string) ([]MemberTenant, error) {
	auth0Subject = strings.TrimSpace(auth0Subject)
	email = strings.ToLower(strings.TrimSpace(email))
	if auth0Subject == "" && email == "" {
		return []MemberTenant{}, nil
	}

	where := []string{"m.status = 'ACTIVE'", "t.status = 'ACTIVE'"}
	args := []any{}
	principalFilters := []string{}
	if auth0Subject != "" {
		args = append(args, auth0Subject)
		principalFilters = append(principalFilters, fmt.Sprintf("m.auth0_subject = $%d", len(args)))
	}
	if email != "" {
		args = append(args, email)
		principalFilters = append(principalFilters, fmt.Sprintf("LOWER(m.email) = $%d", len(args)))
	}
	if len(principalFilters) > 0 {
		where = append(where, "("+strings.Join(principalFilters, " OR ")+")")
	}

	query := fmt.Sprintf(`
		SELECT DISTINCT t.id::text, t.tenant_key
		FROM tenant_admin_memberships m
		JOIN tenants t ON t.id = m.tenant_id
		WHERE %s
		ORDER BY t.tenant_key ASC
	`, strings.Join(where, " AND "))

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list active member tenants: %w", err)
	}
	defer rows.Close()

	out := make([]MemberTenant, 0)
	for rows.Next() {
		var m MemberTenant
		if err := rows.Scan(&m.ID, &m.TenantKey); err != nil {
			return nil, fmt.Errorf("failed to scan member tenant: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate member tenants: %w", err)
	}
	return out, nil
}

// ChannelIDByKeys resolves a channel_key to its UUID for a given tenant.
// Returns ("", ErrTenantChannelNotFound) when no active row matches.
var ErrTenantChannelNotFound = errors.New("tenant channel not found")

func (r *NotificationRepository) ChannelIDByKeys(ctx context.Context, tenantID, channelKey string) (string, error) {
	tenantID = strings.TrimSpace(tenantID)
	channelKey = strings.TrimSpace(channelKey)
	if tenantID == "" || channelKey == "" {
		return "", fmt.Errorf("tenantID and channelKey are required")
	}

	var channelID string
	err := r.db.QueryRowContext(ctx, `
		SELECT id::text FROM tenant_channels
		WHERE tenant_id = $1 AND channel_key = $2 AND status = 'ACTIVE'
		LIMIT 1
	`, tenantID, channelKey).Scan(&channelID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrTenantChannelNotFound
	}
	if err != nil {
		return "", err
	}
	return channelID, nil
}

// GenerateCacheKey generates a unique cache key for query filters.
func (r *NotificationRepository) GenerateCacheKey(startDate, endDate time.Time, tenantID, channelID, partnerRole, msisdn, channel, notificationType string, page, pageSize int) string {
	return fmt.Sprintf("notifications:%s:%s:%s:%s:%s:%s:%s:%s:%d:%d", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"), tenantID, channelID, partnerRole, msisdn, channel, notificationType, page, pageSize)
}

// FetchNotifications retrieves notifications with filtering, pagination, and caching support.
func (r *NotificationRepository) FetchNotifications(startDate, endDate time.Time, tenantID, channelID, partnerRole, msisdn, entryChannel, notificationType string, page, pageSize int) (*domain.ListResponse, error) {
	tenantID = strings.TrimSpace(tenantID)
	channelID = strings.TrimSpace(channelID)
	partnerRole = strings.TrimSpace(partnerRole)
	msisdn = strings.TrimSpace(msisdn)
	entryChannel = strings.TrimSpace(entryChannel)
	notificationType = strings.TrimSpace(notificationType)

	cacheKey := r.GenerateCacheKey(startDate, endDate, tenantID, channelID, partnerRole, msisdn, entryChannel, notificationType, page, pageSize)

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
            id, tenant_id::text, channel_id::text, partner_role, msisdn, product_id, entry_channel, pricepoint_id, type, created_at
        FROM notifications WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM notifications WHERE 1=1`
	args := []interface{}{}
	argIndex := 1

	// Apply filters if provided
	if tenantID != "" {
		query += fmt.Sprintf(" AND tenant_id::text = $%d", argIndex)
		countQuery += fmt.Sprintf(" AND tenant_id::text = $%d", argIndex)
		args = append(args, tenantID)
		argIndex++
	}
	if channelID != "" {
		query += fmt.Sprintf(" AND channel_id::text = $%d", argIndex)
		countQuery += fmt.Sprintf(" AND channel_id::text = $%d", argIndex)
		args = append(args, channelID)
		argIndex++
	}
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
	var tenantID sql.NullString
	var channelID sql.NullString
	var partnerRole int
	var msisdn string
	var productID int
	var entryChannel sql.NullString
	var pricepointID int
	var notificationType sql.NullString
	var createdAt time.Time

	if err := scanner.Scan(
		&id,
		&tenantID,
		&channelID,
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
		TenantID:     nullStringPtr(tenantID),
		ChannelID:    nullStringPtr(channelID),
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
            tenant_id, channel_id, partner_role, external_tx_id, product_id, pricepoint_id, mcc, mnc, msisdn,
            large_account, transaction_uuid, mno_delivery_code, entry_channel, message_type,
            message, tags, type
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17
        )`
	_, err := r.db.Exec(query,
		nullStringPtrValue(notification.TenantID),
		nullStringPtrValue(notification.ChannelID),
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

func nullStringPtrValue(value *string) sql.NullString {
	if value == nil || strings.TrimSpace(*value) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: strings.TrimSpace(*value), Valid: true}
}
