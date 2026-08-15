package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"go.uber.org/zap"
)

// feedSelectColumns is shared by ListFeed and GetFeedItem: content already
// delivered (message_outbox status='SENT') to the given msisdn, enriched
// with the owning product's slug/name and this msisdn's read state.
const feedSelectColumns = `
    ci.id AS content_item_id,
    COALESCE(camp.slug, '') AS product_slug,
    COALESCE(p.name, '') AS product_name,
    ci.title,
    ci.message_text,
    ci.content_kind,
    ci.link_url,
    ci.cta_label,
    COALESCE(mo.sent_at, mo.planned_send_at) AS published_at,
    (rs.content_item_id IS NOT NULL) AS is_read
FROM message_outbox mo
JOIN subscriptions s ON s.id = mo.subscription_id
JOIN message_content_items ci ON ci.id = mo.content_item_id
LEFT JOIN LATERAL (
    SELECT slug FROM campaigns
    WHERE offer_product_id = s.product_id
      AND (s.partner_role_id IS NULL OR partner_role_id = s.partner_role_id)
      AND enabled = true
    ORDER BY created_at DESC
    LIMIT 1
) camp ON true
LEFT JOIN products p ON p.product_id = s.product_id::text
LEFT JOIN app_feed_read_state rs ON rs.msisdn = s.user_identifier AND rs.content_item_id = ci.id
WHERE s.user_identifier = $1
  AND mo.status = 'SENT'`

// AppFeedRepository backs the Dayline app feed, device-registration, and
// notification-preference endpoints. It is intentionally self-contained
// (not part of SubscriptionRepositoryInterface) since it owns a distinct,
// contract-bound surface.
type AppFeedRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewAppFeedRepository builds an AppFeedRepository over the shared
// subscription-external Postgres connection.
func NewAppFeedRepository(db *sql.DB, logger *zap.Logger) *AppFeedRepository {
	return &AppFeedRepository{db: db, logger: logger}
}

// deriveTitle applies the contract's title fallback: use the stored title
// when present, else the first 60 runes of the message body.
func deriveTitle(title sql.NullString, body string) string {
	if title.Valid {
		trimmed := strings.TrimSpace(title.String)
		if trimmed != "" {
			return trimmed
		}
	}
	runes := []rune(strings.TrimSpace(body))
	const fallbackLen = 60
	if len(runes) <= fallbackLen {
		return string(runes)
	}
	return string(runes[:fallbackLen])
}

func scanFeedItem(row interface {
	Scan(dest ...interface{}) error
}) (domain.AppFeedItem, error) {
	var (
		id          int64
		productSlug string
		productName string
		title       sql.NullString
		body        string
		contentKind string
		linkURL     sql.NullString
		ctaLabel    sql.NullString
		publishedAt time.Time
		read        bool
	)
	if err := row.Scan(&id, &productSlug, &productName, &title, &body, &contentKind, &linkURL, &ctaLabel, &publishedAt, &read); err != nil {
		return domain.AppFeedItem{}, err
	}
	return domain.AppFeedItem{
		ID:          id,
		ProductSlug: productSlug,
		ProductName: productName,
		Title:       deriveTitle(title, body),
		Body:        body,
		ContentKind: contentKind,
		LinkURL:     appFeedNullStringPtr(linkURL),
		CTALabel:    appFeedNullStringPtr(ctaLabel),
		PublishedAt: publishedAt,
		Read:        read,
	}, nil
}

func appFeedNullStringPtr(val sql.NullString) *string {
	if !val.Valid || strings.TrimSpace(val.String) == "" {
		return nil
	}
	s := val.String
	return &s
}

// ListFeed returns delivered feed items for msisdn, most recent first,
// capped at limit.
func (r *AppFeedRepository) ListFeed(ctx context.Context, msisdn string, limit int) ([]domain.AppFeedItem, error) {
	query := `SELECT * FROM (
    SELECT DISTINCT ON (ci.id)` + feedSelectColumns + `
    ORDER BY ci.id, published_at DESC
) feed
ORDER BY published_at DESC
LIMIT $2`

	rows, err := r.db.QueryContext(ctx, query, msisdn, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.AppFeedItem, 0)
	for rows.Next() {
		item, err := scanFeedItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// GetFeedItem returns a single delivered feed item for msisdn, or
// sql.ErrNoRows when it does not exist or was not delivered to this msisdn.
func (r *AppFeedRepository) GetFeedItem(ctx context.Context, msisdn string, id int64) (*domain.AppFeedItem, error) {
	query := `SELECT` + feedSelectColumns + `
  AND ci.id = $2
ORDER BY published_at DESC
LIMIT 1`

	row := r.db.QueryRowContext(ctx, query, msisdn, id)
	item, err := scanFeedItem(row)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// MarkRead records msisdn having read content item id. Returns
// sql.ErrNoRows when the item was never delivered to this msisdn.
func (r *AppFeedRepository) MarkRead(ctx context.Context, msisdn string, id int64) error {
	var exists int
	checkQuery := `SELECT 1 FROM message_outbox mo
JOIN subscriptions s ON s.id = mo.subscription_id
WHERE s.user_identifier = $1 AND mo.content_item_id = $2 AND mo.status = 'SENT'
LIMIT 1`
	if err := r.db.QueryRowContext(ctx, checkQuery, msisdn, id).Scan(&exists); err != nil {
		return err
	}

	upsertQuery := `INSERT INTO app_feed_read_state (msisdn, content_item_id, read_at)
VALUES ($1, $2, NOW())
ON CONFLICT (msisdn, content_item_id) DO UPDATE SET read_at = EXCLUDED.read_at`
	_, err := r.db.ExecContext(ctx, upsertQuery, msisdn, id)
	return err
}

// UpsertDevice registers or updates a push device for msisdn. Re-registering
// an existing fcm_token (e.g. reinstall) upserts in place, keyed on the
// unique token.
func (r *AppFeedRepository) UpsertDevice(ctx context.Context, msisdn, tenantKey, fcmToken, platform string) error {
	query := `INSERT INTO app_devices (msisdn, tenant_key, fcm_token, platform, created_at, updated_at)
VALUES ($1, $2, $3, $4, NOW(), NOW())
ON CONFLICT (fcm_token) DO UPDATE SET
    msisdn = EXCLUDED.msisdn,
    tenant_key = EXCLUDED.tenant_key,
    platform = EXCLUDED.platform,
    updated_at = NOW()`
	_, err := r.db.ExecContext(ctx, query, msisdn, tenantKey, fcmToken, platform)
	return err
}

// UpsertNotificationPref sets msisdn's delivery-channel preference for a
// product.
func (r *AppFeedRepository) UpsertNotificationPref(ctx context.Context, msisdn, productSlug, channel string) error {
	query := `INSERT INTO app_notification_prefs (msisdn, product_slug, channel, updated_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (msisdn, product_slug) DO UPDATE SET
    channel = EXCLUDED.channel,
    updated_at = NOW()`
	_, err := r.db.ExecContext(ctx, query, msisdn, productSlug, channel)
	return err
}
