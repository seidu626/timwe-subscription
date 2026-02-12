package handler

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"time"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// ReportsHandler handles admin reporting endpoints
type ReportsHandler struct {
	reportsRepo *repository.ReportsRepository
	logger      *zap.Logger
}

// NewReportsHandler creates a new reports handler
func NewReportsHandler(
	reportsRepo *repository.ReportsRepository,
	logger *zap.Logger,
) *ReportsHandler {
	return &ReportsHandler{
		reportsRepo: reportsRepo,
		logger:      logger,
	}
}

// parseFilters extracts common report filters from query parameters
func (h *ReportsHandler) parseFilters(ctx *fasthttp.RequestCtx) (domain.ReportFilters, error) {
	filters := domain.ReportFilters{}

	// Parse start date (default: 30 days ago)
	startDateStr := string(ctx.QueryArgs().Peek("startDate"))
	if startDateStr == "" {
		filters.StartDate = time.Now().UTC().AddDate(0, 0, -30).Truncate(24 * time.Hour)
	} else {
		t, err := time.Parse("2006-01-02", startDateStr)
		if err != nil {
			return filters, &domain.ValidationError{Field: "startDate", Message: "invalid date format; use YYYY-MM-DD"}
		}
		filters.StartDate = t
	}

	// Parse end date (default: now)
	endDateStr := string(ctx.QueryArgs().Peek("endDate"))
	if endDateStr == "" {
		filters.EndDate = time.Now().UTC().AddDate(0, 0, 1).Truncate(24 * time.Hour) // Include today
	} else {
		t, err := time.Parse("2006-01-02", endDateStr)
		if err != nil {
			return filters, &domain.ValidationError{Field: "endDate", Message: "invalid date format; use YYYY-MM-DD"}
		}
		filters.EndDate = t.AddDate(0, 0, 1) // Include end date
	}

	// Validate date range
	if filters.EndDate.Before(filters.StartDate) {
		return filters, &domain.ValidationError{Field: "endDate", Message: "endDate must be after startDate"}
	}

	// Optional campaign slug filter
	campaignSlug := string(ctx.QueryArgs().Peek("campaignSlug"))
	if campaignSlug != "" {
		filters.CampaignSlug = &campaignSlug
	}

	// Optional country filter
	country := string(ctx.QueryArgs().Peek("country"))
	if country != "" {
		filters.Country = &country
	}

	return filters, nil
}

// GetKPIs handles GET /v1/admin/reports/kpis
func (h *ReportsHandler) GetKPIs(ctx *fasthttp.RequestCtx) {
	filters, err := h.parseFilters(ctx)
	if err != nil {
		h.errorResponse(ctx, err.Error(), fasthttp.StatusBadRequest)
		return
	}

	kpis, err := h.reportsRepo.GetKPIs(filters)
	if err != nil {
		h.logger.Error("Failed to get KPIs", zap.Error(err))
		h.errorResponse(ctx, "Failed to retrieve KPIs", fasthttp.StatusInternalServerError)
		return
	}

	h.jsonResponse(ctx, kpis)
}

// GetAcquisitionFunnel handles GET /v1/admin/reports/acquisition-funnel
func (h *ReportsHandler) GetAcquisitionFunnel(ctx *fasthttp.RequestCtx) {
	filters, err := h.parseFilters(ctx)
	if err != nil {
		h.errorResponse(ctx, err.Error(), fasthttp.StatusBadRequest)
		return
	}

	funnel, err := h.reportsRepo.GetAcquisitionFunnel(filters)
	if err != nil {
		h.logger.Error("Failed to get acquisition funnel", zap.Error(err))
		h.errorResponse(ctx, "Failed to retrieve funnel data", fasthttp.StatusInternalServerError)
		return
	}

	h.jsonResponse(ctx, funnel)
}

// GetCampaignPerformance handles GET /v1/admin/reports/campaign-performance
func (h *ReportsHandler) GetCampaignPerformance(ctx *fasthttp.RequestCtx) {
	filters, err := h.parseFilters(ctx)
	if err != nil {
		h.errorResponse(ctx, err.Error(), fasthttp.StatusBadRequest)
		return
	}

	performance, err := h.reportsRepo.GetCampaignPerformance(filters)
	if err != nil {
		h.logger.Error("Failed to get campaign performance", zap.Error(err))
		h.errorResponse(ctx, "Failed to retrieve campaign performance", fasthttp.StatusInternalServerError)
		return
	}

	h.jsonResponse(ctx, performance)
}

