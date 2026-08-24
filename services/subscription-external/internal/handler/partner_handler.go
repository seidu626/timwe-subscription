package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/seidu626/subscription-manager/subscription-external/internal/service"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// PartnerHandler handles External Partner API endpoints
// It exposes MT and Direct Billing charge endpoints per swagger 1.4
// and delegates business logic to SubscriptionService.
type PartnerHandler struct {
	logger     *zap.Logger
	svc        *service.SubscriptionService
	cfg        *config.Config
	tenantRepo gatewayTenantLookup
	// nonceStore is a process-lifetime in-memory nonce store that prevents
	// HMAC signature replay within the 5-minute trusted-service skew window (FIX 4).
	nonceStore tenantctx.NonceStore
	// acqClient notifies acquisition-api of tenant partner-route
	// optin/confirm events so they are counted by acquisition reporting.
	// Optional: nil unless WithAcquisitionClient is called, in which case
	// notifyAcquisitionPartnerSubscription is a no-op (e.g. in unit tests).
	acqClient *service.AcquisitionClient
	// optinNotifier records an already-active provider response through
	// notification-service so its template and outbox flow still runs when the
	// carrier does not emit a second USER_OPTIN callback.
	optinNotifier existingSubscriptionOptinNotifier
}

type existingSubscriptionOptinNotifier interface {
	NotifyUserOptin(context.Context, domain.TenantRouteContext, int, *domain.NotificationRequest) error
}

func NewPartnerHandler(logger *zap.Logger, svc *service.SubscriptionService, cfg *config.Config) *PartnerHandler {
	return &PartnerHandler{
		logger:     logger,
		svc:        svc,
		cfg:        cfg,
		nonceStore: tenantctx.NewMemoryNonceStore(),
	}
}

// WithTenantRepo sets the repository used by gateway-trust partner subscription handlers.
// Call this after NewPartnerHandler when the concrete repository implements gatewayTenantLookup.
func (h *PartnerHandler) WithTenantRepo(repo gatewayTenantLookup) *PartnerHandler {
	h.tenantRepo = repo
	return h
}

// WithAcquisitionClient sets the client used to notify acquisition-api of
// tenant partner-route optin/confirm events for acquisition reporting.
// Call this after NewPartnerHandler; omit it (leaving acqClient nil) to
// disable the notification entirely, e.g. in unit tests.
func (h *PartnerHandler) WithAcquisitionClient(client *service.AcquisitionClient) *PartnerHandler {
	h.acqClient = client
	return h
}

// WithOptinNotifier sets the notification-service client used for
// existing-subscription optin responses (OPTIN_ALREADY_ACTIVE and
// OPTIN_ACTIVE_WAIT_CHARGING).
func (h *PartnerHandler) WithOptinNotifier(notifier existingSubscriptionOptinNotifier) *PartnerHandler {
	h.optinNotifier = notifier
	return h
}

// partnerMtRequest is a DTO matching the swagger PartnerMtRequest shape
// swagger:parameters PartnerMt
// Note: We purposely keep this internal to handler and map to domain.MTRequest
// to enforce a single domain model.
type partnerMtRequest struct {
	ProductID     int    `json:"productId"`
	PricepointID  int    `json:"pricepointId"`
	MCC           string `json:"mcc"`
	MNC           string `json:"mnc"`
	Text          string `json:"text"` // Not used by current upstream optin API, accepted for compatibility
	MSISDN        string `json:"msisdn"`
	LargeAccount  string `json:"largeAccount"`
	SendDate      string `json:"sendDate"`
	Priority      string `json:"priority"`
	Timezone      string `json:"timezone"`
	Context       string `json:"context"`
	MoTransaction string `json:"moTransactionUUID"`
	ChannelID     string `json:"channelId,omitempty"`
	ChannelKey    string `json:"channelKey,omitempty"`
}

// PartnerMTHandler godoc
// @Summary Send MT to TIMWE Partner MA Platform
// @Description Implements /api/external/v1/{realm}/{channel}/mt/{partnerRole}
// @Tags PartnerMt
// @Accept json
// @Produce json
// @Param channel path string true "Channel (SMS/WEB/IVR/USSD)"
// @Param body body partnerMtRequest true "MT request body"
// @Success 200 {object} domain.MTResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/external/v1/{channel}/mt [post]
func (h *PartnerHandler) PartnerMTHandler(ctx *fasthttp.RequestCtx, channel string) {
	if !h.checkGatewayTrust(ctx) {
		return
	}
	// Parse body
	var req partnerMtRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload")
		return
	}
	route, err := tenantRouteFromRequestWithNonce(ctx, h.cfg, true, req.ChannelID, req.ChannelKey, h.nonceStore)
	if err != nil {
		writeError(ctx, tenantRouteStatus(err), "TENANT_CONTEXT_REQUIRED", err.Error())
		return
	}

	// Map to domain.MTRequest
	mtReq := domain.MTRequest{
		ProductID:          req.ProductID,
		PricepointID:       req.PricepointID,
		MCC:                req.MCC,
		MNC:                req.MNC,
		UserIdentifier:     req.MSISDN,
		UserIdentifierType: "MSISDN",
		EntryChannel:       channel,
		LargeAccount:       req.LargeAccount,
		SendDate:           req.SendDate,
		Priority:           req.Priority,
		Timezone:           req.Timezone,
		Context:            req.Context,
		MoTransactionUUID:  req.MoTransaction,
		TenantRoute:        route,
	}

	resp, err := h.svc.SendMT(mtReq, h.cfg.Application.TIMWE.Realm, channel)
	if err != nil {
		h.logger.Error("Partner MT failed", zap.Error(err))
		writeError(ctx, serviceErrorStatus(err), serviceErrorCode(err), err.Error())
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(resp)
}

