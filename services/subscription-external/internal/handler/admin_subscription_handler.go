package handler

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

func parseAdminHistoryDate(raw string, endOfDay bool) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	if len(raw) == len("2006-01-02") {
		if t, err := time.Parse("2006-01-02", raw); err == nil {
			if endOfDay {
				return t.Add(24*time.Hour - time.Nanosecond)
			}
			return t
		}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}
	return time.Time{}
}

func firstQueryArg(queryArgs *fasthttp.Args, names ...string) string {
	for _, name := range names {
		if v := strings.TrimSpace(string(queryArgs.Peek(name))); v != "" {
			return v
		}
	}
	return ""
}

type adminActionAuditRepository interface {
	CreateAdminActionLog(log *domain.AdminSubscriptionActionLog) error
	ListAdminActionLogs(filter domain.AdminActionLogFilter) ([]domain.AdminActionLogSummary, int64, error)
	GetAdminActionLogByID(id string) (*domain.AdminSubscriptionActionLog, error)
}

type adminTenantResolver interface {
	TenantIDByKey(tenantKey string) (string, error)
}

func normalizeAdminOperation(operation string) (domain.AdminActionOperation, bool) {
	switch strings.ToLower(strings.TrimSpace(operation)) {
	case string(domain.AdminActionOptin):
		return domain.AdminActionOptin, true
	case string(domain.AdminActionOptout):
		return domain.AdminActionOptout, true
	case string(domain.AdminActionConfirm):
		return domain.AdminActionConfirm, true
	case string(domain.AdminActionStatus):
		return domain.AdminActionStatus, true
	default:
		return "", false
	}
}

func (h *SubscriptionHandler) getAdminAuditRepository() (adminActionAuditRepository, bool) {
	repo := h.service.GetRepository()
	auditRepo, ok := repo.(adminActionAuditRepository)
	return auditRepo, ok
}

func (h *SubscriptionHandler) handleAdminAction(ctx *fasthttp.RequestCtx, operation domain.AdminActionOperation) {
	var req domain.AdminSubscriptionActionRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid request payload", fasthttp.StatusBadRequest)
		return
	}
	route, err := tenantRouteFromRequest(ctx, h.config, req.ChannelID != "" || req.ChannelKey != "" || req.TenantID != "" || req.TenantKey != "", req.ChannelID, req.ChannelKey)
	if err != nil {
		ctx.Error(err.Error(), tenantRouteStatus(err))
		return
	}
	if route.TenantID != "" || route.TenantKey != "" {
		req.TenantID = route.TenantID
		req.TenantKey = route.TenantKey
		req.ChannelID = route.ChannelID
		req.ChannelKey = route.ChannelKey
	}

	req.MSISDN = strings.TrimSpace(req.MSISDN)
	if req.MSISDN == "" {
		ctx.Error("msisdn is required", fasthttp.StatusBadRequest)
		return
	}
	if req.ProductID <= 0 {
		ctx.Error("productId must be greater than zero", fasthttp.StatusBadRequest)
		return
	}
	if operation == domain.AdminActionConfirm && strings.TrimSpace(req.TransactionAuthCode) == "" {
		ctx.Error("transactionAuthCode is required for confirm", fasthttp.StatusBadRequest)
		return
	}

	if req.AdminRequestID == "" {
		req.AdminRequestID = string(ctx.Request.Header.Peek("x-admin-request-id"))
	}
	if req.AdminRequestID == "" {
		req.AdminRequestID = string(ctx.Request.Header.Peek("x-requestid"))
	}
	if req.AdminRequestID == "" {
		req.AdminRequestID = uuid.NewString()
	}

	if req.ExternalTxID == "" {
		req.ExternalTxID = string(ctx.Request.Header.Peek("external-tx-id"))
	}

	logEntry, execErr := h.service.ExecuteAdminSubscriptionAction(operation, req)
	if logEntry == nil {
		h.logger.Error("Admin subscription action failed before request execution",
			zap.String("operation", string(operation)),
			zap.Error(execErr),
		)
		ctx.Error("Failed to execute admin subscription action", fasthttp.StatusInternalServerError)
		return
	}

	auditRepo, ok := h.getAdminAuditRepository()
	if !ok {
		h.logger.Error("Admin action audit repository not available on subscription repository implementation")
		ctx.Error("Admin action auditing is not configured", fasthttp.StatusInternalServerError)
		return
	}

	if err := auditRepo.CreateAdminActionLog(logEntry); err != nil {
		h.logger.Error("Failed to persist admin subscription action audit log",
			zap.String("actionId", logEntry.ID),
			zap.String("operation", string(operation)),
			zap.Error(err),
		)
		ctx.Error("Failed to persist admin action audit log", fasthttp.StatusInternalServerError)
		return
	}

	statusCode := fasthttp.StatusOK
	if execErr != nil {
		statusCode = fasthttp.StatusBadGateway
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(statusCode)
	if err := json.NewEncoder(ctx).Encode(logEntry.ToDetail()); err != nil {
		h.logger.Error("Failed to encode admin action response", zap.Error(err))
		ctx.Error("Failed to encode admin action response", fasthttp.StatusInternalServerError)
		return
	}
}

