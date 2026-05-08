package handler

import (
	"fmt"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// ClickOutHandler handles outbound click redirect requests
type ClickOutHandler struct {
	repo   *repository.OutboundClickRepository
	config *ClickOutConfig
	logger *zap.Logger
}

// ClickOutConfig holds the handler configuration
type ClickOutConfig struct {
	// Destinations maps dest_key to destination config
	Destinations map[string]DestinationConfig
	// DefaultClickIDParam is the default query param name for click_id
	DefaultClickIDParam string
	// RateLimitPerIPPerHour limits clicks per IP per hour (0 = no limit)
	RateLimitPerIPPerHour int
	// CookieDomain for setting click_id cookie (empty = request domain)
	CookieDomain string
	// CookieSecure sets Secure flag on cookie
	CookieSecure bool
}

// DestinationConfig defines an allowlisted destination
type DestinationConfig struct {
	BaseURL        string   // Base URL to redirect to
	ClickIDParam   string   // Query param name for click_id (partner-specific)
	PassthroughParams []string // Params to copy from incoming request
	AllowedPartners []string // Restrict to specific partners (empty = all)
}

// NewClickOutHandler creates a new click-out handler
func NewClickOutHandler(
	repo *repository.OutboundClickRepository,
	config *ClickOutConfig,
	logger *zap.Logger,
) *ClickOutHandler {
	if config == nil {
		config = defaultClickOutConfig()
	}
	if config.DefaultClickIDParam == "" {
		config.DefaultClickIDParam = "click_id"
	}
	return &ClickOutHandler{
		repo:   repo,
		config: config,
		logger: logger,
	}
}

// defaultClickOutConfig returns a default configuration with common destinations
func defaultClickOutConfig() *ClickOutConfig {
	return &ClickOutConfig{
		// No default destinations: force allowlist to be configured explicitly.
		Destinations:         map[string]DestinationConfig{},
		DefaultClickIDParam:   "click_id",
		RateLimitPerIPPerHour: 100,
		CookieSecure:          true,
	}
}

// HandleClickOut handles GET /v1/click/out
// Query params:
//   - partner: required, partner/provider name (free-form string)
//   - dest: required, destination key from allowlist
//   - campaign: optional, campaign slug
//   - offer_id: optional, offer product ID
//   - ... other params passed through to destination
func (h *ClickOutHandler) HandleClickOut(ctx *fasthttp.RequestCtx) {
	// Parse query params
	args := ctx.QueryArgs()
	partner := string(args.Peek("partner"))
	destKey := string(args.Peek("dest"))

	// Validate required params
	if partner == "" {
		ctx.Error("partner is required", fasthttp.StatusBadRequest)
		return
	}
	if destKey == "" {
		ctx.Error("dest is required", fasthttp.StatusBadRequest)
		return
	}

	// Look up destination in allowlist
	destConfig, ok := h.config.Destinations[destKey]
	if !ok {
		h.logger.Warn("Unknown destination requested",
			zap.String("dest", destKey),
			zap.String("partner", partner),
		)
		ctx.Error("invalid destination", fasthttp.StatusBadRequest)
		return
	}

	// Check partner restriction if configured
	if len(destConfig.AllowedPartners) > 0 {
		allowed := false
		for _, p := range destConfig.AllowedPartners {
			if p == partner {
				allowed = true
				break
			}
		}
		if !allowed {
			ctx.Error("partner not allowed for this destination", fasthttp.StatusForbidden)
			return
		}
	}

	// Check destination is configured
	if destConfig.BaseURL == "" {
		h.logger.Error("Destination base URL not configured", zap.String("dest", destKey))
		ctx.Error("destination not configured", fasthttp.StatusInternalServerError)
		return
	}

	// Extract request metadata
	ipAddr := string(ctx.RemoteIP().String())
	userAgent := string(ctx.UserAgent())
	referrer := string(ctx.Referer())

	// Hash IP and UA for privacy
	ipHash := hashString(ipAddr)
	uaHash := hashString(userAgent)

	// Extract referrer domain
	var referrerDomain *string
	if referrer != "" {
		if u, err := url.Parse(referrer); err == nil && u.Host != "" {
			referrerDomain = &u.Host
		}
	}

	// Rate limiting check
	if h.config.RateLimitPerIPPerHour > 0 {
		since := time.Now().Add(-1 * time.Hour)
		count, err := h.repo.CountByPartnerSince(partner, ipHash, since)
		if err != nil {
			h.logger.Error("Failed to check rate limit", zap.Error(err))
			// Continue anyway - don't block on rate limit check failures
		} else if count >= h.config.RateLimitPerIPPerHour {
			h.logger.Warn("Rate limit exceeded",
				zap.String("partner", partner),
				zap.String("ip_hash", ipHash),
				zap.Int("count", count),
			)
			ctx.Error("rate limit exceeded", fasthttp.StatusTooManyRequests)
			return
		}
	}

	// Generate click_id
	clickID := uuid.New()

	// Collect query params snapshot
	queryParams := make(map[string]string)
	args.VisitAll(func(key, value []byte) {
		queryParams[string(key)] = string(value)
	})

	// Extract optional campaign/offer info
	var campaignSlug *string
	var offerProductID *int
	if cs := args.Peek("campaign"); len(cs) > 0 {
		s := string(cs)
		campaignSlug = &s
	}
	if oid := args.GetUintOrZero("offer_id"); oid > 0 {
		id := int(oid)
		offerProductID = &id
	}

	// Build destination URL
	destURL, err := h.buildDestinationURL(destConfig, clickID, args)
	if err != nil {
		h.logger.Error("Failed to build destination URL", zap.Error(err))
		ctx.Error("internal error", fasthttp.StatusInternalServerError)
		return
	}

	// Create outbound click record
	click := &domain.OutboundClick{
		ClickID:        clickID,
		Partner:        partner,
		CampaignSlug:   campaignSlug,
		OfferProductID: offerProductID,
		DestKey:        destKey,
		DestURL:        destURL,
		QueryParams:    queryParams,
		ReferrerDomain: referrerDomain,
		IPHash:         &ipHash,
		UserAgentHash:  &uaHash,
		Status:         domain.OutboundClickStatusCreated,
	}

	if err := h.repo.Create(click); err != nil {
		h.logger.Error("Failed to persist outbound click",
			zap.String("click_id", clickID.String()),
			zap.Error(err),
		)
		// Continue with redirect even if persistence fails (best effort)
	}

	// Set click_id cookie
	cookie := &fasthttp.Cookie{}
	cookie.SetKey("click_id")
	cookie.SetValue(clickID.String())
	cookie.SetPath("/")
	cookie.SetHTTPOnly(true)
	cookie.SetSameSite(fasthttp.CookieSameSiteLaxMode)
	if h.config.CookieSecure {
		cookie.SetSecure(true)
	}
	if h.config.CookieDomain != "" {
		cookie.SetDomain(h.config.CookieDomain)
	}
	// Cookie expires in 30 days
	cookie.SetExpire(time.Now().Add(30 * 24 * time.Hour))
	ctx.Response.Header.SetCookie(cookie)

	// Also set partner cookie
	partnerCookie := &fasthttp.Cookie{}
	partnerCookie.SetKey("click_partner")
	partnerCookie.SetValue(partner)
	partnerCookie.SetPath("/")
	partnerCookie.SetHTTPOnly(true)
	partnerCookie.SetSameSite(fasthttp.CookieSameSiteLaxMode)
	if h.config.CookieSecure {
		partnerCookie.SetSecure(true)
	}
	if h.config.CookieDomain != "" {
		partnerCookie.SetDomain(h.config.CookieDomain)
	}
	partnerCookie.SetExpire(time.Now().Add(30 * 24 * time.Hour))
	ctx.Response.Header.SetCookie(partnerCookie)

	h.logger.Info("Click-out redirect",
		zap.String("click_id", clickID.String()),
		zap.String("partner", partner),
		zap.String("dest_key", destKey),
		zap.String("dest_url", destURL),
	)

	// 302 redirect
	ctx.Redirect(destURL, fasthttp.StatusFound)
}

// buildDestinationURL constructs the redirect URL with click_id and passthrough params
func (h *ClickOutHandler) buildDestinationURL(config DestinationConfig, clickID uuid.UUID, args *fasthttp.Args) (string, error) {
	u, err := url.Parse(config.BaseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}

	// Start with existing query params from base URL
	q := u.Query()

	// Add click_id with the configured param name
	clickIDParam := config.ClickIDParam
	if clickIDParam == "" {
		clickIDParam = h.config.DefaultClickIDParam
	}
	q.Set(clickIDParam, clickID.String())

	// Add passthrough params
	for _, param := range config.PassthroughParams {
		if value := args.Peek(param); len(value) > 0 {
			q.Set(param, string(value))
		}
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

// hashString and extractReferrerDomain are defined in analytics_handler.go