// PartnerChargeHandler godoc
// @Summary Request Direct Billing charge to TIMWE Partner MA Platform
// @Description Implements /api/external/v1/{realm}/charge/dob/{partnerRole}
// @Tags PartnerDobCharging
// @Accept json
// @Produce json
// @Param body body domain.ChargeRequest true "Charging request body"
// @Success 200 {object} domain.ChargeResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/external/v1/charge/dob [post]
func (h *PartnerHandler) PartnerChargeHandler(ctx *fasthttp.RequestCtx) {
	if !h.checkGatewayTrust(ctx) {
		return
	}

	var req domain.ChargeRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload")
		return
	}
	route, err := tenantRouteFromRequestWithNonce(ctx, h.cfg, true, "", "", h.nonceStore)
	if err != nil {
		writeError(ctx, tenantRouteStatus(err), "TENANT_CONTEXT_REQUIRED", err.Error())
		return
	}
	req.TenantRoute = route
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		req.IdempotencyKey = strings.TrimSpace(string(ctx.Request.Header.Peek("external-tx-id")))
	}

	resp, err := h.svc.RequestCharge(req)
	if err != nil {
		h.logger.Error("Partner charge failed", zap.Error(err))
		writeError(ctx, serviceErrorStatus(err), serviceErrorCode(err), err.Error())
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(resp)
}

// PartnerStatusHandler godoc
// @Summary Get subscription status from TIMWE Partner MA Platform
// @Description Implements /api/external/v1/{realm}/subscription/status/{partnerRole}
// @Tags PartnerStatus
// @Accept json
// @Produce json
// @Param body body domain.GetStatusRequest true "Status request body"
// @Success 200 {object} domain.MTResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/external/v1/subscription/status [post]
func (h *PartnerHandler) PartnerStatusHandler(ctx *fasthttp.RequestCtx) {
	if !h.checkGatewayTrust(ctx) {
		return
	}

	// Parse body
	var req domain.GetStatusRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload")
		return
	}
	route, err := tenantRouteFromRequestWithNonce(ctx, h.cfg, true, "", "", h.nonceStore)
	if err != nil {
		writeError(ctx, tenantRouteStatus(err), "TENANT_CONTEXT_REQUIRED", err.Error())
		return
	}
	req.TenantRoute = route

	resp, err := h.svc.SendStatusCheck(req, h.cfg.Application.TIMWE.Realm)
	if err != nil {
		h.logger.Error("Partner status check failed", zap.Error(err))
		writeError(ctx, serviceErrorStatus(err), serviceErrorCode(err), err.Error())
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(resp)
}

// PartnerOptoutHandler godoc
// @Summary Unsubscribe user via TIMWE Partner MA Platform
// @Description Implements /api/external/v1/subscription/optout/{partnerRole}
// @Tags PartnerOptout
// @Accept json
// @Produce json
// @Param body body domain.UnsubscriptionRequest true "Unsubscription request body"
// @Success 200 {object} domain.MTResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/external/v1/subscription/optout [post]
func (h *PartnerHandler) PartnerOptoutHandler(ctx *fasthttp.RequestCtx) {
	if !h.checkGatewayTrust(ctx) {
		return
	}
	var req domain.UnsubscriptionRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload")
		return
	}
	route, err := tenantRouteFromRequestWithNonce(ctx, h.cfg, true, "", "", h.nonceStore)
	if err != nil {
		writeError(ctx, tenantRouteStatus(err), "TENANT_CONTEXT_REQUIRED", err.Error())
		return
	}
	req.TenantRoute = route
	resp, err := h.svc.SendOptout(req, h.cfg.Application.TIMWE.Realm)
	if err != nil {
		h.logger.Error("Partner optout failed", zap.Error(err))
		writeError(ctx, serviceErrorStatus(err), serviceErrorCode(err), err.Error())
		return
	}
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(resp)
}

// PartnerOptinConfirmHandler godoc
// @Summary Confirm double opt-in via TIMWE Partner MA Platform
// @Description Implements /api/external/v1/{realm}/subscription/optin/confirm/{partnerRole}
// @Tags PartnerOptinConfirm
// @Accept json
// @Produce json
// @Param body body domain.SubscriptionConfirmationRequest true "Confirmation request body"
// @Success 200 {object} domain.MTResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/external/v1/subscription/optin/confirm [post]
func (h *PartnerHandler) PartnerOptinConfirmHandler(ctx *fasthttp.RequestCtx) {
	if !h.checkGatewayTrust(ctx) {
		return
	}
	var req domain.SubscriptionConfirmationRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload")
		return
	}
	route, err := tenantRouteFromRequestWithNonce(ctx, h.cfg, true, "", "", h.nonceStore)
	if err != nil {
		writeError(ctx, tenantRouteStatus(err), "TENANT_CONTEXT_REQUIRED", err.Error())
		return
	}
	req.TenantRoute = route
	resp, err := h.svc.SendOptinConfirm(req, h.cfg.Application.TIMWE.Realm)
	if err != nil {
		h.logger.Error("Partner optin confirm failed", zap.Error(err))
		writeError(ctx, serviceErrorStatus(err), serviceErrorCode(err), err.Error())
		return
	}
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(resp)
}

// gatewayTenantLookup is the minimal repo interface needed by tenantRouteFromGatewayHeaders.
type gatewayTenantLookup interface {
	TenantIDByKey(tenantKey string) (string, error)
	ChannelIDByKeys(tenantID, channelKey string) (string, error)
}

