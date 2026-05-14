package handler

import (
	"encoding/json"
	"errors"
	"fmt"
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
}

func NewPartnerHandler(logger *zap.Logger, svc *service.SubscriptionService, cfg *config.Config) *PartnerHandler {
	return &PartnerHandler{logger: logger, svc: svc, cfg: cfg}
}

// WithTenantRepo sets the repository used by gateway-trust partner subscription handlers.
// Call this after NewPartnerHandler when the concrete repository implements gatewayTenantLookup.
func (h *PartnerHandler) WithTenantRepo(repo gatewayTenantLookup) *PartnerHandler {
	h.tenantRepo = repo
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
	// Parse body
	var req partnerMtRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload")
		return
	}
	route, err := tenantRouteFromRequest(ctx, h.cfg, true, req.ChannelID, req.ChannelKey)
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

	var req domain.ChargeRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload")
		return
	}
	route, err := tenantRouteFromRequest(ctx, h.cfg, true, "", "")
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

	// Parse body
	var req domain.GetStatusRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload")
		return
	}
	route, err := tenantRouteFromRequest(ctx, h.cfg, true, "", "")
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
	var req domain.UnsubscriptionRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload")
		return
	}
	route, err := tenantRouteFromRequest(ctx, h.cfg, true, "", "")
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
	var req domain.SubscriptionConfirmationRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, "INVALID_REQUEST", "Invalid request payload")
		return
	}
	route, err := tenantRouteFromRequest(ctx, h.cfg, true, "", "")
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

// tenantRouteFromGatewayHeaders resolves tenant context from KrakenD-injected
// headers/query params (path captures: {tenant_key}/{channel_key}).
//
// GatewayTrusted is set to true because KrakenD's martian header.Modifier
// injects both X-Tenant-Key and X-Channel-Key from path captures, establishing
// gateway trust structurally.
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

// PartnerSubscriptionOptin handles POST /api/v1/subscription-external/partners/optin.
// Tenant context is resolved from KrakenD-injected headers (no trusted-service HMAC required).
func (h *PartnerHandler) PartnerSubscriptionOptin(ctx *fasthttp.RequestCtx) {
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
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(resp)
}

// PartnerSubscriptionConfirm handles POST /api/v1/subscription-external/partners/confirm.
func (h *PartnerHandler) PartnerSubscriptionConfirm(ctx *fasthttp.RequestCtx) {
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
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(resp)
}

// PartnerSubscriptionOptout handles POST /api/v1/subscription-external/partners/optout.
func (h *PartnerHandler) PartnerSubscriptionOptout(ctx *fasthttp.RequestCtx) {
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
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(resp)
}

// PartnerSubscriptionStatus handles POST /api/v1/subscription-external/partners/status.
func (h *PartnerHandler) PartnerSubscriptionStatus(ctx *fasthttp.RequestCtx) {
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

type fastHTTPHeaderGetter struct {
	ctx *fasthttp.RequestCtx
}

func (g fastHTTPHeaderGetter) Get(name string) string {
	return string(g.ctx.Request.Header.Peek(name))
}

func tenantRouteFromRequest(ctx *fasthttp.RequestCtx, cfg *config.Config, required bool, bodyChannelID, bodyChannelKey string) (domain.TenantRouteContext, error) {
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
			Secret:  cfg.Auth.JwtToken.Secret,
			MaxSkew: 5 * time.Minute,
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
