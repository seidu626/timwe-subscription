package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/lib/pq"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/service"
	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// CampaignHandler handles campaign-related HTTP requests
type CampaignHandler struct {
	service        *service.CampaignService
	assetService   *service.CampaignAssetService
	tenantResolver campaignTenantResolver
	logger         *zap.Logger
}

type campaignTenantResolver interface {
	ResolveCurrentTenant(identity tenantctx.Identity) (*domain.AdminTenant, error)
}

// NewCampaignHandler creates a new campaign handler
func NewCampaignHandler(campaignService *service.CampaignService, assetService *service.CampaignAssetService, logger *zap.Logger) *CampaignHandler {
	return &CampaignHandler{
		service:      campaignService,
		assetService: assetService,
		logger:       logger,
	}
}

func (h *CampaignHandler) SetTenantResolver(resolver campaignTenantResolver) {
	h.tenantResolver = resolver
}

var slugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var themeColorRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func extractLastPathSegment(path string) (string, bool) {
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return "", false
	}
	seg := parts[len(parts)-1]
	if seg == "" {
		return "", false
	}
	return seg, true
}

func extractCampaignSlugFromPath(path string) (string, bool) {
	// /v1/.../campaigns/:slug or /v1/admin/campaigns/:slug
	parts := strings.Split(path, "/")
	if len(parts) < 4 {
		return "", false
	}
	slug := parts[len(parts)-1]
	if slug == "" {
		return "", false
	}
	return slug, true
}

func extractTenantAndCampaignSlugFromPath(path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 || parts[0] != "v1" || parts[1] != "campaigns" {
		return "", "", false
	}
	tenantKey := strings.TrimSpace(parts[2])
	slug := strings.TrimSpace(parts[3])
	if tenantKey == "" || slug == "" {
		return "", "", false
	}
	return tenantKey, slug, true
}

func extractCampaignSlugBeforeSuffix(path, suffix string) (string, bool) {
	// /v1/admin/campaigns/:slug/<suffix>
	if !strings.HasSuffix(path, suffix) {
		return "", false
	}
	trimmed := strings.TrimSuffix(path, suffix)
	return extractLastPathSegment(trimmed)
}

func writeJSON(ctx *fasthttp.RequestCtx, status int, v any) {
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(status)
	_ = json.NewEncoder(ctx).Encode(v)
}

// GetBySlug handles GET /v1/campaigns/:slug
func (h *CampaignHandler) GetBySlug(ctx *fasthttp.RequestCtx) {
	// Extract slug from path
	path := string(ctx.Path())
	slug, ok := extractCampaignSlugFromPath(path)
	if !ok {
		ctx.Error("Invalid path", fasthttp.StatusBadRequest)
		return
	}

	if hasPublicTenantHeaders(ctx) {
		identity, err := trustedPublicTenantIdentityFromRequest(ctx)
		if err != nil || strings.TrimSpace(identity.TenantKey) == "" {
			ctx.Error("Tenant context invalid", fasthttp.StatusForbidden)
			return
		}
		campaign, err := h.service.GetByTenantKeyAndSlug(identity.TenantKey, slug)
		if err != nil {
			h.logger.Error("Failed to get trusted tenant campaign", zap.String("tenant_key", identity.TenantKey), zap.String("slug", slug), zap.Error(err))
			ctx.Error("Campaign not found", fasthttp.StatusNotFound)
			return
		}
		writeJSON(ctx, fasthttp.StatusOK, campaign)
		return
	}

	ctx.Error("Tenant context required", fasthttp.StatusForbidden)
}

type fastHTTPHeaderGetter struct {
	header *fasthttp.RequestHeader
}

func (g fastHTTPHeaderGetter) Get(name string) string {
	if g.header == nil {
		return ""
	}
	return string(g.header.Peek(name))
}

func hasPublicTenantHeaders(ctx *fasthttp.RequestCtx) bool {
	return len(ctx.Request.Header.Peek(tenantctx.HeaderTenantID)) > 0 ||
		len(ctx.Request.Header.Peek(tenantctx.HeaderTenantKey)) > 0
}

func trustedPublicTenantIdentityFromRequest(ctx *fasthttp.RequestCtx) (tenantctx.Identity, error) {
	secret := strings.TrimSpace(os.Getenv("TENANT_TRUSTED_HEADER_SECRET"))
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("TRUSTED_SERVICE_TENANT_SECRET"))
	}
	return tenantctx.IdentityFromTrustedRequest(
		string(ctx.Method()),
		string(ctx.Path()),
		fastHTTPHeaderGetter{header: &ctx.Request.Header},
		tenantctx.TrustedHeaderOptions{Secret: secret},
	)
}