// tenantRouteFromGatewayHeaders resolves tenant context from KrakenD-forwarded
// query params. Public path captures are rewritten by KrakenD into backend
// tenant_key/channel_key query params.
//
// GatewayTrusted is set to true because these partner endpoints are reachable
// publicly through KrakenD, which owns the public path-to-backend rewrite.
//
// Error codes mapped by callers:
//   - ErrTenantKeyConflict  → 409 TENANT_KEY_CONFLICT
//   - "TENANT_CONTEXT_REQUIRED" → 400
//   - "UNKNOWN_TENANT"          → 400
//   - "UNKNOWN_CHANNEL"         → 400
func tenantRouteFromGatewayHeaders(
	ctx *fasthttp.RequestCtx,
	repo gatewayTenantLookup,
) (domain.TenantRouteContext, error) {
	tenantKeyQuery := strings.TrimSpace(string(ctx.QueryArgs().Peek("tenant_key")))
	channelKeyQuery := strings.TrimSpace(string(ctx.QueryArgs().Peek("channel_key")))

	// GatewayTrusted=true is the locked Option B design choice (see slice TMP-072
	// spec). In production traffic, KrakenD rewrites public
	// /{tenant_key}/{channel_key}/subscriptions routes into backend
	// tenant_key/channel_key query params. A direct-to-service request can
	// supply those query params and pass this gate; that bypass is mitigated
	// operationally by nginx network isolation (subscription-external is not
	// directly internet-exposed) and by the downstream DB lookup that still has
	// to resolve to a real tenant and channel. Do NOT switch to
	// GatewayTrusted=false here without a matching change in KrakenD to forward
	// an HMAC trust marker.
	pair, err := tenantctx.ResolveKeyPair(
		fastHTTPHeaderGetter{ctx: ctx},
		tenantctx.KeyPair{TenantKey: tenantKeyQuery, ChannelKey: channelKeyQuery},
		tenantctx.ResolveKeyPairOptions{GatewayTrusted: true},
	)
	if err != nil {
		// Preserve ErrTenantKeyConflict so caller can map to 409.
		return domain.TenantRouteContext{}, err
	}

	tenantKey := pair.TenantKey
	channelKey := pair.ChannelKey

	if strings.TrimSpace(tenantKey) == "" {
		return domain.TenantRouteContext{}, fmt.Errorf("TENANT_CONTEXT_REQUIRED: tenant_key is required")
	}
	if strings.TrimSpace(channelKey) == "" {
		return domain.TenantRouteContext{}, fmt.Errorf("TENANT_CONTEXT_REQUIRED: channel_key is required")
	}

	tenantID, err := repo.TenantIDByKey(tenantKey)
	if err != nil || strings.TrimSpace(tenantID) == "" {
		return domain.TenantRouteContext{}, fmt.Errorf("UNKNOWN_TENANT: tenant_key %q not found", tenantKey)
	}

	channelID, err := repo.ChannelIDByKeys(tenantID, channelKey)
	if err != nil || strings.TrimSpace(channelID) == "" {
		return domain.TenantRouteContext{}, fmt.Errorf("UNKNOWN_CHANNEL: channel_key %q not found for tenant", channelKey)
	}

	return domain.TenantRouteContext{
		TenantID:   tenantID,
		TenantKey:  tenantKey,
		ChannelID:  channelID,
		ChannelKey: channelKey,
	}, nil
}

// gatewayRouteStatus maps tenantRouteFromGatewayHeaders errors to HTTP status codes.
func gatewayRouteStatus(err error) (int, string) {
	if errors.Is(err, tenantctx.ErrTenantKeyConflict) {
		return fasthttp.StatusConflict, "TENANT_KEY_CONFLICT"
	}
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "TENANT_CONTEXT_REQUIRED"):
		return fasthttp.StatusBadRequest, "TENANT_CONTEXT_REQUIRED"
	case strings.HasPrefix(msg, "UNKNOWN_TENANT"):
		return fasthttp.StatusBadRequest, "UNKNOWN_TENANT"
	case strings.HasPrefix(msg, "UNKNOWN_CHANNEL"):
		return fasthttp.StatusBadRequest, "UNKNOWN_CHANNEL"
	default:
		return fasthttp.StatusBadRequest, "TENANT_CONTEXT_REQUIRED"
	}
}

// GatewayPartnerMTHandler handles POST /api/v1/subscription-external/partners/mt.
//
// Tenant context is resolved from KrakenD-forwarded X-Tenant-Key/X-Channel-Key headers
// (martian modifier injects them from the public path captures of
// /api/external/v1/{tenant_key}/{channel_key}/mt).
//
// Deprecated legacy path: /api/external/v1/{channel}/mt — still routed to PartnerMTHandler.
// This gateway-trust handler is the canonical replacement.
func (h *PartnerHandler) GatewayPartnerMTHandler(ctx *fasthttp.RequestCtx) {
	if !h.checkGatewayTrust(ctx) {
		return
	}
	if h.tenantRepo == nil {
		writeError(ctx, fasthttp.StatusInternalServerError, "INTERNAL_ERROR", "tenant repository not configured")
		return
	}
	route, err := tenantRouteFromGatewayHeaders(ctx, h.tenantRepo)
	if err != nil {
		status, code := gatewayRouteStatus(err)
		writeError(ctx, status, code, err.Error())
		return
	}
	h.logger.Info("gateway partner MT",
		zap.String("tenant_id", route.TenantID),
		zap.String("channel_id", route.ChannelID),
	)

	var req partnerMtRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload")
		return
	}

	// channel_key from resolved route is the canonical channel; allow override via query.
	channel := route.ChannelKey
	if ch := strings.TrimSpace(string(ctx.QueryArgs().Peek("channel"))); ch != "" {
		channel = ch
	}

	mtReq := domain.MTRequest{
		ProductID:          req.ProductID,
		PricepointID:       req.PricepointID,
		MCC:                req.MCC,
		MNC:                req.MNC,
		UserIdentifier:     req.MSISDN,
		UserIdentifierType: "MSISDN",
		EntryChannel:       channel,
		LargeAccount:       req.LargeAccount,
		SendDate:           req.SendDate,
		Priority:           req.Priority,
		Timezone:           req.Timezone,
		Context:            req.Context,
		MoTransactionUUID:  req.MoTransaction,
		TenantRoute:        route,
	}

	resp, err := h.svc.SendMT(mtReq, h.cfg.Application.TIMWE.Realm, channel)
	if err != nil {
		h.logger.Error("Gateway partner MT failed", zap.Error(err))
		writeError(ctx, serviceErrorStatus(err), serviceErrorCode(err), err.Error())
		return
	}
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(resp)
}

