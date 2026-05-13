package handler

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/service"
	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/valyala/fasthttp"
	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"
)

type AdminManagementHandler struct {
	service *service.AdminManagementService
	logger  *zap.Logger
}

func NewAdminManagementHandler(service *service.AdminManagementService, logger *zap.Logger) *AdminManagementHandler {
	return &AdminManagementHandler{service: service, logger: logger}
}

type listProductsResponse struct {
	Products   []*domain.AdminProduct `json:"products"`
	TotalCount int                    `json:"total_count"`
	Page       int                    `json:"page"`
	PageSize   int                    `json:"page_size"`
}

type productPayload struct {
	ProductID       string  `json:"product_id"`
	Name            string  `json:"name"`
	PricePointID    int     `json:"price_point_id"`
	PricePointValue float64 `json:"price_point_value"`
	ShortCode       string  `json:"short_code"`
	PerformedBy     string  `json:"performed_by,omitempty"`
}

type batchProductPayload struct {
	Products    []productPayload `json:"products"`
	PerformedBy string           `json:"performed_by,omitempty"`
}

type listChannelsResponse struct {
	Channels   []*domain.AdminChannel `json:"channels"`
	TotalCount int                    `json:"total_count"`
	Page       int                    `json:"page"`
	PageSize   int                    `json:"page_size"`
}

type channelPayload struct {
	ChannelKey   string   `json:"channel_key,omitempty"`
	Provider     string   `json:"provider"`
	Country      string   `json:"country"`
	Operator     *string  `json:"operator,omitempty"`
	Capabilities []string `json:"capabilities"`
	Enabled      *bool    `json:"enabled,omitempty"`
	PerformedBy  string   `json:"performed_by,omitempty"`
}

type channelEnabledPayload struct {
	Enabled     *bool  `json:"enabled"`
	PerformedBy string `json:"performed_by,omitempty"`
}

type listChannelCredentialsResponse struct {
	Credentials []*domain.AdminChannelCredential `json:"credentials"`
	TotalCount  int                              `json:"total_count"`
	Page        int                              `json:"page"`
	PageSize    int                              `json:"page_size"`
}

type channelCredentialPayload struct {
	Purpose          string `json:"purpose,omitempty"`
	SecretRef        string `json:"secret_ref,omitempty"`
	SecretValue      string `json:"secret_value,omitempty"`
	SecretRefDisplay string `json:"redacted_display,omitempty"`
	PerformedBy      string `json:"performed_by,omitempty"`
}

