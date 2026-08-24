package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/service"
	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

func TestValidateTrackingConfig(t *testing.T) {
	t.Run("accepts empty or null", func(t *testing.T) {
		if err := validateTrackingConfig(nil); err != nil {
			t.Fatalf("expected nil error for empty config, got %v", err)
		}
		if err := validateTrackingConfig([]byte("null")); err != nil {
			t.Fatalf("expected nil error for null config, got %v", err)
		}
	})

	t.Run("accepts valid schema", func(t *testing.T) {
		raw := []byte(`{
			"pixels": {
				"facebook": {"pixel_id": "fb-123", "enabled": true},
				"google": {"measurement_id": "G-123", "ads_id": "AW-123", "enabled": false},
				"tiktok": {"pixel_id": "tt-456", "enabled": true}
			},
			"attribution": {"model": "last_touch", "window_days": 7},
			"visual": {"background_image_url": "https://cdn.example.com/bg.png", "theme_color": "#FFCC00"},
			"redirect_url": "https://partner.example.com/subscribe",
			"redirect": {"url": "https://partner.example.com/subscribe-alt"},
			"custom_events": [{"name": "signup", "trigger": "submit"}]
		}`)
		if err := validateTrackingConfig(raw); err != nil {
			t.Fatalf("expected nil error for valid config, got %v", err)
		}
	})

	t.Run("rejects unknown fields", func(t *testing.T) {
		raw := []byte(`{
			"pixels": {
				"facebook": {"pixel_id": "fb-123", "enabled": true, "extra": "nope"}
			}
		}`)
		if err := validateTrackingConfig(raw); err == nil {
			t.Fatal("expected error for unknown fields")
		}
	})

	t.Run("rejects missing required fields", func(t *testing.T) {
		raw := []byte(`{
			"pixels": {
				"facebook": {"pixel_id": "fb-123"}
			}
		}`)
		if err := validateTrackingConfig(raw); err == nil {
			t.Fatal("expected error for missing required fields")
		}
	})

	t.Run("rejects invalid attribution", func(t *testing.T) {
		raw := []byte(`{
			"attribution": {"model": "unknown", "window_days": 7}
		}`)
		if err := validateTrackingConfig(raw); err == nil {
			t.Fatal("expected error for invalid attribution model")
		}
	})

	t.Run("rejects invalid visual theme color", func(t *testing.T) {
		raw := []byte(`{
			"visual": {"theme_color": "yellow"}
		}`)
		if err := validateTrackingConfig(raw); err == nil {
			t.Fatal("expected error for invalid theme color")
		}
	})

	t.Run("rejects invalid visual background URL", func(t *testing.T) {
		raw := []byte(`{
			"visual": {"background_image_url": "javascript:alert(1)"}
		}`)
		if err := validateTrackingConfig(raw); err == nil {
			t.Fatal("expected error for invalid background URL")
		}
	})

	t.Run("rejects invalid redirect url", func(t *testing.T) {
		raw := []byte(`{
			"redirect_url": "javascript:alert(1)"
		}`)
		if err := validateTrackingConfig(raw); err == nil {
			t.Fatal("expected error for invalid redirect_url")
		}
	})

	t.Run("rejects missing redirect.url value", func(t *testing.T) {
		raw := []byte(`{
			"redirect": {}
		}`)
		if err := validateTrackingConfig(raw); err == nil {
			t.Fatal("expected error for missing redirect.url")
		}
	})
}

