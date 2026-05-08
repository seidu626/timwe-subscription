package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// AnalyticsHandler handles analytics/event ingestion endpoints
type AnalyticsHandler struct {
	landingEventRepo *repository.LandingEventRepository
	logger           *zap.Logger
}

// NewAnalyticsHandler creates a new analytics handler
func NewAnalyticsHandler(
	landingEventRepo *repository.LandingEventRepository,
	logger *zap.Logger,
) *AnalyticsHandler {
	return &AnalyticsHandler{
		landingEventRepo: landingEventRepo,
		logger:           logger,
	}
}

// CreateLandingEvent handles POST /v1/analytics/landing/events
// This is a public endpoint (no admin auth required) for landing page event ingestion
func (h *AnalyticsHandler) CreateLandingEvent(ctx *fasthttp.RequestCtx) {
	var req domain.CreateLandingEventRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		h.logger.Warn("Invalid request body", zap.Error(err))
		ctx.SetContentType("application/json")
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		json.NewEncoder(ctx).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	// Validate the request
	if err := req.Validate(); err != nil {
		h.logger.Warn("Validation failed", zap.Error(err))
		ctx.SetContentType("application/json")
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		json.NewEncoder(ctx).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Extract and hash request metadata (privacy-preserving)
	ipHash := hashString(extractClientIP(ctx))
	userAgentHash := hashString(string(ctx.Request.Header.UserAgent()))
	referrerDomain := extractReferrerDomain(string(ctx.Request.Header.Referer()))

	// Build the event
	event := &domain.LandingEvent{
		EventType:      req.EventType,
		CampaignSlug:   req.CampaignSlug,
		ClickID:        req.ClickID,
		AdProvider:     req.AdProvider,
		SessionID:      req.SessionID,
		IPHash:         &ipHash,
		UserAgentHash:  &userAgentHash,
		ReferrerDomain: referrerDomain,
		CreatedAt:      time.Now().UTC(),
	}

	// Persist the event
	if err := h.landingEventRepo.Create(event); err != nil {
		h.logger.Error("Failed to create landing event",
			zap.String("campaign_slug", req.CampaignSlug),
			zap.String("event_type", string(req.EventType)),
			zap.Error(err),
		)
		ctx.SetContentType("application/json")
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		json.NewEncoder(ctx).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to record event",
		})
		return
	}

	h.logger.Debug("Landing event recorded",
		zap.String("campaign_slug", req.CampaignSlug),
		zap.String("event_type", string(req.EventType)),
		zap.Int64("event_id", event.ID),
	)

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	json.NewEncoder(ctx).Encode(domain.CreateLandingEventResponse{
		Success: true,
		Message: "Event recorded",
	})
}

// hashString returns a SHA256 hash of the input string
func hashString(s string) string {
	if s == "" {
		return ""
	}
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// extractClientIP extracts client IP from request headers or connection
func extractClientIP(ctx *fasthttp.RequestCtx) string {
	// Check X-Forwarded-For header first (for proxied requests)
	xff := string(ctx.Request.Header.Peek("X-Forwarded-For"))
	if xff != "" {
		// Take the first IP in the chain
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	// Check X-Real-IP header
	xri := string(ctx.Request.Header.Peek("X-Real-IP"))
	if xri != "" {
		return xri
	}

	// Fall back to remote address
	return ctx.RemoteAddr().String()
}

// extractReferrerDomain extracts just the domain from a referrer URL
func extractReferrerDomain(referrer string) *string {
	if referrer == "" {
		return nil
	}

	parsed, err := url.Parse(referrer)
	if err != nil {
		return nil
	}

	if parsed.Host == "" {
		return nil
	}

	return &parsed.Host
}