// GetByTenantAndSlug handles GET /v1/campaigns/:tenant_key/:slug
func (h *CampaignHandler) GetByTenantAndSlug(ctx *fasthttp.RequestCtx) {
	path := string(ctx.Path())
	tenantKey, slug, ok := extractTenantAndCampaignSlugFromPath(path)
	if !ok {
		ctx.Error("Invalid path", fasthttp.StatusBadRequest)
		return
	}
	if !slugRe.MatchString(tenantKey) || !slugRe.MatchString(slug) {
		ctx.Error("Invalid tenant or campaign slug", fasthttp.StatusBadRequest)
		return
	}

	campaign, err := h.service.GetByTenantKeyAndSlug(tenantKey, slug)
	if err != nil {
		h.logger.Error("Failed to get tenant campaign", zap.String("tenant_key", tenantKey), zap.String("slug", slug), zap.Error(err))
		ctx.Error("Campaign not found", fasthttp.StatusNotFound)
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, campaign)
}

// ListEnabled handles GET /v1/campaigns
func (h *CampaignHandler) ListEnabled(ctx *fasthttp.RequestCtx) {
	ctx.Error("Tenant context required", fasthttp.StatusForbidden)
}

type adminCampaignUpsertRequest struct {
	Slug               string          `json:"slug"`
	Language           string          `json:"language"`
	Country            string          `json:"country"`
	Operator           *string         `json:"operator,omitempty"`
	ChannelID          *string         `json:"channel_id,omitempty"`
	OfferProductID     int             `json:"offer_product_id"`
	PricepointID       *int            `json:"pricepoint_id,omitempty"`
	PartnerRoleID      *int            `json:"partner_role_id,omitempty"`
	FlowType           domain.FlowType `json:"flow_type"`
	ShortCode          *string         `json:"short_code,omitempty"`
	SMSKeyword         *string         `json:"sms_keyword,omitempty"`
	Price              *float64        `json:"price,omitempty"`
	BillingCycle       *string         `json:"billing_cycle,omitempty"`
	TrialFlags         json.RawMessage `json:"trial_flags,omitempty"`
	TermsURL           *string         `json:"terms_url,omitempty"`
	InlineTermsText    *string         `json:"inline_terms_text,omitempty"`
	ConsentRequired    bool            `json:"consent_required"`
	ConsentVersion     *string         `json:"consent_version,omitempty"`
	AttributionMapping json.RawMessage `json:"attribution_mapping,omitempty"`
	PostbackRules      json.RawMessage `json:"postback_rules,omitempty"`
	Throttles          json.RawMessage `json:"throttles,omitempty"`
	AllowedReferrers   []string        `json:"allowed_referrers,omitempty"`
	AllowedSources     []string        `json:"allowed_sources,omitempty"`
	LandingPageURLs    []string        `json:"landing_page_urls,omitempty"`
	TrackingConfig     json.RawMessage `json:"tracking_config,omitempty"`
	LPCopy             json.RawMessage `json:"lp_copy,omitempty"`
	Enabled            bool            `json:"enabled"`
	CreatedBy          *string         `json:"created_by,omitempty"`
	UpdatedBy          *string         `json:"updated_by,omitempty"`
}

type adminSetEnabledRequest struct {
	Enabled   bool    `json:"enabled"`
	UpdatedBy *string `json:"updated_by,omitempty"`
}

type adminCloneCampaignRequest struct {
	NewSlug   string  `json:"new_slug"`
	CreatedBy *string `json:"created_by,omitempty"`
}

type trackingConfig struct {
	Pixels       *trackingPixels       `json:"pixels,omitempty"`
	Attribution  *trackingAttribution  `json:"attribution,omitempty"`
	Visual       *trackingVisual       `json:"visual,omitempty"`
	RedirectURL  string                `json:"redirect_url,omitempty"`
	Redirect     *trackingRedirect     `json:"redirect,omitempty"`
	CustomEvents []trackingCustomEvent `json:"custom_events,omitempty"`
}

type trackingPixels struct {
	Facebook *trackingFacebookPixel `json:"facebook,omitempty"`
	Google   *trackingGoogleTag     `json:"google,omitempty"`
	TikTok   *trackingTikTokPixel   `json:"tiktok,omitempty"`
}

type trackingFacebookPixel struct {
	PixelID string `json:"pixel_id"`
	Enabled *bool  `json:"enabled"`
}

type trackingGoogleTag struct {
	MeasurementID string  `json:"measurement_id"`
	AdsID         *string `json:"ads_id,omitempty"`
	Enabled       *bool   `json:"enabled"`
}

type trackingTikTokPixel struct {
	PixelID string `json:"pixel_id"`
	Enabled *bool  `json:"enabled"`
}

type trackingAttribution struct {
	Model      string `json:"model"`
	WindowDays int    `json:"window_days"`
}

type trackingVisual struct {
	BackgroundImageURL string `json:"background_image_url,omitempty"`
	ThemeColor         string `json:"theme_color,omitempty"`
}

type trackingCustomEvent struct {
	Name    string `json:"name"`
	Trigger string `json:"trigger"`
}

type trackingRedirect struct {
	URL string `json:"url"`
}

