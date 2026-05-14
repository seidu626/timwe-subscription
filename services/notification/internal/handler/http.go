package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/seidu626/subscription-manager/notification/internal/domain"
	"github.com/seidu626/subscription-manager/notification/internal/service"
	"github.com/valyala/fasthttp"
)

const tenantIDHeader = "X-Tenant-Id"

type NotificationHandler struct {
	service *service.NotificationService
}

func NewNotificationHandler(service *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{service: service}
}

func (h *NotificationHandler) ListNotifications(ctx *fasthttp.RequestCtx) {
	log.Println("Processing notification list request")
	tenantID := h.tenantIDForAdminRead(ctx)
	if tenantID == "" {
		ctx.Error("tenant context required", fasthttp.StatusForbidden)
		return
	}

	// Extract query parameters
	entryChannel := string(ctx.QueryArgs().Peek("entry_channel"))
	if entryChannel == "" {
		entryChannel = string(ctx.QueryArgs().Peek("entryChannel"))
	}
	channelID := firstQuery(ctx, "channel_id", "channelId")
	queryParams := map[string]string{
		"startDate":     string(ctx.QueryArgs().Peek("startDate")),
		"endDate":       string(ctx.QueryArgs().Peek("endDate")),
		"tenantId":      tenantID,
		"channelId":     channelID,
		"partnerRole":   string(ctx.QueryArgs().Peek("partnerRole")),
		"msisdn":        string(ctx.QueryArgs().Peek("msisdn")),
		"type":          string(ctx.QueryArgs().Peek("type")),
		"entry_channel": entryChannel,
		"entryChannel":  entryChannel,
		"page":          string(ctx.QueryArgs().Peek("page")),
		"pageSize":      string(ctx.QueryArgs().Peek("pageSize")),
	}

	// Pass queryParams to the service 	layer
	listResponse, err := h.service.GetNotifications(queryParams)
	if err != nil {
		log.Println(err)
		ctx.Error("Error fetching listResponse", fasthttp.StatusInternalServerError)
		return
	}

	// Marshal the subscriptions to JSON
	response, err := json.Marshal(listResponse)
	if err != nil {
		ctx.Error("Error formatting response", fasthttp.StatusInternalServerError)
		return
	}

	// Prepare pagination data to be added in headers
	paginationData := struct {
		Page        int  `json:"page"`
		PageSize    int  `json:"pageSize"`
		TotalCount  int  `json:"totalCount"`
		TotalPages  int  `json:"totalPages"`
		HasNextPage bool `json:"hasNextPage"`
		HasPrevPage bool `json:"hasPrevPage"`
	}{
		Page:        listResponse.Page,
		PageSize:    listResponse.PageSize,
		TotalCount:  listResponse.TotalCount,
		TotalPages:  listResponse.TotalPages,
		HasNextPage: listResponse.HasNextPage,
		HasPrevPage: listResponse.HasPrevPage,
	}

	// Convert pagination data to JSON and set in the X-Pagination header
	paginationJSON, err := json.Marshal(paginationData)
	if err != nil {
		log.Println("Error marshalling pagination data:", err)
		ctx.Error("Error formatting pagination data", fasthttp.StatusInternalServerError)
		return
	}
	ctx.Response.Header.Set("X-Pagination", string(paginationJSON))

	// Set response headers and body
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetBody(response)
}

func (h *NotificationHandler) tenantIDForAdminRead(ctx *fasthttp.RequestCtx) string {
	identity, ok := tenantIdentityFromRequest(ctx)
	if !ok {
		return ""
	}
	if tenantID := strings.TrimSpace(identity.TenantID); tenantID != "" {
		return tenantID
	}
	if !identity.PlatformScoped {
		return ""
	}
	if tenantID := headerOrQueryTenantID(ctx); tenantID != "" {
		return tenantID
	}
	// Resolve tenant key through the canonical resolver so header-vs-query
	// conflict is detected and mixed-case keys are normalised.
	pair, err := tenantctx.ResolveKeyPair(
		fasthttpHeaderGetter{ctx: ctx},
		tenantctx.KeyPair{TenantKey: firstQuery(ctx, "tenant_key", "tenantKey")},
		tenantctx.ResolveKeyPairOptions{
			GatewayTrusted: identity.TrustSource == tenantctx.TrustSourceTrustedService,
		},
	)
	if err != nil {
		log.Printf("tenant key resolution error in notification handler: %v", err)
		return ""
	}
	tenantKey := firstNonBlank(identity.TenantKey, pair.TenantKey)
	if tenantKey == "" || h.service == nil {
		return ""
	}
	tenantID, err := h.service.TenantIDByKey(context.Background(), tenantKey)
	if err != nil {
		log.Printf("failed to resolve notification tenant key %q: %v", tenantKey, err)
		return ""
	}
	return tenantID
}

