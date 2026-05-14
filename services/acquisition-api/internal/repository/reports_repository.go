package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"go.uber.org/zap"
)

// ReportsRepository handles reporting data aggregation.
type ReportsRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewReportsRepository creates a new reports repository.
func NewReportsRepository(db *sql.DB, logger *zap.Logger) *ReportsRepository {
	return &ReportsRepository{
		db:     db,
		logger: logger,
	}
}

// ChannelBelongsToTenant verifies that a channel is available in the tenant's catalog.
func (r *ReportsRepository) ChannelBelongsToTenant(tenantID, channelID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM tenant_channels
			WHERE tenant_id = $1::uuid
			  AND id = $2::uuid
			  AND status = 'ACTIVE'
		)
	`, tenantID, channelID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to validate tenant channel: %w", err)
	}
	return exists, nil
}

// TenantIDByKey resolves a public tenant key to its tenant UUID for scoped reporting.
func (r *ReportsRepository) TenantIDByKey(tenantKey string) (string, error) {
	var tenantID string
	err := r.db.QueryRow(`
		SELECT id
		FROM tenants
		WHERE tenant_key = $1
		  AND status = 'ACTIVE'
	`, tenantKey).Scan(&tenantID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", ErrAdminNotFound
		}
		return "", fmt.Errorf("failed to resolve tenant by key: %w", err)
	}
	return tenantID, nil
}

// GetKPIs retrieves aggregated KPIs for the given filters.
func (r *ReportsRepository) GetKPIs(filters domain.ReportFilters) (*domain.KPIsResponse, error) {
	kpis := &domain.KPIsResponse{Filters: filters}

	landingQuery, landingArgs := buildLandingAggregateQuery(filters)
	if err := r.db.QueryRow(landingQuery, landingArgs...).Scan(&kpis.LandingViews, &kpis.LandingClicks); err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get landing events: %w", err)
	}

	txQuery, txArgs := buildTransactionAggregateQuery(filters)
	if err := r.db.QueryRow(txQuery, txArgs...).Scan(&kpis.Transactions, &kpis.Subscribed, &kpis.Charged); err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get transaction counts: %w", err)
	}

	revenueQuery, revenueArgs := buildRevenueAggregateQuery(filters)
	if err := r.db.QueryRow(revenueQuery, revenueArgs...).Scan(&kpis.EstimatedRevenue); err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to calculate revenue: %w", err)
	}

	if kpis.LandingViews > 0 {
		kpis.ViewToClickRate = float64(kpis.LandingClicks) / float64(kpis.LandingViews) * 100
		kpis.OverallConversionRate = float64(kpis.Charged) / float64(kpis.LandingViews) * 100
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

	return kpis, nil
}

// GetAcquisitionFunnel retrieves funnel data.
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

// GetCampaignPerformance retrieves per-campaign performance metrics.
func (r *ReportsRepository) GetCampaignPerformance(filters domain.ReportFilters) (*domain.CampaignPerformanceResponse, error) {
	args := []interface{}{filters.StartDate, filters.EndDate}
	campaignWhere := []string{"c.enabled = true"}
	addCampaignFilters(&campaignWhere, &args, filters, "c")

	query := fmt.Sprintf(`
		SELECT
			c.slug,
			c.country,
			COALESCE((
				SELECT COUNT(*)
				FROM landing_events le
				WHERE le.campaign_slug = c.slug
				  AND le.created_at >= $1 AND le.created_at < $2
				  AND (%s)
			), 0) AS views,
			COALESCE((
				SELECT COUNT(*)
				FROM acquisition_transactions at
				WHERE at.campaign_slug = c.slug
				  AND at.created_at >= $1 AND at.created_at < $2
				  AND (%s)
			), 0) AS transactions,
			COALESCE((
				SELECT COUNT(*)
				FROM acquisition_transactions at
				WHERE at.campaign_slug = c.slug
				  AND at.status IN ('SUBSCRIBED', 'CHARGED')
				  AND at.created_at >= $1 AND at.created_at < $2
				  AND (%s)
			), 0) AS subscribed,
			COALESCE((
				SELECT COUNT(*)
				FROM acquisition_transactions at
				WHERE at.campaign_slug = c.slug
				  AND at.status = 'CHARGED'
				  AND at.created_at >= $1 AND at.created_at < $2
				  AND (%s)
			), 0) AS charged,
			COALESCE((
				SELECT SUM(
					CASE
						WHEN at.charge_payout IS NOT NULL
						  AND at.charge_payout > 0
						THEN at.charge_payout
						ELSE COALESCE(c.price, 0)
					END
				)
				FROM acquisition_transactions at
				WHERE at.campaign_slug = c.slug
				  AND at.status = 'CHARGED'
				  AND at.created_at >= $1 AND at.created_at < $2
				  AND (%s)
			), 0) AS revenue
		FROM campaigns c
		WHERE %s
		ORDER BY views DESC
	`, campaignJoinPredicate("c", "le"), transactionCampaignPredicate("c", "at"), transactionCampaignPredicate("c", "at"), transactionCampaignPredicate("c", "at"), transactionCampaignPredicate("c", "at"), joinConditions(campaignWhere, " AND "))

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get campaign performance: %w", err)
	}
	defer rows.Close()

	campaigns := make([]domain.CampaignPerformance, 0)
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
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read campaign performance: %w", err)
	}

	return &domain.CampaignPerformanceResponse{
		Filters:   filters,
		Campaigns: campaigns,
	}, nil
}

// GetTimeSeries retrieves time series data.
func (r *ReportsRepository) GetTimeSeries(filters domain.ReportFilters, interval string) (*domain.TimeSeriesResponse, error) {
	truncFunc := "day"
	if interval == "hourly" {
		truncFunc = "hour"
	}

	landingQuery, landingArgs := buildLandingTimeSeriesQuery(filters, truncFunc)
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
	if err := landingRows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read landing time series: %w", err)
	}

	txQuery, txArgs := buildTransactionTimeSeriesQuery(filters, truncFunc)
	txRows, err := r.db.Query(txQuery, txArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction time series: %w", err)
	}
	defer txRows.Close()

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
	if err := txRows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read transaction time series: %w", err)
	}

	for ts, views := range landingData {
		if pt, ok := dataMap[ts]; ok {
			pt.LandingViews = views
		} else {
			dataMap[ts] = &domain.TimeSeriesPoint{Timestamp: ts, LandingViews: views}
		}
	}

	dataPoints := make([]domain.TimeSeriesPoint, 0, len(dataMap))
	for _, pt := range dataMap {
		dataPoints = append(dataPoints, *pt)
	}
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

func buildLandingAggregateQuery(filters domain.ReportFilters) (string, []interface{}) {
	args := []interface{}{filters.StartDate, filters.EndDate}
	where := []string{"le.created_at >= $1", "le.created_at < $2"}
	addLandingFilters(&where, &args, filters, "le")
	return fmt.Sprintf(`
		SELECT
			COALESCE(SUM(CASE WHEN le.event_type = 'landing_view' THEN 1 ELSE 0 END), 0) AS views,
			COALESCE(SUM(CASE WHEN le.event_type = 'landing_click' THEN 1 ELSE 0 END), 0) AS clicks
		FROM landing_events le
		WHERE %s
	`, joinConditions(where, " AND ")), args
}

func buildTransactionAggregateQuery(filters domain.ReportFilters) (string, []interface{}) {
	args := []interface{}{filters.StartDate, filters.EndDate}
	where := []string{"at.created_at >= $1", "at.created_at < $2"}
	addTransactionFilters(&where, &args, filters, "at")
	return fmt.Sprintf(`
		SELECT
			COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN at.status = 'SUBSCRIBED' THEN 1 ELSE 0 END), 0) AS subscribed,
			COALESCE(SUM(CASE WHEN at.status = 'CHARGED' THEN 1 ELSE 0 END), 0) AS charged
		FROM acquisition_transactions at
		WHERE %s
	`, joinConditions(where, " AND ")), args
}

func buildRevenueAggregateQuery(filters domain.ReportFilters) (string, []interface{}) {
	args := []interface{}{filters.StartDate, filters.EndDate}
	where := []string{"at.status = 'CHARGED'", "at.created_at >= $1", "at.created_at < $2"}
	addTransactionFilters(&where, &args, filters, "at")
	return fmt.Sprintf(`
		SELECT COALESCE(SUM(
			CASE
				WHEN at.charge_payout IS NOT NULL
				  AND at.charge_payout > 0
				THEN at.charge_payout
				ELSE COALESCE(c.price, 0)
			END
		), 0) AS revenue
		FROM acquisition_transactions at
		LEFT JOIN campaigns c ON %s
		WHERE %s
	`, transactionCampaignPredicate("c", "at"), joinConditions(where, " AND ")), args
}

func buildLandingTimeSeriesQuery(filters domain.ReportFilters, truncFunc string) (string, []interface{}) {
	args := []interface{}{filters.StartDate, filters.EndDate}
	where := []string{"le.created_at >= $1", "le.created_at < $2"}
	addLandingFilters(&where, &args, filters, "le")
	return fmt.Sprintf(`
		SELECT
			date_trunc('%s', le.created_at) AS ts,
			COUNT(*) FILTER (WHERE le.event_type = 'landing_view') AS views
		FROM landing_events le
		WHERE %s
		GROUP BY date_trunc('%s', le.created_at)
		ORDER BY ts
	`, truncFunc, joinConditions(where, " AND "), truncFunc), args
}

func buildTransactionTimeSeriesQuery(filters domain.ReportFilters, truncFunc string) (string, []interface{}) {
	args := []interface{}{filters.StartDate, filters.EndDate}
	where := []string{"at.created_at >= $1", "at.created_at < $2"}
	addTransactionFilters(&where, &args, filters, "at")
	return fmt.Sprintf(`
		SELECT
			date_trunc('%s', at.created_at) AS ts,
			COUNT(*) AS transactions,
			COUNT(*) FILTER (WHERE at.status IN ('SUBSCRIBED', 'CHARGED')) AS subscribed,
			COUNT(*) FILTER (WHERE at.status = 'CHARGED') AS charged,
			COALESCE(SUM(
				CASE
					WHEN at.status = 'CHARGED'
					  AND at.charge_payout IS NOT NULL
					  AND at.charge_payout > 0
					THEN at.charge_payout
					ELSE 0
				END
			), 0) AS revenue
		FROM acquisition_transactions at
		WHERE %s
		GROUP BY date_trunc('%s', at.created_at)
		ORDER BY ts
	`, truncFunc, joinConditions(where, " AND "), truncFunc), args
}

func addLandingFilters(where *[]string, args *[]interface{}, filters domain.ReportFilters, landingAlias string) {
	if filters.CampaignSlug != nil {
		*args = append(*args, *filters.CampaignSlug)
		*where = append(*where, fmt.Sprintf("%s.campaign_slug = $%d", landingAlias, len(*args)))
	}
	if filters.TenantID != nil || filters.ChannelID != nil || filters.Country != nil {
		campaignWhere := []string{campaignJoinPredicate("c", landingAlias)}
		addCampaignFilters(&campaignWhere, args, filters, "c")
		*where = append(*where, fmt.Sprintf("EXISTS (SELECT 1 FROM campaigns c WHERE %s)", joinConditions(campaignWhere, " AND ")))
	}
}

func addTransactionFilters(where *[]string, args *[]interface{}, filters domain.ReportFilters, txAlias string) {
	if filters.TenantID != nil {
		*args = append(*args, *filters.TenantID)
		*where = append(*where, fmt.Sprintf("%s.tenant_id = $%d::uuid", txAlias, len(*args)))
	}
	if filters.CampaignSlug != nil {
		*args = append(*args, *filters.CampaignSlug)
		*where = append(*where, fmt.Sprintf("%s.campaign_slug = $%d", txAlias, len(*args)))
	}
	if filters.ChannelID != nil || filters.Country != nil {
		campaignWhere := []string{transactionCampaignPredicate("c", txAlias)}
		addCampaignFilters(&campaignWhere, args, filters, "c")
		*where = append(*where, fmt.Sprintf("EXISTS (SELECT 1 FROM campaigns c WHERE %s)", joinConditions(campaignWhere, " AND ")))
	}
}

func addCampaignFilters(where *[]string, args *[]interface{}, filters domain.ReportFilters, campaignAlias string) {
	if filters.TenantID != nil {
		*args = append(*args, *filters.TenantID)
		*where = append(*where, fmt.Sprintf("%s.tenant_id = $%d::uuid", campaignAlias, len(*args)))
	}
	if filters.ChannelID != nil {
		*args = append(*args, *filters.ChannelID)
		*where = append(*where, fmt.Sprintf("%s.channel_id = $%d::uuid", campaignAlias, len(*args)))
	}
	if filters.CampaignSlug != nil {
		*args = append(*args, *filters.CampaignSlug)
		*where = append(*where, fmt.Sprintf("%s.slug = $%d", campaignAlias, len(*args)))
	}
	if filters.Country != nil {
		*args = append(*args, *filters.Country)
		*where = append(*where, fmt.Sprintf("%s.country = $%d", campaignAlias, len(*args)))
	}
}

func campaignJoinPredicate(campaignAlias, landingAlias string) string {
	return fmt.Sprintf("%s.slug = %s.campaign_slug", campaignAlias, landingAlias)
}

func transactionCampaignPredicate(campaignAlias, txAlias string) string {
	return fmt.Sprintf("%s.slug = %s.campaign_slug AND %s.tenant_id = %s.tenant_id", campaignAlias, txAlias, campaignAlias, txAlias)
}

func joinConditions(conditions []string, sep string) string {
	if len(conditions) == 0 {
		return "1=1"
	}
	out := conditions[0]
	for i := 1; i < len(conditions); i++ {
		out += sep + conditions[i]
	}
	return out
}
