package handler

import (
	"encoding/json"

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
	logger *zap.Logger
	svc    *service.SubscriptionService
	cfg    *config.Config
}

func NewPartnerHandler(logger *zap.Logger, svc *service.SubscriptionService, cfg *config.Config) *PartnerHandler {
	return &PartnerHandler{logger: logger, svc: svc, cfg: cfg}
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
	}

	resp, err := h.svc.SendMT(mtReq, h.cfg.Application.TIMWE.Realm, channel)
	if err != nil {
		h.logger.Error("Partner MT failed", zap.Error(err))
		writeError(ctx, fasthttp.StatusBadRequest, "INTERNAL_ERROR", err.Error())
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

	resp, err := h.svc.RequestCharge(req)
	if err != nil {
		h.logger.Error("Partner charge failed", zap.Error(err))
		writeError(ctx, fasthttp.StatusBadRequest, "INTERNAL_ERROR", err.Error())
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

	resp, err := h.svc.SendStatusCheck(req, h.cfg.Application.TIMWE.Realm)
	if err != nil {
		h.logger.Error("Partner status check failed", zap.Error(err))
		writeError(ctx, fasthttp.StatusBadRequest, "INTERNAL_ERROR", err.Error())
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
	resp, err := h.svc.SendOptout(req, h.cfg.Application.TIMWE.Realm)
	if err != nil {
		h.logger.Error("Partner optout failed", zap.Error(err))
		writeError(ctx, fasthttp.StatusBadRequest, "INTERNAL_ERROR", err.Error())
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
	resp, err := h.svc.SendOptinConfirm(req, h.cfg.Application.TIMWE.Realm)
	if err != nil {
		h.logger.Error("Partner optin confirm failed", zap.Error(err))
		writeError(ctx, fasthttp.StatusBadRequest, "INTERNAL_ERROR", err.Error())
		return
	}
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(resp)
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
