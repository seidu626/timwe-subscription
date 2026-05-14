package handler

import (
	"encoding/json"
	"net"
	"strings"

	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/service"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// TransactionHandler handles transaction-related HTTP requests
type TransactionHandler struct {
	service      *service.TransactionService
	heMiddleware *HEContextMiddleware
	logger       *zap.Logger
}

// NewTransactionHandler creates a new transaction handler
func NewTransactionHandler(transactionService *service.TransactionService, logger *zap.Logger) *TransactionHandler {
	return &TransactionHandler{
		service:      transactionService,
		heMiddleware: NewHEContextMiddleware(DefaultHEContextConfig(), logger),
		logger:       logger,
	}
}

// NewTransactionHandlerWithHE creates a new transaction handler with custom HE config
func NewTransactionHandlerWithHE(transactionService *service.TransactionService, heConfig *HEContextConfig, logger *zap.Logger) *TransactionHandler {
	return &TransactionHandler{
		service:      transactionService,
		heMiddleware: NewHEContextMiddleware(heConfig, logger),
		logger:       logger,
	}
}

// CreateTransaction handles POST /v1/acquisition/transactions
func (h *TransactionHandler) CreateTransaction(ctx *fasthttp.RequestCtx) {
	var req domain.CreateTransactionRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		h.logger.Error("Failed to parse request", zap.Error(err))
		writeJSONError(ctx, fasthttp.StatusBadRequest, "Invalid request body")
		return
	}

	// Extract IP address from request if not provided
	if req.IPAddress == nil {
		ip := h.getClientIP(ctx)
		req.IPAddress = &ip
	}

	// Extract User-Agent if not provided
	if req.UserAgent == nil {
		ua := string(ctx.UserAgent())
		req.UserAgent = &ua
	}

	// Extract HE identity from headers (real or simulated)
	heIdentity := h.heMiddleware.ExtractIdentity(ctx)
	if heIdentity != nil {
		heSource := domain.HESource(heIdentity.Source)
		req.HESource = &heSource
		req.HEMSISDN = &heIdentity.MSISDN
		if heIdentity.OperatorID != "" {
			req.HEOperator = &heIdentity.OperatorID
		}

		h.logger.Info("HE identity detected for transaction",
			zap.String("he_source", string(heIdentity.Source)),
			zap.String("msisdn_hash", hashMSISDN(heIdentity.MSISDN)),
			zap.String("operator_id", heIdentity.OperatorID),
			zap.Bool("he_detected", true),
		)
	} else {
		h.logger.Debug("No HE identity detected, using OTP flow")
	}

	if req.TenantKey == nil || strings.TrimSpace(*req.TenantKey) == "" {
		if hasPublicTenantHeaders(ctx) {
			identity, err := trustedPublicTenantIdentityFromRequest(ctx)
			if err != nil || strings.TrimSpace(identity.TenantKey) == "" {
				writeJSONError(ctx, fasthttp.StatusForbidden, "Tenant context invalid")
				return
			}
			tenantKey := strings.TrimSpace(identity.TenantKey)
			req.TenantKey = &tenantKey
		}
	}

	response, err := h.service.CreateTransaction(&req)
	if err != nil {
		h.logger.Error("Failed to create transaction", zap.Error(err))
		writeJSONError(ctx, mapCreateTransactionStatus(err), err.Error())
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	json.NewEncoder(ctx).Encode(response)
}

// ConfirmTransaction handles POST /v1/acquisition/transactions/:id/confirm
func (h *TransactionHandler) ConfirmTransaction(ctx *fasthttp.RequestCtx) {
	// Extract transaction ID from path
	path := string(ctx.Path())
	parts := strings.Split(path, "/")
	if len(parts) < 5 {
		writeJSONError(ctx, fasthttp.StatusBadRequest, "Invalid path")
		return
	}

	transactionIDStr := parts[len(parts)-2] // /transactions/:id/confirm
	transactionID, err := uuid.Parse(transactionIDStr)
	if err != nil {
		writeJSONError(ctx, fasthttp.StatusBadRequest, "Invalid transaction ID")
		return
	}

	var req domain.ConfirmTransactionRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		h.logger.Error("Failed to parse request", zap.Error(err))
		writeJSONError(ctx, fasthttp.StatusBadRequest, "Invalid request body")
		return
	}

	req.TransactionID = transactionID

	response, err := h.service.ConfirmTransaction(transactionID, req.AuthCode)
	if err != nil {
		h.logger.Error("Failed to confirm transaction", zap.Error(err))
		writeJSONError(ctx, mapConfirmTransactionStatus(err), err.Error())
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	json.NewEncoder(ctx).Encode(response)
}

// GetTransactionStatus handles GET /v1/acquisition/transactions/:id/status
func (h *TransactionHandler) GetTransactionStatus(ctx *fasthttp.RequestCtx) {
	// Extract transaction ID from path
	path := string(ctx.Path())
	parts := strings.Split(path, "/")
	if len(parts) < 5 {
		writeJSONError(ctx, fasthttp.StatusBadRequest, "Invalid path")
		return
	}

	transactionIDStr := parts[len(parts)-2] // /transactions/:id/status
	transactionID, err := uuid.Parse(transactionIDStr)
	if err != nil {
		writeJSONError(ctx, fasthttp.StatusBadRequest, "Invalid transaction ID")
		return
	}

	response, err := h.service.GetTransactionStatus(transactionID)
	if err != nil {
		h.logger.Error("Failed to get transaction status", zap.Error(err))
		writeJSONError(ctx, fasthttp.StatusNotFound, "Transaction not found")
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	json.NewEncoder(ctx).Encode(response)
}

// getClientIP extracts the client IP address from the request
func (h *TransactionHandler) getClientIP(ctx *fasthttp.RequestCtx) string {
	// Try X-Forwarded-For header first
	if xff := ctx.Request.Header.Peek("X-Forwarded-For"); len(xff) > 0 {
		ips := strings.Split(string(xff), ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	// Try X-Real-IP header
	if xri := ctx.Request.Header.Peek("X-Real-IP"); len(xri) > 0 {
		ip := strings.TrimSpace(string(xri))
		if net.ParseIP(ip) != nil {
			return ip
		}
	}

	// Fallback to remote address
	addr := ctx.RemoteAddr()
	if addr != nil {
		if ip, _, err := net.SplitHostPort(addr.String()); err == nil {
			return ip
		}
		return addr.String()
	}

	return "unknown"
}

func writeJSONError(ctx *fasthttp.RequestCtx, status int, message string) {
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(status)
	_ = json.NewEncoder(ctx).Encode(map[string]string{
		"error": message,
	})
}

func mapCreateTransactionStatus(err error) int {
	if err == nil {
		return fasthttp.StatusBadRequest
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "request throttled"):
		return fasthttp.StatusTooManyRequests
	case strings.Contains(msg, "campaign not found"):
		return fasthttp.StatusNotFound
	default:
		return fasthttp.StatusBadRequest
	}
}

func mapConfirmTransactionStatus(err error) int {
	if err == nil {
		return fasthttp.StatusBadRequest
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "transaction not found"):
		return fasthttp.StatusNotFound
	case strings.Contains(msg, "transaction is not in confirm_required status"):
		return fasthttp.StatusConflict
	default:
		return fasthttp.StatusBadRequest
	}
}
