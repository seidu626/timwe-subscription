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
		ctx.Error("Invalid request body", fasthttp.StatusBadRequest)
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

	response, err := h.service.CreateTransaction(&req)
	if err != nil {
		h.logger.Error("Failed to create transaction", zap.Error(err))
		ctx.Error(err.Error(), fasthttp.StatusBadRequest)
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
		ctx.Error("Invalid path", fasthttp.StatusBadRequest)
		return
	}
	
	transactionIDStr := parts[len(parts)-2] // /transactions/:id/confirm
	transactionID, err := uuid.Parse(transactionIDStr)
	if err != nil {
		ctx.Error("Invalid transaction ID", fasthttp.StatusBadRequest)
		return
	}
	
	var req domain.ConfirmTransactionRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		h.logger.Error("Failed to parse request", zap.Error(err))
		ctx.Error("Invalid request body", fasthttp.StatusBadRequest)
		return
	}
	
	req.TransactionID = transactionID
	
	response, err := h.service.ConfirmTransaction(transactionID, req.AuthCode)
	if err != nil {
		h.logger.Error("Failed to confirm transaction", zap.Error(err))
		ctx.Error(err.Error(), fasthttp.StatusBadRequest)
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
		ctx.Error("Invalid path", fasthttp.StatusBadRequest)
		return
	}
	
	transactionIDStr := parts[len(parts)-2] // /transactions/:id/status
	transactionID, err := uuid.Parse(transactionIDStr)
	if err != nil {
		ctx.Error("Invalid transaction ID", fasthttp.StatusBadRequest)
		return
	}
	
	response, err := h.service.GetTransactionStatus(transactionID)
	if err != nil {
		h.logger.Error("Failed to get transaction status", zap.Error(err))
		ctx.Error("Transaction not found", fasthttp.StatusNotFound)
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
