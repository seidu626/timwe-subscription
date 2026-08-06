package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/seidu626/subscription-manager/notification/internal/domain"
	"github.com/valyala/fasthttp"
)

// ListSMSTemplates lists configuration owned by the authenticated tenant.
func (h *NotificationHandler) ListSMSTemplates(ctx *fasthttp.RequestCtx) {
	tenantID := h.tenantIDForAdminRead(ctx)
	if tenantID == "" {
		ctx.Error("tenant context required", fasthttp.StatusForbidden)
		return
	}
	items, err := h.service.ListSMSTemplates(context.Background(), tenantID)
	if err != nil {
		ctx.Error("failed to list SMS templates", fasthttp.StatusInternalServerError)
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, items)
}

// GetSMSTemplate returns one tenant-owned product/event configuration.
func (h *NotificationHandler) GetSMSTemplate(ctx *fasthttp.RequestCtx, productID int) {
	tenantID := h.tenantIDForAdminRead(ctx)
	if tenantID == "" {
		ctx.Error("tenant context required", fasthttp.StatusForbidden)
		return
	}
	item, err := h.service.GetSMSTemplate(context.Background(), tenantID, productID, templateEventType(ctx))
	if errors.Is(err, sql.ErrNoRows) {
		ctx.Error("SMS template not found", fasthttp.StatusNotFound)
		return
	}
	if err != nil {
		ctx.Error("failed to get SMS template", fasthttp.StatusInternalServerError)
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, item)
}

// UpsertSMSTemplate creates or replaces one tenant-owned configuration.
func (h *NotificationHandler) UpsertSMSTemplate(ctx *fasthttp.RequestCtx, productID int) {
	tenantID := h.tenantIDForAdminRead(ctx)
	if tenantID == "" {
		ctx.Error("tenant context required", fasthttp.StatusForbidden)
		return
	}
	var input domain.SMSTemplateUpsert
	if err := json.Unmarshal(ctx.PostBody(), &input); err != nil {
		ctx.Error("invalid request payload", fasthttp.StatusBadRequest)
		return
	}
	item, err := h.service.UpsertSMSTemplate(context.Background(), tenantID, productID, input)
	if err != nil {
		ctx.Error(err.Error(), fasthttp.StatusBadRequest)
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, item)
}

// SetSMSTemplateEnabled toggles one existing tenant-owned configuration.
func (h *NotificationHandler) SetSMSTemplateEnabled(ctx *fasthttp.RequestCtx, productID int) {
	tenantID := h.tenantIDForAdminRead(ctx)
	if tenantID == "" {
		ctx.Error("tenant context required", fasthttp.StatusForbidden)
		return
	}
	var input struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.Unmarshal(ctx.PostBody(), &input); err != nil || input.Enabled == nil {
		ctx.Error("enabled boolean is required", fasthttp.StatusBadRequest)
		return
	}
	item, err := h.service.SetSMSTemplateEnabled(context.Background(), tenantID, productID, templateEventType(ctx), *input.Enabled)
	if errors.Is(err, sql.ErrNoRows) {
		ctx.Error("SMS template not found", fasthttp.StatusNotFound)
		return
	}
	if err != nil {
		ctx.Error("failed to update SMS template", fasthttp.StatusInternalServerError)
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, item)
}

func templateEventType(ctx *fasthttp.RequestCtx) string {
	return firstNonBlank(string(ctx.QueryArgs().Peek("event_type")), string(ctx.QueryArgs().Peek("eventType")), domain.UserOptinEvent)
}

// ParseTemplateProductPath parses the product and optional enabled subresource.
func (*NotificationHandler) ParseTemplateProductPath(path string) (productID int, enablePath bool, ok bool) {
	const prefix = "/api/v1/notification/sms-templates/"
	remainder := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	parts := strings.Split(remainder, "/")
	if len(parts) == 2 && parts[1] == "enabled" {
		enablePath = true
	} else if len(parts) != 1 {
		return 0, false, false
	}
	productID, err := strconv.Atoi(parts[0])
	return productID, enablePath, err == nil && productID > 0
}

func writeJSON(ctx *fasthttp.RequestCtx, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		ctx.Error("failed to format response", fasthttp.StatusInternalServerError)
		return
	}
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(status)
	ctx.SetBody(body)
}