// GatewayPartnerChargeHandler handles POST /api/v1/subscription-external/partners/charge.
//
// Tenant context is resolved from KrakenD-forwarded X-Tenant-Key/X-Channel-Key headers
// (martian modifier injects them from the public path captures of
// /api/external/v1/{tenant_key}/{channel_key}/charges).
//
// Deprecated legacy path: /api/external/v1/charge/dob — still routed to PartnerChargeHandler.
// This gateway-trust handler is the canonical replacement.
func (h *PartnerHandler) GatewayPartnerChargeHandler(ctx *fasthttp.RequestCtx) {
	if !h.checkGatewayTrust(ctx) {
		return
	}
	if h.tenantRepo == nil {
		writeError(ctx, fasthttp.StatusInternalServerError, "INTERNAL_ERROR", "tenant repository not configured")
		return
	}
	route, err := tenantRouteFromGatewayHeaders(ctx, h.tenantRepo)
	if err != nil {
		status, code := gatewayRouteStatus(err)
		writeError(ctx, status, code, err.Error())
		return
	}
	h.logger.Info("gateway partner charge",
		zap.String("tenant_id", route.TenantID),
		zap.String("channel_id", route.ChannelID),
	)

	var req domain.ChargeRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload")
		return
	}
	req.TenantRoute = route
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		req.IdempotencyKey = strings.TrimSpace(string(ctx.Request.Header.Peek("external-tx-id")))
	}

	resp, err := h.svc.RequestCharge(req)
	if err != nil {
		h.logger.Error("Gateway partner charge failed", zap.Error(err))
		writeError(ctx, serviceErrorStatus(err), serviceErrorCode(err), err.Error())
		return
	}
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(resp)
}

// PartnerSubscriptionOptin handles POST /api/v1/subscription-external/partners/optin.
// Tenant context is resolved from KrakenD-injected headers (no trusted-service HMAC required).
func (h *PartnerHandler) PartnerSubscriptionOptin(ctx *fasthttp.RequestCtx) {
	if !h.checkGatewayTrust(ctx) {
		return
	}
	if h.tenantRepo == nil {
		writeError(ctx, fasthttp.StatusInternalServerError, "INTERNAL_ERROR", "tenant repository not configured")
		return
	}
	route, err := tenantRouteFromGatewayHeaders(ctx, h.tenantRepo)
	if err != nil {
		status, code := gatewayRouteStatus(err)
		writeError(ctx, status, code, err.Error())
		return
	}
	h.logger.Info("partner subscription optin",
		zap.String("tenant_id", route.TenantID),
		zap.String("channel_id", route.ChannelID),
	)

	var req domain.MTRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload")
		return
	}
	req.TenantRoute = route

	resp, err := h.svc.SendMT(req, h.cfg.Application.TIMWE.Realm, strings.TrimSpace(string(ctx.QueryArgs().Peek("channel"))))
	if err != nil {
		h.logger.Error("Partner subscription optin failed", zap.Error(err))
		writeError(ctx, serviceErrorStatus(err), serviceErrorCode(err), err.Error())
		return
	}
	h.persistPartnerOptinSubscription(route, req, resp)
	h.notifyAcquisitionPartnerSubscription(partnerAcquisitionActionOptin, route, req.UserIdentifier, req.ProductID, req.EntryChannel, resp)
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(resp)
}

// PartnerSubscriptionConfirm handles POST /api/v1/subscription-external/partners/confirm.
func (h *PartnerHandler) PartnerSubscriptionConfirm(ctx *fasthttp.RequestCtx) {
	if !h.checkGatewayTrust(ctx) {
		return
	}
	if h.tenantRepo == nil {
		writeError(ctx, fasthttp.StatusInternalServerError, "INTERNAL_ERROR", "tenant repository not configured")
		return
	}
	route, err := tenantRouteFromGatewayHeaders(ctx, h.tenantRepo)
	if err != nil {
		status, code := gatewayRouteStatus(err)
		writeError(ctx, status, code, err.Error())
		return
	}
	h.logger.Info("partner subscription confirm",
		zap.String("tenant_id", route.TenantID),
		zap.String("channel_id", route.ChannelID),
	)

	var req domain.SubscriptionConfirmationRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload")
		return
	}
	req.TenantRoute = route

	resp, err := h.svc.SendOptinConfirm(req, h.cfg.Application.TIMWE.Realm)
	if err != nil {
		h.logger.Error("Partner subscription confirm failed", zap.Error(err))
		writeError(ctx, serviceErrorStatus(err), serviceErrorCode(err), err.Error())
		return
	}
	h.persistPartnerConfirmSubscription(route, req)
	entryChannel := ""
	if req.EntryChannel != nil {
		entryChannel = *req.EntryChannel
	}
	h.notifyAcquisitionPartnerSubscription(partnerAcquisitionActionConfirm, route, req.UserIdentifier, req.ProductId, entryChannel, resp)
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(resp)
}

