package handler

import (
	"encoding/json"
	"time"

	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/seidu626/subscription-manager/subscription-external/internal/service"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// NotificationWebhookHandler handles incoming TIMWE notification webhooks
type NotificationWebhookHandler struct {
	logger            *zap.Logger
	svc               *service.SubscriptionService
	acquisitionClient *service.AcquisitionClient
}

// NewNotificationWebhookHandler creates a new notification webhook handler
func NewNotificationWebhookHandler(
	logger *zap.Logger,
	svc *service.SubscriptionService,
	acquisitionClient *service.AcquisitionClient,
) *NotificationWebhookHandler {
	return &NotificationWebhookHandler{
		logger:            logger,
		svc:               svc,
		acquisitionClient: acquisitionClient,
	}
}

// TimweNotificationRequest represents the webhook payload from TIMWE
type TimweNotificationRequest struct {
	PartnerRole     int      `json:"partnerRole"`
	ExternalTxID    string   `json:"externalTxId"`
	ProductID       int      `json:"productId"`
	PricepointID    int      `json:"pricepointId"`
	MCC             string   `json:"mcc"`
	MNC             string   `json:"mnc"`
	MSISDN          string   `json:"msisdn"`
	LargeAccount    string   `json:"largeAccount"`
	TransactionUUID string   `json:"transactionUuid"`
	EntryChannel    string   `json:"entryChannel"`
	MessageType     string   `json:"messageType"`
	Message         string   `json:"message"`
	MnoDeliveryCode string   `json:"mnoDeliveryCode"`
	Tags            []string `json:"tags"`
	Type            string   `json:"type"` // CHARGE, USER_RENEWED, USER_OPTIN, USER_OPTOUT, RENEWAL
	Amount          string   `json:"amount,omitempty"`
	Currency        string   `json:"currency,omitempty"`
	TransactionID   string   `json:"transactionId,omitempty"` // TIMWE transaction ID
}

// HandleNotificationWebhook processes incoming TIMWE notification webhooks
// @Summary Receive TIMWE notification webhook
// @Description Handles incoming notification webhooks from TIMWE for events like CHARGE, USER_RENEWED, USER_OPTIN, USER_OPTOUT
// @Tags Webhook
// @Accept json
// @Produce json
// @Param body body TimweNotificationRequest true "Notification request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/webhooks/timwe/notification [post]
func (h *NotificationWebhookHandler) HandleNotificationWebhook(ctx *fasthttp.RequestCtx) {
	var req TimweNotificationRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		h.logger.Error("Failed to parse notification webhook", zap.Error(err))
		ctx.Error("Invalid request payload", fasthttp.StatusBadRequest)
		return
	}

	h.logger.Info("Received TIMWE notification webhook",
		zap.String("type", req.Type),
		zap.String("msisdn", req.MSISDN),
		zap.Int("product_id", req.ProductID),
		zap.String("external_tx_id", req.ExternalTxID),
		zap.String("transaction_uuid", req.TransactionUUID),
	)

	// Store notification in database
	notification := &domain.NotificationRequest{
		PartnerRole:     req.PartnerRole,
		ExternalTxID:    req.ExternalTxID,
		ProductID:       req.ProductID,
		PricepointID:    req.PricepointID,
		MCC:             req.MCC,
		MNC:             req.MNC,
		MSISDN:          req.MSISDN,
		LargeAccount:    req.LargeAccount,
		TransactionUUID: req.TransactionUUID,
		EntryChannel:    req.EntryChannel,
		MessageType:     req.MessageType,
		Message:         req.Message,
		MnoDeliveryCode: req.MnoDeliveryCode,
		Tags:            req.Tags,
		Type:            req.Type,
	}

	repo := h.svc.GetRepository()
	if err := repo.CreateNotification(notification); err != nil {
		h.logger.Error("Failed to store notification", zap.Error(err))
		// Continue processing even if storage fails - we still want to notify acquisition-api
	}

	// For CHARGE and USER_RENEWED notifications, notify acquisition-api
	// This triggers conversion postbacks for ad partners (e.g., Mobplus)
	if req.Type == "CHARGE" || req.Type == "USER_RENEWED" {
		h.notifyAcquisitionAPI(&req)
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	response := map[string]interface{}{
		"status":    "ok",
		"message":   "Notification processed",
		"type":      req.Type,
		"msisdn":    req.MSISDN,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	json.NewEncoder(ctx).Encode(response)
}

// notifyAcquisitionAPI sends charge success notification to acquisition-api
func (h *NotificationWebhookHandler) notifyAcquisitionAPI(req *TimweNotificationRequest) {
	// Use transaction_uuid or external_tx_id as the TIMWE transaction ID
	timweTransactionID := req.TransactionUUID
	if timweTransactionID == "" {
		timweTransactionID = req.ExternalTxID
	}

	if timweTransactionID == "" {
		h.logger.Warn("No transaction ID in notification, skipping acquisition-api callback",
			zap.String("type", req.Type),
			zap.String("msisdn", req.MSISDN))
		return
	}

	chargeReq := &service.ChargeSuccessRequest{
		TimweTransactionID: timweTransactionID,
		MSISDN:             req.MSISDN,
		ProductID:          req.ProductID,
		ChargedAt:          time.Now().Format(time.RFC3339),
		Payout:             req.Amount, // Use amount as payout if available
	}

	// Call acquisition-api asynchronously to not block the webhook response
	h.acquisitionClient.NotifyChargeSuccessAsync(chargeReq)
}