type adminPresignBackgroundUploadRequest struct {
	CampaignSlug string `json:"campaign_slug"`
	FileName     string `json:"file_name"`
	ContentType  string `json:"content_type"`
	SizeBytes    int64  `json:"size_bytes"`
}

type lpCopyPayload struct {
	En *lpCopyText `json:"en"`
	Ar *lpCopyText `json:"ar,omitempty"`
}

type lpCopyText struct {
	HeroTitle         string `json:"heroTitle"`
	HEDescription     string `json:"heDescription"`
	HECTA             string `json:"heCta"`
	HEModalTitle      string `json:"heModalTitle"`
	HEModalConfirm    string `json:"heModalConfirm"`
	MSISDNDescription string `json:"msisdnDescription"`
	MSISDNPlaceholder string `json:"msisdnPlaceholder"`
	MSISDNCTA         string `json:"msisdnCta"`
	OTPDescription    string `json:"otpDescription"`
	OTPPlaceholder    string `json:"otpPlaceholder"`
	OTPCTA            string `json:"otpCta"`
	SuccessTitle      string `json:"successTitle"`
	SuccessBody       string `json:"successBody"`
	ConsentPrefix     string `json:"consentPrefix"`
	ConsentTerms      string `json:"consentTerms"`
	TermsHeading      string `json:"termsHeading"`
	Legal             string `json:"legal"`
	PhoneRequired     string `json:"phoneRequired"`
	PhoneInvalid      string `json:"phoneInvalid"`
	OTPInvalid        string `json:"otpInvalid"`
	ConsentRequired   string `json:"consentRequired"`
}

var defaultLPCopy = json.RawMessage(`{
  "en": {
    "heroTitle": "Subscribe to unlock premium content.",
    "heDescription": "To continue, tap Subscribe.",
    "heCta": "Subscribe",
    "heModalTitle": "Almost there. Please confirm to continue.",
    "heModalConfirm": "Confirm",
    "msisdnDescription": "Enter your mobile number to receive your PIN code.",
    "msisdnPlaceholder": "Mobile number (9 digits)",
    "msisdnCta": "Subscribe",
    "otpDescription": "Enter the 4-digit PIN sent to your phone.",
    "otpPlaceholder": "4-digit PIN",
    "otpCta": "Confirm",
    "successTitle": "Subscription successful",
    "successBody": "You will receive a text message with your access details.",
    "consentPrefix": "I agree to the",
    "consentTerms": "Terms and Conditions",
    "termsHeading": "Terms and Conditions",
    "legal": "Your subscription renews automatically until cancelled. You must be 18+ years old or have parental permission to use this service.",
    "phoneRequired": "Phone number is required.",
    "phoneInvalid": "Enter a valid 9-digit mobile number.",
    "otpInvalid": "PIN must be exactly 4 digits.",
    "consentRequired": "You must accept terms to continue."
  }
}`)

// cleanLandingPageURLs trims whitespace, removes empty strings, and deduplicates URLs
func cleanLandingPageURLs(urls []string) []string {
	if len(urls) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	result := make([]string, 0, len(urls))
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if !seen[u] {
			seen[u] = true
			result = append(result, u)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func validateAdminUpsert(req *adminCampaignUpsertRequest, requireSlug bool) error {
	if requireSlug {
		if strings.TrimSpace(req.Slug) == "" {
			return fmt.Errorf("slug is required")
		}
		if !slugRe.MatchString(req.Slug) {
			return fmt.Errorf("slug must match %s", slugRe.String())
		}
	}
	if strings.TrimSpace(req.Language) == "" {
		return fmt.Errorf("language is required")
	}
	if strings.TrimSpace(req.Country) == "" {
		return fmt.Errorf("country is required")
	}
	if strings.TrimSpace(string(req.FlowType)) == "" {
		return fmt.Errorf("flow_type is required")
	}
	switch req.FlowType {
	case domain.FlowTypeClickToSMS:
		if req.ShortCode == nil || strings.TrimSpace(*req.ShortCode) == "" {
			return fmt.Errorf("short_code is required for flow_type=CLICK_TO_SMS")
		}
		if req.SMSKeyword == nil || strings.TrimSpace(*req.SMSKeyword) == "" {
			return fmt.Errorf("sms_keyword is required for flow_type=CLICK_TO_SMS")
		}
	case domain.FlowTypeOTP, domain.FlowTypeRedirect, domain.FlowTypeMixed:
		// ok
	default:
		return fmt.Errorf("invalid flow_type")
	}
	if req.OfferProductID <= 0 {
		return fmt.Errorf("offer_product_id is required")
	}
	if err := validateTrackingConfig(req.TrackingConfig); err != nil {
		return err
	}
	if req.FlowType == domain.FlowTypeRedirect {
		if _, err := resolveRedirectURL(req.TrackingConfig, req.LandingPageURLs); err != nil {
			return fmt.Errorf("redirect flow requires a valid destination: %w", err)
		}
	}
	normalizedLPCopy, err := normalizeAndValidateLPCopy(req.LPCopy)
	if err != nil {
		return err
	}
	req.LPCopy = normalizedLPCopy
	// Validate landing page URLs (each must be a valid absolute http(s) URL)
	for i, lpURL := range req.LandingPageURLs {
		lpURL = strings.TrimSpace(lpURL)
		if lpURL == "" {
			continue // empty strings will be filtered out later
		}
		u, err := url.Parse(lpURL)
		if err != nil {
			return fmt.Errorf("landing_page_urls[%d]: invalid URL: %w", i, err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("landing_page_urls[%d]: must be http or https URL", i)
		}
		if u.Host == "" {
			return fmt.Errorf("landing_page_urls[%d]: URL must have a host", i)
		}
	}
	return nil
}

func normalizeAndValidateLPCopy(raw json.RawMessage) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		trimmed = defaultLPCopy
	}

	var payload lpCopyPayload
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&payload); err != nil {
		return nil, fmt.Errorf("lp_copy: %w", err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("lp_copy: invalid trailing data")
	}

	if payload.En == nil {
		return nil, fmt.Errorf("lp_copy.en is required")
	}
	if err := validateLPCopyText("lp_copy.en", payload.En); err != nil {
		return nil, err
	}
	if payload.Ar != nil {
		if err := validateLPCopyText("lp_copy.ar", payload.Ar); err != nil {
			return nil, err
		}
	}

	normalized, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("lp_copy: failed to normalize: %w", err)
	}
	return json.RawMessage(normalized), nil
}

