package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// HEBootstrapHandler handles HTTP-only Header Enrichment bootstrap flow.
// It captures MSISDN from operator-injected headers, creates a short-lived token,
// and redirects to HTTPS where the token can be exchanged for a subscriber context.
type HEBootstrapHandler struct {
	redisClient     *redis.Client
	logger          *zap.Logger
	heMiddleware    *HEContextMiddleware
	tokenTTL        time.Duration
	tokenSecret     string
	httpsHost       string
	landingBasePath string
}

// HEBootstrapConfig holds configuration for the HE bootstrap handler
type HEBootstrapConfig struct {
	// Redis client for token storage
	RedisClient *redis.Client
	// Token TTL (default: 60 seconds)
	TokenTTL time.Duration
	// Secret for token signing (optional, for additional security)
	TokenSecret string
	// HTTPS host to redirect to (e.g., "landing.nouveauricheglobalgroup.com")
	HTTPSHost string
	// Landing page base path (default: "/")
	LandingBasePath string
	// HE context middleware for extracting identity
	HEMiddleware *HEContextMiddleware
}

// DefaultHEBootstrapConfig returns default configuration from environment
func DefaultHEBootstrapConfig() *HEBootstrapConfig {
	ttlSeconds := 60
	if s := os.Getenv("HE_BOOTSTRAP_TOKEN_TTL"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			ttlSeconds = v
		}
	}

	httpsHost := os.Getenv("HE_BOOTSTRAP_HTTPS_HOST")
	if httpsHost == "" {
		httpsHost = "landing.nouveauricheglobalgroup.com"
	}

	return &HEBootstrapConfig{
		TokenTTL:        time.Duration(ttlSeconds) * time.Second,
		TokenSecret:     os.Getenv("HE_BOOTSTRAP_TOKEN_SECRET"),
		HTTPSHost:       httpsHost,
		LandingBasePath: "/",
	}
}

// NewHEBootstrapHandler creates a new HE bootstrap handler
func NewHEBootstrapHandler(config *HEBootstrapConfig, logger *zap.Logger) *HEBootstrapHandler {
	if config == nil {
		config = DefaultHEBootstrapConfig()
	}
	if config.TokenTTL == 0 {
		config.TokenTTL = 60 * time.Second
	}
	if config.LandingBasePath == "" {
		config.LandingBasePath = "/"
	}

	return &HEBootstrapHandler{
		redisClient:     config.RedisClient,
		logger:          logger,
		heMiddleware:    config.HEMiddleware,
		tokenTTL:        config.TokenTTL,
		tokenSecret:     config.TokenSecret,
		httpsHost:       config.HTTPSHost,
		landingBasePath: config.LandingBasePath,
	}
}

// tokenKey returns the Redis key for a bootstrap token
func (h *HEBootstrapHandler) tokenKey(token string) string {
	return fmt.Sprintf("he_bootstrap:%s", token)
}