func TestNormalizeAndValidateLPCopy(t *testing.T) {
	t.Run("defaults when empty", func(t *testing.T) {
		normalized, err := normalizeAndValidateLPCopy(nil)
		if err != nil {
			t.Fatalf("expected nil error for empty lp_copy, got %v", err)
		}

		var payload map[string]any
		if err := json.Unmarshal(normalized, &payload); err != nil {
			t.Fatalf("expected valid normalized JSON, got %v", err)
		}
		if _, ok := payload["en"]; !ok {
			t.Fatal("expected normalized payload to contain en")
		}
	})

	t.Run("preserves empty object for landing-page-less campaigns", func(t *testing.T) {
		normalized, err := normalizeAndValidateLPCopy([]byte(` {} `))
		if err != nil {
			t.Fatalf("expected empty lp_copy object to be accepted, got %v", err)
		}
		if string(normalized) != "{}" {
			t.Fatalf("expected empty object to be preserved, got %s", normalized)
		}
	})

	t.Run("rejects missing en block", func(t *testing.T) {
		raw := []byte(`{"ar":{"heroTitle":"x"}}`)
		_, err := normalizeAndValidateLPCopy(raw)
		if err == nil || !strings.Contains(err.Error(), "lp_copy.en is required") {
			t.Fatalf("expected lp_copy.en required error, got %v", err)
		}
	})

	t.Run("rejects missing required fields", func(t *testing.T) {
		raw := []byte(`{"en":{"heroTitle":"Hello"}}`)
		_, err := normalizeAndValidateLPCopy(raw)
		if err == nil || !strings.Contains(err.Error(), "lp_copy.en.heDescription is required") {
			t.Fatalf("expected missing field error, got %v", err)
		}
	})

	t.Run("rejects unknown fields", func(t *testing.T) {
		raw := []byte(`{"en":{"heroTitle":"A","heDescription":"B","heCta":"C","heModalTitle":"D","heModalConfirm":"E","msisdnDescription":"F","msisdnPlaceholder":"G","msisdnCta":"H","otpDescription":"I","otpPlaceholder":"J","otpCta":"K","successTitle":"L","successBody":"M","consentPrefix":"N","consentTerms":"O","termsHeading":"P","legal":"Q","phoneRequired":"R","phoneInvalid":"S","otpInvalid":"T","consentRequired":"U","unexpected":"bad"}}`)
		_, err := normalizeAndValidateLPCopy(raw)
		if err == nil || !strings.Contains(err.Error(), "lp_copy:") {
			t.Fatalf("expected unknown field error, got %v", err)
		}
	})

	t.Run("accepts valid lp copy", func(t *testing.T) {
		raw := []byte(`{
			"en": {
				"heroTitle": "A",
				"heDescription": "B",
				"heCta": "C",
				"heModalTitle": "D",
				"heModalConfirm": "E",
				"msisdnDescription": "F",
				"msisdnPlaceholder": "G",
				"msisdnCta": "H",
				"otpDescription": "I",
				"otpPlaceholder": "J",
				"otpCta": "K",
				"successTitle": "L",
				"successBody": "M",
				"consentPrefix": "N",
				"consentTerms": "O",
				"termsHeading": "P",
				"legal": "Q",
				"phoneRequired": "R",
				"phoneInvalid": "S",
				"otpInvalid": "T",
				"consentRequired": "U"
			}
		}`)
		if _, err := normalizeAndValidateLPCopy(raw); err != nil {
			t.Fatalf("expected valid lp_copy, got %v", err)
		}
	})
}

func TestValidateAdminUpsert_RedirectFlowRequiresDestination(t *testing.T) {
	req := &adminCampaignUpsertRequest{
		Slug:           "gh-redirect",
		Language:       "en",
		Country:        "GH",
		OfferProductID: 1001,
		FlowType:       domain.FlowTypeRedirect,
		LPCopy:         nil,
	}

	err := validateAdminUpsert(req, true)
	if err == nil || !strings.Contains(err.Error(), "redirect flow requires a valid destination") {
		t.Fatalf("expected redirect destination validation error, got %v", err)
	}

	req.TrackingConfig = []byte(`{"redirect_url":"https://partner.example.com/subscribe"}`)
	if err := validateAdminUpsert(req, true); err != nil {
		t.Fatalf("expected valid redirect config, got %v", err)
	}
}

