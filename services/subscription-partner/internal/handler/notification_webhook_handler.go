package handler

import (
	"encoding/json"
	"time"

	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription/internal/domain"
	"github.com/seidu626/subscription-manager/subscription/internal/service"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// NotificationWebhookHandler handles incoming TIMWE notification webhooks.
type NotificationWebhookHandler struct {
	logger            *zap.Logger
	svc               *service.SubscriptionService
	acquisitionClient *service.AcquisitionClient
	tenantRepo        gatewayTenantLookup
	cfg               *config.Config
}

// NewNotificationWebhookHandler creates a new notification webhook handler.
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

// WithTenantRepo sets the repository used for gateway-trust tenant resolution.
func (h *NotificationWebhookHandler) WithTenantRepo(repo gatewayTenantLookup) *NotificationWebhookHandler {
	h.tenantRepo = repo
	return h
}

// WithConfig sets the shared config (needed for gateway trust verification).
func (h *NotificationWebhookHandler) WithConfig(cfg *config.Config) *NotificationWebhookHandler {
	h.cfg = cfg
	return h
}

// TimweNotificationRequest represents the webhook payload from TIMWE.
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
}

// HandleNotificationWebhook processes incoming TIMWE notification webhooks.
func (h *NotificationWebhookHandler) HandleNotificationWebhook(ctx *fasthttp.RequestCtx) {
	if !checkGatewayTrust(ctx, h.cfg) {
		return
	}
	var req TimweNotificationRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		h.logger.Error("Failed to parse notification webhook", zap.Error(err))
		ctx.Error("Invalid request payload", fasthttp.StatusBadRequest)
		return
	}

	// Resolve tenant context when tenantRepo is configured.
	// Tenantless webhook requests are rejected with 422.
	var tenantRoute domain.TenantRouteContext
	if h.tenantRepo != nil {
		var routeErr error
		tenantRoute, routeErr = tenantRouteFromGatewayHeaders(ctx, h.tenantRepo)
		if routeErr != nil {
			statusCode, errCode := gatewayRouteStatus(routeErr)
			h.logger.Warn("Webhook rejected: missing tenant context", zap.Error(routeErr))
			writeJSONResponse(ctx, statusCode, map[string]interface{}{
				"message": routeErr.Error(),
				"code":    errCode,
				"inError": true,
			})
			return
		}
	}

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
		TenantRoute:     tenantRoute,
	}

	if err := h.svc.ProcessNotification(notification); err != nil {
		h.logger.Error("Failed to process notification", zap.Error(err))
		ctx.Error("Failed to process notification", fasthttp.StatusInternalServerError)
		return
	}

	if req.Type == "CHARGE" || req.Type == "USER_RENEWED" {
		h.notifyAcquisitionAPI(&req)
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(map[string]interface{}{
		"status":    "ok",
		"message":   "Notification processed",
		"type":      req.Type,
		"msisdn":    req.MSISDN,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

func (h *NotificationWebhookHandler) notifyAcquisitionAPI(req *TimweNotificationRequest) {
	timweTransactionID := req.TransactionUUID
	if timweTransactionID == "" {
		timweTransactionID = req.ExternalTxID
	}
	if timweTransactionID == "" {
		return
	}

	chargeReq := &service.ChargeSuccessRequest{
		TimweTransactionID: timweTransactionID,
		MSISDN:             req.MSISDN,
		ProductID:          req.ProductID,
		ChargedAt:          time.Now().Format(time.RFC3339),
		Payout:             req.Amount,
	}
	h.acquisitionClient.NotifyChargeSuccessAsync(chargeReq)
}