// GetTimeSeries handles GET /v1/admin/reports/timeseries
func (h *ReportsHandler) GetTimeSeries(ctx *fasthttp.RequestCtx) {
	filters, err := h.parseFilters(ctx)
	if err != nil {
		h.errorResponse(ctx, err.Error(), fasthttp.StatusBadRequest)
		return
	}

	// Get interval from query params (default: daily)
	interval := string(ctx.QueryArgs().Peek("interval"))
	if interval != "hourly" && interval != "daily" {
		interval = "daily"
	}

	timeseries, err := h.reportsRepo.GetTimeSeries(filters, interval)
	if err != nil {
		h.logger.Error("Failed to get time series", zap.Error(err))
		h.errorResponse(ctx, "Failed to retrieve time series data", fasthttp.StatusInternalServerError)
		return
	}

	h.jsonResponse(ctx, timeseries)
}

// jsonResponse sends a JSON response
func (h *ReportsHandler) jsonResponse(ctx *fasthttp.RequestCtx, data interface{}) {
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	json.NewEncoder(ctx).Encode(data)
}

// errorResponse sends an error response
func (h *ReportsHandler) errorResponse(ctx *fasthttp.RequestCtx, message string, statusCode int) {
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(statusCode)
	json.NewEncoder(ctx).Encode(map[string]interface{}{
		"error": message,
	})
}

// ExportCampaignPerformanceCSV handles GET /v1/admin/reports/campaign-performance/export
// Returns CSV file with campaign performance data
func (h *ReportsHandler) ExportCampaignPerformanceCSV(ctx *fasthttp.RequestCtx) {
	filters, err := h.parseFilters(ctx)
	if err != nil {
		h.errorResponse(ctx, err.Error(), fasthttp.StatusBadRequest)
		return
	}

	performance, err := h.reportsRepo.GetCampaignPerformance(filters)
	if err != nil {
		h.logger.Error("Failed to get campaign performance for export", zap.Error(err))
		h.errorResponse(ctx, "Failed to retrieve campaign performance", fasthttp.StatusInternalServerError)
		return
	}

	// Generate filename with date range
	startStr := filters.StartDate.Format("2006-01-02")
	endStr := filters.EndDate.AddDate(0, 0, -1).Format("2006-01-02") // Subtract 1 day (we added 1 for inclusive range)
	filename := fmt.Sprintf("campaign-performance_%s_to_%s.csv", startStr, endStr)

	// Set response headers for CSV download
	ctx.SetContentType("text/csv; charset=utf-8")
	ctx.Response.Header.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	ctx.SetStatusCode(fasthttp.StatusOK)

	// Write CSV data
	writer := csv.NewWriter(ctx)

	// Write header row
	headers := []string{
		"Campaign",
		"Country",
		"Landing Views",
		"Transactions",
		"Subscribed",
		"Charged",
		"Estimated Revenue",
		"Conversion Rate (%)",
	}
	if err := writer.Write(headers); err != nil {
		h.logger.Error("Failed to write CSV header", zap.Error(err))
		return
	}

	// Write data rows
	for _, cp := range performance.Campaigns {
		row := []string{
			cp.CampaignSlug,
			cp.Country,
			fmt.Sprintf("%d", cp.LandingViews),
			fmt.Sprintf("%d", cp.Transactions),
			fmt.Sprintf("%d", cp.Subscribed),
			fmt.Sprintf("%d", cp.Charged),
			fmt.Sprintf("%.2f", cp.EstimatedRevenue),
			fmt.Sprintf("%.2f", cp.ConversionRate),
		}
		if err := writer.Write(row); err != nil {
			h.logger.Error("Failed to write CSV row", zap.Error(err), zap.String("campaign", cp.CampaignSlug))
			return
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		h.logger.Error("Failed to flush CSV writer", zap.Error(err))
	}
}