// PartnerSubscriptionOptout handles POST /api/v1/subscription-external/partners/optout.
func (h *PartnerHandler) PartnerSubscriptionOptout(ctx *fasthttp.RequestCtx) {
	if !h.checkGatewayTrust(ctx) {
		return
	}
	if h.tenantRepo == nil {
		writeError(ctx, fasthttp.StatusInternalServerError, "INTERNAL_ERROR", "tenant repository not configured")
		return
	}
	route, err := tenantRouteFromGatewayHeaders(ctx, h.tenantRepo)
	if err != nil {
		status, code := gatewayRouteStatus(err)
		writeError(ctx, status, code, err.Error())
		return
	}
	h.logger.Info("partner subscription optout",
		zap.String("tenant_id", route.TenantID),
		zap.String("channel_id", route.ChannelID),
	)

	var req domain.UnsubscriptionRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload")
		return
	}
	req.TenantRoute = route

	resp, err := h.svc.SendOptout(req, h.cfg.Application.TIMWE.Realm)
	if err != nil {
		h.logger.Error("Partner subscription optout failed", zap.Error(err))
		writeError(ctx, serviceErrorStatus(err), serviceErrorCode(err), err.Error())
		return
	}
	h.persistPartnerOptoutSubscription(route, req)
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(resp)
}

// PartnerSubscriptionStatus handles POST /api/v1/subscription-external/partners/status.
func (h *PartnerHandler) PartnerSubscriptionStatus(ctx *fasthttp.RequestCtx) {
	if !h.checkGatewayTrust(ctx) {
		return
	}
	if h.tenantRepo == nil {
		writeError(ctx, fasthttp.StatusInternalServerError, "INTERNAL_ERROR", "tenant repository not configured")
		return
	}
	route, err := tenantRouteFromGatewayHeaders(ctx, h.tenantRepo)
	if err != nil {
		status, code := gatewayRouteStatus(err)
		writeError(ctx, status, code, err.Error())
		return
	}
	h.logger.Info("partner subscription status",
		zap.String("tenant_id", route.TenantID),
		zap.String("channel_id", route.ChannelID),
	)

	var req domain.GetStatusRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload")
		return
	}
	req.TenantRoute = route

	resp, err := h.svc.SendStatusCheck(req, h.cfg.Application.TIMWE.Realm)
	if err != nil {
		h.logger.Error("Partner subscription status failed", zap.Error(err))
		writeError(ctx, serviceErrorStatus(err), serviceErrorCode(err), err.Error())
		return
	}
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(resp)
}

// subscriptionPersister is the minimal repository surface the gateway-trust
// partner subscription handlers need to persist a tenant-scoped subscription
// row after a successful TIMWE response (see repository.SubscriptionRepository,
// which the production h.tenantRepo is bound to via WithTenantRepo). A runtime
// type assertion is used instead of widening gatewayTenantLookup so existing
// gatewayTenantLookup-only test stubs keep compiling; when the assertion fails
// (e.g. in unit tests, or if the repo is ever swapped for something narrower),
// persistence is silently skipped. It must never fail the proxied response
// because the TIMWE action already happened.
type subscriptionPersister interface {
	CreateSubscription(request *domain.SubscriptionRequest) error
	UpdateSubscriptionStatus(msisdn string, productID string, status string) error
}

// Tenant-scoped subscriptions.status values used by the partner persistence
// helpers below. "active"/"inactive" mirror the lowercase vocabulary already
// established for this column (repository.CheckSubscriptionExists filters
// status = 'active'; worker/notification_monitor.go calls
// UpsertSubscriptionStatus with "active"/"inactive"; the column default is
// 'active'). renewal_service.go writes different uppercase, multi-word values
// ("PENDING_RENEWAL", etc.) to the same column for its own renewal-cycle
// bookkeeping - that is a separate, pre-existing convention this change does
// not touch. "preactive" has no prior precedent in this column; it is a
// conservative, lowercase-consistent placeholder for a subscription still
// awaiting double opt-in confirmation (PartnerSubscriptionConfirm promotes it
// to "active").
const (
	partnerSubscriptionStatusActive    = "active"
	partnerSubscriptionStatusPreactive = "preactive"
	partnerSubscriptionStatusInactive  = "inactive"
)

// partnerAcquisitionActionOptin/Confirm are the values sent as
// service.PartnerSubscriptionRequest.Action, matching acquisition-api's own
// PartnerSubscriptionAction vocabulary ("optin"/"confirm") exactly.
const (
	partnerAcquisitionActionOptin   = "optin"
	partnerAcquisitionActionConfirm = "confirm"
)

