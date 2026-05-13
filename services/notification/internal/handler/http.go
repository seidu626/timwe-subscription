package handler

import (
	"context"
	"encoding/json"
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
	if identity.PlatformScoped {
		if tenantID := headerOrQueryTenantID(ctx); tenantID != "" {
			return tenantID
		}
		tenantKey := firstNonBlank(
			identity.TenantKey,
			string(ctx.Request.Header.Peek(tenantctx.HeaderTenantKey)),
			firstQuery(ctx, "tenant_key", "tenantKey"),
		)
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
	return ""
}

func tenantIdentityFromRequest(ctx *fasthttp.RequestCtx) (tenantctx.Identity, bool) {
	value := ctx.UserValue(tenantctx.FastHTTPUserValueKey)
	identity, ok := value.(tenantctx.Identity)
	return identity, ok
}

func tenantIDFromRequest(ctx *fasthttp.RequestCtx) string {
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

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
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

	var notification domain.NotificationRequest
	if err := json.Unmarshal(ctx.PostBody(), &notification); err != nil {
		ctx.Error("Invalid request payload", fasthttp.StatusBadRequest)
		return
	}

	notification.PartnerRole = partnerRole
	notification.Type = notificationType
	if tenantID := tenantIDFromRequest(ctx); tenantID != "" {
		notification.TenantID = &tenantID
	}
	if channelID := channelIDFromRequest(ctx); channelID != "" {
		notification.ChannelID = &channelID
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
