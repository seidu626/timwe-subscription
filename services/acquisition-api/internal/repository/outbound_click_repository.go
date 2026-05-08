package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"go.uber.org/zap"
)

// OutboundClickRepository handles outbound click data access
type OutboundClickRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewOutboundClickRepository creates a new outbound click repository
func NewOutboundClickRepository(db *sql.DB, logger *zap.Logger) *OutboundClickRepository {
	return &OutboundClickRepository{
		db:     db,
		logger: logger,
	}
}

// Create inserts a new outbound click record
func (r *OutboundClickRepository) Create(click *domain.OutboundClick) error {
	if click == nil {
		return fmt.Errorf("click is nil")
	}

	// Generate UUID if not set
	if click.ClickID == uuid.Nil {
		click.ClickID = uuid.New()
	}

	query := `
		INSERT INTO outbound_clicks (
			click_id, partner, campaign_slug, offer_product_id,
			dest_key, dest_url, query_params,
			referrer_domain, ip_hash, user_agent_hash,
			status, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7,
			$8, $9, $10,
			$11, $12, $13
		)
	`

	// Serialize query_params to JSON
	queryParamsJSON := []byte("{}")
	if click.QueryParams != nil && len(click.QueryParams) > 0 {
		var err error
		queryParamsJSON, err = json.Marshal(click.QueryParams)
		if err != nil {
			return fmt.Errorf("failed to marshal query_params: %w", err)
		}
	}

	now := time.Now()
	click.CreatedAt = now
	click.UpdatedAt = now

	if click.Status == "" {
		click.Status = domain.OutboundClickStatusCreated
	}

	_, err := r.db.Exec(
		query,
		click.ClickID,
		click.Partner,
		click.CampaignSlug,
		click.OfferProductID,
		click.DestKey,
		click.DestURL,
		queryParamsJSON,
		click.ReferrerDomain,
		click.IPHash,
		click.UserAgentHash,
		string(click.Status),
		click.CreatedAt,
		click.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert outbound click: %w", err)
	}

	return nil
}

// GetByClickID retrieves an outbound click by its click_id
func (r *OutboundClickRepository) GetByClickID(clickID uuid.UUID) (*domain.OutboundClick, error) {
	query := `
		SELECT click_id, partner, campaign_slug, offer_product_id,
		       dest_key, dest_url, query_params,
		       referrer_domain, ip_hash, user_agent_hash,
		       status, created_at, updated_at
		FROM outbound_clicks
		WHERE click_id = $1
	`

	var click domain.OutboundClick
	var campaignSlug, referrerDomain, ipHash, userAgentHash sql.NullString
	var offerProductID sql.NullInt64
	var queryParamsJSON []byte
	var status string

	err := r.db.QueryRow(query, clickID).Scan(
		&click.ClickID,
		&click.Partner,
		&campaignSlug,
		&offerProductID,
		&click.DestKey,
		&click.DestURL,
		&queryParamsJSON,
		&referrerDomain,
		&ipHash,
		&userAgentHash,
		&status,
		&click.CreatedAt,
		&click.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("outbound click not found: %s", clickID.String())
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get outbound click: %w", err)
	}

	// Map nullable fields
	if campaignSlug.Valid {
		click.CampaignSlug = &campaignSlug.String
	}
	if offerProductID.Valid {
		id := int(offerProductID.Int64)
		click.OfferProductID = &id
	}
	if referrerDomain.Valid {
		click.ReferrerDomain = &referrerDomain.String
	}
	if ipHash.Valid {
		click.IPHash = &ipHash.String
	}
	if userAgentHash.Valid {
		click.UserAgentHash = &userAgentHash.String
	}

	click.Status = domain.OutboundClickStatus(status)

	// Unmarshal query_params
	if len(queryParamsJSON) > 0 {
		if err := json.Unmarshal(queryParamsJSON, &click.QueryParams); err != nil {
			r.logger.Warn("Failed to unmarshal query_params", zap.Error(err))
		}
	}

	return &click, nil
}

// UpdateStatus updates the status of an outbound click
func (r *OutboundClickRepository) UpdateStatus(clickID uuid.UUID, status domain.OutboundClickStatus) error {
	query := `
		UPDATE outbound_clicks
		SET status = $1, updated_at = $2
		WHERE click_id = $3
	`

	result, err := r.db.Exec(query, string(status), time.Now(), clickID)
	if err != nil {
		return fmt.Errorf("failed to update outbound click status: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("outbound click not found: %s", clickID.String())
	}

	return nil
}

// CountByPartnerSince counts clicks for rate limiting
func (r *OutboundClickRepository) CountByPartnerSince(partner string, ipHash string, since time.Time) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM outbound_clicks
		WHERE partner = $1 AND ip_hash = $2 AND created_at >= $3
	`

	var count int
	err := r.db.QueryRow(query, partner, ipHash, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count outbound clicks: %w", err)
	}

	return count, nil
}