// notifyAcquisitionPartnerSubscription fires a best-effort, non-blocking
// notification to acquisition-api so this tenant partner-route
// optin/confirm becomes visible to acquisition reporting (KPIs, funnel,
// transactions), which otherwise only ever sees web-checkout acquisitions
// created through CreateTransaction/ConfirmTransaction. It must never affect
// the already-computed TIMWE-proxied response: failures are only logged,
// inside AcquisitionClient itself, via NotifyPartnerSubscriptionAsync.
func (h *PartnerHandler) notifyAcquisitionPartnerSubscription(action string, route domain.TenantRouteContext, msisdn string, productID int, entryChannel string, resp *domain.MTResponse) {
	if h.acqClient == nil {
		return
	}
	timweTxID, _ := transactionIDFromResponse(resp)
	h.acqClient.NotifyPartnerSubscriptionAsync(&service.PartnerSubscriptionRequest{
		Action:             action,
		TenantID:           route.TenantID,
		ChannelID:          route.ChannelID,
		ChannelKey:         route.ChannelKey,
		MSISDN:             msisdn,
		ProductID:          productID,
		TimweTransactionID: timweTxID,
		SubscriptionResult: subscriptionResultFromResponse(resp),
		EntryChannel:       entryChannel,
	})
}

// partnerOptinStatusFromResult maps a TIMWE optin subscriptionResult to the
// status persistPartnerOptinSubscription should apply. Any other or absent
// result (including the OPTIN_CONFIG_NOT_FOUND passthrough SendMT allows for
// SMS retry) is intentionally left unmapped so the caller skips persistence.
func partnerOptinStatusFromResult(result string) (string, bool) {
	switch result {
	case service.SubscriptionResultOptinAlreadyActive, service.SubscriptionResultOptinActiveWaitCharging:
		return partnerSubscriptionStatusActive, true
	case service.SubscriptionResultOptinPreactiveWaitConf:
		return partnerSubscriptionStatusPreactive, true
	default:
		return "", false
	}
}

// subscriptionResultFromResponse extracts responseData.subscriptionResult
// (the same key service.SubscriptionService reads internally, e.g.
// isSubscriptionAlreadyActive) from a TIMWE MTResponse.
func subscriptionResultFromResponse(resp *domain.MTResponse) string {
	if resp == nil || resp.ResponseData == nil {
		return ""
	}
	if v, ok := resp.ResponseData["subscriptionResult"]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// transactionIDFromResponse extracts responseData.transactionId (the same key
// service.SubscriptionService.getTransactionID reads) from a TIMWE MTResponse.
func transactionIDFromResponse(resp *domain.MTResponse) (string, bool) {
	if resp == nil || resp.ResponseData == nil {
		return "", false
	}
	v, ok := resp.ResponseData["transactionId"]
	if !ok || v == nil {
		return "", false
	}
	s, ok := v.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return "", false
	}
	return s, true
}

// externalTxIDFromResponse extracts the provider request id retained by
// sendMTWithRetry. It is the idempotency input shared with carrier callbacks.
func externalTxIDFromResponse(resp *domain.MTResponse) (string, bool) {
	if resp == nil || resp.ResponseData == nil {
		return "", false
	}
	v, ok := resp.ResponseData["externalTxId"]
	if !ok || v == nil {
		return "", false
	}
	s, ok := v.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return "", false
	}
	return s, true
}

// persistPartnerOptinSubscription upserts a tenant-scoped subscription row
// after a successful gateway-trust partner optin (h.svc.SendMT returned
// err == nil, meaning validateMTResponse already accepted the response).
// Persistence failures are logged and swallowed: the TIMWE action already
// happened and the proxied response body must never change because of a
// local DB error.
func (h *PartnerHandler) persistPartnerOptinSubscription(route domain.TenantRouteContext, req domain.MTRequest, resp *domain.MTResponse) {
	persister, ok := h.tenantRepo.(subscriptionPersister)
	if !ok {
		return
	}
	subscriptionResult := subscriptionResultFromResponse(resp)
	status, mapped := partnerOptinStatusFromResult(subscriptionResult)
	if !mapped {
		return
	}
	transactionID, ok := transactionIDFromResponse(resp)
	if !ok {
		h.logger.Warn("partner optin succeeded but transactionId missing from response; skipping persistence",
			zap.String("tenant_id", route.TenantID),
			zap.String("channel_id", route.ChannelID))
		return
	}
	partnerRoleID, err := h.svc.PartnerRoleIDForRoute(context.Background(), service.ChannelOperationMT, route)
	if err != nil {
		h.logger.Error("partner optin succeeded but partner role could not be resolved; skipping persistence",
			zap.Error(err),
			zap.String("tenant_id", route.TenantID),
			zap.String("channel_id", route.ChannelID))
		return
	}
	subscriptionRequest := domain.MapMTRequestToSubscriptionRequest(req, transactionID, partnerRoleID, "GATEWAY", "GATEWAY")
	if err := persister.CreateSubscription(&subscriptionRequest); err != nil {
		h.logger.Error("failed to persist partner optin subscription",
			zap.Error(err),
			zap.String("tenant_id", route.TenantID),
			zap.String("channel_id", route.ChannelID))
		return
	}
	// CreateSubscription's INSERT/ON CONFLICT statement does not include the
	// status column, so every call leaves status at the table default
	// ("active"). That is correct for the two "already active" results, but
	// wrong for the preactive (awaiting confirmation) result, which needs an
	// explicit follow-up update.
	if status == partnerSubscriptionStatusPreactive {
		if err := persister.UpdateSubscriptionStatus(req.UserIdentifier, strconv.Itoa(req.ProductID), status); err != nil {
			h.logger.Error("failed to set preactive status on partner optin subscription",
				zap.Error(err),
				zap.String("tenant_id", route.TenantID),
				zap.String("channel_id", route.ChannelID))
		}
	}
	// Both results mean the carrier already holds an active subscription (the
	// same pair isSubscriptionAlreadyActive recognises), so no USER_OPTIN
	// callback will follow and the confirmation SMS must be triggered here.
	// Observed in production 2026-08-19: a WAP re-optin for an active
	// subscriber returned OPTIN_ACTIVE_WAIT_CHARGING and no callback arrived.
	if subscriptionResult == service.SubscriptionResultOptinAlreadyActive ||
		subscriptionResult == service.SubscriptionResultOptinActiveWaitCharging {
		h.notifyExistingSubscriptionOptin(route, partnerRoleID, req, transactionID, subscriptionResult, resp)
	}
}

