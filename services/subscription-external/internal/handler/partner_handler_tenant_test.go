// slice-harness: allow-new-canonical-path: TMP-007 tests partner tenant context enforcement.
package handler

import (
	"encoding/json"
	"testing"

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