func validateLPCopyText(path string, copy *lpCopyText) error {
	required := []struct {
		field string
		value string
	}{
		{field: "heroTitle", value: copy.HeroTitle},
		{field: "heDescription", value: copy.HEDescription},
		{field: "heCta", value: copy.HECTA},
		{field: "heModalTitle", value: copy.HEModalTitle},
		{field: "heModalConfirm", value: copy.HEModalConfirm},
		{field: "msisdnDescription", value: copy.MSISDNDescription},
		{field: "msisdnPlaceholder", value: copy.MSISDNPlaceholder},
		{field: "msisdnCta", value: copy.MSISDNCTA},
		{field: "otpDescription", value: copy.OTPDescription},
		{field: "otpPlaceholder", value: copy.OTPPlaceholder},
		{field: "otpCta", value: copy.OTPCTA},
		{field: "successTitle", value: copy.SuccessTitle},
		{field: "successBody", value: copy.SuccessBody},
		{field: "consentPrefix", value: copy.ConsentPrefix},
		{field: "consentTerms", value: copy.ConsentTerms},
		{field: "termsHeading", value: copy.TermsHeading},
		{field: "legal", value: copy.Legal},
		{field: "phoneRequired", value: copy.PhoneRequired},
		{field: "phoneInvalid", value: copy.PhoneInvalid},
		{field: "otpInvalid", value: copy.OTPInvalid},
		{field: "consentRequired", value: copy.ConsentRequired},
	}

	for _, entry := range required {
		if strings.TrimSpace(entry.value) == "" {
			return fmt.Errorf("%s.%s is required", path, entry.field)
		}
	}
	return nil
}

func validateTrackingConfig(raw json.RawMessage) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}

	var cfg trackingConfig
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return fmt.Errorf("tracking_config: %w", err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("tracking_config: invalid trailing data")
	}

	if cfg.Pixels != nil {
		if cfg.Pixels.Facebook != nil {
			if strings.TrimSpace(cfg.Pixels.Facebook.PixelID) == "" {
				return fmt.Errorf("tracking_config.pixels.facebook.pixel_id is required")
			}
			if cfg.Pixels.Facebook.Enabled == nil {
				return fmt.Errorf("tracking_config.pixels.facebook.enabled is required")
			}
		}
		if cfg.Pixels.Google != nil {
			if strings.TrimSpace(cfg.Pixels.Google.MeasurementID) == "" {
				return fmt.Errorf("tracking_config.pixels.google.measurement_id is required")
			}
			if cfg.Pixels.Google.Enabled == nil {
				return fmt.Errorf("tracking_config.pixels.google.enabled is required")
			}
		}
		if cfg.Pixels.TikTok != nil {
			if strings.TrimSpace(cfg.Pixels.TikTok.PixelID) == "" {
				return fmt.Errorf("tracking_config.pixels.tiktok.pixel_id is required")
			}
			if cfg.Pixels.TikTok.Enabled == nil {
				return fmt.Errorf("tracking_config.pixels.tiktok.enabled is required")
			}
		}
	}

	if cfg.Attribution != nil {
		model := strings.TrimSpace(cfg.Attribution.Model)
		switch model {
		case "first_touch", "last_touch", "linear":
		default:
			return fmt.Errorf("tracking_config.attribution.model must be one of first_touch, last_touch, linear")
		}
		if cfg.Attribution.WindowDays <= 0 {
			return fmt.Errorf("tracking_config.attribution.window_days must be greater than 0")
		}
	}

	for i, event := range cfg.CustomEvents {
		if strings.TrimSpace(event.Name) == "" {
			return fmt.Errorf("tracking_config.custom_events[%d].name is required", i)
		}
		if strings.TrimSpace(event.Trigger) == "" {
			return fmt.Errorf("tracking_config.custom_events[%d].trigger is required", i)
		}
	}

	if cfg.Visual != nil {
		if v := strings.TrimSpace(cfg.Visual.BackgroundImageURL); v != "" {
			if err := validateBackgroundImageURL(v); err != nil {
				return fmt.Errorf("tracking_config.visual.background_image_url: %w", err)
			}
		}
		if v := strings.TrimSpace(cfg.Visual.ThemeColor); v != "" {
			if !themeColorRe.MatchString(v) {
				return fmt.Errorf("tracking_config.visual.theme_color must be a #RRGGBB color")
			}
		}
	}

	if strings.TrimSpace(cfg.RedirectURL) != "" {
		if err := validateBackgroundImageURL(strings.TrimSpace(cfg.RedirectURL)); err != nil {
			return fmt.Errorf("tracking_config.redirect_url: %w", err)
		}
	}

	if cfg.Redirect != nil {
		if strings.TrimSpace(cfg.Redirect.URL) == "" {
			return fmt.Errorf("tracking_config.redirect.url is required")
		}
		if err := validateBackgroundImageURL(strings.TrimSpace(cfg.Redirect.URL)); err != nil {
			return fmt.Errorf("tracking_config.redirect.url: %w", err)
		}
	}

	return nil
}