func (h *SubscriptionHandler) AdminOptinHandler(ctx *fasthttp.RequestCtx) {
	h.handleAdminAction(ctx, domain.AdminActionOptin)
}

func (h *SubscriptionHandler) AdminOptoutHandler(ctx *fasthttp.RequestCtx) {
	h.handleAdminAction(ctx, domain.AdminActionOptout)
}

func (h *SubscriptionHandler) AdminConfirmHandler(ctx *fasthttp.RequestCtx) {
	h.handleAdminAction(ctx, domain.AdminActionConfirm)
}

func (h *SubscriptionHandler) AdminStatusHandler(ctx *fasthttp.RequestCtx) {
	h.handleAdminAction(ctx, domain.AdminActionStatus)
}

func (h *SubscriptionHandler) AdminActionHistoryHandler(ctx *fasthttp.RequestCtx) {
	queryArgs := ctx.QueryArgs()

	operation := queryArgs.Peek("operation")
	operationValue := domain.AdminActionOperation("")
	if len(operation) > 0 {
		normalized, ok := normalizeAdminOperation(string(operation))
		if !ok {
			ctx.Error("invalid operation filter", fasthttp.StatusBadRequest)
			return
		}
		operationValue = normalized
	}

	page, err := strconv.Atoi(string(queryArgs.Peek("page")))
	if err != nil || page <= 0 {
		page = 1
	}
	pageSize, err := strconv.Atoi(string(queryArgs.Peek("pageSize")))
	if err != nil || pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	auditRepo, ok := h.getAdminAuditRepository()
	if !ok {
		h.logger.Error("Admin action audit repository not available on subscription repository implementation")
		ctx.Error("Admin action auditing is not configured", fasthttp.StatusInternalServerError)
		return
	}

	tenantID, ok := adminActionTenantIDFromRequest(ctx, queryArgs, auditRepo)
	if !ok {
		ctx.Error("tenant context is required", fasthttp.StatusForbidden)
		return
	}

	productID := 0
	if raw := firstQueryArg(queryArgs, "productId", "product_id"); raw != "" {
		if pid, convErr := strconv.Atoi(raw); convErr == nil {
			productID = pid
		}
	}

	result := strings.ToLower(firstQueryArg(queryArgs, "result", "hasError", "has_error"))
	switch result {
	case "ok", "success", "false":
		result = "ok"
	case "error", "failed", "true":
		result = "error"
	default:
		result = ""
	}

	filter := domain.AdminActionLogFilter{
		TenantID:       tenantID,
		Operation:      operationValue,
		MSISDN:         strings.TrimSpace(string(queryArgs.Peek("msisdn"))),
		ExternalTxID:   strings.TrimSpace(string(queryArgs.Peek("externalTxId"))),
		AdminRequestID: strings.TrimSpace(string(queryArgs.Peek("adminRequestId"))),
		ProductID:      productID,
		StartDate:      parseAdminHistoryDate(firstQueryArg(queryArgs, "startDate", "start_date"), false),
		EndDate:        parseAdminHistoryDate(firstQueryArg(queryArgs, "endDate", "end_date"), true),
		Result:         result,
		SortBy:         firstQueryArg(queryArgs, "sort_by", "sortBy"),
		SortDir:        firstQueryArg(queryArgs, "sort_dir", "sortDir"),
		Page:           page,
		PageSize:       pageSize,
	}

	summaries, totalCount, err := auditRepo.ListAdminActionLogs(filter)
	if err != nil {
		h.logger.Error("Failed to list admin action history", zap.Error(err))
		ctx.Error("Failed to load admin action history", fasthttp.StatusInternalServerError)
		return
	}

	totalPages := int(math.Ceil(float64(totalCount) / float64(pageSize)))
	if totalPages == 0 {
		totalPages = 1
	}

	response := domain.AdminActionLogListResponse{
		Data:       summaries,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	if err := json.NewEncoder(ctx).Encode(response); err != nil {
		h.logger.Error("Failed to encode admin action history response", zap.Error(err))
		ctx.Error("Failed to encode admin action history response", fasthttp.StatusInternalServerError)
		return
	}
}

func adminActionTenantIDFromRequest(ctx *fasthttp.RequestCtx, queryArgs *fasthttp.Args, repo adminActionAuditRepository) (string, bool) {
	if queryArgs != nil {
		if tenantID := strings.TrimSpace(string(queryArgs.Peek("tenantId"))); tenantID != "" {
			return tenantID, true
		}
		if tenantID := strings.TrimSpace(string(queryArgs.Peek("tenant_id"))); tenantID != "" {
			return tenantID, true
		}
		if tenantKey := strings.TrimSpace(string(queryArgs.Peek("tenantKey"))); tenantKey != "" {
			return resolveAdminTenantKey(tenantKey, repo)
		}
		if tenantKey := strings.TrimSpace(string(queryArgs.Peek("tenant_key"))); tenantKey != "" {
			return resolveAdminTenantKey(tenantKey, repo)
		}
	}
	if tenantID := firstHeader(ctx, tenantctx.HeaderTenantID, "X-Tenant-ID"); tenantID != "" {
		return tenantID, true
	}
	if tenantKey := firstHeader(ctx, tenantctx.HeaderTenantKey, "X-Tenant-Key"); tenantKey != "" {
		return resolveAdminTenantKey(tenantKey, repo)
	}
	return "", false
}

func resolveAdminTenantKey(tenantKey string, repo adminActionAuditRepository) (string, bool) {
	resolver, ok := repo.(adminTenantResolver)
	if !ok {
		return "", false
	}
	tenantID, err := resolver.TenantIDByKey(tenantKey)
	if err != nil {
		return "", false
	}
	return tenantID, tenantID != ""
}

func (h *SubscriptionHandler) AdminActionDetailHandler(ctx *fasthttp.RequestCtx, id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		ctx.Error("action id is required", fasthttp.StatusBadRequest)
		return
	}

	auditRepo, ok := h.getAdminAuditRepository()
	if !ok {
		h.logger.Error("Admin action audit repository not available on subscription repository implementation")
		ctx.Error("Admin action auditing is not configured", fasthttp.StatusInternalServerError)
		return
	}

	tenantID, ok := adminActionTenantIDFromRequest(ctx, ctx.QueryArgs(), auditRepo)
	if !ok {
		ctx.Error("tenant context is required", fasthttp.StatusForbidden)
		return
	}

	entry, err := auditRepo.GetAdminActionLogByID(id)
	if err != nil {
		h.logger.Error("Failed to fetch admin action detail", zap.String("actionId", id), zap.Error(err))
		ctx.Error("Failed to load admin action detail", fasthttp.StatusInternalServerError)
		return
	}
	if entry == nil {
		ctx.Error("admin action not found", fasthttp.StatusNotFound)
		return
	}
	if entry.TenantID == nil || strings.TrimSpace(*entry.TenantID) != tenantID {
		ctx.Error("admin action not found", fasthttp.StatusNotFound)
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	if err := json.NewEncoder(ctx).Encode(entry.ToDetail()); err != nil {
		h.logger.Error("Failed to encode admin action detail response", zap.Error(err))
		ctx.Error("Failed to encode admin action detail response", fasthttp.StatusInternalServerError)
		return
	}
}
