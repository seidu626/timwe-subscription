package handler

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/appauth"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/service"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// AppHandler serves the Dayline mobile app's /v1/app/* endpoints: auth OTP,
// catalog, and subscription management. It is a thin HTTP layer over
// AppOTPService/AppCatalogService/AppSubscriptionService and the appauth JWT
// validator; it never talks to the database or TIMWE directly.
type AppHandler struct {
	otpService   *service.AppOTPService
	catalog      *service.AppCatalogService
	subs         *service.AppSubscriptionService
	jwtValidator *appauth.Validator
	logger       *zap.Logger
}

// NewAppHandler creates a new AppHandler.
func NewAppHandler(
	otpService *service.AppOTPService,
	catalog *service.AppCatalogService,
	subs *service.AppSubscriptionService,
	jwtValidator *appauth.Validator,
	logger *zap.Logger,
) *AppHandler {
	return &AppHandler{
		otpService:   otpService,
		catalog:      catalog,
		subs:         subs,
		jwtValidator: jwtValidator,
		logger:       logger,
	}
}

// appErrorEnvelope is the contract's error response shape:
// {"error": {"code": "STRING_CODE", "message": "human readable"}}.
type appErrorEnvelope struct {
	Error appErrorBody `json:"error"`
}

type appErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeAppError writes the contract's nested error envelope. It is the only
// error writer used under /v1/app/*; the flat writeJSONError used by the rest
// of this package does not match the app contract's shape.
func writeAppError(ctx *fasthttp.RequestCtx, code domain.AppErrorCode, message string) {
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(appErrorStatus(code))
	_ = json.NewEncoder(ctx).Encode(appErrorEnvelope{Error: appErrorBody{Code: string(code), Message: message}})
}

// writeAppErr writes err as the contract's error envelope, translating a
// *domain.AppError to its declared code/status and collapsing anything else
// to a non-leaky VALIDATION 400.
func writeAppErr(ctx *fasthttp.RequestCtx, err error) {
	if appErr, ok := err.(*domain.AppError); ok {
		writeAppError(ctx, appErr.Code, appErr.Message)
		return
	}
	writeAppError(ctx, domain.AppErrValidation, "invalid request")
}

// appErrorStatus maps the contract's fixed error codes to HTTP status codes.
func appErrorStatus(code domain.AppErrorCode) int {
	switch code {
	case domain.AppErrInvalidMSISDN, domain.AppErrValidation, domain.AppErrOTPInvalid, domain.AppErrOTPExpired:
		return fasthttp.StatusBadRequest
	case domain.AppErrUnauthorized:
		return fasthttp.StatusUnauthorized
	case domain.AppErrNotFound:
		return fasthttp.StatusNotFound
	case domain.AppErrConflict:
		return fasthttp.StatusConflict
	case domain.AppErrRateLimited:
		return fasthttp.StatusTooManyRequests
	case domain.AppErrProviderError:
		return fasthttp.StatusBadGateway
	default:
		return fasthttp.StatusBadRequest
	}
}

// authenticate extracts and validates the caller's Dayline app session JWT.
// On failure it writes the UNAUTHORIZED envelope itself and returns ok=false;
// callers must return immediately when ok is false.
func (h *AppHandler) authenticate(ctx *fasthttp.RequestCtx) (claims *appauth.Claims, ok bool) {
	if h.jwtValidator == nil {
		writeAppError(ctx, domain.AppErrUnauthorized, "session validation unavailable")
		return nil, false
	}
	authHeader := string(ctx.Request.Header.Peek("Authorization"))
	token := appauth.BearerToken(authHeader)
	if token == "" {
		writeAppError(ctx, domain.AppErrUnauthorized, "missing bearer token")
		return nil, false
	}
	claims, err := h.jwtValidator.Parse(token)
	if err != nil {
		writeAppError(ctx, domain.AppErrUnauthorized, "invalid or expired session")
		return nil, false
	}
	return claims, true
}

type appOTPRequestBody struct {
	MSISDN string `json:"msisdn"`
	Tenant string `json:"tenant"`
}

