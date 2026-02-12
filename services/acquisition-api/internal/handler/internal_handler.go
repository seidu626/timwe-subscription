package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"time"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/service"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// InternalHandler handles internal API requests (from subscription-external)
type InternalHandler struct {
	transactionService *service.TransactionService
	logger             *zap.Logger
	internalSecret     string
}

// NewInternalHandler creates a new internal handler
func NewInternalHandler(transactionService *service.TransactionService, logger *zap.Logger) *InternalHandler {
	// Get internal secret from environment (required for production)
	secret := os.Getenv("INTERNAL_API_SECRET")
	if secret == "" {
		// Default for development - MUST be overridden in production
		secret = "dev-internal-secret-change-in-production"
		logger.Warn("INTERNAL_API_SECRET not set, using development default - DO NOT USE IN PRODUCTION")
	}

	return &InternalHandler{
		transactionService: transactionService,
		logger:             logger,
		internalSecret:     secret,
	}
}

// validateInternalAuth validates the HMAC signature from internal services
func (h *InternalHandler) validateInternalAuth(ctx *fasthttp.RequestCtx) bool {
	// Get signature from header
	signature := string(ctx.Request.Header.Peek("X-Internal-Signature"))
	timestamp := string(ctx.Request.Header.Peek("X-Internal-Timestamp"))

	if signature == "" || timestamp == "" {
		h.logger.Warn("Missing internal auth headers",
			zap.String("path", string(ctx.Path())),
			zap.String("remote_addr", ctx.RemoteAddr().String()),
		)
		return false
	}

	// Verify timestamp is within acceptable window (5 minutes)
	ts, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		h.logger.Warn("Invalid timestamp format", zap.String("timestamp", timestamp))
		return false
	}

	if time.Since(ts).Abs() > 5*time.Minute {
		h.logger.Warn("Timestamp outside acceptable window",
			zap.String("timestamp", timestamp),
			zap.Duration("age", time.Since(ts)),
		)
		return false
	}

	// Compute expected signature: HMAC-SHA256(timestamp + body)
	body := ctx.PostBody()
	message := timestamp + string(body)
	mac := hmac.New(sha256.New, []byte(h.internalSecret))
	mac.Write([]byte(message))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		h.logger.Warn("Invalid internal signature",
			zap.String("path", string(ctx.Path())),
			zap.String("remote_addr", ctx.RemoteAddr().String()),
		)
		return false
	}

	return true
}

// HandleChargeSuccess handles POST /internal/acquisition/charge-success
// This endpoint is called by subscription-external when a charge succeeds
func (h *InternalHandler) HandleChargeSuccess(ctx *fasthttp.RequestCtx) {
	// Validate internal authentication
	if !h.validateInternalAuth(ctx) {
		ctx.Error("Unauthorized", fasthttp.StatusUnauthorized)
		return
	}

	// Parse request body
	var req service.ChargeSuccessRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		h.logger.Error("Failed to parse charge success request", zap.Error(err))
		ctx.Error("Invalid request body", fasthttp.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.TimweTransactionID == "" {
		ctx.Error("timwe_transaction_id is required", fasthttp.StatusBadRequest)
		return
	}

	// Process charge success
	if err := h.transactionService.HandleChargeSuccess(&req); err != nil {
		h.logger.Error("Failed to handle charge success",
			zap.String("timwe_transaction_id", req.TimweTransactionID),
			zap.Error(err),
		)

		// Return 404 if transaction not found, 500 otherwise
		if err.Error() == "transaction not found" || 
		   (len(err.Error()) > 24 && err.Error()[:24] == "transaction not found") {
			ctx.Error("Transaction not found", fasthttp.StatusNotFound)
			return
		}

		ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
		return
	}

	h.logger.Info("Charge success processed successfully",
		zap.String("timwe_transaction_id", req.TimweTransactionID),
	)

	// Return success response
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	response := map[string]interface{}{
		"success": true,
		"message": "Charge success processed, conversion postback enqueued",
	}
	json.NewEncoder(ctx).Encode(response)
}

// ChargeSuccessResponse represents the response for charge success endpoint
type ChargeSuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}
