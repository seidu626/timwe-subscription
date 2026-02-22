package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/lib/pq"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/valyala/fasthttp"
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
