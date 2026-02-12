package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"go.uber.org/zap"
)

// ReportsRepository handles reporting data aggregation
type ReportsRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewReportsRepository creates a new reports repository
func NewReportsRepository(db *sql.DB, logger *zap.Logger) *ReportsRepository {
	return &ReportsRepository{
		db:     db,
		logger: logger,
	}
}

// GetKPIs retrieves aggregated KPIs for the given filters
func (r *ReportsRepository) GetKPIs(filters domain.ReportFilters) (*domain.KPIsResponse, error) {
	kpis := &domain.KPIsResponse{
		Filters: filters,
	}

	// Get landing event counts
	landingQuery := `
		SELECT 
			COALESCE(SUM(CASE WHEN event_type = 'landing_view' THEN 1 ELSE 0 END), 0) as views,
			COALESCE(SUM(CASE WHEN event_type = 'landing_click' THEN 1 ELSE 0 END), 0) as clicks
		FROM landing_events
		WHERE created_at >= $1 AND created_at < $2
	`
	args := []interface{}{filters.StartDate, filters.EndDate}
	argIdx := 3

	if filters.CampaignSlug != nil {
		landingQuery = `
			SELECT 
				COALESCE(SUM(CASE WHEN event_type = 'landing_view' THEN 1 ELSE 0 END), 0) as views,
				COALESCE(SUM(CASE WHEN event_type = 'landing_click' THEN 1 ELSE 0 END), 0) as clicks
			FROM landing_events
			WHERE created_at >= $1 AND created_at < $2 AND campaign_slug = $3
		`
		args = append(args, *filters.CampaignSlug)
		argIdx++
	}

	err := r.db.QueryRow(landingQuery, args...).Scan(&kpis.LandingViews, &kpis.LandingClicks)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get landing events: %w", err)
	}

	// Get transaction counts
	txQuery := `
		SELECT 
			COUNT(*) as total,
			COALESCE(SUM(CASE WHEN status = 'SUBSCRIBED' THEN 1 ELSE 0 END), 0) as subscribed,
			COALESCE(SUM(CASE WHEN status = 'CHARGED' THEN 1 ELSE 0 END), 0) as charged
		FROM acquisition_transactions
		WHERE created_at >= $1 AND created_at < $2
	`
	txArgs := []interface{}{filters.StartDate, filters.EndDate}

	if filters.CampaignSlug != nil {
		txQuery = `
			SELECT 
				COUNT(*) as total,
				COALESCE(SUM(CASE WHEN status = 'SUBSCRIBED' THEN 1 ELSE 0 END), 0) as subscribed,
				COALESCE(SUM(CASE WHEN status = 'CHARGED' THEN 1 ELSE 0 END), 0) as charged
			FROM acquisition_transactions
			WHERE created_at >= $1 AND created_at < $2 AND campaign_slug = $3
		`
		txArgs = append(txArgs, *filters.CampaignSlug)
	}

	err = r.db.QueryRow(txQuery, txArgs...).Scan(&kpis.Transactions, &kpis.Subscribed, &kpis.Charged)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get transaction counts: %w", err)
	}

	// Get estimated revenue
	revenueQuery := `
		SELECT COALESCE(SUM(
			CASE 
				WHEN at.charge_payout > 0 
				THEN at.charge_payout
				ELSE COALESCE(c.price, 0)
			END
		), 0) as revenue
		FROM acquisition_transactions at
		LEFT JOIN campaigns c ON at.campaign_slug = c.slug
		WHERE at.status = 'CHARGED'
		  AND at.created_at >= $1 AND at.created_at < $2
	`
	revenueArgs := []interface{}{filters.StartDate, filters.EndDate}

	if filters.CampaignSlug != nil {
		revenueQuery = `
			SELECT COALESCE(SUM(
				CASE 
					WHEN at.charge_payout > 0 
					THEN at.charge_payout
					ELSE COALESCE(c.price, 0)
				END
			), 0) as revenue
			FROM acquisition_transactions at
			LEFT JOIN campaigns c ON at.campaign_slug = c.slug
			WHERE at.status = 'CHARGED'
			  AND at.created_at >= $1 AND at.created_at < $2
			  AND at.campaign_slug = $3
		`
		revenueArgs = append(revenueArgs, *filters.CampaignSlug)
	}

	err = r.db.QueryRow(revenueQuery, revenueArgs...).Scan(&kpis.EstimatedRevenue)
	if err != nil && err != sql.ErrNoRows {
		r.logger.Warn("Failed to calculate revenue, using 0", zap.Error(err))
		kpis.EstimatedRevenue = 0
	}

	// Calculate conversion rates
	if kpis.LandingViews > 0 {
		kpis.ViewToClickRate = float64(kpis.LandingClicks) / float64(kpis.LandingViews) * 100
	}
	if kpis.LandingClicks > 0 {
		kpis.ClickToTransactionRate = float64(kpis.Transactions) / float64(kpis.LandingClicks) * 100
	}
	if kpis.Transactions > 0 {
		kpis.TransactionToSubRate = float64(kpis.Subscribed) / float64(kpis.Transactions) * 100
	}
	if kpis.Subscribed > 0 {
		kpis.SubToChargedRate = float64(kpis.Charged) / float64(kpis.Subscribed) * 100
	}
	if kpis.LandingViews > 0 {
		kpis.OverallConversionRate = float64(kpis.Charged) / float64(kpis.LandingViews) * 100
	}

	return kpis, nil
}