// tenantResolution is the tri-state result of resolveNotificationTenant.
type tenantResolution struct {
	TenantID     string // populated only when successfully resolved
	ChannelID    string // populated only when TenantID is set and channel was supplied
	ContextGiven bool   // true when the caller supplied tenant_key/X-Tenant-Key or tenant_id/X-Tenant-Id
	ChannelGiven bool   // true when the caller supplied channel_key/X-Channel-Key
	Invalid      bool   // true when context was supplied but did not resolve
	Reason       string // short reason code, e.g. "UNKNOWN_TENANT", "CHANNEL_REQUIRED"
}

// resolveNotificationTenant implements the partner-callback tenant resolution
// contract for handleNotification. It distinguishes three states:
//
//  1. No tenant context supplied at all → ContextGiven=false (legacy path, caller may proceed with nil tenant).
//  2. tenant_key supplied and resolves, but channel_key absent → Invalid=true, Reason="CHANNEL_REQUIRED".
//  3. tenant_key supplied but unknown → Invalid=true, Reason="UNKNOWN_TENANT".
//
// The UUID (X-Tenant-Id / tenant_id) path is treated as a trusted admin path
// and does NOT require a paired channel_key.
//
// Middleware-populated identity with a TenantID is returned directly (admin/platform path).
func (h *NotificationHandler) resolveNotificationTenant(ctx *fasthttp.RequestCtx) tenantResolution {
	// --- 1. Middleware identity: admin / platform path ---
	identity, ok := tenantIdentityFromRequest(ctx)
	if ok && strings.TrimSpace(identity.TenantID) != "" {
		// Verified identity carries a tenant UUID → return immediately (no channel requirement).
		// Still propagate any channel ID that was supplied alongside the identity.
		return tenantResolution{TenantID: strings.TrimSpace(identity.TenantID), ChannelID: channelIDFromRequest(ctx)}
	}

	// --- 2. Raw UUID via X-Tenant-Id header or tenant_id query (legacy admin path) ---
	rawTenantID := headerOrQueryTenantID(ctx)
	if rawTenantID != "" {
		// UUID-based callers do not go through the tenant_key/channel_key contract.
		// Still propagate any channel ID that was supplied (e.g. X-Tenant-Channel-Id).
		return tenantResolution{TenantID: rawTenantID, ChannelID: channelIDFromRequest(ctx), ContextGiven: true}
	}

	// --- 3. tenant_key / X-Tenant-Key partner-callback path ---
	// Route through ResolveKeyPair to preserve header-vs-query conflict detection
	// and the GatewayTrusted gate.
	tenantKeyQuery := strings.TrimSpace(firstQuery(ctx, "tenant_key", "tenantKey"))
	channelKeyQuery := strings.TrimSpace(firstQuery(ctx, "channel_key", "channelKey"))

	// Only enter the tenant_key branch if either the header or query param is present.
	hTenantKey := strings.TrimSpace(string(ctx.Request.Header.Peek(tenantctx.HeaderTenantKey)))
	hChannelKey := strings.TrimSpace(string(ctx.Request.Header.Peek(tenantctx.HeaderChannelKey)))
	if hTenantKey == "" && tenantKeyQuery == "" && hChannelKey == "" && channelKeyQuery == "" {
		// No tenant context of any kind supplied → legacy behaviour, caller proceeds with nil tenant.
		return tenantResolution{}
	}

	pair, err := tenantctx.ResolveKeyPair(
		fasthttpHeaderGetter{ctx: ctx},
		tenantctx.KeyPair{TenantKey: tenantKeyQuery, ChannelKey: channelKeyQuery},
		tenantctx.ResolveKeyPairOptions{GatewayTrusted: false},
	)
	if err != nil {
		if errors.Is(err, tenantctx.ErrTenantKeyConflict) {
			return tenantResolution{ContextGiven: true, Invalid: true, Reason: "TENANT_KEY_CONFLICT"}
		}
		// GatewayTrusted refusal or other resolver error → unknown context.
		return tenantResolution{ContextGiven: true, Invalid: true, Reason: "UNKNOWN_TENANT"}
	}
	tenantKey := pair.TenantKey
	channelKey := pair.ChannelKey

	if tenantKey == "" {
		// No tenant key after resolution → legacy behaviour.
		return tenantResolution{}
	}

	// Tenant key was supplied → it must resolve.
	if h.service == nil {
		return tenantResolution{ContextGiven: true, Invalid: true, Reason: "UNKNOWN_TENANT"}
	}
	tenantID, err := h.service.TenantIDByKey(context.Background(), tenantKey)
	if err != nil || tenantID == "" {
		log.Printf("failed to resolve notification tenant key %q: %v", tenantKey, err)
		return tenantResolution{ContextGiven: true, Invalid: true, Reason: "UNKNOWN_TENANT"}
	}

	// Tenant resolved → channel_key must also be present.
	channelGiven := channelKey != ""
	if !channelGiven {
		return tenantResolution{ContextGiven: true, Invalid: true, Reason: "CHANNEL_REQUIRED"}
	}

	// Resolve channel_key → UUID via the repository. A slug is NOT a valid UUID.
	channelID, err := h.service.ChannelIDByKeys(context.Background(), tenantID, channelKey)
	if err != nil || channelID == "" {
		log.Printf("failed to resolve channel key %q for tenant %q: %v", channelKey, tenantID, err)
		return tenantResolution{ContextGiven: true, ChannelGiven: true, Invalid: true, Reason: "UNKNOWN_CHANNEL"}
	}

	return tenantResolution{
		TenantID:     tenantID,
		ChannelID:    channelID,
		ContextGiven: true,
		ChannelGiven: true,
	}
}