// RequestOTP handles POST /v1/app/auth/otp/request.
func (h *AppHandler) RequestOTP(ctx *fasthttp.RequestCtx) {
	var req appOTPRequestBody
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeAppError(ctx, domain.AppErrValidation, "invalid request body")
		return
	}
	if err := h.otpService.RequestOTP(req.MSISDN, req.Tenant); err != nil {
		writeAppErr(ctx, err)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

type appOTPVerifyBody struct {
	MSISDN string `json:"msisdn"`
	Tenant string `json:"tenant"`
	Code   string `json:"code"`
}

type appOTPVerifyResponse struct {
	Token     string `json:"token"`
	ExpiresIn int64  `json:"expires_in"`
}

// VerifyOTP handles POST /v1/app/auth/otp/verify.
func (h *AppHandler) VerifyOTP(ctx *fasthttp.RequestCtx) {
	var req appOTPVerifyBody
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeAppError(ctx, domain.AppErrValidation, "invalid request body")
		return
	}
	if err := h.otpService.VerifyOTP(req.MSISDN, req.Tenant, req.Code); err != nil {
		writeAppErr(ctx, err)
		return
	}

	if h.jwtValidator == nil {
		writeAppError(ctx, domain.AppErrUnauthorized, "session issuance unavailable")
		return
	}

	token, err := h.jwtValidator.IssueToken(strings.TrimSpace(req.MSISDN), strings.TrimSpace(req.Tenant), time.Now())
	if err != nil {
		h.logger.Error("failed to issue app session token", zap.Error(err))
		writeAppError(ctx, domain.AppErrProviderError, "failed to issue session token")
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(appOTPVerifyResponse{
		Token:     token,
		ExpiresIn: int64(appauth.TokenTTL.Seconds()),
	})
}

type appCatalogResponse struct {
	Products []*domain.AppCatalogProduct `json:"products"`
}

type appMarketplaceResponse struct {
	Tenants []*domain.AppMarketplaceTenant `json:"tenants"`
}

// Catalog handles GET /v1/app/catalog. No auth required per the contract.
// With a tenant query arg it returns that tenant's flat product list; without
// one it returns the marketplace view, grouped per tenant.
func (h *AppHandler) Catalog(ctx *fasthttp.RequestCtx) {
	tenant := strings.TrimSpace(string(ctx.QueryArgs().Peek("tenant")))
	country := string(ctx.QueryArgs().Peek("country"))

	ctx.SetContentType("application/json")
	if tenant == "" {
		tenants, err := h.catalog.Marketplace(country)
		if err != nil {
			writeAppErr(ctx, err)
			return
		}
		ctx.SetStatusCode(fasthttp.StatusOK)
		_ = json.NewEncoder(ctx).Encode(appMarketplaceResponse{Tenants: tenants})
		return
	}

	products, err := h.catalog.List(tenant, country)
	if err != nil {
		writeAppErr(ctx, err)
		return
	}

	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(appCatalogResponse{Products: products})
}

type appCreateSubscriptionBody struct {
	CampaignSlug string `json:"campaign_slug"`
	// Tenant optionally names the tenant that owns campaign_slug. The
	// marketplace sells across tenants, so the product's tenant wins over
	// the login tenant; empty keeps the JWT tenant for older clients.
	Tenant string `json:"tenant"`
}

type appCreateSubscriptionResponse struct {
	SubscriptionRef string                 `json:"subscription_ref"`
	NextAction      domain.NextAction      `json:"next_action"`
	Message         string                 `json:"message,omitempty"`
	Payload         map[string]interface{} `json:"payload,omitempty"`
}

// CreateSubscription handles POST /v1/app/subscriptions (auth required).
func (h *AppHandler) CreateSubscription(ctx *fasthttp.RequestCtx) {
	claims, ok := h.authenticate(ctx)
	if !ok {
		return
	}

	var req appCreateSubscriptionBody
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeAppError(ctx, domain.AppErrValidation, "invalid request body")
		return
	}

	tenant := strings.TrimSpace(req.Tenant)
	if tenant == "" {
		tenant = claims.Tenant
	}
	resp, err := h.subs.Create(claims.MSISDN, tenant, req.CampaignSlug)
	if err != nil {
		writeAppErr(ctx, err)
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(appCreateSubscriptionResponse{
		SubscriptionRef: resp.TransactionID.String(),
		NextAction:      resp.NextAction,
		Payload:         resp.Payload,
	})
}

type appConfirmSubscriptionBody struct {
	PIN string `json:"pin"`
}

type appConfirmSubscriptionResponse struct {
	Status string `json:"status"`
}

// ConfirmSubscription handles POST /v1/app/subscriptions/{ref}/confirm (auth required).
func (h *AppHandler) ConfirmSubscription(ctx *fasthttp.RequestCtx) {
	claims, ok := h.authenticate(ctx)
	if !ok {
		return
	}

	ref := appSubscriptionRefFromPath(string(ctx.Path()), "/confirm")
	if ref == "" {
		writeAppError(ctx, domain.AppErrNotFound, "subscription not found")
		return
	}

	var req appConfirmSubscriptionBody
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeAppError(ctx, domain.AppErrValidation, "invalid request body")
		return
	}

	resp, err := h.subs.Confirm(ref, claims.MSISDN, req.PIN)
	if err != nil {
		writeAppErr(ctx, err)
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(appConfirmSubscriptionResponse{
		Status: domain.MapTransactionStatusToApp(resp.Status),
	})
}

type appListSubscriptionsResponse struct {
	Subscriptions []*domain.AppSubscription `json:"subscriptions"`
}

// ListSubscriptions handles GET /v1/app/subscriptions (auth required).
func (h *AppHandler) ListSubscriptions(ctx *fasthttp.RequestCtx) {
	claims, ok := h.authenticate(ctx)
	if !ok {
		return
	}

	subs, err := h.subs.List(claims.MSISDN)
	if err != nil {
		writeAppErr(ctx, err)
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(appListSubscriptionsResponse{Subscriptions: subs})
}

// CancelSubscription handles DELETE /v1/app/subscriptions/{ref} (auth required).
func (h *AppHandler) CancelSubscription(ctx *fasthttp.RequestCtx) {
	claims, ok := h.authenticate(ctx)
	if !ok {
		return
	}

	ref := appSubscriptionRefFromPath(string(ctx.Path()), "")
	if ref == "" {
		writeAppError(ctx, domain.AppErrNotFound, "subscription not found")
		return
	}

	if err := h.subs.Cancel(ref, claims.MSISDN); err != nil {
		writeAppErr(ctx, err)
		return
	}

	ctx.SetStatusCode(fasthttp.StatusAccepted)
}

// appSubscriptionRefFromPath extracts the {ref} path segment from
// /v1/app/subscriptions/{ref} or /v1/app/subscriptions/{ref}/confirm.
func appSubscriptionRefFromPath(path, suffix string) string {
	path = strings.TrimSuffix(path, suffix)
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