func TestAdminPresignBackgroundUploadRequiresTenantContext(t *testing.T) {
	h := newCampaignAssetTestHandler(t)
	var ctx fasthttp.RequestCtx
	ctx.Request.SetBodyString(`{"campaign_slug":"gh-campaign","file_name":"background.png","content_type":"image/png","size_bytes":1024}`)

	h.AdminPresignBackgroundUpload(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusForbidden {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	if !strings.Contains(string(ctx.Response.Body()), "Tenant context required") {
		t.Fatalf("expected tenant context error, got %q", ctx.Response.Body())
	}
}

func TestAdminListCampaignsRequiresTenantContext(t *testing.T) {
	h := NewCampaignHandler(nil, nil, zap.NewNop())
	var ctx fasthttp.RequestCtx
	ctx.Request.SetRequestURI("/v1/admin/campaigns")

	h.AdminList(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusForbidden {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
}

func TestExtractTenantAndCampaignSlugFromPath(t *testing.T) {
	tenantKey, slug, ok := extractTenantAndCampaignSlugFromPath("/v1/campaigns/tenant-a/daily")
	if !ok || tenantKey != "tenant-a" || slug != "daily" {
		t.Fatalf("unexpected parse result: tenant=%q slug=%q ok=%v", tenantKey, slug, ok)
	}

	if _, _, ok := extractTenantAndCampaignSlugFromPath("/v1/campaigns/daily"); ok {
		t.Fatal("legacy campaign route must not parse as tenant route")
	}
}

// stubCampaignRepo embeds the CampaignRepo interface so tests only implement
// the methods a code path exercises; calling anything else panics loudly.
type stubCampaignRepo struct {
	service.CampaignRepo
	getEnabledBySlugFn func(string) (*domain.Campaign, error)
}

func (s *stubCampaignRepo) GetEnabledBySlug(slug string) (*domain.Campaign, error) {
	return s.getEnabledBySlugFn(slug)
}

// The public single-segment slug route (/v1/campaigns/{slug}) intentionally
// requires no tenant headers: the owning tenant is resolved server-side from
// the globally-unique slug and returned as tenant_key so the landing page can
// drive the tenant-scoped opt-in (see landing-web campaign-aliases.ts).
// Unknown slugs must surface as 404, never as a tenant-context error.
func TestPublicCampaignSlugRouteResolvesTenantServerSide(t *testing.T) {
	tenantKey := "tenant-a"
	repo := &stubCampaignRepo{
		getEnabledBySlugFn: func(slug string) (*domain.Campaign, error) {
			if slug != "daily" {
				return nil, errors.New("campaign not found: " + slug)
			}
			return &domain.Campaign{Slug: slug, Enabled: true, TenantKey: &tenantKey}, nil
		},
	}
	h := NewCampaignHandler(service.NewCampaignService(repo, zap.NewNop()), nil, zap.NewNop())

	t.Run("known slug resolves without tenant headers", func(t *testing.T) {
		var ctx fasthttp.RequestCtx
		ctx.Request.SetRequestURI("/v1/campaigns/daily")
		ctx.Request.Header.SetMethod(fasthttp.MethodGet)

		h.GetBySlug(&ctx)

		if ctx.Response.StatusCode() != fasthttp.StatusOK {
			t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
		}
		var got domain.PublicCampaign
		if err := json.Unmarshal(ctx.Response.Body(), &got); err != nil {
			t.Fatalf("invalid JSON response %q: %v", ctx.Response.Body(), err)
		}
		if got.Slug != "daily" || got.TenantKey == nil || *got.TenantKey != tenantKey {
			t.Fatalf("expected slug=daily tenant_key=%q, got %+v", tenantKey, got)
		}
	})

	t.Run("unknown slug returns 404", func(t *testing.T) {
		var ctx fasthttp.RequestCtx
		ctx.Request.SetRequestURI("/v1/campaigns/nope")
		ctx.Request.Header.SetMethod(fasthttp.MethodGet)

		h.GetBySlug(&ctx)

		if ctx.Response.StatusCode() != fasthttp.StatusNotFound {
			t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
		}
		if !strings.Contains(string(ctx.Response.Body()), "Campaign not found") {
			t.Fatalf("expected not-found error, got %q", ctx.Response.Body())
		}
	})
}

func TestPublicCampaignListRequiresTenantContext(t *testing.T) {
	h := NewCampaignHandler(nil, nil, zap.NewNop())
	var ctx fasthttp.RequestCtx
	ctx.Request.SetRequestURI("/v1/campaigns")
	ctx.Request.Header.SetMethod(fasthttp.MethodGet)

	h.ListEnabled(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusForbidden {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	if !strings.Contains(string(ctx.Response.Body()), "Tenant context required") {
		t.Fatalf("expected tenant context error, got %q", ctx.Response.Body())
	}
}

func TestTrustedPublicTenantIdentityRequiresSignature(t *testing.T) {
	t.Setenv("TENANT_TRUSTED_HEADER_SECRET", "secret")
	var ctx fasthttp.RequestCtx
	ctx.Request.SetRequestURI("/v1/campaigns/daily")
	ctx.Request.Header.SetMethod(fasthttp.MethodGet)
	ctx.Request.Header.Set(tenantctx.HeaderTenantKey, "tenant-a")
	ctx.Request.Header.Set(tenantctx.HeaderServiceID, "gateway")

	if _, err := trustedPublicTenantIdentityFromRequest(&ctx); err == nil {
		t.Fatal("expected unsigned tenant headers to be rejected")
	}
}

func TestTrustedPublicTenantIdentityAcceptsSignedTenantKey(t *testing.T) {
	t.Setenv("TENANT_TRUSTED_HEADER_SECRET", "secret")
	now := time.Now().UTC().Format(time.RFC3339)

	var ctx fasthttp.RequestCtx
	ctx.Request.SetRequestURI("/v1/campaigns/daily")
	ctx.Request.Header.SetMethod(fasthttp.MethodGet)
	ctx.Request.Header.Set(tenantctx.HeaderTenantKey, "tenant-a")
	ctx.Request.Header.Set(tenantctx.HeaderServiceID, "gateway")
	ctx.Request.Header.Set(tenantctx.HeaderServiceTimestamp, now)
	ctx.Request.Header.Set(tenantctx.HeaderServiceSignature, tenantctx.SignServiceRequest("secret", tenantctx.SignInput{
		Method:    fasthttp.MethodGet,
		Path:      "/v1/campaigns/daily",
		Timestamp: now,
		ServiceID: "gateway",
		TenantKey: "tenant-a",
	}))

	identity, err := trustedPublicTenantIdentityFromRequest(&ctx)
	if err != nil {
		t.Fatalf("expected signed tenant headers to pass, got %v", err)
	}
	if identity.TenantKey != "tenant-a" || identity.TrustSource != tenantctx.TrustSourceTrustedService {
		t.Fatalf("unexpected identity: %#v", identity)
	}
}

func TestAdminPresignBackgroundUploadRejectsPathTraversalFileName(t *testing.T) {
	h := newCampaignAssetTestHandler(t)
	var ctx fasthttp.RequestCtx
	ctx.SetUserValue(tenantctx.FastHTTPUserValueKey, tenantctx.Identity{
		TenantID:    "22222222-2222-2222-2222-222222222222",
		TenantKey:   "tenant-a",
		Subject:     "auth0|tenant-admin",
		TrustSource: tenantctx.TrustSourceJWT,
	})
	ctx.Request.SetBodyString(`{"campaign_slug":"gh-campaign","file_name":"../background.png","content_type":"image/png","size_bytes":1024}`)

	h.AdminPresignBackgroundUpload(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusBadRequest {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	if !strings.Contains(string(ctx.Response.Body()), "simple file name") {
		t.Fatalf("expected file name validation error, got %q", ctx.Response.Body())
	}
}

func TestValidateCloneCampaignRequest(t *testing.T) {
	t.Run("valid request", func(t *testing.T) {
		req := &adminCloneCampaignRequest{NewSlug: "gh-new-campaign-v2"}
		if err := validateCloneCampaignRequest("gh-source-campaign", req); err != nil {
			t.Fatalf("expected valid clone request, got %v", err)
		}
	})

	t.Run("missing new slug", func(t *testing.T) {
		req := &adminCloneCampaignRequest{NewSlug: "   "}
		err := validateCloneCampaignRequest("gh-source-campaign", req)
		if err == nil || !strings.Contains(err.Error(), "new_slug is required") {
			t.Fatalf("expected new_slug required error, got %v", err)
		}
	})

	t.Run("same slug as source", func(t *testing.T) {
		req := &adminCloneCampaignRequest{NewSlug: "gh-source-campaign"}
		err := validateCloneCampaignRequest("gh-source-campaign", req)
		if err == nil || !strings.Contains(err.Error(), "must be different from source slug") {
			t.Fatalf("expected same slug validation error, got %v", err)
		}
	})
}

func newCampaignAssetTestHandler(t *testing.T) *CampaignHandler {
	t.Helper()

	assetSvc, err := service.NewCampaignAssetService(service.CampaignAssetStorageConfig{
		Enabled:            true,
		Endpoint:           "s3.example.com",
		Bucket:             "campaign-assets",
		AccessKeyID:        "access-key",
		SecretAccessKey:    "secret-key",
		PublicBaseURL:      "https://cdn.example.com/assets",
		MaxUploadSizeBytes: 2 * 1024 * 1024,
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create campaign asset service: %v", err)
	}
	return NewCampaignHandler(nil, assetSvc, zap.NewNop())
}

func TestMapCampaignCloneErrorStatus(t *testing.T) {
	t.Run("maps unique violation to conflict", func(t *testing.T) {
		err := fmt.Errorf("wrap: %w", &pq.Error{Code: "23505"})
		if got := mapCampaignCloneErrorStatus(err); got != fasthttp.StatusConflict {
			t.Fatalf("expected %d, got %d", fasthttp.StatusConflict, got)
		}
	})

	t.Run("maps source not found to not found", func(t *testing.T) {
		err := errors.New("failed to get source campaign: campaign not found: gh-source")
		if got := mapCampaignCloneErrorStatus(err); got != fasthttp.StatusNotFound {
			t.Fatalf("expected %d, got %d", fasthttp.StatusNotFound, got)
		}
	})
}

func TestValidateAdminUpsert_AppCategoryLengthCap(t *testing.T) {
	tooLong := strings.Repeat("x", 81)
	atCap := strings.Repeat("x", 80)
	req := &adminCampaignUpsertRequest{
		Slug:           "gh-app-campaign",
		Language:       "en",
		Country:        "GH",
		OfferProductID: 1001,
		FlowType:       domain.FlowTypeOTP,
		AppCategory:    &tooLong,
	}

	err := validateAdminUpsert(req, true)
	if err == nil || !strings.Contains(err.Error(), "app_category") {
		t.Fatalf("expected app_category length error, got %v", err)
	}

	req.AppCategory = &atCap
	if err := validateAdminUpsert(req, true); err != nil {
		t.Fatalf("expected 80-char app_category to pass, got %v", err)
	}

	req.AppCategory = nil
	if err := validateAdminUpsert(req, true); err != nil {
		t.Fatalf("expected nil app_category to pass, got %v", err)
	}
}

func TestAdminPresignArtworkUploadRequiresTenantContext(t *testing.T) {
	h := newCampaignAssetTestHandler(t)
	var ctx fasthttp.RequestCtx
	ctx.Request.SetBodyString(`{"campaign_slug":"gh-campaign","file_name":"artwork.png","content_type":"image/png","size_bytes":1024}`)

	h.AdminPresignArtworkUpload(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusForbidden {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	if !strings.Contains(string(ctx.Response.Body()), "Tenant context required") {
		t.Fatalf("expected tenant context error, got %q", ctx.Response.Body())
	}
}