func resolveRedirectURL(trackingRaw json.RawMessage, landingPageURLs []string) (string, error) {
	if len(bytes.TrimSpace(trackingRaw)) > 0 && string(bytes.TrimSpace(trackingRaw)) != "null" {
		var cfg trackingConfig
		dec := json.NewDecoder(bytes.NewReader(trackingRaw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&cfg); err == nil {
			if v := strings.TrimSpace(cfg.RedirectURL); v != "" {
				return v, nil
			}
			if cfg.Redirect != nil {
				if v := strings.TrimSpace(cfg.Redirect.URL); v != "" {
					return v, nil
				}
			}
		}
	}

	for _, lpURL := range landingPageURLs {
		v := strings.TrimSpace(lpURL)
		if v == "" {
			continue
		}
		u, err := url.Parse(v)
		if err != nil {
			continue
		}
		if (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
			return u.String(), nil
		}
	}

	return "", fmt.Errorf("set tracking_config.redirect_url (or tracking_config.redirect.url) or provide landing_page_urls")
}

func validateBackgroundImageURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("must be an http or https URL")
	}
	if strings.TrimSpace(u.Host) == "" {
		return fmt.Errorf("host is required")
	}
	return nil
}

func (h *CampaignHandler) AdminGetBySlug(ctx *fasthttp.RequestCtx) {
	path := string(ctx.Path())
	slug, ok := extractCampaignSlugFromPath(path)
	if !ok {
		ctx.Error("Invalid path", fasthttp.StatusBadRequest)
		return
	}

	tenant, _, ok := h.currentCampaignTenantFromRequest(ctx)
	if !ok {
		return
	}

	campaign, err := h.service.AdminGetByTenantAndSlug(tenant.ID, slug)
	if err != nil {
		h.logger.Error("Failed to get campaign (admin)", zap.String("slug", slug), zap.Error(err))
		ctx.Error("Campaign not found", fasthttp.StatusNotFound)
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, campaign)
}

func (h *CampaignHandler) AdminList(ctx *fasthttp.RequestCtx) {
	tenant, _, ok := h.currentCampaignTenantFromRequest(ctx)
	if !ok {
		return
	}

	// Parse query string manually; fasthttp provides QueryArgs but simplest is URL parse.
	raw := string(ctx.URI().FullURI())
	u, err := url.Parse(raw)
	if err != nil {
		ctx.Error("Invalid query", fasthttp.StatusBadRequest)
		return
	}

	var enabled *bool
	if v := u.Query().Get("enabled"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			ctx.Error("enabled must be true or false", fasthttp.StatusBadRequest)
			return
		}
		enabled = &b
	}

	var country *string
	if v := strings.TrimSpace(u.Query().Get("country")); v != "" {
		country = &v
	}

	campaigns, err := h.service.AdminListForTenant(tenant.ID, enabled, country)
	if err != nil {
		h.logger.Error("Failed to list campaigns (admin)", zap.Error(err))
		ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, map[string]any{"campaigns": campaigns})
}

