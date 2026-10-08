package domain

import "time"

// ReportFilters represents common filters for reporting queries
type ReportFilters struct {
	StartDate    time.Time `json:"start_date"`
	EndDate      time.Time `json:"end_date"`
	TenantID     *string   `json:"tenant_id,omitempty"`
	ChannelID    *string   `json:"channel_id,omitempty"`
	CampaignSlug *string   `json:"campaign_slug,omitempty"`
	Country      *string   `json:"country,omitempty"`
	AllTenants   bool      `json:"all_tenants,omitempty"`
}

// KPIsResponse represents the KPIs response
type KPIsResponse struct {
	Filters          ReportFilters `json:"filters"`
	LandingViews     int64         `json:"landing_views"`
	LandingClicks    int64         `json:"landing_clicks"`
	Transactions     int64         `json:"transactions"`
	Subscribed       int64         `json:"subscribed"`
	Charged          int64         `json:"charged"`
	EstimatedRevenue float64       `json:"estimated_revenue"`

	// Conversion rates (percentages)
	ViewToClickRate        float64 `json:"view_to_click_rate"`
	ClickToTransactionRate float64 `json:"click_to_transaction_rate"`
	TransactionToSubRate   float64 `json:"transaction_to_sub_rate"`
	SubToChargedRate       float64 `json:"sub_to_charged_rate"`
	OverallConversionRate  float64 `json:"overall_conversion_rate"` // view → charged
}

// AcquisitionFunnelResponse represents the acquisition funnel response
type AcquisitionFunnelResponse struct {
	Filters ReportFilters `json:"filters"`
	Stages  []FunnelStage `json:"stages"`
}

// FunnelStage represents one stage in the funnel
type FunnelStage struct {
	Name           string  `json:"name"`
	Count          int64   `json:"count"`
	DropoffPercent float64 `json:"dropoff_percent"` // % lost from previous stage
}

// CampaignPerformanceResponse represents campaign performance data
type CampaignPerformanceResponse struct {
	Filters   ReportFilters         `json:"filters"`
	Campaigns []CampaignPerformance `json:"campaigns"`
}

// CampaignPerformance represents performance metrics for a single campaign
type CampaignPerformance struct {
	CampaignSlug     string  `json:"campaign_slug"`
	Country          string  `json:"country"`
	LandingViews     int64   `json:"landing_views"`
	Transactions     int64   `json:"transactions"`
	Subscribed       int64   `json:"subscribed"`
	Charged          int64   `json:"charged"`
	EstimatedRevenue float64 `json:"estimated_revenue"`
	ConversionRate   float64 `json:"conversion_rate"` // view → charged
}

// TimeSeriesResponse represents time series data for charts
type TimeSeriesResponse struct {
	Filters    ReportFilters     `json:"filters"`
	Interval   string            `json:"interval"` // "daily" or "hourly"
	DataPoints []TimeSeriesPoint `json:"data_points"`
}

// TimeSeriesPoint represents a single data point in a time series
type TimeSeriesPoint struct {
	Timestamp        time.Time `json:"timestamp"`
	LandingViews     int64     `json:"landing_views"`
	Transactions     int64     `json:"transactions"`
	Subscribed       int64     `json:"subscribed"`
	Charged          int64     `json:"charged"`
	EstimatedRevenue float64   `json:"estimated_revenue"`
}

// SubscriptionHealthResponse keeps local state and provider billing observations separate.
// Provider revenue is a decimal string because the report does not declare a currency.
type SubscriptionHealthResponse struct {
	AsOf          time.Time               `json:"as_of"`
	Timezone      string                  `json:"timezone"`
	Subscriptions SubscriptionStateCounts `json:"subscriptions"`
	Provider      TIMWEProviderSummary    `json:"provider"`
}

type SubscriptionStateCounts struct {
	Active   int64 `json:"active"`
	Inactive int64 `json:"inactive"`
	Other    int64 `json:"other"`
	Total    int64 `json:"total"`
}

type TIMWEProviderSummary struct {
	State           string           `json:"state"`
	LastImportedAt  *time.Time       `json:"last_imported_at"`
	SuccessBillings *int64           `json:"success_billings"`
	Revenue         *string          `json:"revenue"`
	Currency        *string          `json:"currency"`
	Daily           []TIMWEDailyItem `json:"daily"`
}

type TIMWEDailyItem struct {
	Date            string `json:"date"`
	Shortcode       string `json:"shortcode"`
	ProductID       int    `json:"product_id"`
	ProductName     string `json:"product_name"`
	PricepointID    int    `json:"pricepoint_id"`
	SuccessBillings int64  `json:"success_billings"`
	Revenue         string `json:"revenue"`
}
