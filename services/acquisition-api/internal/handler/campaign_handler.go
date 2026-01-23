package handler

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/service"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// CampaignHandler handles campaign-related HTTP requests
type CampaignHandler struct {
	service *service.CampaignService
	logger  *zap.Logger
}

// NewCampaignHandler creates a new campaign handler
func NewCampaignHandler(campaignService *service.CampaignService, logger *zap.Logger) *CampaignHandler {
	return &CampaignHandler{
		service: campaignService,
		logger:  logger,
	}
}

var slugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

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
	
	campaign, err := h.service.GetBySlug(slug)
	if err != nil {
		h.logger.Error("Failed to get campaign", zap.String("slug", slug), zap.Error(err))
		ctx.Error("Campaign not found", fasthttp.StatusNotFound)
		return
	}
	
	writeJSON(ctx, fasthttp.StatusOK, campaign)
}

// ListEnabled handles GET /v1/campaigns
func (h *CampaignHandler) ListEnabled(ctx *fasthttp.RequestCtx) {
	campaigns, err := h.service.ListEnabled()
	if err != nil {
		h.logger.Error("Failed to list campaigns", zap.Error(err))
		ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
		return
	}
	
	writeJSON(ctx, fasthttp.StatusOK, map[string]interface{}{
		"campaigns": campaigns,
	})
}

type adminCampaignUpsertRequest struct {
	Slug              string          `json:"slug"`
	Language          string          `json:"language"`
	Country           string          `json:"country"`
	Operator          *string         `json:"operator,omitempty"`
	OfferProductID    int             `json:"offer_product_id"`
	PricepointID      *int            `json:"pricepoint_id,omitempty"`
	PartnerRoleID     *int            `json:"partner_role_id,omitempty"`
	FlowType          domain.FlowType `json:"flow_type"`
	ShortCode         *string         `json:"short_code,omitempty"`
	SMSKeyword        *string         `json:"sms_keyword,omitempty"`
	Price             *float64        `json:"price,omitempty"`
	BillingCycle      *string         `json:"billing_cycle,omitempty"`
	TrialFlags        json.RawMessage `json:"trial_flags,omitempty"`
	TermsURL          *string         `json:"terms_url,omitempty"`
	InlineTermsText   *string         `json:"inline_terms_text,omitempty"`
	ConsentRequired   bool            `json:"consent_required"`
	ConsentVersion    *string         `json:"consent_version,omitempty"`
	AttributionMapping json.RawMessage `json:"attribution_mapping,omitempty"`
	PostbackRules      json.RawMessage `json:"postback_rules,omitempty"`
	Throttles          json.RawMessage `json:"throttles,omitempty"`
	AllowedReferrers   []string        `json:"allowed_referrers,omitempty"`
	AllowedSources     []string        `json:"allowed_sources,omitempty"`
	LandingPageURLs    []string        `json:"landing_page_urls,omitempty"`
	TrackingConfig     json.RawMessage `json:"tracking_config,omitempty"`
	Enabled           bool            `json:"enabled"`
	CreatedBy         *string         `json:"created_by,omitempty"`
	UpdatedBy         *string         `json:"updated_by,omitempty"`
}

type adminSetEnabledRequest struct {
	Enabled   bool    `json:"enabled"`
	UpdatedBy *string `json:"updated_by,omitempty"`
}

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

func (h *CampaignHandler) AdminGetBySlug(ctx *fasthttp.RequestCtx) {
	path := string(ctx.Path())
	slug, ok := extractCampaignSlugFromPath(path)
	if !ok {
		ctx.Error("Invalid path", fasthttp.StatusBadRequest)
		return
	}

	campaign, err := h.service.AdminGetBySlug(slug)
	if err != nil {
		h.logger.Error("Failed to get campaign (admin)", zap.String("slug", slug), zap.Error(err))
		ctx.Error("Campaign not found", fasthttp.StatusNotFound)
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, campaign)
}

func (h *CampaignHandler) AdminList(ctx *fasthttp.RequestCtx) {
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

	campaigns, err := h.service.AdminList(enabled, country)
	if err != nil {
		h.logger.Error("Failed to list campaigns (admin)", zap.Error(err))
		ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, map[string]any{"campaigns": campaigns})
}

func (h *CampaignHandler) AdminCreate(ctx *fasthttp.RequestCtx) {
	var req adminCampaignUpsertRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Invalid request body", fasthttp.StatusBadRequest)
		return
	}
	if err := validateAdminUpsert(&req, true); err != nil {
		ctx.Error(err.Error(), fasthttp.StatusBadRequest)
		return
	}

	created, err := h.service.AdminCreate(&domain.Campaign{
		Slug:               req.Slug,
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
		Enabled:            req.Enabled,
		CreatedBy:          req.CreatedBy,
		UpdatedBy:          req.UpdatedBy,
	})
	if err != nil {
		h.logger.Error("Failed to create campaign (admin)", zap.String("slug", req.Slug), zap.Error(err))
		ctx.Error("Failed to create campaign", fasthttp.StatusInternalServerError)
		return
	}

	writeJSON(ctx, fasthttp.StatusCreated, created)
}

func (h *CampaignHandler) AdminUpdate(ctx *fasthttp.RequestCtx) {
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

	updated, err := h.service.AdminUpdate(slug, &domain.Campaign{
		Slug:               slug,
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
		Enabled:            req.Enabled,
		UpdatedBy:          req.UpdatedBy,
	})
	if err != nil {
		h.logger.Error("Failed to update campaign (admin)", zap.String("slug", slug), zap.Error(err))
		ctx.Error("Failed to update campaign", fasthttp.StatusInternalServerError)
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, updated)
}

func (h *CampaignHandler) AdminSetEnabled(ctx *fasthttp.RequestCtx) {
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

	updated, err := h.service.AdminSetEnabled(slug, req.Enabled, req.UpdatedBy)
	if err != nil {
		h.logger.Error("Failed to set enabled (admin)", zap.String("slug", slug), zap.Error(err))
		ctx.Error("Failed to update campaign", fasthttp.StatusInternalServerError)
		return
	}

	writeJSON(ctx, fasthttp.StatusOK, updated)
}