func (h *CampaignHandler) AdminCreate(ctx *fasthttp.RequestCtx) {
	tenant, _, ok := h.currentCampaignTenantFromRequest(ctx)
	if !ok {
		return
	}

	var req adminCampaignUpsertRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid request body", fasthttp.StatusBadRequest)
		return
	}
	if err := validateAdminUpsert(&req, true); err != nil {
		ctx.Error(err.Error(), fasthttp.StatusBadRequest)
		return
	}

	created, err := h.service.AdminCreateForTenant(tenant.ID, &domain.Campaign{
		Slug:               req.Slug,
		TenantID:           &tenant.ID,
		ChannelID:          normalizedOptionalString(req.ChannelID),
		Language:           req.Language,
		Country:            req.Country,
		Operator:           req.Operator,
		OfferProductID:     req.OfferProductID,
		PricepointID:       req.PricepointID,
		PartnerRoleID:      req.PartnerRoleID,
		FlowType:           req.FlowType,
		ShortCode:          req.ShortCode,
		SMSKeyword:         req.SMSKeyword,
		Price:              req.Price,
		BillingCycle:       req.BillingCycle,
		TrialFlags:         req.TrialFlags,
		TermsURL:           req.TermsURL,
		InlineTermsText:    req.InlineTermsText,
		ConsentRequired:    req.ConsentRequired,
		ConsentVersion:     req.ConsentVersion,
		AttributionMapping: req.AttributionMapping,
		PostbackRules:      req.PostbackRules,
		Throttles:          req.Throttles,
		AllowedReferrers:   req.AllowedReferrers,
		AllowedSources:     req.AllowedSources,
		LandingPageURLs:    cleanLandingPageURLs(req.LandingPageURLs),
		TrackingConfig:     req.TrackingConfig,
		LPCopy:             req.LPCopy,
		Enabled:            req.Enabled,
		CreatedBy:          req.CreatedBy,
		UpdatedBy:          req.UpdatedBy,
	})
	if err != nil {
		if status := mapTenantCampaignErrorStatus(err); status != fasthttp.StatusInternalServerError {
			ctx.Error(err.Error(), status)
			return
		}
		if isCampaignConfigValidationError(err) {
			ctx.Error(err.Error(), fasthttp.StatusBadRequest)
			return
		}
		h.logger.Error("Failed to create campaign (admin)", zap.String("slug", req.Slug), zap.Error(err))
		ctx.Error("Failed to create campaign", fasthttp.StatusInternalServerError)
		return
	}

	writeJSON(ctx, fasthttp.StatusCreated, created)
}

func (h *CampaignHandler) AdminUpdate(ctx *fasthttp.RequestCtx) {
	tenant, _, ok := h.currentCampaignTenantFromRequest(ctx)
	if !ok {
		return
	}

	path := string(ctx.Path())
	slug, ok := extractCampaignSlugFromPath(path)
	if !ok {
		ctx.Error("Invalid path", fasthttp.StatusBadRequest)
		return
	}

	var req adminCampaignUpsertRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid request body", fasthttp.StatusBadRequest)
		return
	}
	// For update, slug comes from path; ignore any body slug.
	if err := validateAdminUpsert(&req, false); err != nil {
		ctx.Error(err.Error(), fasthttp.StatusBadRequest)
		return
	}

	updated, err := h.service.AdminUpdateForTenant(tenant.ID, slug, &domain.Campaign{
		Slug:               slug,
		TenantID:           &tenant.ID,
		ChannelID:          normalizedOptionalString(req.ChannelID),
		Language:           req.Language,
		Country:            req.Country,
		Operator:           req.Operator,
		OfferProductID:     req.OfferProductID,
		PricepointID:       req.PricepointID,
		PartnerRoleID:      req.PartnerRoleID,
		FlowType:           req.FlowType,
		ShortCode:          req.ShortCode,
		SMSKeyword:         req.SMSKeyword,
		Price:              req.Price,
		BillingCycle:       req.BillingCycle,
		TrialFlags:         req.TrialFlags,
		TermsURL:           req.TermsURL,
		InlineTermsText:    req.InlineTermsText,
		ConsentRequired:    req.ConsentRequired,
		ConsentVersion:     req.ConsentVersion,
		AttributionMapping: req.AttributionMapping,
		PostbackRules:      req.PostbackRules,
		Throttles:          req.Throttles,
		AllowedReferrers:   req.AllowedReferrers,
		AllowedSources:     req.AllowedSources,
		LandingPageURLs:    cleanLandingPageURLs(req.LandingPageURLs),
		TrackingConfig:     req.TrackingConfig,
		LPCopy:             req.LPCopy,
		Enabled:            req.Enabled,
		UpdatedBy:          req.UpdatedBy,
	})
	if err != nil {
		if status := mapTenantCampaignErrorStatus(err); status != fasthttp.StatusInternalServerError {
			ctx.Error(err.Error(), status)
			return
		}
		if isCampaignConfigValidationError(err) {
			ctx.Error(err.Error(), fasthttp.StatusBadRequest)
			return
		}
		h.logger.Error("Failed to update campaign (admin)", zap.String("slug", slug), zap.Error(err))
		ctx.Error("Failed to update campaign", fasthttp.StatusInternalServerError)
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, updated)
}