// generateToken creates a cryptographically secure random token
func (h *HEBootstrapHandler) generateToken() (string, error) {
	bytes := make([]byte, 32) // 256-bit token
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// HandleBootstrap handles GET /v1/he/bootstrap
// This is called from HTTP (port 80) with operator HE headers.
// It extracts the MSISDN, creates a token, and redirects to HTTPS.
func (h *HEBootstrapHandler) HandleBootstrap(ctx *fasthttp.RequestCtx) {
	// Check if request came from trusted proxy (set by NGINX)
	trustedProxy := string(ctx.Request.Header.Peek("X-HE-Trusted-Proxy"))
	if trustedProxy != "1" {
		h.logger.Warn("HE bootstrap request from untrusted source",
			zap.String("remote_addr", ctx.RemoteAddr().String()),
			zap.String("trusted_proxy_header", trustedProxy),
		)
		ctx.Error("Forbidden: Not from trusted operator proxy", fasthttp.StatusForbidden)
		return
	}

	// Extract HE identity from headers
	identity := h.heMiddleware.ExtractIdentity(ctx)
	if identity == nil || identity.MSISDN == "" {
		h.logger.Debug("No HE identity found in bootstrap request",
			zap.String("remote_addr", ctx.RemoteAddr().String()),
		)
		// No HE headers - redirect to HTTPS landing page without token
		// The user will go through OTP flow
		h.redirectWithoutToken(ctx, "")
		return
	}

	// Generate single-use token
	token, err := h.generateToken()
	if err != nil {
		h.logger.Error("Failed to generate bootstrap token", zap.Error(err))
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	// Store token data in Redis
	if err := h.storeToken(ctx, token, identity); err != nil {
		h.logger.Error("Failed to store bootstrap token",
			zap.Error(err),
			zap.String("msisdn_hash", hashMSISDN(identity.MSISDN)),
		)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	h.logger.Info("HE bootstrap token created",
		zap.String("msisdn_hash", hashMSISDN(identity.MSISDN)),
		zap.String("operator_id", identity.OperatorID),
		zap.String("source", string(identity.Source)),
		zap.Duration("ttl", h.tokenTTL),
	)

	// Preserve query params from original request
	originalQuery := string(ctx.QueryArgs().QueryString())

	// Redirect to HTTPS with token
	h.redirectWithToken(ctx, token, "", originalQuery)
}

// HandleBootstrapWithCampaign handles GET /v1/he/bootstrap/campaign/:slug
// Same as HandleBootstrap but preserves campaign context in the redirect.
func (h *HEBootstrapHandler) HandleBootstrapWithCampaign(ctx *fasthttp.RequestCtx) {
	// Check if request came from trusted proxy (set by NGINX)
	trustedProxy := string(ctx.Request.Header.Peek("X-HE-Trusted-Proxy"))
	if trustedProxy != "1" {
		h.logger.Warn("HE bootstrap request from untrusted source",
			zap.String("remote_addr", ctx.RemoteAddr().String()),
			zap.String("trusted_proxy_header", trustedProxy),
		)
		ctx.Error("Forbidden: Not from trusted operator proxy", fasthttp.StatusForbidden)
		return
	}

	// Extract campaign slug from path: /v1/he/bootstrap/campaign/{slug}
	path := string(ctx.Path())
	parts := strings.Split(path, "/")
	var campaignSlug string
	for i, p := range parts {
		if p == "campaign" && i+1 < len(parts) {
			campaignSlug = parts[i+1]
			break
		}
	}

	// Extract HE identity from headers
	identity := h.heMiddleware.ExtractIdentity(ctx)
	if identity == nil || identity.MSISDN == "" {
		h.logger.Debug("No HE identity found in campaign bootstrap request",
			zap.String("remote_addr", ctx.RemoteAddr().String()),
			zap.String("campaign", campaignSlug),
		)
		// No HE headers - redirect to HTTPS campaign page without token
		h.redirectWithoutToken(ctx, campaignSlug)
		return
	}

	// Generate single-use token
	token, err := h.generateToken()
	if err != nil {
		h.logger.Error("Failed to generate bootstrap token", zap.Error(err))
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	// Store token data in Redis (include campaign context)
	if err := h.storeTokenWithCampaign(ctx, token, identity, campaignSlug); err != nil {
		h.logger.Error("Failed to store bootstrap token",
			zap.Error(err),
			zap.String("msisdn_hash", hashMSISDN(identity.MSISDN)),
			zap.String("campaign", campaignSlug),
		)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	h.logger.Info("HE bootstrap token created with campaign",
		zap.String("msisdn_hash", hashMSISDN(identity.MSISDN)),
		zap.String("operator_id", identity.OperatorID),
		zap.String("source", string(identity.Source)),
		zap.String("campaign", campaignSlug),
		zap.Duration("ttl", h.tokenTTL),
	)

	// Preserve query params from original request
	originalQuery := string(ctx.QueryArgs().QueryString())

	// Redirect to HTTPS with token
	h.redirectWithToken(ctx, token, campaignSlug, originalQuery)
}

// HandleTokenExchange handles POST /v1/he/token/exchange
// This is called from HTTPS to exchange the bootstrap token for HE identity.
func (h *HEBootstrapHandler) HandleTokenExchange(ctx *fasthttp.RequestCtx) {
	token := string(ctx.QueryArgs().Peek("token"))
	if token == "" {
		token = string(ctx.Request.Header.Peek("X-HE-Bootstrap-Token"))
	}
	if token == "" {
		ctx.Error("Missing token parameter", fasthttp.StatusBadRequest)
		return
	}

	// Validate token format
	if len(token) != 64 { // 32 bytes = 64 hex chars
		ctx.Error("Invalid token format", fasthttp.StatusBadRequest)
		return
	}

	// Exchange token (single-use: get and delete atomically)
	identity, campaign, err := h.exchangeToken(ctx, token)
	if err != nil {
		if err == redis.Nil {
			h.logger.Debug("Bootstrap token not found or expired", zap.String("token_prefix", token[:8]))
			ctx.Error("Token not found or expired", fasthttp.StatusNotFound)
			return
		}
		h.logger.Error("Failed to exchange bootstrap token", zap.Error(err))
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	h.logger.Info("HE bootstrap token exchanged",
		zap.String("msisdn_hash", hashMSISDN(identity.MSISDN)),
		zap.String("operator_id", identity.OperatorID),
		zap.String("campaign", campaign),
	)

	// Return identity as JSON
	writeJSON(ctx, fasthttp.StatusOK, map[string]interface{}{
		"msisdn":      identity.MSISDN,
		"operator_id": identity.OperatorID,
		"mcc":         identity.MCC,
		"mnc":         identity.MNC,
		"source":      string(identity.Source),
		"campaign":    campaign,
	})
}

// storeToken stores token data in Redis
func (h *HEBootstrapHandler) storeToken(ctx *fasthttp.RequestCtx, token string, identity *HEIdentity) error {
	return h.storeTokenWithCampaign(ctx, token, identity, "")
}

// storeTokenWithCampaign stores token data in Redis with campaign context
func (h *HEBootstrapHandler) storeTokenWithCampaign(ctx *fasthttp.RequestCtx, token string, identity *HEIdentity, campaign string) error {
	if h.redisClient == nil {
		return fmt.Errorf("redis client not configured")
	}

	key := h.tokenKey(token)
	data := map[string]interface{}{
		"msisdn":      identity.MSISDN,
		"operator_id": identity.OperatorID,
		"mcc":         identity.MCC,
		"mnc":         identity.MNC,
		"source":      string(identity.Source),
		"campaign":    campaign,
		"created_at":  time.Now().Unix(),
		"remote_addr": ctx.RemoteAddr().String(),
	}

	// Use HSET with expiry
	pipe := h.redisClient.Pipeline()
	pipe.HSet(context.Background(), key, data)
	pipe.Expire(context.Background(), key, h.tokenTTL)
	_, err := pipe.Exec(context.Background())
	return err
}

// exchangeToken retrieves and deletes token data atomically (single-use)
func (h *HEBootstrapHandler) exchangeToken(ctx *fasthttp.RequestCtx, token string) (*HEIdentity, string, error) {
	if h.redisClient == nil {
		return nil, "", fmt.Errorf("redis client not configured")
	}

	key := h.tokenKey(token)
	bgCtx := context.Background()

	// Get all fields
	result, err := h.redisClient.HGetAll(bgCtx, key).Result()
	if err != nil {
		return nil, "", err
	}
	if len(result) == 0 {
		return nil, "", redis.Nil
	}

	// Delete immediately (single-use)
	if err := h.redisClient.Del(bgCtx, key).Err(); err != nil {
		h.logger.Warn("Failed to delete used bootstrap token", zap.Error(err), zap.String("token_prefix", token[:8]))
	}

	identity := &HEIdentity{
		MSISDN:     result["msisdn"],
		OperatorID: result["operator_id"],
		MCC:        result["mcc"],
		MNC:        result["mnc"],
		Source:     HESource(result["source"]),
	}
	campaign := result["campaign"]

	return identity, campaign, nil
}

// getRedirectHost returns the host to redirect to on HTTPS.
// By default, uses the same host as the incoming request (same-host redirect).
// Falls back to configured httpsHost if request host is empty or if explicitly configured.
func (h *HEBootstrapHandler) getRedirectHost(ctx *fasthttp.RequestCtx) string {
	// Get the host from the incoming request
	requestHost := string(ctx.Host())
	
	// Strip port if present (HTTP is port 80, we redirect to HTTPS on 443)
	if idx := strings.Index(requestHost, ":"); idx != -1 {
		requestHost = requestHost[:idx]
	}
	
	// Use request host if available, otherwise fall back to configured host
	if requestHost != "" {
		return requestHost
	}
	
	// Fallback to configured host
	if h.httpsHost != "" {
		return h.httpsHost
	}
	
	// Default fallback
	return "landing.nouveauricheglobalgroup.com"
}

// redirectWithToken redirects to HTTPS landing page with the bootstrap token
func (h *HEBootstrapHandler) redirectWithToken(ctx *fasthttp.RequestCtx, token, campaign, originalQuery string) {
	var targetPath string
	if campaign != "" {
		// Redirect to canonical landing path /lp/:slug (landing-web serves this)
		targetPath = fmt.Sprintf("/lp/%s", campaign)
	} else {
		targetPath = h.landingBasePath
	}

	// Get redirect host (same host as request by default)
	redirectHost := h.getRedirectHost(ctx)

	// Build redirect URL
	redirectURL := url.URL{
		Scheme: "https",
		Host:   redirectHost,
		Path:   targetPath,
	}

	// Parse and merge query params
	query := redirectURL.Query()
	query.Set("he_token", token)

	// Preserve original query params
	if originalQuery != "" {
		originalParams, _ := url.ParseQuery(originalQuery)
		for k, values := range originalParams {
			if k != "he_token" { // Don't overwrite our token
				for _, v := range values {
					query.Add(k, v)
				}
			}
		}
	}

	redirectURL.RawQuery = query.Encode()

	h.logger.Debug("HE bootstrap redirect",
		zap.String("request_host", string(ctx.Host())),
		zap.String("redirect_host", redirectHost),
		zap.String("redirect_url", redirectURL.String()),
	)

	ctx.Redirect(redirectURL.String(), fasthttp.StatusFound)
}

// redirectWithoutToken redirects to HTTPS landing page without token (OTP flow)
func (h *HEBootstrapHandler) redirectWithoutToken(ctx *fasthttp.RequestCtx, campaign string) {
	var targetPath string
	if campaign != "" {
		// Redirect to canonical landing path /lp/:slug (landing-web serves this)
		targetPath = fmt.Sprintf("/lp/%s", campaign)
	} else {
		targetPath = h.landingBasePath
	}

	// Get redirect host (same host as request by default)
	redirectHost := h.getRedirectHost(ctx)

	// Build redirect URL
	redirectURL := url.URL{
		Scheme: "https",
		Host:   redirectHost,
		Path:   targetPath,
	}

	// Preserve original query params
	originalQuery := string(ctx.QueryArgs().QueryString())
	if originalQuery != "" {
		redirectURL.RawQuery = originalQuery
	}

	h.logger.Debug("HE bootstrap redirect (no token)",
		zap.String("request_host", string(ctx.Host())),
		zap.String("redirect_host", redirectHost),
		zap.String("redirect_url", redirectURL.String()),
	)

	ctx.Redirect(redirectURL.String(), fasthttp.StatusFound)
}
