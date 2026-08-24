package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/seidu626/subscription-manager/subscription-external/internal/appauth"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// AppFeedRepo is the persistence surface AppHandler depends on. Defined here
// (rather than imported from repository) so handler tests can supply a fake
// without touching a database.
type AppFeedRepo interface {
	ListFeed(ctx context.Context, msisdn string, limit int) ([]domain.AppFeedItem, error)
	GetFeedItem(ctx context.Context, msisdn string, id int64) (*domain.AppFeedItem, error)
	MarkRead(ctx context.Context, msisdn string, id int64) error
	UpsertDevice(ctx context.Context, msisdn, tenantKey, fcmToken, platform string) error
	UpsertNotificationPref(ctx context.Context, msisdn, productSlug, channel string) error
	ListNotificationPrefs(ctx context.Context, msisdn string) ([]domain.AppNotificationPref, error)
}

// appBearerValidator is the subset of *appauth.Validator that AppHandler
// depends on, so tests can inject a fake.
type appBearerValidator interface {
	ValidateBearer(authorizationHeader string) (*appauth.Claims, error)
}

// AppHandler implements the Dayline app's feed, device-registration, and
// notification-preference endpoints under /v1/app/*.
// See docs/dayline-app-api-contract.md.
type AppHandler struct {
	repo      AppFeedRepo
	validator appBearerValidator
	logger    *zap.Logger
}

// NewAppHandler builds an AppHandler. validator may be nil when
// DAYLINE_APP_JWT_SECRET is unset; in that case every request fails closed
// with 401 UNAUTHORIZED rather than skipping authentication.
func NewAppHandler(repo AppFeedRepo, validator appBearerValidator, logger *zap.Logger) *AppHandler {
	return &AppHandler{repo: repo, validator: validator, logger: logger}
}

const feedListLimit = 50

func writeAppJSON(ctx *fasthttp.RequestCtx, status int, v interface{}) {
	body, err := json.Marshal(v)
	if err != nil {
		writeAppError(ctx, fasthttp.StatusInternalServerError, "PROVIDER_ERROR", "failed to encode response")
		return
	}
	ctx.SetStatusCode(status)
	ctx.SetContentType("application/json")
	ctx.SetBody(body)
}

// writeAppError writes the contract error envelope:
// {"error": {"code": "...", "message": "..."}}.
func writeAppError(ctx *fasthttp.RequestCtx, status int, code, message string) {
	ctx.SetStatusCode(status)
	ctx.SetContentType("application/json")
	body, _ := json.Marshal(map[string]interface{}{
		"error": map[string]string{"code": code, "message": message},
	})
	ctx.SetBody(body)
}

func (h *AppHandler) authenticate(ctx *fasthttp.RequestCtx) (*appauth.Claims, bool) {
	if h.validator == nil {
		writeAppError(ctx, fasthttp.StatusUnauthorized, "UNAUTHORIZED", "authentication is not configured")
		return nil, false
	}
	claims, err := h.validator.ValidateBearer(string(ctx.Request.Header.Peek("Authorization")))
	if err != nil {
		writeAppError(ctx, fasthttp.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid bearer token")
		return nil, false
	}
	return claims, true
}

// GetFeed handles GET /v1/app/feed.
func (h *AppHandler) GetFeed(ctx *fasthttp.RequestCtx) {
	claims, ok := h.authenticate(ctx)
	if !ok {
		return
	}
	items, err := h.repo.ListFeed(ctx, claims.Subject, feedListLimit)
	if err != nil {
		h.logger.Error("dayline app: list feed failed", zap.Error(err))
		writeAppError(ctx, fasthttp.StatusInternalServerError, "PROVIDER_ERROR", "failed to load feed")
		return
	}
	writeAppJSON(ctx, fasthttp.StatusOK, domain.AppFeedListResponse{Items: items})
}

// GetFeedItem handles GET /v1/app/feed/items/{id}.
func (h *AppHandler) GetFeedItem(ctx *fasthttp.RequestCtx, idParam string) {
	claims, ok := h.authenticate(ctx)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(idParam), 10, 64)
	if err != nil {
		writeAppError(ctx, fasthttp.StatusBadRequest, "VALIDATION", "invalid feed item id")
		return
	}
	item, err := h.repo.GetFeedItem(ctx, claims.Subject, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAppError(ctx, fasthttp.StatusNotFound, "NOT_FOUND", "feed item not found")
			return
		}
		h.logger.Error("dayline app: get feed item failed", zap.Error(err))
		writeAppError(ctx, fasthttp.StatusInternalServerError, "PROVIDER_ERROR", "failed to load feed item")
		return
	}
	writeAppJSON(ctx, fasthttp.StatusOK, item)
}