func (h *CampaignHandler) AdminSetEnabled(ctx *fasthttp.RequestCtx) {
	tenant, _, ok := h.currentCampaignTenantFromRequest(ctx)
	if !ok {
		return
	}

	path := string(ctx.Path())
	slug, ok := extractCampaignSlugBeforeSuffix(path, "/enabled")
	if !ok {
		ctx.Error("Invalid path", fasthttp.StatusBadRequest)
		return
	}

	var req adminSetEnabledRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid request body", fasthttp.StatusBadRequest)
		return
	}

	updated, err := h.service.AdminSetEnabledForTenant(tenant.ID, slug, req.Enabled, req.UpdatedBy)
	if err != nil {
		if status := mapTenantCampaignErrorStatus(err); status != fasthttp.StatusInternalServerError {
			ctx.Error(err.Error(), status)
			return
		}
		h.logger.Error("Failed to set enabled (admin)", zap.String("slug", slug), zap.Error(err))
		ctx.Error("Failed to update campaign", fasthttp.StatusInternalServerError)
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, updated)
}

func (h *CampaignHandler) AdminClone(ctx *fasthttp.RequestCtx) {
	path := string(ctx.Path())
	sourceSlug, ok := extractCampaignSlugBeforeSuffix(path, "/clone")
	if !ok {
		ctx.Error("Invalid path", fasthttp.StatusBadRequest)
		return
	}

	var req adminCloneCampaignRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid request body", fasthttp.StatusBadRequest)
		return
	}

	if err := validateCloneCampaignRequest(sourceSlug, &req); err != nil {
		ctx.Error(err.Error(), fasthttp.StatusBadRequest)
		return
	}

	newSlug := strings.TrimSpace(req.NewSlug)

	var createdBy *string
	if req.CreatedBy != nil {
		trimmed := strings.TrimSpace(*req.CreatedBy)
		if trimmed != "" {
			createdBy = &trimmed
		}
	}

	cloned, err := h.service.AdminClone(sourceSlug, newSlug, createdBy)
	if err != nil {
		status := mapCampaignCloneErrorStatus(err)
		if status >= fasthttp.StatusInternalServerError {
			h.logger.Error("Failed to clone campaign (admin)",
				zap.String("source_slug", sourceSlug),
				zap.String("new_slug", newSlug),
				zap.Error(err),
			)
		} else {
			h.logger.Warn("Campaign clone rejected (admin)",
				zap.String("source_slug", sourceSlug),
				zap.String("new_slug", newSlug),
				zap.Error(err),
			)
		}
		ctx.Error(err.Error(), status)
		return
	}

	writeJSON(ctx, fasthttp.StatusCreated, cloned)
}

func validateCloneCampaignRequest(sourceSlug string, req *adminCloneCampaignRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	newSlug := strings.TrimSpace(req.NewSlug)
	if newSlug == "" {
		return fmt.Errorf("new_slug is required")
	}
	if !slugRe.MatchString(newSlug) {
		return fmt.Errorf("new_slug must match %s", slugRe.String())
	}
	if strings.EqualFold(sourceSlug, newSlug) {
		return fmt.Errorf("new_slug must be different from source slug")
	}

	return nil
}

func mapCampaignCloneErrorStatus(err error) int {
	if err == nil {
		return fasthttp.StatusInternalServerError
	}

	var pqErr *pq.Error
	if errors.As(err, &pqErr) && string(pqErr.Code) == "23505" {
		return fasthttp.StatusConflict
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "campaign not found"):
		return fasthttp.StatusNotFound
	case strings.Contains(msg, "duplicate key value"):
		return fasthttp.StatusConflict
	case strings.Contains(msg, "already exists"):
		return fasthttp.StatusConflict
	default:
		return fasthttp.StatusInternalServerError
	}
}

func mapTenantCampaignErrorStatus(err error) int {
	if err == nil {
		return fasthttp.StatusInternalServerError
	}
	switch {
	case errors.Is(err, service.ErrCampaignConflict):
		return fasthttp.StatusConflict
	case errors.Is(err, service.ErrCampaignChannelCapabilityMismatch):
		return fasthttp.StatusUnprocessableEntity
	case errors.Is(err, service.ErrCampaignChannelInactive):
		return fasthttp.StatusConflict
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "channel_id is required"),
		strings.Contains(msg, "tenant_id is required"),
		strings.Contains(msg, "invalid campaign offer mapping"),
		strings.Contains(msg, "invalid campaign channel binding"):
		return fasthttp.StatusBadRequest
	case strings.Contains(msg, "campaign not found"):
		return fasthttp.StatusNotFound
	default:
		return fasthttp.StatusInternalServerError
	}
}

