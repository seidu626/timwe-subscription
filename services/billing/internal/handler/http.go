package handler

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/seidu626/subscription-manager/billing/internal/service"
	"github.com/valyala/fasthttp"
)

type BillingHandler struct {
	service *service.BillingService
}

func NewBillingHandler(service *service.BillingService) *BillingHandler {
	return &BillingHandler{service: service}
}

// ProcessPayment handles payment processing requests
func (h *BillingHandler) ProcessPayment(ctx *fasthttp.RequestCtx) {
	var req struct {
		MSISDN    string  `json:"msisdn"`
		ProductID int     `json:"product_id"`
		Amount    float64 `json:"amount"`
	}

	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetBodyString("Invalid request payload")
		return
	}

	tx, err := h.service.ProcessPayment(req.MSISDN, req.ProductID, req.Amount)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		ctx.SetBodyString("Failed to process payment")
		return
	}

	ctx.SetContentType("application/json")
	json.NewEncoder(ctx).Encode(tx)
}

// ListTransactions handles GET /api/v1/billing/transactions
func (h *BillingHandler) ListTransactions(ctx *fasthttp.RequestCtx) {
	msisdn := string(ctx.QueryArgs().Peek("msisdn"))
	if msisdn == "" {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetBodyString("msisdn query parameter is required")
		return
	}

	transactions, err := h.service.FindByMSISDN(msisdn)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		ctx.SetBodyString("Failed to list transactions")
		return
	}

	ctx.SetContentType("application/json")
	json.NewEncoder(ctx).Encode(transactions)
}

// CreateTransaction handles POST /api/v1/billing/transactions
func (h *BillingHandler) CreateTransaction(ctx *fasthttp.RequestCtx) {
	h.ProcessPayment(ctx)
}

// GetTransaction handles GET /api/v1/billing/transaction/:id
func (h *BillingHandler) GetTransaction(ctx *fasthttp.RequestCtx) {
	path := string(ctx.Path())
	// Extract ID from path /api/v1/billing/transaction/:id
	parts := strings.Split(path, "/")
	if len(parts) < 5 {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetBodyString("Invalid transaction ID")
		return
	}

	idStr := parts[len(parts)-1]
	_, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetBodyString("Invalid transaction ID format")
		return
	}

	// For now, return not implemented as we don't have GetByID in repository
	ctx.SetStatusCode(fasthttp.StatusNotImplemented)
	ctx.SetBodyString("Get transaction by ID not implemented")
}