// MarkFeedItemRead handles POST /v1/app/feed/items/{id}/read.
func (h *AppHandler) MarkFeedItemRead(ctx *fasthttp.RequestCtx, idParam string) {
	claims, ok := h.authenticate(ctx)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(idParam), 10, 64)
	if err != nil {
		writeAppError(ctx, fasthttp.StatusBadRequest, "VALIDATION", "invalid feed item id")
		return
	}
	if err := h.repo.MarkRead(ctx, claims.Subject, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAppError(ctx, fasthttp.StatusNotFound, "NOT_FOUND", "feed item not found")
			return
		}
		h.logger.Error("dayline app: mark feed item read failed", zap.Error(err))
		writeAppError(ctx, fasthttp.StatusInternalServerError, "PROVIDER_ERROR", "failed to mark feed item read")
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

// RegisterDevice handles POST /v1/app/devices.
func (h *AppHandler) RegisterDevice(ctx *fasthttp.RequestCtx) {
	claims, ok := h.authenticate(ctx)
	if !ok {
		return
	}
	var req domain.AppDeviceRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeAppError(ctx, fasthttp.StatusBadRequest, "VALIDATION", "invalid request body")
		return
	}
	token := strings.TrimSpace(req.FCMToken)
	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	if token == "" || (platform != "android" && platform != "ios") {
		writeAppError(ctx, fasthttp.StatusBadRequest, "VALIDATION", "fcm_token and platform (android|ios) are required")
		return
	}
	if err := h.repo.UpsertDevice(ctx, claims.Subject, claims.Tenant, token, platform); err != nil {
		h.logger.Error("dayline app: register device failed", zap.Error(err))
		writeAppError(ctx, fasthttp.StatusInternalServerError, "PROVIDER_ERROR", "failed to register device")
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

// GetNotificationPrefs handles GET /v1/app/notification-prefs. Products with
// no stored row are simply absent; the app treats them as its default (BOTH).
func (h *AppHandler) GetNotificationPrefs(ctx *fasthttp.RequestCtx) {
	claims, ok := h.authenticate(ctx)
	if !ok {
		return
	}
	prefs, err := h.repo.ListNotificationPrefs(ctx, claims.Subject)
	if err != nil {
		h.logger.Error("dayline app: list notification prefs failed", zap.Error(err))
		writeAppError(ctx, fasthttp.StatusInternalServerError, "PROVIDER_ERROR", "failed to load notification preferences")
		return
	}
	writeAppJSON(ctx, fasthttp.StatusOK, domain.AppNotificationPrefsResponse{Prefs: prefs})
}

// SetNotificationPrefs handles PUT /v1/app/notification-prefs.
func (h *AppHandler) SetNotificationPrefs(ctx *fasthttp.RequestCtx) {
	claims, ok := h.authenticate(ctx)
	if !ok {
		return
	}
	var req domain.AppNotificationPrefRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeAppError(ctx, fasthttp.StatusBadRequest, "VALIDATION", "invalid request body")
		return
	}
	slug := strings.TrimSpace(req.ProductSlug)
	channel := strings.ToUpper(strings.TrimSpace(req.Channel))
	if slug == "" || (channel != "PUSH" && channel != "SMS" && channel != "BOTH") {
		writeAppError(ctx, fasthttp.StatusBadRequest, "VALIDATION", "product_slug and channel (PUSH|SMS|BOTH) are required")
		return
	}
	if err := h.repo.UpsertNotificationPref(ctx, claims.Subject, slug, channel); err != nil {
		h.logger.Error("dayline app: set notification prefs failed", zap.Error(err))
		writeAppError(ctx, fasthttp.StatusInternalServerError, "PROVIDER_ERROR", "failed to set notification preference")
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}
