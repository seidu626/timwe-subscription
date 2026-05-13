package handler

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// PostbackAdminHandler exposes postback diagnostics for admins.
// This is useful because postbacks are server-to-server and won't appear in browser Network logs.
type PostbackAdminHandler struct {
	repo   *repository.PostbackRepository
	logger *zap.Logger
}

func NewPostbackAdminHandler(repo *repository.PostbackRepository, logger *zap.Logger) *PostbackAdminHandler {
	return &PostbackAdminHandler{repo: repo, logger: logger}
}

// GetByTransactionID handles:
// GET /v1/admin/postbacks?transaction_id=<uuid>
func (h *PostbackAdminHandler) GetByTransactionID(ctx *fasthttp.RequestCtx) {
	tenantID, ok := h.postbackTenantIDFromRequest(ctx)
	if !ok {
		ctx.Error("Tenant context required", fasthttp.StatusForbidden)
		return
	}

	raw := string(ctx.QueryArgs().Peek("transaction_id"))
	if raw == "" {
		ctx.Error("transaction_id is required", fasthttp.StatusBadRequest)
		return
	}

	txID, err := uuid.Parse(raw)
	if err != nil {
		ctx.Error("transaction_id must be a valid UUID", fasthttp.StatusBadRequest)
		return
	}

	outbox, err := h.repo.GetOutboxByTransactionIDForTenant(tenantID, txID)
	if err != nil {
		h.logger.Error("Failed to fetch postback outbox", zap.String("transaction_id", txID.String()), zap.Error(err))
		ctx.Error("Failed to fetch postbacks", fasthttp.StatusInternalServerError)
		return
	}

	outboxIDs := make([]uuid.UUID, 0, len(outbox))
	for _, o := range outbox {
		outboxIDs = append(outboxIDs, o.ID)
	}

	attemptsByOutbox, err := h.repo.GetAttemptsByOutboxIDs(outboxIDs)
	if err != nil {
		h.logger.Error("Failed to fetch postback attempts", zap.String("transaction_id", txID.String()), zap.Error(err))
		ctx.Error("Failed to fetch postback attempts", fasthttp.StatusInternalServerError)
		return
	}

	// Flatten attempts into a single list (admin UI expects this shape)
	var allAttempts []*domain.PostbackAttempt
	for _, attempts := range attemptsByOutbox {
		allAttempts = append(allAttempts, attempts...)
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(map[string]any{
		"transaction_id": txID.String(),
		"postbacks":      outbox,
		"attempts":       allAttempts,
	})
}

// ListByStatus handles:
// GET /v1/admin/postbacks/status/:status?limit=50&offset=0
func (h *PostbackAdminHandler) ListByStatus(ctx *fasthttp.RequestCtx) {
	tenantID, ok := h.postbackTenantIDFromRequest(ctx)
	if !ok {
		ctx.Error("Tenant context required", fasthttp.StatusForbidden)
		return
	}

	path := string(ctx.Path())
	statusStr := strings.TrimPrefix(path, "/v1/admin/postbacks/status/")
	if statusStr == "" || statusStr == path {
		ctx.Error("status is required", fasthttp.StatusBadRequest)
		return
	}

	status := domain.PostbackStatus(strings.ToUpper(statusStr))

	limit := 50
	if raw := string(ctx.QueryArgs().Peek("limit")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}

	offset := 0
	if raw := string(ctx.QueryArgs().Peek("offset")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			offset = v
		}
	}

	outbox, totalCount, err := h.repo.GetByStatusForTenant(tenantID, status, limit, offset)
	if err != nil {
		h.logger.Error("Failed to fetch postbacks by status", zap.String("status", string(status)), zap.Error(err))
		ctx.Error("Failed to fetch postbacks", fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(map[string]any{
		"status": status,
		"count":  totalCount,
		"limit":  limit,
		"offset": offset,
		"items":  outbox,
	})
}

// RetryPostback handles:
// POST /v1/admin/postbacks/:id/retry
func (h *PostbackAdminHandler) RetryPostback(ctx *fasthttp.RequestCtx) {
	tenantID, ok := h.postbackTenantIDFromRequest(ctx)
	if !ok {
		ctx.Error("Tenant context required", fasthttp.StatusForbidden)
		return
	}

	path := string(ctx.Path())
	// Extract ID from /v1/admin/postbacks/<id>/retry
	trimmed := strings.TrimPrefix(path, "/v1/admin/postbacks/")
	idStr := strings.TrimSuffix(trimmed, "/retry")

	id, err := uuid.Parse(idStr)
	if err != nil {
		ctx.Error("id must be a valid UUID", fasthttp.StatusBadRequest)
		return
	}

	if err := h.repo.ResetForRetryForTenant(tenantID, id); err != nil {
		h.logger.Error("Failed to retry postback", zap.String("id", id.String()), zap.Error(err))
		if strings.Contains(err.Error(), "not found") {
			ctx.Error("Postback not found", fasthttp.StatusNotFound)
			return
		}
		ctx.Error("Failed to retry postback: "+err.Error(), fasthttp.StatusUnprocessableEntity)
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(map[string]any{
		"id":     id.String(),
		"status": "PENDING",
	})
}

// BulkRequeueDLQ handles:
// POST /v1/admin/postbacks/requeue-dlq?limit=100&offset=0
func (h *PostbackAdminHandler) BulkRequeueDLQ(ctx *fasthttp.RequestCtx) {
	tenantID, ok := h.postbackTenantIDFromRequest(ctx)
	if !ok {
		ctx.Error("Tenant context required", fasthttp.StatusForbidden)
		return
	}

	limit := 100
	if raw := string(ctx.QueryArgs().Peek("limit")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}

	offset := 0
	if raw := string(ctx.QueryArgs().Peek("offset")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			offset = v
		}
	}

	count, err := h.repo.BulkResetDLQForTenant(tenantID, limit, offset)
	if err != nil {
		h.logger.Error("Failed to bulk requeue DLQ postbacks", zap.Error(err))
		ctx.Error("Failed to requeue DLQ postbacks", fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(map[string]any{
		"requeued": count,
		"limit":    limit,
		"offset":   offset,
	})
}

// GetStats handles:
// GET /v1/admin/postbacks/stats
func (h *PostbackAdminHandler) GetStats(ctx *fasthttp.RequestCtx) {
	tenantID, ok := h.postbackTenantIDFromRequest(ctx)
	if !ok {
		ctx.Error("Tenant context required", fasthttp.StatusForbidden)
		return
	}

	stats, err := h.repo.GetPostbackStatsForTenant(tenantID)
	if err != nil {
		h.logger.Error("Failed to fetch postback stats", zap.Error(err))
		ctx.Error("Failed to fetch postback stats", fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	_ = json.NewEncoder(ctx).Encode(map[string]any{
		"pending":    stats.Pending,
		"processing": stats.Processing,
		"success":    stats.Success,
		"failed":     stats.Failed,
		"dlq":        stats.DLQ,
		"total":      stats.Total,
		"alert":      stats.DLQ > 0,
	})
}

func (h *PostbackAdminHandler) String() string {
	return "PostbackAdminHandler"
}

func (h *PostbackAdminHandler) postbackTenantIDFromRequest(ctx *fasthttp.RequestCtx) (string, bool) {
	identity, ok := tenantIdentityFromRequest(ctx)
	if !ok {
		return "", false
	}
	if tenantID := strings.TrimSpace(identity.TenantID); tenantID != "" {
		return tenantID, true
	}
	if !identity.PlatformScoped {
		return "", false
	}
	if tenantID := strings.TrimSpace(string(ctx.Request.Header.Peek(tenantctx.HeaderTenantID))); tenantID != "" {
		return tenantID, true
	}
	tenantKey := firstNonBlankString(
		identity.TenantKey,
		string(ctx.Request.Header.Peek(tenantctx.HeaderTenantKey)),
		string(ctx.QueryArgs().Peek("tenantKey")),
		string(ctx.QueryArgs().Peek("tenant_key")),
	)
	if tenantKey == "" || h.repo == nil {
		return "", false
	}
	tenantID, err := h.repo.TenantIDByKey(tenantKey)
	if err != nil {
		h.logger.Warn("Failed to resolve postback tenant key", zap.String("tenant_key", tenantKey), zap.Error(err))
		return "", false
	}
	return tenantID, tenantID != ""
}

func firstNonBlankString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
