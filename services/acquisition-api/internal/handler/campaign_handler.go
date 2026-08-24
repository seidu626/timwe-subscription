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
		writeJSONError(ctx, fasthttp.StatusBadRequest, "Invalid path")
		return
	}

	if hasPublicTenantHeaders(ctx) {
		identity, err := trustedPublicTenantIdentityFromRequest(ctx)
		if err != nil || strings.TrimSpace(identity.TenantKey) == "" {
			writeJSONError(ctx, fasthttp.StatusForbidden, "Tenant context invalid")
			return
		}
		campaign, err := h.service.GetByTenantKeyAndSlug(identity.TenantKey, slug)
		if err != nil {
			h.logger.Error("Failed to get trusted tenant campaign", zap.String("tenant_key", identity.TenantKey), zap.String("slug", slug), zap.Error(err))
			writeJSONError(ctx, fasthttp.StatusNotFound, "Campaign not found")
			return
		}
		writeJSON(ctx, fasthttp.StatusOK, campaign)
		return
	}

	// Public single-segment slug URL (/lp/{slug}): resolve the owning tenant
	// server-side from the globally-unique slug instead of requiring a tenant in
	// the path or a hardcoded alias map. The response carries tenant_key so the
	// landing page can drive the tenant-scoped opt-in.
	campaign, err := h.service.GetEnabledBySlug(slug)
	if err != nil {
		h.logger.Warn("Failed to resolve public campaign by slug", zap.String("slug", slug), zap.Error(err))
		writeJSONError(ctx, fasthttp.StatusNotFound, "Campaign not found")
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, campaign)
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
		writeJSONError(ctx, fasthttp.StatusBadRequest, "Invalid path")
		return
	}
	if !slugRe.MatchString(tenantKey) || !slugRe.MatchString(slug) {
		writeJSONError(ctx, fasthttp.StatusBadRequest, "Invalid tenant or campaign slug")
		return
	}

	campaign, err := h.service.GetByTenantKeyAndSlug(tenantKey, slug)
	if err != nil {
		h.logger.Error("Failed to get tenant campaign", zap.String("tenant_key", tenantKey), zap.String("slug", slug), zap.Error(err))
		writeJSONError(ctx, fasthttp.StatusNotFound, "Campaign not found")
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, campaign)
}