// GetAcquisitionFunnel retrieves funnel data
func (r *ReportsRepository) GetAcquisitionFunnel(filters domain.ReportFilters) (*domain.AcquisitionFunnelResponse, error) {
	kpis, err := r.GetKPIs(filters)
	if err != nil {
		return nil, err
	}

	stages := []domain.FunnelStage{
		{Name: "Landing Views", Count: kpis.LandingViews, DropoffPercent: 0},
		{Name: "Landing Clicks", Count: kpis.LandingClicks, DropoffPercent: calcDropoff(kpis.LandingViews, kpis.LandingClicks)},
		{Name: "Transactions", Count: kpis.Transactions, DropoffPercent: calcDropoff(kpis.LandingClicks, kpis.Transactions)},
		{Name: "Subscribed", Count: kpis.Subscribed, DropoffPercent: calcDropoff(kpis.Transactions, kpis.Subscribed)},
		{Name: "Charged", Count: kpis.Charged, DropoffPercent: calcDropoff(kpis.Subscribed, kpis.Charged)},
	}

	return &domain.AcquisitionFunnelResponse{
		Filters: filters,
		Stages:  stages,
	}, nil
}

func calcDropoff(prev, curr int64) float64 {
	if prev == 0 {
		return 0
	}
	return float64(prev-curr) / float64(prev) * 100
}

// GetCampaignPerformance retrieves per-campaign performance metrics
func (r *ReportsRepository) GetCampaignPerformance(filters domain.ReportFilters) (*domain.CampaignPerformanceResponse, error) {
	query := `
		WITH landing_stats AS (
			SELECT 
				campaign_slug,
				COUNT(*) FILTER (WHERE event_type = 'landing_view') as views
			FROM landing_events
			WHERE created_at >= $1 AND created_at < $2
			GROUP BY campaign_slug
		),
		tx_stats AS (
			SELECT 
				campaign_slug,
				COUNT(*) as transactions,
				COUNT(*) FILTER (WHERE status IN ('SUBSCRIBED', 'CHARGED')) as subscribed,
				COUNT(*) FILTER (WHERE status = 'CHARGED') as charged
			FROM acquisition_transactions
			WHERE created_at >= $1 AND created_at < $2
			GROUP BY campaign_slug
		),
		revenue_stats AS (
			SELECT 
				at.campaign_slug,
				COALESCE(SUM(
					CASE 
						WHEN at.charge_payout > 0 
						THEN at.charge_payout
						ELSE COALESCE(c.price, 0)
					END
				), 0) as revenue
			FROM acquisition_transactions at
			LEFT JOIN campaigns c ON at.campaign_slug = c.slug
			WHERE at.status = 'CHARGED'
			  AND at.created_at >= $1 AND at.created_at < $2
			GROUP BY at.campaign_slug
		)
		SELECT 
			c.slug,
			c.country,
			COALESCE(ls.views, 0) as views,
			COALESCE(ts.transactions, 0) as transactions,
			COALESCE(ts.subscribed, 0) as subscribed,
			COALESCE(ts.charged, 0) as charged,
			COALESCE(rs.revenue, 0) as revenue
		FROM campaigns c
		LEFT JOIN landing_stats ls ON c.slug = ls.campaign_slug
		LEFT JOIN tx_stats ts ON c.slug = ts.campaign_slug
		LEFT JOIN revenue_stats rs ON c.slug = rs.campaign_slug
		WHERE c.enabled = true
		ORDER BY COALESCE(ls.views, 0) DESC
	`

	rows, err := r.db.Query(query, filters.StartDate, filters.EndDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get campaign performance: %w", err)
	}
	defer rows.Close()

	var campaigns []domain.CampaignPerformance
	for rows.Next() {
		var cp domain.CampaignPerformance
		if err := rows.Scan(
			&cp.CampaignSlug,
			&cp.Country,
			&cp.LandingViews,
			&cp.Transactions,
			&cp.Subscribed,
			&cp.Charged,
			&cp.EstimatedRevenue,
		); err != nil {
			return nil, fmt.Errorf("failed to scan campaign performance: %w", err)
		}
		if cp.LandingViews > 0 {
			cp.ConversionRate = float64(cp.Charged) / float64(cp.LandingViews) * 100
		}
		campaigns = append(campaigns, cp)
	}

	return &domain.CampaignPerformanceResponse{
		Filters:   filters,
		Campaigns: campaigns,
	}, nil
}