func (h *PartnerHandler) notifyExistingSubscriptionOptin(route domain.TenantRouteContext, partnerRoleID int, req domain.MTRequest, transactionID, subscriptionResult string, resp *domain.MTResponse) {
	if h.optinNotifier == nil {
		return
	}
	externalTxID, ok := externalTxIDFromResponse(resp)
	if !ok {
		h.logger.Warn("existing-subscription optin response missing externalTxId; skipping confirmation SMS",
			zap.String("subscription_result", subscriptionResult),
			zap.String("tenant_id", route.TenantID),
			zap.String("channel_id", route.ChannelID))
		return
	}
	tenantID := route.TenantID
	channelID := route.ChannelID
	notification := &domain.NotificationRequest{
		TenantID:        &tenantID,
		ChannelID:       &channelID,
		PartnerRole:     partnerRoleID,
		ExternalTxID:    externalTxID,
		ProductID:       req.ProductID,
		PricepointID:    req.PricepointID,
		MCC:             req.MCC,
		MNC:             req.MNC,
		MSISDN:          req.UserIdentifier,
		LargeAccount:    req.LargeAccount,
		TransactionUUID: transactionID,
		EntryChannel:    req.EntryChannel,
		MessageType:     subscriptionResult,
		Message:         "Subscription already active",
		Tags:            []string{"subscription-external", "existing-subscription"},
		Type:            "USER_OPTIN",
	}
	if err := h.optinNotifier.NotifyUserOptin(context.Background(), route, partnerRoleID, notification); err != nil {
		h.logger.Error("failed to enqueue existing-subscription optin confirmation SMS",
			zap.Error(err),
			zap.String("subscription_result", subscriptionResult),
			zap.String("tenant_id", route.TenantID),
			zap.String("channel_id", route.ChannelID))
	}
}

// persistPartnerConfirmSubscription activates a tenant-scoped subscription
// row after a successful gateway-trust partner optin confirm
// (h.svc.SendOptinConfirm returned err == nil). See
// persistPartnerOptinSubscription for the never-fail-the-response contract.
func (h *PartnerHandler) persistPartnerConfirmSubscription(route domain.TenantRouteContext, req domain.SubscriptionConfirmationRequest) {
	persister, ok := h.tenantRepo.(subscriptionPersister)
	if !ok {
		return
	}
	if err := persister.UpdateSubscriptionStatus(req.UserIdentifier, strconv.Itoa(req.ProductId), partnerSubscriptionStatusActive); err != nil {
		h.logger.Error("failed to activate partner subscription after optin confirm",
			zap.Error(err),
			zap.String("tenant_id", route.TenantID),
			zap.String("channel_id", route.ChannelID))
	}
}

// persistPartnerOptoutSubscription deactivates a tenant-scoped subscription
// row after a successful gateway-trust partner optout (h.svc.SendOptout
// returned err == nil). See persistPartnerOptinSubscription for the
// never-fail-the-response contract.
func (h *PartnerHandler) persistPartnerOptoutSubscription(route domain.TenantRouteContext, req domain.UnsubscriptionRequest) {
	persister, ok := h.tenantRepo.(subscriptionPersister)
	if !ok {
		return
	}
	if err := persister.UpdateSubscriptionStatus(req.UserIdentifier, strconv.Itoa(req.ProductId), partnerSubscriptionStatusInactive); err != nil {
		h.logger.Error("failed to deactivate partner subscription after optout",
			zap.Error(err),
			zap.String("tenant_id", route.TenantID),
			zap.String("channel_id", route.ChannelID))
	}
}

// checkGatewayTrust verifies the X-Gateway-Trust header injected by KrakenD.
//
// When GATEWAY_TRUST_REQUIRED=false (default), a missing or invalid header only
// logs a structured warning — no request is rejected. This allows services to be
// deployed before the KrakenD configuration change without breaking live traffic.
//
// Set GATEWAY_TRUST_REQUIRED=true after KrakenD is confirmed to be injecting the
// header on all gateway-routed requests.
//
// Returns true when the caller should proceed (either trust verified, or
// enforcement is off). Returns false and writes a 403 response when enforcement
// is on and the header is missing or invalid.
func (h *PartnerHandler) checkGatewayTrust(ctx *fasthttp.RequestCtx) bool {
	if h.cfg == nil {
		return true
	}
	secret := strings.TrimSpace(h.cfg.Auth.GatewayTrust.Secret)
	required := h.cfg.Auth.GatewayTrust.Required

	err := tenantctx.VerifyGatewayTrust(fastHTTPHeaderGetter{ctx: ctx}, tenantctx.GatewayTrustOptions{Secret: secret})
	if err == nil {
		return true
	}
	if !required {
		h.logger.Warn("gateway trust marker missing or invalid (enforcement disabled)",
			zap.String("error", err.Error()),
			zap.String("path", string(ctx.Path())),
		)
		return true
	}
	writeError(ctx, fasthttp.StatusForbidden, "GATEWAY_TRUST_REQUIRED", "request must originate from the API gateway")
	return false
}

type fastHTTPHeaderGetter struct {
	ctx *fasthttp.RequestCtx
}