type fasthttpHeaderGetter struct {
	ctx *fasthttp.RequestCtx
}

func (g fasthttpHeaderGetter) Get(name string) string {
	return string(g.ctx.Request.Header.Peek(name))
}

func firstNonBlank(values ...string) string {
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func tenantIdentityFromRequest(ctx *fasthttp.RequestCtx) (tenantctx.Identity, bool) {
	value := ctx.UserValue(tenantctx.FastHTTPUserValueKey)
	identity, ok := value.(tenantctx.Identity)
	return identity, ok
}

func (h *NotificationHandler) tenantIDFromRequest(ctx *fasthttp.RequestCtx) string {
	if tenantID := h.tenantIDForAdminRead(ctx); tenantID != "" {
		return tenantID
	}
	return headerOrQueryTenantID(ctx)
}

func headerOrQueryTenantID(ctx *fasthttp.RequestCtx) string {
	if value := strings.TrimSpace(string(ctx.Request.Header.Peek(tenantIDHeader))); value != "" {
		return value
	}
	return strings.TrimSpace(firstQuery(ctx, "tenant_id", "tenantId"))
}

func firstQuery(ctx *fasthttp.RequestCtx, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(string(ctx.QueryArgs().Peek(key))); value != "" {
			return value
		}
	}
	return ""
}