// GetTimeSeries retrieves time series data
func (r *ReportsRepository) GetTimeSeries(filters domain.ReportFilters, interval string) (*domain.TimeSeriesResponse, error) {
	// Determine date truncation based on interval
	truncFunc := "day"
	if interval == "hourly" {
		truncFunc = "hour"
	}

	// Get landing events time series
	landingQuery := fmt.Sprintf(`
		SELECT 
			date_trunc('%s', created_at) as ts,
			COUNT(*) FILTER (WHERE event_type = 'landing_view') as views
		FROM landing_events
		WHERE created_at >= $1 AND created_at < $2
	`, truncFunc)
	
	if filters.CampaignSlug != nil {
		landingQuery += ` AND campaign_slug = $3`
	}
	landingQuery += fmt.Sprintf(` GROUP BY date_trunc('%s', created_at) ORDER BY ts`, truncFunc)

	landingArgs := []interface{}{filters.StartDate, filters.EndDate}
	if filters.CampaignSlug != nil {
		landingArgs = append(landingArgs, *filters.CampaignSlug)
	}

	// Get transaction time series
	txQuery := fmt.Sprintf(`
		SELECT 
			date_trunc('%s', created_at) as ts,
			COUNT(*) as transactions,
			COUNT(*) FILTER (WHERE status IN ('SUBSCRIBED', 'CHARGED')) as subscribed,
			COUNT(*) FILTER (WHERE status = 'CHARGED') as charged,
			COALESCE(SUM(
				CASE 
					WHEN status = 'CHARGED' AND charge_payout > 0 
					THEN charge_payout
					ELSE 0
				END
			), 0) as revenue
		FROM acquisition_transactions
		WHERE created_at >= $1 AND created_at < $2
	`, truncFunc)

	if filters.CampaignSlug != nil {
		txQuery += ` AND campaign_slug = $3`
	}
	txQuery += fmt.Sprintf(` GROUP BY date_trunc('%s', created_at) ORDER BY ts`, truncFunc)

	txArgs := []interface{}{filters.StartDate, filters.EndDate}
	if filters.CampaignSlug != nil {
		txArgs = append(txArgs, *filters.CampaignSlug)
	}

	// Execute landing events query
	landingRows, err := r.db.Query(landingQuery, landingArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to get landing time series: %w", err)
	}
	defer landingRows.Close()

	landingData := make(map[time.Time]int64)
	for landingRows.Next() {
		var ts time.Time
		var views int64
		if err := landingRows.Scan(&ts, &views); err != nil {
			return nil, fmt.Errorf("failed to scan landing row: %w", err)
		}
		landingData[ts] = views
	}

	// Execute transactions query
	txRows, err := r.db.Query(txQuery, txArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction time series: %w", err)
	}
	defer txRows.Close()

	// Merge data
	dataMap := make(map[time.Time]*domain.TimeSeriesPoint)
	for txRows.Next() {
		var ts time.Time
		var transactions, subscribed, charged int64
		var revenue float64
		if err := txRows.Scan(&ts, &transactions, &subscribed, &charged, &revenue); err != nil {
			return nil, fmt.Errorf("failed to scan tx row: %w", err)
		}
		dataMap[ts] = &domain.TimeSeriesPoint{
			Timestamp:        ts,
			Transactions:     transactions,
			Subscribed:       subscribed,
			Charged:          charged,
			EstimatedRevenue: revenue,
		}
	}

	// Merge landing data
	for ts, views := range landingData {
		if pt, ok := dataMap[ts]; ok {
			pt.LandingViews = views
		} else {
			dataMap[ts] = &domain.TimeSeriesPoint{
				Timestamp:    ts,
				LandingViews: views,
			}
		}
	}

	// Convert to slice and sort
	var dataPoints []domain.TimeSeriesPoint
	for _, pt := range dataMap {
		dataPoints = append(dataPoints, *pt)
	}

	// Sort by timestamp
	for i := 0; i < len(dataPoints); i++ {
		for j := i + 1; j < len(dataPoints); j++ {
			if dataPoints[i].Timestamp.After(dataPoints[j].Timestamp) {
				dataPoints[i], dataPoints[j] = dataPoints[j], dataPoints[i]
			}
		}
	}

	return &domain.TimeSeriesResponse{
		Filters:    filters,
		Interval:   interval,
		DataPoints: dataPoints,
	}, nil
}
