package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"go.uber.org/zap"
)

// LandingEventRepository handles landing event data access
type LandingEventRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewLandingEventRepository creates a new landing event repository
func NewLandingEventRepository(db *sql.DB, logger *zap.Logger) *LandingEventRepository {
	return &LandingEventRepository{
		db:     db,
		logger: logger,
	}
}

// Create inserts a new landing event
func (r *LandingEventRepository) Create(event *domain.LandingEvent) error {
	query := `
		INSERT INTO landing_events (
			event_type, campaign_slug, click_id, ad_provider, session_id,
			ip_hash, user_agent_hash, referrer_domain, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`

	var clickID, adProvider, sessionID, ipHash, userAgentHash, referrerDomain sql.NullString

	if event.ClickID != nil {
		clickID.String = *event.ClickID
		clickID.Valid = true
	}
	if event.AdProvider != nil {
		adProvider.String = *event.AdProvider
		adProvider.Valid = true
	}
	if event.SessionID != nil {
		sessionID.String = *event.SessionID
		sessionID.Valid = true
	}
	if event.IPHash != nil {
		ipHash.String = *event.IPHash
		ipHash.Valid = true
	}
	if event.UserAgentHash != nil {
		userAgentHash.String = *event.UserAgentHash
		userAgentHash.Valid = true
	}
	if event.ReferrerDomain != nil {
		referrerDomain.String = *event.ReferrerDomain
		referrerDomain.Valid = true
	}

	err := r.db.QueryRow(query,
		event.EventType,
		event.CampaignSlug,
		clickID,
		adProvider,
		sessionID,
		ipHash,
		userAgentHash,
		referrerDomain,
		event.CreatedAt,
	).Scan(&event.ID)

	if err != nil {
		return fmt.Errorf("failed to create landing event: %w", err)
	}

	return nil
}

// CountByTypeAndCampaign counts events by type for a campaign within a date range
func (r *LandingEventRepository) CountByTypeAndCampaign(
	eventType domain.LandingEventType,
	campaignSlug string,
	startDate, endDate time.Time,
) (int64, error) {
	query := `
		SELECT COUNT(*) 
		FROM landing_events 
		WHERE event_type = $1 
		  AND campaign_slug = $2 
		  AND created_at >= $3 
		  AND created_at < $4
	`

	var count int64
	err := r.db.QueryRow(query, eventType, campaignSlug, startDate, endDate).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count landing events: %w", err)
	}

	return count, nil
}

// CountByType counts all events of a type within a date range
func (r *LandingEventRepository) CountByType(
	eventType domain.LandingEventType,
	startDate, endDate time.Time,
) (int64, error) {
	query := `
		SELECT COUNT(*) 
		FROM landing_events 
		WHERE event_type = $1 
		  AND created_at >= $3 
		  AND created_at < $4
	`

	var count int64
	err := r.db.QueryRow(query, eventType, startDate, endDate).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count landing events: %w", err)
	}

	return count, nil
}

// GetDailyCountsByCampaign returns daily event counts for a campaign
func (r *LandingEventRepository) GetDailyCountsByCampaign(
	campaignSlug string,
	startDate, endDate time.Time,
) ([]EventDailyCount, error) {
	query := `
		SELECT 
			DATE(created_at) as date,
			event_type,
			COUNT(*) as count
		FROM landing_events
		WHERE campaign_slug = $1
		  AND created_at >= $2
		  AND created_at < $3
		GROUP BY DATE(created_at), event_type
		ORDER BY date
	`

	rows, err := r.db.Query(query, campaignSlug, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily counts: %w", err)
	}
	defer rows.Close()

	var results []EventDailyCount
	for rows.Next() {
		var result EventDailyCount
		if err := rows.Scan(&result.Date, &result.EventType, &result.Count); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		results = append(results, result)
	}

	return results, nil
}

// GetDailyCounts returns daily event counts across all campaigns
func (r *LandingEventRepository) GetDailyCounts(
	startDate, endDate time.Time,
) ([]EventDailyCount, error) {
	query := `
		SELECT 
			DATE(created_at) as date,
			event_type,
			COUNT(*) as count
		FROM landing_events
		WHERE created_at >= $1
		  AND created_at < $2
		GROUP BY DATE(created_at), event_type
		ORDER BY date
	`

	rows, err := r.db.Query(query, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily counts: %w", err)
	}
	defer rows.Close()

	var results []EventDailyCount
	for rows.Next() {
		var result EventDailyCount
		if err := rows.Scan(&result.Date, &result.EventType, &result.Count); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		results = append(results, result)
	}

	return results, nil
}

// EventDailyCount holds daily count data
type EventDailyCount struct {
	Date      time.Time                `json:"date"`
	EventType domain.LandingEventType `json:"event_type"`
	Count     int64                    `json:"count"`
}