func (h *CampaignHandler) currentCampaignTenantFromRequest(ctx *fasthttp.RequestCtx) (*domain.AdminTenant, tenantctx.Identity, bool) {
	identity, ok := tenantIdentityFromRequest(ctx)
	if !ok {
		ctx.Error("Tenant context required", fasthttp.StatusForbidden)
		return nil, tenantctx.Identity{}, false
	}
	if h.tenantResolver != nil {
		tenant, err := h.tenantResolver.ResolveCurrentTenant(identity)
		if err != nil || tenant == nil || strings.TrimSpace(tenant.ID) == "" {
			ctx.Error("Tenant context required", fasthttp.StatusForbidden)
			return nil, tenantctx.Identity{}, false
		}
		return tenant, identity, true
	}
	if strings.TrimSpace(identity.TenantID) == "" {
		ctx.Error("Tenant context required", fasthttp.StatusForbidden)
		return nil, tenantctx.Identity{}, false
	}
	return &domain.AdminTenant{ID: strings.TrimSpace(identity.TenantID), TenantKey: strings.TrimSpace(identity.TenantKey), Status: domain.TenantStatusActive}, identity, true
}

func normalizedOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func isCampaignConfigValidationError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "invalid campaign offer mapping") ||
		strings.Contains(msg, "offer_product_id") ||
		strings.Contains(msg, "pricepoint_id")
}

func (h *CampaignHandler) AdminPresignBackgroundUpload(ctx *fasthttp.RequestCtx) {
	if h.assetService == nil || !h.assetService.Enabled() {
		ctx.Error("Campaign asset upload is not configured", fasthttp.StatusNotImplemented)
		return
	}
	identity, ok := tenantIdentityFromRequest(ctx)
	if !ok || !identity.HasTenant() {
		ctx.Error("Tenant context required", fasthttp.StatusForbidden)
		return
	}

	var req adminPresignBackgroundUploadRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid request body", fasthttp.StatusBadRequest)
		return
	}

	slug := strings.TrimSpace(req.CampaignSlug)
	if slug == "" {
		ctx.Error("campaign_slug is required", fasthttp.StatusBadRequest)
		return
	}
	if !slugRe.MatchString(slug) {
		ctx.Error(fmt.Sprintf("campaign_slug must match %s", slugRe.String()), fasthttp.StatusBadRequest)
		return
	}

	resp, err := h.assetService.PresignBackgroundUpload(context.Background(), service.CampaignAssetUploadRequest{
		TenantNamespace: assetTenantNamespace(identity),
		CampaignSlug:    slug,
		FileName:        req.FileName,
		ContentType:     req.ContentType,
		SizeBytes:       req.SizeBytes,
	})
	if err != nil {
		if errors.Is(err, service.ErrCampaignAssetStorageUnavailable) {
			ctx.Error("Campaign asset storage unavailable", fasthttp.StatusServiceUnavailable)
			return
		}
		ctx.Error(err.Error(), fasthttp.StatusBadRequest)
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, resp)
}

func assetTenantNamespace(identity tenantctx.Identity) string {
	if v := strings.TrimSpace(identity.TenantID); v != "" {
		return v
	}
	return strings.TrimSpace(identity.TenantKey)
}

// AdminGetPostbackRules handles GET /v1/admin/campaigns/:slug/postback-rules
func (h *CampaignHandler) AdminGetPostbackRules(ctx *fasthttp.RequestCtx) {
	path := string(ctx.Path())
	slug, ok := extractCampaignSlugBeforeSuffix(path, "/postback-rules")
	if !ok {
		ctx.Error("Invalid path", fasthttp.StatusBadRequest)
		return
	}

	rules, err := h.service.AdminGetPostbackRules(slug)
	if err != nil {
		h.logger.Error("Failed to get postback rules", zap.String("slug", slug), zap.Error(err))
		if strings.Contains(err.Error(), "campaign not found") {
			ctx.Error("Campaign not found", fasthttp.StatusNotFound)
		} else {
			ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
		}
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.Write(rules)
}

// AdminUpdatePostbackRules handles PUT /v1/admin/campaigns/:slug/postback-rules
func (h *CampaignHandler) AdminUpdatePostbackRules(ctx *fasthttp.RequestCtx) {
	path := string(ctx.Path())
	slug, ok := extractCampaignSlugBeforeSuffix(path, "/postback-rules")
	if !ok {
		ctx.Error("Invalid path", fasthttp.StatusBadRequest)
		return
	}

	body := ctx.PostBody()
	if len(body) == 0 {
		ctx.Error("Request body is required", fasthttp.StatusBadRequest)
		return
	}

	// Validate it's valid JSON
	if !json.Valid(body) {
		ctx.Error("Invalid JSON body", fasthttp.StatusBadRequest)
		return
	}

	if err := h.service.AdminUpdatePostbackRules(slug, body); err != nil {
		h.logger.Error("Failed to update postback rules", zap.String("slug", slug), zap.Error(err))
		if strings.Contains(err.Error(), "invalid postback_rules") {
			ctx.Error(err.Error(), fasthttp.StatusBadRequest)
		} else if strings.Contains(err.Error(), "campaign not found") {
			ctx.Error("Campaign not found", fasthttp.StatusNotFound)
		} else {
			ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
		}
		return
	}

	// Return the updated rules
	rules, err := h.service.AdminGetPostbackRules(slug)
	if err != nil {
		h.logger.Error("Failed to fetch updated postback rules", zap.String("slug", slug), zap.Error(err))
		ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.Write(rules)
}
