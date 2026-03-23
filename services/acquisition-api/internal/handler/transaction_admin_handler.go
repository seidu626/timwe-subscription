package handler

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/service"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// TransactionAdminHandler handles admin transaction-related HTTP requests
type TransactionAdminHandler struct {
	txRepo  *repository.TransactionRepository
	txSvc   *service.TransactionService
	logger  *zap.Logger
}

// NewTransactionAdminHandler creates a new transaction admin handler
func NewTransactionAdminHandler(txRepo *repository.TransactionRepository, txSvc *service.TransactionService, logger *zap.Logger) *TransactionAdminHandler {
	return &TransactionAdminHandler{
		txRepo: txRepo,
		txSvc:  txSvc,
		logger: logger,
	}
}

// TransactionListResponse represents the response for listing transactions
type TransactionListResponse struct {
	Transactions []TransactionSummary `json:"transactions"`
	TotalCount   int                  `json:"total_count"`
	Page         int                  `json:"page"`
	PageSize     int                  `json:"page_size"`
}

// TransactionSummary represents a summary of a transaction for admin listing
type TransactionSummary struct {
	ID                     string     `json:"id"`
	CorrelationID          string     `json:"correlation_id"`
	CampaignSlug           string     `json:"campaign_slug"`
	MSISDN                 string     `json:"msisdn"`
	Status                 string     `json:"status"`
	AdProvider             *string    `json:"ad_provider,omitempty"`
	ClickID                *string    `json:"click_id,omitempty"`
	TimweTransactionID     *string    `json:"timwe_transaction_id,omitempty"`
	TimweStatus            *string    `json:"timwe_status,omitempty"`
	ConversionPostbackSent bool       `json:"conversion_postback_sent"`
	ChargedAt              *time.Time `json:"charged_at,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

// ListTransactions handles GET /v1/admin/transactions
func (h *TransactionAdminHandler) ListTransactions(ctx *fasthttp.RequestCtx) {
	// Parse query parameters
	args := ctx.QueryArgs()

	// Pagination
	page := 1
	pageSize := 20
	if p := args.GetUintOrZero("page"); p > 0 {
		page = int(p)
	}
	if ps := args.GetUintOrZero("page_size"); ps > 0 && ps <= 100 {
		pageSize = int(ps)
	}

	// Filters
	filter := &repository.TransactionListFilter{
		Limit:  pageSize,
		Offset: (page - 1) * pageSize,
	}

	if campaignSlug := string(args.Peek("campaign_slug")); campaignSlug != "" {
		filter.CampaignSlug = campaignSlug
	}
	if status := string(args.Peek("status")); status != "" {
		filter.Status = status
	}
	if provider := string(args.Peek("provider")); provider != "" {
		filter.Provider = provider
	}
	if startDate := string(args.Peek("start_date")); startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			filter.StartDate = &t
		}
	}
	if endDate := string(args.Peek("end_date")); endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			// End of day
			t = t.Add(24*time.Hour - time.Second)
			filter.EndDate = &t
		}
	}

	// Query transactions
	result, err := h.txRepo.ListTransactions(filter)
	if err != nil {
		h.logger.Error("Failed to list transactions", zap.Error(err))
		ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
		return
	}

	// Build response
	transactions := make([]TransactionSummary, 0, len(result.Transactions))
	for _, tx := range result.Transactions {
		summary := TransactionSummary{
			ID:                     tx.ID.String(),
			CorrelationID:          tx.CorrelationID.String(),
			CampaignSlug:           tx.CampaignSlug,
			MSISDN:                 tx.MSISDN,
			Status:                 string(tx.Status),
			AdProvider:             tx.AdProvider,
			ClickID:                tx.ClickID,
			TimweTransactionID:     tx.TimweTransactionID,
			TimweStatus:            tx.TimweStatus,
			ConversionPostbackSent: tx.ConversionPostbackSent,
			ChargedAt:              tx.ChargedAt,
			CreatedAt:              tx.CreatedAt,
			UpdatedAt:              tx.UpdatedAt,
		}
		transactions = append(transactions, summary)
	}

	response := TransactionListResponse{
		Transactions: transactions,
		TotalCount:   result.TotalCount,
		Page:         page,
		PageSize:     pageSize,
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	json.NewEncoder(ctx).Encode(response)
}

// GetTransaction handles GET /v1/admin/transactions/:id
func (h *TransactionAdminHandler) GetTransaction(ctx *fasthttp.RequestCtx) {
	// Extract transaction ID from path
	path := string(ctx.Path())
	// /v1/admin/transactions/{id}
	parts := splitPathParts(path)
	if len(parts) < 4 {
		ctx.Error("Invalid path", fasthttp.StatusBadRequest)
		return
	}

	txIDStr := parts[len(parts)-1]
	txID, err := parseTransactionUUID(txIDStr)
	if err != nil {
		ctx.Error("Invalid transaction ID", fasthttp.StatusBadRequest)
		return
	}

	tx, err := h.txRepo.GetByID(txID)
	if err != nil {
		h.logger.Error("Transaction not found", zap.String("id", txIDStr), zap.Error(err))
		ctx.Error("Transaction not found", fasthttp.StatusNotFound)
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	json.NewEncoder(ctx).Encode(tx)
}

// GetTransactionStats handles GET /v1/admin/transactions/stats
func (h *TransactionAdminHandler) GetTransactionStats(ctx *fasthttp.RequestCtx) {
	// Parse date range
	args := ctx.QueryArgs()

	var startDate, endDate *time.Time
	if sd := string(args.Peek("start_date")); sd != "" {
		if t, err := time.Parse("2006-01-02", sd); err == nil {
			startDate = &t
		}
	}
	if ed := string(args.Peek("end_date")); ed != "" {
		if t, err := time.Parse("2006-01-02", ed); err == nil {
			t = t.Add(24*time.Hour - time.Second)
			endDate = &t
		}
	}

	// Default to last 7 days if not specified
	if startDate == nil {
		t := time.Now().AddDate(0, 0, -7)
		startDate = &t
	}
	if endDate == nil {
		t := time.Now()
		endDate = &t
	}

	// Build stats query
	query := `
		SELECT 
			status,
			COUNT(*) as count
		FROM acquisition_transactions
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY status
		ORDER BY count DESC
	`

	rows, err := h.txRepo.DB().Query(query, startDate, endDate)
	if err != nil {
		h.logger.Error("Failed to get transaction stats", zap.Error(err))
		ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
		return
	}
	defer rows.Close()

	statusCounts := make(map[string]int)
	totalCount := 0
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			continue
		}
		statusCounts[status] = count
		totalCount += count
	}

	response := map[string]interface{}{
		"start_date":    startDate.Format("2006-01-02"),
		"end_date":      endDate.Format("2006-01-02"),
		"total_count":   totalCount,
		"status_counts": statusCounts,
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	json.NewEncoder(ctx).Encode(response)
}

// Helper to split path into parts
func splitPathParts(path string) []string {
	var parts []string
	current := ""
	for _, c := range path {
		if c == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// TriggerPostback handles POST /v1/admin/transactions/:id/trigger-postback
// Manually enqueues a postback for a transaction that never had one fired.
func (h *TransactionAdminHandler) TriggerPostback(ctx *fasthttp.RequestCtx) {
	path := string(ctx.Path())
	parts := splitPathParts(path)
	// /v1/admin/transactions/{id}/trigger-postback => [v1, admin, transactions, {id}, trigger-postback]
	if len(parts) < 5 {
		ctx.Error("Invalid path", fasthttp.StatusBadRequest)
		return
	}

	txIDStr := parts[len(parts)-2]
	txID, err := parseTransactionUUID(txIDStr)
	if err != nil {
		writeJSONError(ctx, fasthttp.StatusBadRequest, "Invalid transaction ID")
		return
	}

	// Default to conversion event; allow override via query param
	event := domain.PostbackEventConversion
	if e := string(ctx.QueryArgs().Peek("event")); e != "" {
		event = domain.PostbackEvent(e)
	}

	if err := h.txSvc.TriggerPostback(txID, event); err != nil {
		h.logger.Error("Failed to trigger postback",
			zap.String("transaction_id", txIDStr),
			zap.Error(err))
		writeJSONError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	json.NewEncoder(ctx).Encode(map[string]interface{}{
		"status":         "ok",
		"transaction_id": txID.String(),
		"event":          string(event),
		"message":        "Postback enqueued successfully",
	})
}

// Helper to parse UUID string
func parseTransactionUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}
