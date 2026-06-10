// slice-harness: allow-new-canonical-path: TMP-007 tests partner tenant context enforcement.
package handler

import (
	"encoding/json"
	"testing"

	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription-external/internal/service"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

func TestPartnerMTHandlerRejectsMissingTenantContextBeforeProviderCall(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.JwtToken.Secret = "test-secret"
	h := NewPartnerHandler(zap.NewNop(), &service.SubscriptionService{}, cfg)

	body, _ := json.Marshal(map[string]interface{}{
		"productId": 14397,
		"msisdn":    "233241234567",
		"channelId": "11111111-1111-1111-1111-111111111111",
	})
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/api/external/v1/WEB/mt")
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetBody(body)

	h.PartnerMTHandler(ctx, "WEB")

	if ctx.Response.StatusCode() != fasthttp.StatusForbidden {
		t.Fatalf("expected 403 for unsigned tenant context, got %d body=%s", ctx.Response.StatusCode(), ctx.Response.Body())
	}
}

// newLegacyHandlerWithTrustRequired returns a PartnerHandler with
// GATEWAY_TRUST_REQUIRED=true so we can assert legacy handlers enforce the flag.
func newLegacyHandlerWithTrustRequired() *PartnerHandler {
	cfg := &config.Config{}
	cfg.Auth.GatewayTrust.Secret = "test-gateway-secret"
	cfg.Auth.GatewayTrust.Required = true
	return NewPartnerHandler(zap.NewNop(), &service.SubscriptionService{}, cfg)
}

func legacyCtxWithBody() *fasthttp.RequestCtx {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetBody([]byte(`{"productId":1,"msisdn":"233241234567"}`))
	return ctx
}

// TestLegacyPartnerHandlers_FlagOff_Passes verifies that when
// GATEWAY_TRUST_REQUIRED=false (default) legacy handlers pass through the
// gateway-trust gate without the X-Gateway-Trust header (permissive mode).
// The request will still fail downstream (missing trusted-service HMAC -> 403),
// but the error code must NOT be GATEWAY_TRUST_REQUIRED.
func TestLegacyPartnerHandlers_FlagOff_Passes(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.JwtToken.Secret = "test-secret"
	h := NewPartnerHandler(zap.NewNop(), &service.SubscriptionService{}, cfg)

	ctx := legacyCtxWithBody()
	h.PartnerChargeHandler(ctx)
	var m map[string]interface{}
	_ = json.Unmarshal(ctx.Response.Body(), &m)
	if code, _ := m["code"].(string); code == "GATEWAY_TRUST_REQUIRED" {
		t.Fatalf("legacy handler must not return GATEWAY_TRUST_REQUIRED when flag is off, body=%s", ctx.Response.Body())
	}
}

// TestLegacyPartnerHandlers_FlagOn_MissingMarker_403 verifies that when
// GATEWAY_TRUST_REQUIRED=true and the X-Gateway-Trust header is absent,
// all five legacy handlers return 403 GATEWAY_TRUST_REQUIRED.
func TestLegacyPartnerHandlers_FlagOn_MissingMarker_403(t *testing.T) {
	type legacyCall struct {
		name string
		fn   func(*PartnerHandler, *fasthttp.RequestCtx)
	}
	calls := []legacyCall{
		{"PartnerMTHandler", func(h *PartnerHandler, ctx *fasthttp.RequestCtx) { h.PartnerMTHandler(ctx, "WEB") }},
		{"PartnerChargeHandler", func(h *PartnerHandler, ctx *fasthttp.RequestCtx) { h.PartnerChargeHandler(ctx) }},
		{"PartnerStatusHandler", func(h *PartnerHandler, ctx *fasthttp.RequestCtx) { h.PartnerStatusHandler(ctx) }},
		{"PartnerOptoutHandler", func(h *PartnerHandler, ctx *fasthttp.RequestCtx) { h.PartnerOptoutHandler(ctx) }},
		{"PartnerOptinConfirmHandler", func(h *PartnerHandler, ctx *fasthttp.RequestCtx) { h.PartnerOptinConfirmHandler(ctx) }},
	}

	for _, c := range calls {
		t.Run(c.name, func(t *testing.T) {
			h := newLegacyHandlerWithTrustRequired()
			ctx := legacyCtxWithBody()
			c.fn(h, ctx)
			if ctx.Response.StatusCode() != fasthttp.StatusForbidden {
				t.Errorf("want 403 GATEWAY_TRUST_REQUIRED, got status=%d body=%s",
					ctx.Response.StatusCode(), ctx.Response.Body())
				return
			}
			var m map[string]interface{}
			_ = json.Unmarshal(ctx.Response.Body(), &m)
			if code, _ := m["code"].(string); code != "GATEWAY_TRUST_REQUIRED" {
				t.Errorf("want code=GATEWAY_TRUST_REQUIRED, got %q body=%s", code, ctx.Response.Body())
			}
		})
	}
}

// TestLegacyPartnerMTHandler_FlagOn_InvalidMarker_403 mirrors the gateway
// handler tests: an invalid (wrong-HMAC) trust marker is rejected when flag=on.
func TestLegacyPartnerMTHandler_FlagOn_InvalidMarker_403(t *testing.T) {
	h := newLegacyHandlerWithTrustRequired()
	ctx := legacyCtxWithBody()
	ctx.Request.Header.Set(tenantctx.HeaderGatewayTrust, "bad-marker")

	h.PartnerMTHandler(ctx, "WEB")

	if ctx.Response.StatusCode() != fasthttp.StatusForbidden {
		t.Errorf("want 403 for invalid trust marker, got %d body=%s",
			ctx.Response.StatusCode(), ctx.Response.Body())
	}
}