type tenantCreatePayload struct {
	TenantKey      string          `json:"tenant_key"`
	Name           string          `json:"name"`
	Status         string          `json:"status"`
	DefaultCountry string          `json:"default_country"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	PerformedBy    string          `json:"performed_by,omitempty"`
}

type tenantCreateResponse struct {
	*domain.AdminTenant
	AuditLogID string `json:"audit_log_id"`
}

type tenantUpdatePayload struct {
	Name           *string          `json:"name,omitempty"`
	Status         *string          `json:"status,omitempty"`
	DefaultCountry *string          `json:"default_country,omitempty"`
	Metadata       *json.RawMessage `json:"metadata,omitempty"`
	PerformedBy    string           `json:"performed_by,omitempty"`
}

type tenantListResponse struct {
	Tenants    []*domain.AdminTenant `json:"tenants"`
	TotalCount int                   `json:"total_count"`
	Page       int                   `json:"page"`
	PageSize   int                   `json:"page_size"`
}

type tenantUpdateResponse struct {
	*domain.AdminTenant
	AuditLogID string `json:"audit_log_id"`
}

type tenantMemberPayload struct {
	Auth0Subject string  `json:"auth0_subject"`
	Email        *string `json:"email,omitempty"`
	Role         string  `json:"role,omitempty"`
	Status       string  `json:"status,omitempty"`
	PerformedBy  string  `json:"performed_by,omitempty"`
}

type tenantMemberListResponse struct {
	Members    []*domain.AdminTenantMember `json:"members"`
	TotalCount int                         `json:"total_count"`
	Page       int                         `json:"page"`
	PageSize   int                         `json:"page_size"`
}

type tenantMemberMutationResponse struct {
	*domain.AdminTenantMember
	AuditLogID string `json:"audit_log_id"`
}

type tenantMemberDeleteResponse struct {
	AuditLogID string `json:"audit_log_id"`
}

func (h *AdminManagementHandler) CreateTenant(ctx *fasthttp.RequestCtx) {
	identity, ok := tenantIdentityFromRequest(ctx)
	if !ok {
		ctx.Error("Tenant context required", fasthttp.StatusForbidden)
		return
	}

	var req tenantCreatePayload
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid request body", fasthttp.StatusBadRequest)
		return
	}

	input := &domain.TenantCreateInput{
		TenantKey:      req.TenantKey,
		Name:           req.Name,
		Status:         domain.TenantStatus(req.Status),
		DefaultCountry: req.DefaultCountry,
		Metadata:       req.Metadata,
	}
	actor := actorFromPayloadIdentityOrRequest(req.PerformedBy, identity, ctx)
	requestID := requestIDFromHeader(ctx)
	tenant, auditLogID, err := h.service.CreateTenant(input, identity, actor, requestID)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}

	writeJSON(ctx, fasthttp.StatusCreated, tenantCreateResponse{
		AdminTenant: tenant,
		AuditLogID:  auditLogID,
	})
}

func (h *AdminManagementHandler) ListTenants(ctx *fasthttp.RequestCtx) {
	identity, ok := tenantIdentityFromRequest(ctx)
	if !ok {
		ctx.Error("Tenant context required", fasthttp.StatusForbidden)
		return
	}
	page, pageSize := parsePageArgs(ctx, 20, 200)
	tenants, total, err := h.service.ListTenants(identity, &domain.TenantListFilter{
		Limit:  pageSize,
		Offset: (page - 1) * pageSize,
		Status: domain.TenantStatus(strings.TrimSpace(string(ctx.QueryArgs().Peek("status")))),
		Query:  strings.TrimSpace(string(ctx.QueryArgs().Peek("q"))),
	})
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, tenantListResponse{
		Tenants:    tenants,
		TotalCount: total,
		Page:       page,
		PageSize:   pageSize,
	})
}

func (h *AdminManagementHandler) UpdateTenant(ctx *fasthttp.RequestCtx) {
	identity, ok := tenantIdentityFromRequest(ctx)
	if !ok {
		ctx.Error("Tenant context required", fasthttp.StatusForbidden)
		return
	}
	tenantID, err := parseTenantIDFromPath(string(ctx.Path()))
	if err != nil {
		ctx.Error("Invalid tenant id", fasthttp.StatusBadRequest)
		return
	}
	var req tenantUpdatePayload
	decoder := json.NewDecoder(bytes.NewReader(ctx.PostBody()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		ctx.Error("Invalid request body", fasthttp.StatusBadRequest)
		return
	}

	var status *domain.TenantStatus
	if req.Status != nil {
		value := domain.TenantStatus(*req.Status)
		status = &value
	}
	input := &domain.TenantUpdateInput{
		Name:           req.Name,
		Status:         status,
		DefaultCountry: req.DefaultCountry,
		Metadata:       req.Metadata,
	}
	actor := actorFromPayloadIdentityOrRequest(req.PerformedBy, identity, ctx)
	requestID := requestIDFromHeader(ctx)
	tenant, auditLogID, err := h.service.UpdateTenant(tenantID, input, identity, actor, requestID)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, tenantUpdateResponse{
		AdminTenant: tenant,
		AuditLogID:  auditLogID,
	})
}

func (h *AdminManagementHandler) GetCurrentTenant(ctx *fasthttp.RequestCtx) {
	identity, ok := tenantIdentityFromRequest(ctx)
	if !ok {
		ctx.Error("Tenant context required", fasthttp.StatusForbidden)
		return
	}

	tenant, err := h.service.ResolveCurrentTenant(identity)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, tenant)
}

func (h *AdminManagementHandler) GetTenantWorkspaces(ctx *fasthttp.RequestCtx) {
	identity, ok := tenantIdentityFromRequest(ctx)
	if !ok {
		ctx.Error("Tenant context required", fasthttp.StatusForbidden)
		return
	}
	workspaces, err := h.service.ListAuthorizedTenantWorkspaces(identity)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, workspaces)
}

func (h *AdminManagementHandler) ListTenantMembers(ctx *fasthttp.RequestCtx) {
	identity, ok := tenantIdentityFromRequest(ctx)
	if !ok {
		ctx.Error("Tenant context required", fasthttp.StatusForbidden)
		return
	}
	tenantID, _, err := parseTenantMemberPath(string(ctx.Path()))
	if err != nil {
		ctx.Error("Invalid tenant member path", fasthttp.StatusBadRequest)
		return
	}
	page, pageSize := parsePageArgs(ctx, 20, 200)
	members, total, err := h.service.ListTenantMembers(tenantID, identity, &domain.TenantMemberListFilter{
		Limit:  pageSize,
		Offset: (page - 1) * pageSize,
		Status: domain.TenantMemberStatus(strings.TrimSpace(string(ctx.QueryArgs().Peek("status")))),
		Query:  strings.TrimSpace(string(ctx.QueryArgs().Peek("q"))),
	})
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, tenantMemberListResponse{
		Members:    members,
		TotalCount: total,
		Page:       page,
		PageSize:   pageSize,
	})
}

func (h *AdminManagementHandler) UpsertTenantMember(ctx *fasthttp.RequestCtx) {
	identity, ok := tenantIdentityFromRequest(ctx)
	if !ok {
		ctx.Error("Tenant context required", fasthttp.StatusForbidden)
		return
	}
	tenantID, _, err := parseTenantMemberPath(string(ctx.Path()))
	if err != nil {
		ctx.Error("Invalid tenant member path", fasthttp.StatusBadRequest)
		return
	}
	var req tenantMemberPayload
	decoder := json.NewDecoder(bytes.NewReader(ctx.PostBody()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		ctx.Error("Invalid request body", fasthttp.StatusBadRequest)
		return
	}
	input := &domain.TenantMemberInput{
		Auth0Subject: req.Auth0Subject,
		Email:        req.Email,
		Role:         domain.TenantMemberRole(req.Role),
		Status:       domain.TenantMemberStatus(req.Status),
	}
	actor := actorFromPayloadIdentityOrRequest(req.PerformedBy, identity, ctx)
	input.CreatedBy = actor
	requestID := requestIDFromHeader(ctx)
	member, auditLogID, err := h.service.UpsertTenantMember(tenantID, input, identity, actor, requestID)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, tenantMemberMutationResponse{
		AdminTenantMember: member,
		AuditLogID:        auditLogID,
	})
}

func (h *AdminManagementHandler) DeactivateTenantMember(ctx *fasthttp.RequestCtx) {
	identity, ok := tenantIdentityFromRequest(ctx)
	if !ok {
		ctx.Error("Tenant context required", fasthttp.StatusForbidden)
		return
	}
	tenantID, auth0Subject, err := parseTenantMemberPath(string(ctx.Path()))
	if err != nil || strings.TrimSpace(auth0Subject) == "" {
		ctx.Error("Invalid tenant member path", fasthttp.StatusBadRequest)
		return
	}
	actor := actorFromPayloadIdentityOrRequest("", identity, ctx)
	requestID := requestIDFromHeader(ctx)
	auditLogID, err := h.service.DeactivateTenantMember(tenantID, auth0Subject, identity, actor, requestID)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, tenantMemberDeleteResponse{AuditLogID: auditLogID})
}

func (h *AdminManagementHandler) ListProducts(ctx *fasthttp.RequestCtx) {
	tenant, _, err := h.currentTenantFromRequest(ctx)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	page, pageSize := parsePageArgs(ctx, 20, 200)
	filter := &domain.ProductListFilter{
		TenantID:  tenant.ID,
		Limit:     pageSize,
		Offset:    (page - 1) * pageSize,
		Query:     strings.TrimSpace(string(ctx.QueryArgs().Peek("q"))),
		ShortCode: strings.TrimSpace(string(ctx.QueryArgs().Peek("short_code"))),
	}

	products, total, err := h.service.ListProducts(filter)
	if err != nil {
		h.logger.Error("Failed to list products", zap.Error(err))
		ctx.Error("Failed to list products", fasthttp.StatusInternalServerError)
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, listProductsResponse{
		Products:   products,
		TotalCount: total,
		Page:       page,
		PageSize:   pageSize,
	})
}

func (h *AdminManagementHandler) CreateProduct(ctx *fasthttp.RequestCtx) {
	tenant, identity, err := h.currentTenantFromRequest(ctx)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	var req productPayload
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid request body", fasthttp.StatusBadRequest)
		return
	}

	product := payloadToProduct(&req)
	actor := actorFromPayloadIdentityOrRequest(req.PerformedBy, identity, ctx)
	requestID := requestIDFromHeader(ctx)
	created, err := h.service.CreateProduct(tenant.ID, product, actor, requestID)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}

	writeJSON(ctx, fasthttp.StatusCreated, created)
}

func (h *AdminManagementHandler) UpdateProduct(ctx *fasthttp.RequestCtx) {
	tenant, identity, err := h.currentTenantFromRequest(ctx)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	id, err := parseProductIDFromPath(string(ctx.Path()))
	if err != nil {
		ctx.Error("Invalid product id", fasthttp.StatusBadRequest)
		return
	}

	var req productPayload
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid request body", fasthttp.StatusBadRequest)
		return
	}

	product := payloadToProduct(&req)
	actor := actorFromPayloadIdentityOrRequest(req.PerformedBy, identity, ctx)
	requestID := requestIDFromHeader(ctx)
	updated, err := h.service.UpdateProduct(tenant.ID, id, product, actor, requestID)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, updated)
}

func (h *AdminManagementHandler) DeleteProduct(ctx *fasthttp.RequestCtx) {
	tenant, identity, err := h.currentTenantFromRequest(ctx)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	id, err := parseProductIDFromPath(string(ctx.Path()))
	if err != nil {
		ctx.Error("Invalid product id", fasthttp.StatusBadRequest)
		return
	}

	actor := actorFromPayloadIdentityOrRequest("", identity, ctx)
	requestID := requestIDFromHeader(ctx)
	if err := h.service.DeleteProduct(tenant.ID, id, actor, requestID); err != nil {
		var depErr *service.ProductDependencyError
		switch {
		case errors.Is(err, service.ErrAdminNotFound):
			ctx.Error("Product not found", fasthttp.StatusNotFound)
		case errors.As(err, &depErr):
			writeJSON(ctx, fasthttp.StatusConflict, map[string]any{
				"error":             "product is referenced and cannot be deleted",
				"dependency_counts": depErr.Counts,
			})
		case errors.Is(err, service.ErrInvalidInput):
			ctx.Error(err.Error(), fasthttp.StatusBadRequest)
		default:
			h.logger.Error("Failed to delete product", zap.Error(err))
			ctx.Error("Failed to delete product", fasthttp.StatusInternalServerError)
		}
		return
	}

	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func (h *AdminManagementHandler) BatchUpsertProducts(ctx *fasthttp.RequestCtx) {
	tenant, identity, err := h.currentTenantFromRequest(ctx)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	var req batchProductPayload
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid request body", fasthttp.StatusBadRequest)
		return
	}

	items := make([]*domain.AdminProduct, 0, len(req.Products))
	for i := range req.Products {
		items = append(items, payloadToProduct(&req.Products[i]))
	}

	actor := actorFromPayloadIdentityOrRequest(req.PerformedBy, identity, ctx)
	requestID := requestIDFromHeader(ctx)
	count, err := h.service.BatchUpsertProducts(tenant.ID, items, actor, requestID)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"message": "Batch upsert completed",
		"count":   count,
	})
}

func (h *AdminManagementHandler) ListChannels(ctx *fasthttp.RequestCtx) {
	tenant, _, err := h.currentTenantFromRequest(ctx)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	page, pageSize := parsePageArgs(ctx, 20, 200)
	filter := &domain.ChannelListFilter{
		TenantID: tenant.ID,
		Limit:    pageSize,
		Offset:   (page - 1) * pageSize,
		Provider: strings.TrimSpace(string(ctx.QueryArgs().Peek("provider"))),
		Country:  strings.TrimSpace(string(ctx.QueryArgs().Peek("country"))),
	}
	if enabledRaw := strings.TrimSpace(string(ctx.QueryArgs().Peek("enabled"))); enabledRaw != "" {
		enabled, parseErr := strconv.ParseBool(enabledRaw)
		if parseErr != nil {
			ctx.Error("enabled must be true or false", fasthttp.StatusBadRequest)
			return
		}
		filter.Enabled = &enabled
	}

	channels, total, err := h.service.ListChannels(filter)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, listChannelsResponse{
		Channels:   channels,
		TotalCount: total,
		Page:       page,
		PageSize:   pageSize,
	})
}

func (h *AdminManagementHandler) CreateChannel(ctx *fasthttp.RequestCtx) {
	tenant, identity, err := h.currentTenantFromRequest(ctx)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	var req channelPayload
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid request body", fasthttp.StatusBadRequest)
		return
	}

	actor := actorFromPayloadIdentityOrRequest(req.PerformedBy, identity, ctx)
	requestID := requestIDFromHeader(ctx)
	created, err := h.service.CreateChannel(tenant.ID, &domain.ChannelCreateInput{
		ChannelKey:   req.ChannelKey,
		Provider:     req.Provider,
		Country:      req.Country,
		Operator:     req.Operator,
		Capabilities: req.Capabilities,
		Enabled:      req.Enabled,
	}, actor, requestID)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	writeJSON(ctx, fasthttp.StatusCreated, created)
}

func (h *AdminManagementHandler) SetChannelEnabled(ctx *fasthttp.RequestCtx) {
	tenant, identity, err := h.currentTenantFromRequest(ctx)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	channelID, err := parseChannelIDFromEnabledPath(string(ctx.Path()))
	if err != nil {
		ctx.Error("Invalid channel id", fasthttp.StatusBadRequest)
		return
	}
	var req channelEnabledPayload
	decoder := json.NewDecoder(bytes.NewReader(ctx.PostBody()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		ctx.Error("Invalid request body", fasthttp.StatusBadRequest)
		return
	}
	if req.Enabled == nil {
		ctx.Error("enabled is required", fasthttp.StatusBadRequest)
		return
	}

	actor := actorFromPayloadIdentityOrRequest(req.PerformedBy, identity, ctx)
	requestID := requestIDFromHeader(ctx)
	channel, err := h.service.SetChannelEnabled(tenant.ID, channelID, *req.Enabled, actor, requestID)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, channel)
}

func (h *AdminManagementHandler) BindChannelCredential(ctx *fasthttp.RequestCtx) {
	tenant, identity, err := h.currentTenantFromRequest(ctx)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	channelID, err := parseChannelIDFromCredentialsPath(string(ctx.Path()))
	if err != nil {
		ctx.Error("Invalid channel id", fasthttp.StatusBadRequest)
		return
	}
	var req channelCredentialPayload
	decoder := json.NewDecoder(bytes.NewReader(ctx.PostBody()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		ctx.Error("Invalid request body", fasthttp.StatusBadRequest)
		return
	}

	actor := actorFromPayloadIdentityOrRequest(req.PerformedBy, identity, ctx)
	requestID := requestIDFromHeader(ctx)
	credential, err := h.service.BindChannelCredential(context.Background(), tenant.ID, channelID, &domain.ChannelCredentialBindInput{
		ChannelID:        channelID,
		Purpose:          req.Purpose,
		SecretRef:        req.SecretRef,
		SecretValue:      req.SecretValue,
		SecretRefDisplay: req.SecretRefDisplay,
	}, actor, requestID)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	writeJSON(ctx, fasthttp.StatusCreated, credential)
}

func (h *AdminManagementHandler) ListChannelCredentials(ctx *fasthttp.RequestCtx) {
	tenant, _, err := h.currentTenantFromRequest(ctx)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	channelID, err := parseChannelIDFromCredentialsPath(string(ctx.Path()))
	if err != nil {
		ctx.Error("Invalid channel id", fasthttp.StatusBadRequest)
		return
	}
	page, pageSize := parsePageArgs(ctx, 20, 200)
	credentials, total, err := h.service.ListChannelCredentials(&domain.ChannelCredentialListFilter{
		TenantID:  tenant.ID,
		ChannelID: channelID,
		Purpose:   strings.TrimSpace(string(ctx.QueryArgs().Peek("purpose"))),
		Limit:     pageSize,
		Offset:    (page - 1) * pageSize,
	})
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, listChannelCredentialsResponse{
		Credentials: credentials,
		TotalCount:  total,
		Page:        page,
		PageSize:    pageSize,
	})
}

type listUserbaseResponse struct {
	Records    []*domain.UserbaseRecord `json:"records"`
	TotalCount int                      `json:"total_count"`
	Page       int                      `json:"page"`
	PageSize   int                      `json:"page_size"`
}

type upsertUserbaseRequest struct {
	MSISDN      string `json:"msisdn"`
	Type        string `json:"type"`
	PerformedBy string `json:"performed_by,omitempty"`
}

func (h *AdminManagementHandler) ListUserbase(ctx *fasthttp.RequestCtx) {
	tenant, _, err := h.currentTenantFromRequest(ctx)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	page, pageSize := parsePageArgs(ctx, 20, 200)
	filter := &domain.UserbaseListFilter{
		TenantID: tenant.ID,
		Limit:    pageSize,
		Offset:   (page - 1) * pageSize,
		MSISDN:   strings.TrimSpace(string(ctx.QueryArgs().Peek("msisdn"))),
		Type:     strings.TrimSpace(string(ctx.QueryArgs().Peek("type"))),
	}

	records, total, err := h.service.ListUserbase(filter)
	if err != nil {
		h.logger.Error("Failed to list userbase", zap.Error(err))
		ctx.Error("Failed to list userbase", fasthttp.StatusInternalServerError)
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, listUserbaseResponse{
		Records:    records,
		TotalCount: total,
		Page:       page,
		PageSize:   pageSize,
	})
}

func (h *AdminManagementHandler) UpsertUserbase(ctx *fasthttp.RequestCtx) {
	tenant, identity, err := h.currentTenantFromRequest(ctx)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	var req upsertUserbaseRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid request body", fasthttp.StatusBadRequest)
		return
	}

	actor := actorFromPayloadIdentityOrRequest(req.PerformedBy, identity, ctx)
	requestID := requestIDFromHeader(ctx)
	record, err := h.service.UpsertUserbase(tenant.ID, req.MSISDN, req.Type, actor, requestID)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, record)
}

func (h *AdminManagementHandler) DeleteUserbase(ctx *fasthttp.RequestCtx) {
	tenant, identity, err := h.currentTenantFromRequest(ctx)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	msisdn, err := parseMSISDNFromPath(string(ctx.Path()))
	if err != nil {
		ctx.Error("Invalid msisdn", fasthttp.StatusBadRequest)
		return
	}

	actor := actorFromPayloadIdentityOrRequest("", identity, ctx)
	requestID := requestIDFromHeader(ctx)
	if err := h.service.DeleteUserbase(tenant.ID, msisdn, actor, requestID); err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func (h *AdminManagementHandler) ImportUserbase(ctx *fasthttp.RequestCtx) {
	tenant, identity, err := h.currentTenantFromRequest(ctx)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		ctx.Error("file is required", fasthttp.StatusBadRequest)
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		ctx.Error("failed to open uploaded file", fasthttp.StatusInternalServerError)
		return
	}
	defer func(file multipart.File) { _ = file.Close() }(file)

	rows, err := parseImportRows(fileHeader.Filename, file)
	if err != nil {
		ctx.Error(fmt.Sprintf("failed to parse import file: %v", err), fasthttp.StatusBadRequest)
		return
	}

	actor := actorFromPayloadIdentityOrRequest("", identity, ctx)
	requestID := requestIDFromHeader(ctx)
	job, importErrors, err := h.service.ImportUserbase(tenant.ID, fileHeader.Filename, rows, actor, requestID)
	if err != nil {
		h.logger.Error("Failed to import userbase", zap.Error(err))
		h.handleServiceError(ctx, err)
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"job":    job,
		"errors": importErrors,
	})
}

func (h *AdminManagementHandler) ListUserbaseImports(ctx *fasthttp.RequestCtx) {
	tenant, _, err := h.currentTenantFromRequest(ctx)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	page, pageSize := parsePageArgs(ctx, 20, 200)
	jobs, total, err := h.service.ListUserbaseImportJobs(tenant.ID, page, pageSize)
	if err != nil {
		h.logger.Error("Failed to list userbase imports", zap.Error(err))
		ctx.Error("Failed to list userbase imports", fasthttp.StatusInternalServerError)
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"jobs":        jobs,
		"total_count": total,
		"page":        page,
		"page_size":   pageSize,
	})
}

func (h *AdminManagementHandler) GetUserbaseImport(ctx *fasthttp.RequestCtx) {
	tenant, _, err := h.currentTenantFromRequest(ctx)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	jobID, err := parseImportIDFromPath(string(ctx.Path()))
	if err != nil {
		ctx.Error("Invalid import job id", fasthttp.StatusBadRequest)
		return
	}

	job, errorsOut, totalErrors, err := h.service.GetUserbaseImportJob(tenant.ID, jobID)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"job":          job,
		"errors":       errorsOut,
		"total_errors": totalErrors,
	})
}

func (h *AdminManagementHandler) ListActivityLogs(ctx *fasthttp.RequestCtx) {
	tenant, _, err := h.currentTenantFromRequest(ctx)
	if err != nil {
		h.handleServiceError(ctx, err)
		return
	}
	page, pageSize := parsePageArgs(ctx, 20, 200)
	filter := &domain.AdminActivityLogFilter{
		TenantID:   tenant.ID,
		Limit:      pageSize,
		Offset:     (page - 1) * pageSize,
		EntityType: strings.TrimSpace(string(ctx.QueryArgs().Peek("entity_type"))),
		Action:     strings.TrimSpace(string(ctx.QueryArgs().Peek("action"))),
		Actor:      strings.TrimSpace(string(ctx.QueryArgs().Peek("actor"))),
	}

	if fromRaw := strings.TrimSpace(string(ctx.QueryArgs().Peek("from"))); fromRaw != "" {
		if t, err := time.Parse("2006-01-02", fromRaw); err == nil {
			filter.From = &t
		}
	}
	if toRaw := strings.TrimSpace(string(ctx.QueryArgs().Peek("to"))); toRaw != "" {
		if t, err := time.Parse("2006-01-02", toRaw); err == nil {
			t = t.Add(24*time.Hour - time.Second)
			filter.To = &t
		}
	}

	items, total, err := h.service.ListActivityLogs(filter)
	if err != nil {
		h.logger.Error("Failed to list activity logs", zap.Error(err))
		ctx.Error("Failed to list activity logs", fasthttp.StatusInternalServerError)
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"items":       items,
		"total_count": total,
		"page":        page,
		"page_size":   pageSize,
	})
}

func (h *AdminManagementHandler) currentTenantFromRequest(ctx *fasthttp.RequestCtx) (*domain.AdminTenant, tenantctx.Identity, error) {
	identity, ok := tenantIdentityFromRequest(ctx)
	if !ok {
		return nil, tenantctx.Identity{}, service.ErrTenantContextMissing
	}
	tenant, err := h.service.ResolveCurrentTenant(identity)
	if err != nil {
		return nil, identity, err
	}
	return tenant, identity, nil
}

func (h *AdminManagementHandler) handleServiceError(ctx *fasthttp.RequestCtx, err error) {
	var depErr *service.ProductDependencyError
	switch {
	case errors.Is(err, service.ErrAdminForbidden):
		ctx.Error("Forbidden", fasthttp.StatusForbidden)
	case errors.Is(err, service.ErrAdminConflict):
		ctx.Error("Resource already exists", fasthttp.StatusConflict)
	case errors.Is(err, service.ErrAdminInvalidState):
		ctx.Error(err.Error(), fasthttp.StatusConflict)
	case errors.Is(err, service.ErrAdminDependencyUnavailable):
		ctx.Error("Dependency unavailable", fasthttp.StatusServiceUnavailable)
	case errors.Is(err, service.ErrTenantContextMissing):
		ctx.Error("Tenant context required", fasthttp.StatusForbidden)
	case errors.Is(err, service.ErrTenantUnavailable):
		ctx.Error("Tenant not available", fasthttp.StatusForbidden)
	case errors.Is(err, service.ErrAdminNotFound):
		ctx.Error("Resource not found", fasthttp.StatusNotFound)
	case errors.Is(err, service.ErrInvalidInput):
		ctx.Error(err.Error(), fasthttp.StatusBadRequest)
	case errors.As(err, &depErr):
		writeJSON(ctx, fasthttp.StatusConflict, map[string]any{
			"error":             depErr.Error(),
			"dependency_counts": depErr.Counts,
		})
	default:
		h.logger.Error("Unhandled admin management error", zap.Error(err))
		ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
	}
}

func payloadToProduct(req *productPayload) *domain.AdminProduct {
	return &domain.AdminProduct{
		ProductID:       req.ProductID,
		Name:            req.Name,
		PricePointID:    req.PricePointID,
		PricePointValue: req.PricePointValue,
		ShortCode:       req.ShortCode,
	}
}

func parsePageArgs(ctx *fasthttp.RequestCtx, defaultSize, maxSize int) (int, int) {
	page := 1
	pageSize := defaultSize
	if p := ctx.QueryArgs().GetUintOrZero("page"); p > 0 {
		page = int(p)
	}
	if ps := ctx.QueryArgs().GetUintOrZero("page_size"); ps > 0 {
		pageSize = int(ps)
	}
	if pageSize > maxSize {
		pageSize = maxSize
	}
	return page, pageSize
}

func parseProductIDFromPath(path string) (int, error) {
	parts := splitPathParts(path)
	if len(parts) < 4 {
		return 0, errors.New("invalid path")
	}
	return strconv.Atoi(parts[len(parts)-1])
}

func parseTenantIDFromPath(path string) (string, error) {
	parts := splitPathParts(path)
	if len(parts) < 4 {
		return "", errors.New("invalid path")
	}
	id := strings.TrimSpace(parts[len(parts)-1])
	if id == "" || id == "current" {
		return "", errors.New("missing tenant id")
	}
	return id, nil
}

func parseTenantMemberPath(path string) (string, string, error) {
	parts := splitPathParts(path)
	if len(parts) < 5 || parts[0] != "v1" || parts[1] != "admin" || parts[2] != "tenants" || parts[4] != "members" {
		return "", "", errors.New("invalid path")
	}
	tenantID := strings.TrimSpace(parts[3])
	if tenantID == "" || tenantID == "current" || tenantID == "workspaces" {
		return "", "", errors.New("missing tenant id")
	}
	if len(parts) == 5 {
		return tenantID, "", nil
	}
	rawSubject := strings.Join(parts[5:], "/")
	subject, err := url.PathUnescape(rawSubject)
	if err != nil {
		return "", "", err
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "", "", errors.New("missing auth0 subject")
	}
	return tenantID, subject, nil
}

func parseMSISDNFromPath(path string) (string, error) {
	parts := splitPathParts(path)
	if len(parts) < 4 {
		return "", errors.New("invalid path")
	}
	msisdn := strings.TrimSpace(parts[len(parts)-1])
	if msisdn == "" {
		return "", errors.New("missing msisdn")
	}
	return msisdn, nil
}

func parseImportIDFromPath(path string) (string, error) {
	parts := splitPathParts(path)
	if len(parts) < 5 {
		return "", errors.New("invalid path")
	}
	id := strings.TrimSpace(parts[len(parts)-1])
	if id == "" {
		return "", errors.New("missing import id")
	}
	return id, nil
}

func parseChannelIDFromEnabledPath(path string) (string, error) {
	parts := splitPathParts(path)
	if len(parts) < 5 || parts[len(parts)-1] != "enabled" {
		return "", errors.New("invalid path")
	}
	id := strings.TrimSpace(parts[len(parts)-2])
	if id == "" {
		return "", errors.New("missing channel id")
	}
	return id, nil
}

func parseChannelIDFromCredentialsPath(path string) (string, error) {
	parts := splitPathParts(path)
	if len(parts) < 5 || parts[len(parts)-1] != "credentials" {
		return "", errors.New("invalid path")
	}
	id := strings.TrimSpace(parts[len(parts)-2])
	if id == "" {
		return "", errors.New("missing channel id")
	}
	return id, nil
}

func actorFromPayloadOrRequest(performedBy string, ctx *fasthttp.RequestCtx) *string {
	performedBy = strings.TrimSpace(performedBy)
	if performedBy != "" {
		return &performedBy
	}
	return actorFromHeader(ctx)
}

func actorFromPayloadIdentityOrRequest(performedBy string, identity tenantctx.Identity, ctx *fasthttp.RequestCtx) *string {
	performedBy = strings.TrimSpace(performedBy)
	if performedBy != "" {
		return &performedBy
	}
	if subject := strings.TrimSpace(identity.Subject); subject != "" {
		return &subject
	}
	return actorFromHeader(ctx)
}

func actorFromHeader(ctx *fasthttp.RequestCtx) *string {
	if actor := strings.TrimSpace(string(ctx.Request.Header.Peek("X-Admin-User"))); actor != "" {
		return &actor
	}
	return nil
}

func tenantIdentityFromRequest(ctx *fasthttp.RequestCtx) (tenantctx.Identity, bool) {
	value := ctx.UserValue(tenantctx.FastHTTPUserValueKey)
	identity, ok := value.(tenantctx.Identity)
	return identity, ok
}

func requestIDFromHeader(ctx *fasthttp.RequestCtx) *string {
	if reqID := strings.TrimSpace(string(ctx.Request.Header.Peek("x-requestid"))); reqID != "" {
		return &reqID
	}
	return nil
}

func parseImportRows(filename string, r io.Reader) ([]domain.UserbaseImportInputRow, error) {
	lower := strings.ToLower(strings.TrimSpace(filename))
	switch {
	case strings.HasSuffix(lower, ".csv"):
		return parseCSVImportRows(r)
	case strings.HasSuffix(lower, ".json"):
		return parseJSONImportRows(r)
	case strings.HasSuffix(lower, ".xlsx"):
		return parseXLSXImportRows(r)
	default:
		return nil, fmt.Errorf("unsupported file format, use .csv, .json, or .xlsx")
	}
}

func parseJSONImportRows(r io.Reader) ([]domain.UserbaseImportInputRow, error) {
	var rows []struct {
		MSISDN   string `json:"msisdn"`
		Type     string `json:"type"`
		TenantID string `json:"tenant_id"`
	}
	if err := json.NewDecoder(r).Decode(&rows); err != nil {
		return nil, err
	}

	out := make([]domain.UserbaseImportInputRow, 0, len(rows))
	for i, row := range rows {
		if strings.TrimSpace(row.TenantID) != "" {
			return nil, fmt.Errorf("row %d must not include tenant_id", i+1)
		}
		rawBytes, _ := json.Marshal(row)
		out = append(out, domain.UserbaseImportInputRow{
			RowNumber: i + 1,
			MSISDN:    strings.TrimSpace(row.MSISDN),
			Type:      strings.TrimSpace(row.Type),
			RawRow:    string(rawBytes),
		})
	}
	return out, nil
}

func parseCSVImportRows(r io.Reader) ([]domain.UserbaseImportInputRow, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1

	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	start := 0
	if len(rows[0]) >= 2 {
		h0 := strings.ToLower(strings.TrimSpace(rows[0][0]))
		h1 := strings.ToLower(strings.TrimSpace(rows[0][1]))
		if h0 == "msisdn" && h1 == "type" {
			start = 1
		}
	}

	out := make([]domain.UserbaseImportInputRow, 0, len(rows)-start)
	for i := start; i < len(rows); i++ {
		row := rows[i]
		if len(row) == 0 {
			continue
		}
		msisdn, userType := "", ""
		if len(row) > 0 {
			msisdn = strings.TrimSpace(row[0])
		}
		if len(row) > 1 {
			userType = strings.TrimSpace(row[1])
		}
		raw := strings.Join(row, ",")
		out = append(out, domain.UserbaseImportInputRow{
			RowNumber: i + 1,
			MSISDN:    msisdn,
			Type:      userType,
			RawRow:    raw,
		})
	}
	return out, nil
}

func parseXLSXImportRows(r io.Reader) ([]domain.UserbaseImportInputRow, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return nil, nil
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	start := 0
	if len(rows[0]) >= 2 {
		h0 := strings.ToLower(strings.TrimSpace(rows[0][0]))
		h1 := strings.ToLower(strings.TrimSpace(rows[0][1]))
		if h0 == "msisdn" && h1 == "type" {
			start = 1
		}
	}

	out := make([]domain.UserbaseImportInputRow, 0, len(rows)-start)
	for i := start; i < len(rows); i++ {
		row := rows[i]
		if len(row) == 0 {
			continue
		}
		msisdn, userType := "", ""
		if len(row) > 0 {
			msisdn = strings.TrimSpace(row[0])
		}
		if len(row) > 1 {
			userType = strings.TrimSpace(row[1])
		}
		raw := strings.Join(row, ",")
		out = append(out, domain.UserbaseImportInputRow{
			RowNumber: i + 1,
			MSISDN:    msisdn,
			Type:      userType,
			RawRow:    raw,
		})
	}
	return out, nil
}