func (h *NotificationHandler) handleNotification(ctx *fasthttp.RequestCtx, notificationType string) {
	log.Printf("Processing notification request: method=%s path=%s type=%s", ctx.Method(), ctx.Path(), notificationType)

	if notificationType == "DEFAULT" {
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetBody([]byte(`{"message": "NotificationRequest processed successfully", "code": "SUCCESS", "inError": "false"}`))
		return
	}

	// Retrieve the partnerRole from ctx.UserValue
	partnerRoleValue := ctx.UserValue("partnerRole")
	partnerRoleStr, ok := partnerRoleValue.(string)
	if !ok || partnerRoleStr == "" {
		ctx.Error(`{"message": "Invalid or missing partnerRole", "code": "FAILURE", "inError": "true"}`, fasthttp.StatusBadRequest)
		return
	}

	// Convert partnerRole to an integer
	partnerRole, err := strconv.Atoi(partnerRoleStr)
	if err != nil {
		log.Printf("Error converting partnerRole: %v", err)
		ctx.Error(`{"message": "Invalid partnerRole", "code": "FAILURE", "inError": "true"}`, fasthttp.StatusBadRequest)
		return
	}

	res := h.resolveNotificationTenant(ctx)
	if res.Invalid {
		human := humanTenantReason(res.Reason)
		body := fmt.Sprintf(`{"message":%s,"code":%s,"inError":"true"}`,
			jsonString(human), jsonString(res.Reason))
		status := fasthttp.StatusBadRequest
		if res.Reason == "TENANT_KEY_CONFLICT" {
			status = fasthttp.StatusConflict
		}
		ctx.Error(body, status)
		return
	}

	var notification domain.NotificationRequest
	if err := json.Unmarshal(ctx.PostBody(), &notification); err != nil {
		ctx.Error("Invalid request payload", fasthttp.StatusBadRequest)
		return
	}

	notification.PartnerRole = partnerRole
	notification.Type = notificationType
	if res.TenantID != "" {
		notification.TenantID = &res.TenantID
	}
	if res.ChannelID != "" {
		notification.ChannelID = &res.ChannelID
	}
	if err := h.service.ProcessNotification(&notification); err != nil {
		log.Printf("Error processing notification: %v", err)
		ctx.Error(`{"message": "Error processing notification", "code": "FAILURE", "inError": "true"}`, fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetBody([]byte(`{"message": "NotificationRequest processed successfully", "code": "SUCCESS", "inError": "false"}`))
}

func channelIDFromRequest(ctx *fasthttp.RequestCtx) string {
	if value := strings.TrimSpace(string(ctx.Request.Header.Peek("X-Tenant-Channel-Id"))); value != "" {
		return value
	}
	if value := strings.TrimSpace(string(ctx.Request.Header.Peek("X-Channel-Id"))); value != "" {
		return value
	}
	return strings.TrimSpace(firstQuery(ctx, "channel_id", "channelId"))
}

// humanTenantReason returns a human-readable message for a resolution reason code.
func humanTenantReason(code string) string {
	switch code {
	case "UNKNOWN_TENANT":
		return "tenant key does not resolve to a known tenant"
	case "CHANNEL_REQUIRED":
		return "channel_key is required when tenant_key is supplied"
	case "UNKNOWN_CHANNEL":
		return "channel_key does not resolve to a known channel for this tenant"
	case "TENANT_KEY_CONFLICT":
		return "tenant key conflict: header and query parameter disagree"
	default:
		return "invalid tenant context"
	}
}

// jsonString returns a JSON-encoded string literal (double-quoted, with
// special characters escaped). It is a minimal replacement for json.Marshal
// on a string to avoid importing additional packages.
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// Endpoint handlers

func (h *NotificationHandler) DefaultHandler(ctx *fasthttp.RequestCtx) {
	h.handleNotification(ctx, "DEFAULT")
}

func (h *NotificationHandler) MOHandler(ctx *fasthttp.RequestCtx) {
	h.handleNotification(ctx, "MO")
}

func (h *NotificationHandler) MTDNHandler(ctx *fasthttp.RequestCtx) {
	h.handleNotification(ctx, "MT_DN")
}

func (h *NotificationHandler) UserOptinHandler(ctx *fasthttp.RequestCtx) {
	h.handleNotification(ctx, "USER_OPTIN")
}

func (h *NotificationHandler) UserRenewedHandler(ctx *fasthttp.RequestCtx) {
	h.handleNotification(ctx, "USER_RENEWED")
}

func (h *NotificationHandler) UserOptoutHandler(ctx *fasthttp.RequestCtx) {
	h.handleNotification(ctx, "USER_OPTOUT")
}

func (h *NotificationHandler) ChargeHandler(ctx *fasthttp.RequestCtx) {
	h.handleNotification(ctx, "CHARGE")
}