func (g fastHTTPHeaderGetter) Get(name string) string {
	return string(g.ctx.Request.Header.Peek(name))
}

func tenantRouteFromRequest(ctx *fasthttp.RequestCtx, cfg *config.Config, required bool, bodyChannelID, bodyChannelKey string) (domain.TenantRouteContext, error) {
	return tenantRouteFromRequestWithNonce(ctx, cfg, required, bodyChannelID, bodyChannelKey, nil)
}

func tenantRouteFromRequestWithNonce(ctx *fasthttp.RequestCtx, cfg *config.Config, required bool, bodyChannelID, bodyChannelKey string, nonceStore tenantctx.NonceStore) (domain.TenantRouteContext, error) {
	if !required && firstHeader(ctx, "X-Tenant-Channel-Id", "X-Channel-Id") == "" &&
		firstHeader(ctx, "X-Tenant-Channel-Key", "X-Channel-Key") == "" &&
		firstHeader(ctx, tenantctx.HeaderTenantID, tenantctx.HeaderTenantKey) == "" {
		return domain.TenantRouteContext{}, nil
	}
	if cfg == nil || strings.TrimSpace(cfg.Auth.JwtToken.Secret) == "" {
		return domain.TenantRouteContext{}, fmt.Errorf("trusted service secret is not configured")
	}
	identity, err := tenantctx.IdentityFromTrustedRequest(
		string(ctx.Method()),
		string(ctx.Path()),
		fastHTTPHeaderGetter{ctx: ctx},
		tenantctx.TrustedHeaderOptions{
			Secret:     cfg.Auth.JwtToken.Secret,
			MaxSkew:    5 * time.Minute,
			NonceStore: nonceStore, // FIX 4: prevent replay within skew window
		},
	)
	if err != nil {
		return domain.TenantRouteContext{}, err
	}

	// Resolve channel key through the canonical resolver so header-vs-query
	// conflict is detected consistently.
	channelPair, err := tenantctx.ResolveKeyPair(
		fastHTTPHeaderGetter{ctx: ctx},
		tenantctx.KeyPair{
			ChannelKey: strings.TrimSpace(string(ctx.QueryArgs().Peek("channel_key"))),
		},
		tenantctx.ResolveKeyPairOptions{
			GatewayTrusted: true, // trusted because IdentityFromTrustedRequest succeeded above
		},
	)
	if err != nil {
		return domain.TenantRouteContext{}, err
	}

	// Channel header wins; body field fills in when header is absent.
	channelID := firstHeader(ctx, "X-Tenant-Channel-Id", "X-Channel-Id")
	if strings.TrimSpace(channelID) == "" {
		channelID = strings.TrimSpace(bodyChannelID)
	}
	channelKey := channelPair.ChannelKey
	if channelKey == "" {
		channelKey = firstHeader(ctx, "X-Tenant-Channel-Key", "X-Channel-Key")
	}
	if strings.TrimSpace(channelKey) == "" {
		channelKey = strings.TrimSpace(bodyChannelKey)
	}

	if strings.TrimSpace(channelID) == "" && strings.TrimSpace(channelKey) == "" {
		return domain.TenantRouteContext{}, fmt.Errorf("tenant channel context is required")
	}
	return domain.TenantRouteContext{
		TenantID:   identity.TenantID,
		TenantKey:  identity.TenantKey,
		ChannelID:  strings.TrimSpace(channelID),
		ChannelKey: strings.TrimSpace(channelKey),
	}, nil
}

func firstHeader(ctx *fasthttp.RequestCtx, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(string(ctx.Request.Header.Peek(name))); value != "" {
			return value
		}
	}
	return ""
}

func tenantRouteStatus(err error) int {
	if err == nil {
		return fasthttp.StatusBadRequest
	}
	if errors.Is(err, tenantctx.ErrTenantKeyConflict) {
		return fasthttp.StatusConflict
	}
	if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "not configured") {
		return fasthttp.StatusBadRequest
	}
	return fasthttp.StatusForbidden
}

func serviceErrorStatus(err error) int {
	switch {
	case errors.Is(err, service.ErrUnsupportedChannelOperation):
		return fasthttp.StatusUnprocessableEntity
	case errors.Is(err, service.ErrTenantCredentialMissing), errors.Is(err, service.ErrTenantCredentialInvalid), errors.Is(err, service.ErrTenantRoutingNotConfigured):
		return fasthttp.StatusFailedDependency
	case errors.Is(err, service.ErrTenantRoutingRequired):
		return fasthttp.StatusBadRequest
	case errors.Is(err, service.ErrTenantChannelNotFound):
		return fasthttp.StatusForbidden
	default:
		return fasthttp.StatusBadRequest
	}
}

func serviceErrorCode(err error) string {
	switch {
	case errors.Is(err, service.ErrUnsupportedChannelOperation):
		return "unsupported_channel_operation"
	case errors.Is(err, service.ErrTenantCredentialMissing), errors.Is(err, service.ErrTenantCredentialInvalid):
		return "tenant_channel_credential_error"
	case errors.Is(err, service.ErrTenantRoutingRequired):
		return "tenant_context_required"
	case errors.Is(err, service.ErrTenantChannelNotFound):
		return "tenant_channel_not_found"
	default:
		return "INTERNAL_ERROR"
	}
}

func writeError(ctx *fasthttp.RequestCtx, statusCode int, code, message string) {
	resp := map[string]interface{}{
		"responseData": map[string]interface{}{},
		"message":      message,
		"inError":      true,
		"requestId":    "",
		"code":         code,
	}
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(statusCode)
	_ = json.NewEncoder(ctx).Encode(resp)
}
