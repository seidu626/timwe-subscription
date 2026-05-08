package handler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription-external/internal/monitoring"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

func TestMonitoringDashboardScopesTenantChannelMetrics(t *testing.T) {
	tenantA := "11111111-1111-1111-1111-111111111111"
	channelA := "22222222-2222-2222-2222-222222222222"
	tenantB := "33333333-3333-3333-3333-333333333333"
	channelB := "44444444-4444-4444-4444-444444444444"

	monitor := monitoring.NewChargingFailureMonitor(zap.NewNop())
	monitor.UpdateScopedMetrics(tenantA, channelA, &monitoring.ChargingFailureMetrics{
		TotalSubscriptions: 20,
		ChargingFailures:   4,
		FailureRate:        20,
		SuccessRate:        80,
		LastUpdated:        time.Now().UTC(),
		ProcessingStatus:   "healthy",
		Metadata:           map[string]interface{}{},
	})
	monitor.UpdateScopedMetrics(tenantB, channelB, &monitoring.ChargingFailureMetrics{
		TotalSubscriptions: 90,
		ChargingFailures:   45,
		FailureRate:        50,
		SuccessRate:        50,
		LastUpdated:        time.Now().UTC(),
		ProcessingStatus:   "attention",
		Metadata:           map[string]interface{}{},
	})

	handler := NewMonitoringHandler(monitor, zap.NewNop(), monitoringTestConfig())
	var ctx fasthttp.RequestCtx
	signMonitoringRequest(&ctx, tenantA, channelA)

	handler.GetDashboardDataHandler(&ctx)
	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("status = %d, body = %s", ctx.Response.StatusCode(), ctx.Response.Body())
	}

	body := decodeMonitoringResponse(t, &ctx)
	if body["status"] != "success" {
		t.Fatalf("status = %v, want success", body["status"])
	}
	metrics := body["data"].(map[string]interface{})["metrics"].(map[string]interface{})
	if got := metrics["charging_failures"].(float64); got != 4 {
		t.Fatalf("charging_failures = %v, want tenant A value 4", got)
	}
	scope := body["data"].(map[string]interface{})["status"].(map[string]interface{})["scope"].(map[string]interface{})
	if scope["tenant_id"] != tenantA || scope["channel_id"] != channelA {
		t.Fatalf("unexpected scope: %#v", scope)
	}
	if scope["degraded"].(bool) {
		t.Fatalf("scope should not be degraded: %#v", scope)
	}
}

func TestMonitoringDashboardShowsDegradedWhenScopedMetricsUnavailable(t *testing.T) {
	monitor := monitoring.NewChargingFailureMonitor(zap.NewNop())
	handler := NewMonitoringHandler(monitor, zap.NewNop(), monitoringTestConfig())
	var ctx fasthttp.RequestCtx
	signMonitoringRequest(&ctx, "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222")

	handler.GetDashboardDataHandler(&ctx)
	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("status = %d, body = %s", ctx.Response.StatusCode(), ctx.Response.Body())
	}

	body := decodeMonitoringResponse(t, &ctx)
	if body["status"] != "degraded" {
		t.Fatalf("status = %v, want degraded", body["status"])
	}
	scope := body["data"].(map[string]interface{})["status"].(map[string]interface{})["scope"].(map[string]interface{})
	if scope["degraded"] != true || scope["degraded_reason"] == "" {
		t.Fatalf("expected degraded scope with reason, got %#v", scope)
	}
	metrics := body["data"].(map[string]interface{})["metrics"].(map[string]interface{})
	if got := metrics["charging_failures"].(float64); got != 0 {
		t.Fatalf("degraded response should be zeroed, got charging_failures=%v", got)
	}
}

func signMonitoringRequest(ctx *fasthttp.RequestCtx, tenantID, channelID string) {
	now := time.Now().UTC().Format(time.RFC3339)
	ctx.Request.SetRequestURI("/api/v1/subscription-external/monitoring/dashboard")
	ctx.Request.Header.SetMethod(fasthttp.MethodGet)
	ctx.Request.Header.Set(tenantctx.HeaderTenantID, tenantID)
	ctx.Request.Header.Set("X-Tenant-Channel-Id", channelID)
	ctx.Request.Header.Set(tenantctx.HeaderServiceID, "gateway")
	ctx.Request.Header.Set(tenantctx.HeaderServiceTimestamp, now)
	ctx.Request.Header.Set(tenantctx.HeaderServiceSignature, tenantctx.SignServiceRequest("secret", tenantctx.SignInput{
		Method:    fasthttp.MethodGet,
		Path:      "/api/v1/subscription-external/monitoring/dashboard",
		Timestamp: now,
		ServiceID: "gateway",
		TenantID:  tenantID,
	}))
}

func monitoringTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Auth.JwtToken.Secret = "secret"
	return cfg
}

func decodeMonitoringResponse(t *testing.T, ctx *fasthttp.RequestCtx) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	if err := json.Unmarshal(ctx.Response.Body(), &body); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, ctx.Response.Body())
	}
	return body
}