// ListEnabled handles GET /v1/campaigns
func (h *CampaignHandler) ListEnabled(ctx *fasthttp.RequestCtx) {
	writeJSONError(ctx, fasthttp.StatusForbidden, "Tenant context required")
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
	AppName            *string         `json:"app_name,omitempty"`
	AppTagline         *string         `json:"app_tagline,omitempty"`
	AppDescription     *string         `json:"app_description,omitempty"`
	AppCategory        *string         `json:"app_category,omitempty"`
	AppArtworkURL      *string         `json:"app_artwork_url,omitempty"`
	AppSampleContent   *string         `json:"app_sample_content,omitempty"`
	AppFeaturedRank    *int            `json:"app_featured_rank,omitempty"`
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

type adminPresignTenantBrandingUploadRequest struct {
	Kind        string `json:"kind"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
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
	// Double opt-in confirm step. Optional so campaigns authored before the
	// mode existed stay valid; the landing page falls back to default copy.
	ConfirmDescription string `json:"confirmDescription,omitempty"`
	ConfirmCTA         string `json:"confirmCta,omitempty"`
	SuccessTitle       string `json:"successTitle"`
	SuccessBody        string `json:"successBody"`
	ConsentPrefix      string `json:"consentPrefix"`
	ConsentTerms       string `json:"consentTerms"`
	TermsHeading       string `json:"termsHeading"`
	Legal              string `json:"legal"`
	PhoneRequired      string `json:"phoneRequired"`
	PhoneInvalid       string `json:"phoneInvalid"`
	OTPInvalid         string `json:"otpInvalid"`
	ConsentRequired    string `json:"consentRequired"`
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
	case domain.FlowTypeOTP, domain.FlowTypeRedirect, domain.FlowTypeMixed,
		domain.FlowTypeDoubleOptin, domain.FlowTypeAuto:
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
	if req.AppCategory != nil && len([]rune(strings.TrimSpace(*req.AppCategory))) > 80 {
		return fmt.Errorf("app_category must be 80 characters or fewer")
	}
	if req.AppFeaturedRank != nil && *req.AppFeaturedRank < 0 {
		return fmt.Errorf("app_featured_rank must be zero or greater")
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
	// Synthetic partner campaigns (see transaction_service.autoProvisionCampaign)
	// store an empty object because they have no landing page. Treat that as
	// "no copy configured" so those rows stay editable from the admin UI.
	if bytes.Equal(trimmed, []byte("{}")) {
		return json.RawMessage(`{}`), nil
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
		writeJSONError(ctx, fasthttp.StatusBadRequest, "Invalid path")
		return
	}

	tenant, _, ok := h.currentCampaignTenantFromRequest(ctx)
	if !ok {
		return
	}

	campaign, err := h.service.AdminGetByTenantAndSlug(tenant.ID, slug)
	if err != nil {
		h.logger.Error("Failed to get campaign (admin)", zap.String("slug", slug), zap.Error(err))
		writeJSONError(ctx, fasthttp.StatusNotFound, "Campaign not found")
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
		writeJSONError(ctx, fasthttp.StatusBadRequest, "Invalid query")
		return
	}

	var enabled *bool
	if v := u.Query().Get("enabled"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			writeJSONError(ctx, fasthttp.StatusBadRequest, "enabled must be true or false")
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
		writeJSONError(ctx, fasthttp.StatusInternalServerError, "Internal server error")
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
		writeJSONError(ctx, fasthttp.StatusBadRequest, "Invalid request body")
		return
	}
	if err := validateAdminUpsert(&req, true); err != nil {
		writeJSONError(ctx, fasthttp.StatusBadRequest, err.Error())
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
		AppName:            req.AppName,
		AppTagline:         req.AppTagline,
		AppDescription:     req.AppDescription,
		AppCategory:        req.AppCategory,
		AppArtworkURL:      req.AppArtworkURL,
		AppSampleContent:   req.AppSampleContent,
		AppFeaturedRank:    req.AppFeaturedRank,
		Enabled:            req.Enabled,
		CreatedBy:          req.CreatedBy,
		UpdatedBy:          req.UpdatedBy,
	})
	if err != nil {
		if status := mapTenantCampaignErrorStatus(err); status != fasthttp.StatusInternalServerError {
			writeJSONError(ctx, status, err.Error())
			return
		}
		if isCampaignConfigValidationError(err) {
			writeJSONError(ctx, fasthttp.StatusBadRequest, err.Error())
			return
		}
		h.logger.Error("Failed to create campaign (admin)", zap.String("slug", req.Slug), zap.Error(err))
		writeJSONError(ctx, fasthttp.StatusInternalServerError, "Failed to create campaign")
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
		writeJSONError(ctx, fasthttp.StatusBadRequest, "Invalid path")
		return
	}

	var req adminCampaignUpsertRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeJSONError(ctx, fasthttp.StatusBadRequest, "Invalid request body")
		return
	}
	// For update, slug comes from path; ignore any body slug.
	if err := validateAdminUpsert(&req, false); err != nil {
		writeJSONError(ctx, fasthttp.StatusBadRequest, err.Error())
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
		AppName:            req.AppName,
		AppTagline:         req.AppTagline,
		AppDescription:     req.AppDescription,
		AppCategory:        req.AppCategory,
		AppArtworkURL:      req.AppArtworkURL,
		AppSampleContent:   req.AppSampleContent,
		AppFeaturedRank:    req.AppFeaturedRank,
		Enabled:            req.Enabled,
		UpdatedBy:          req.UpdatedBy,
	})
	if err != nil {
		if status := mapTenantCampaignErrorStatus(err); status != fasthttp.StatusInternalServerError {
			writeJSONError(ctx, status, err.Error())
			return
		}
		if isCampaignConfigValidationError(err) {
			writeJSONError(ctx, fasthttp.StatusBadRequest, err.Error())
			return
		}
		h.logger.Error("Failed to update campaign (admin)", zap.String("slug", slug), zap.Error(err))
		writeJSONError(ctx, fasthttp.StatusInternalServerError, "Failed to update campaign")
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
		writeJSONError(ctx, fasthttp.StatusBadRequest, "Invalid path")
		return
	}

	var req adminSetEnabledRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeJSONError(ctx, fasthttp.StatusBadRequest, "Invalid request body")
		return
	}

	updated, err := h.service.AdminSetEnabledForTenant(tenant.ID, slug, req.Enabled, req.UpdatedBy)
	if err != nil {
		if status := mapTenantCampaignErrorStatus(err); status != fasthttp.StatusInternalServerError {
			writeJSONError(ctx, status, err.Error())
			return
		}
		h.logger.Error("Failed to set enabled (admin)", zap.String("slug", slug), zap.Error(err))
		writeJSONError(ctx, fasthttp.StatusInternalServerError, "Failed to update campaign")
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, updated)
}

func (h *CampaignHandler) AdminClone(ctx *fasthttp.RequestCtx) {
	path := string(ctx.Path())
	sourceSlug, ok := extractCampaignSlugBeforeSuffix(path, "/clone")
	if !ok {
		writeJSONError(ctx, fasthttp.StatusBadRequest, "Invalid path")
		return
	}

	var req adminCloneCampaignRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeJSONError(ctx, fasthttp.StatusBadRequest, "Invalid request body")
		return
	}

	if err := validateCloneCampaignRequest(sourceSlug, &req); err != nil {
		writeJSONError(ctx, fasthttp.StatusBadRequest, err.Error())
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
		writeJSONError(ctx, status, err.Error())
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
		writeJSONError(ctx, fasthttp.StatusForbidden, "Tenant context required")
		return nil, tenantctx.Identity{}, false
	}
	if h.tenantResolver != nil {
		tenant, err := h.tenantResolver.ResolveCurrentTenant(identity)
		if err != nil || tenant == nil || strings.TrimSpace(tenant.ID) == "" {
			writeJSONError(ctx, fasthttp.StatusForbidden, "Tenant context required")
			return nil, tenantctx.Identity{}, false
		}
		return tenant, identity, true
	}
	if strings.TrimSpace(identity.TenantID) == "" {
		writeJSONError(ctx, fasthttp.StatusForbidden, "Tenant context required")
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
	h.adminPresignAssetUpload(ctx, "background")
}

// AdminPresignArtworkUpload handles POST /v1/admin/campaign-assets/artwork/presign.
// Same request/response shape as the background presign; objects land under
// the campaign-artwork key prefix for the app catalog's app_artwork_url.
func (h *CampaignHandler) AdminPresignArtworkUpload(ctx *fasthttp.RequestCtx) {
	h.adminPresignAssetUpload(ctx, "artwork")
}

// AdminPresignTenantBrandingUpload handles POST /v1/admin/tenant-assets/presign.
// Branding media (logo/banner) shares the campaign asset storage but lands
// under the tenant-branding key prefix, namespaced per tenant.
func (h *CampaignHandler) AdminPresignTenantBrandingUpload(ctx *fasthttp.RequestCtx) {
	if h.assetService == nil || !h.assetService.Enabled() {
		writeJSONError(ctx, fasthttp.StatusNotImplemented, "Campaign asset upload is not configured")
		return
	}
	identity, ok := tenantIdentityFromRequest(ctx)
	if !ok || !identity.HasTenant() {
		writeJSONError(ctx, fasthttp.StatusForbidden, "Tenant context required")
		return
	}

	var req adminPresignTenantBrandingUploadRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeJSONError(ctx, fasthttp.StatusBadRequest, "Invalid request body")
		return
	}

	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	switch kind {
	case "logo", "banner":
	default:
		writeJSONError(ctx, fasthttp.StatusBadRequest, "kind must be logo or banner")
		return
	}

	resp, err := h.assetService.PresignTenantBrandingUpload(context.Background(), service.CampaignAssetUploadRequest{
		TenantNamespace: assetTenantNamespace(identity),
		CampaignSlug:    kind,
		FileName:        req.FileName,
		ContentType:     req.ContentType,
		SizeBytes:       req.SizeBytes,
	})
	if err != nil {
		if errors.Is(err, service.ErrCampaignAssetStorageUnavailable) {
			writeJSONError(ctx, fasthttp.StatusServiceUnavailable, "Campaign asset storage unavailable")
			return
		}
		writeJSONError(ctx, fasthttp.StatusBadRequest, err.Error())
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, resp)
}

func (h *CampaignHandler) adminPresignAssetUpload(ctx *fasthttp.RequestCtx, kind string) {
	if h.assetService == nil || !h.assetService.Enabled() {
		writeJSONError(ctx, fasthttp.StatusNotImplemented, "Campaign asset upload is not configured")
		return
	}
	identity, ok := tenantIdentityFromRequest(ctx)
	if !ok || !identity.HasTenant() {
		writeJSONError(ctx, fasthttp.StatusForbidden, "Tenant context required")
		return
	}

	var req adminPresignBackgroundUploadRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeJSONError(ctx, fasthttp.StatusBadRequest, "Invalid request body")
		return
	}

	slug := strings.TrimSpace(req.CampaignSlug)
	if slug == "" {
		writeJSONError(ctx, fasthttp.StatusBadRequest, "campaign_slug is required")
		return
	}
	if !slugRe.MatchString(slug) {
		writeJSONError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("campaign_slug must match %s", slugRe.String()))
		return
	}

	assetReq := service.CampaignAssetUploadRequest{
		TenantNamespace: assetTenantNamespace(identity),
		CampaignSlug:    slug,
		FileName:        req.FileName,
		ContentType:     req.ContentType,
		SizeBytes:       req.SizeBytes,
	}
	var resp *service.CampaignAssetUploadResponse
	var err error
	if kind == "artwork" {
		resp, err = h.assetService.PresignArtworkUpload(context.Background(), assetReq)
	} else {
		resp, err = h.assetService.PresignBackgroundUpload(context.Background(), assetReq)
	}
	if err != nil {
		if errors.Is(err, service.ErrCampaignAssetStorageUnavailable) {
			writeJSONError(ctx, fasthttp.StatusServiceUnavailable, "Campaign asset storage unavailable")
			return
		}
		writeJSONError(ctx, fasthttp.StatusBadRequest, err.Error())
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
		writeJSONError(ctx, fasthttp.StatusBadRequest, "Invalid path")
		return
	}

	rules, err := h.service.AdminGetPostbackRules(slug)
	if err != nil {
		h.logger.Error("Failed to get postback rules", zap.String("slug", slug), zap.Error(err))
		if strings.Contains(err.Error(), "campaign not found") {
			writeJSONError(ctx, fasthttp.StatusNotFound, "Campaign not found")
		} else {
			writeJSONError(ctx, fasthttp.StatusInternalServerError, "Internal server error")
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
		writeJSONError(ctx, fasthttp.StatusBadRequest, "Invalid path")
		return
	}

	body := ctx.PostBody()
	if len(body) == 0 {
		writeJSONError(ctx, fasthttp.StatusBadRequest, "Request body is required")
		return
	}

	// Validate it's valid JSON
	if !json.Valid(body) {
		writeJSONError(ctx, fasthttp.StatusBadRequest, "Invalid JSON body")
		return
	}

	if err := h.service.AdminUpdatePostbackRules(slug, body); err != nil {
		h.logger.Error("Failed to update postback rules", zap.String("slug", slug), zap.Error(err))
		if strings.Contains(err.Error(), "invalid postback_rules") {
			writeJSONError(ctx, fasthttp.StatusBadRequest, err.Error())
		} else if strings.Contains(err.Error(), "campaign not found") {
			writeJSONError(ctx, fasthttp.StatusNotFound, "Campaign not found")
		} else {
			writeJSONError(ctx, fasthttp.StatusInternalServerError, "Internal server error")
		}
		return
	}

	// Return the updated rules
	rules, err := h.service.AdminGetPostbackRules(slug)
	if err != nil {
		h.logger.Error("Failed to fetch updated postback rules", zap.String("slug", slug), zap.Error(err))
		writeJSONError(ctx, fasthttp.StatusInternalServerError, "Internal server error")
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.Write(rules)
}
