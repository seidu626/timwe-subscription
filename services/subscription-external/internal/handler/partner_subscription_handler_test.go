// slice-harness: allow-new-canonical-path: TMP-072 tests gateway-trusted partner subscription handlers.
package handler

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/seidu626/subscription-manager/subscription-external/internal/service"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// --- stub implementations ---

type stubGatewayRepo struct {
	tenants  map[string]string // tenantKey → tenantID
	channels map[string]string // tenantID+":"+channelKey → channelID
}

func (r *stubGatewayRepo) TenantIDByKey(tenantKey string) (string, error) {
	if id, ok := r.tenants[tenantKey]; ok {
		return id, nil
	}
	return "", fmt.Errorf("tenant not found")
}

func (r *stubGatewayRepo) ChannelIDByKeys(tenantID, channelKey string) (string, error) {
	key := tenantID + ":" + channelKey
	if id, ok := r.channels[key]; ok {
		return id, nil
	}
	return "", fmt.Errorf("channel not found")
}

func newTestPartnerHandler(repo gatewayTenantLookup) *PartnerHandler {
	cfg := &config.Config{}
	h := NewPartnerHandler(zap.NewNop(), &service.SubscriptionService{}, cfg)
	if repo != nil {
		h.WithTenantRepo(repo)
	}
	return h
}

func stubRepo() *stubGatewayRepo {
	return &stubGatewayRepo{
		tenants: map[string]string{
			"careerify": "tenant-uuid-1",
		},
		channels: map[string]string{
			"tenant-uuid-1:web-gh-airteltigo": "channel-uuid-1",
		},
	}
}

func parseErrorResponse(body []byte) map[string]interface{} {
	var out map[string]interface{}
	_ = json.Unmarshal(body, &out)
	return out
}

func errorCode(body []byte) string {
	m := parseErrorResponse(body)
	if v, ok := m["code"].(string); ok {
		return v
	}
	return ""
}

// buildCtx constructs a fasthttp.RequestCtx with the given header and query setup.
func buildCtx(hTenantKey, hChannelKey, qTenantKey, qChannelKey string) *fasthttp.RequestCtx {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/api/v1/subscription-external/partners/status")
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	if hTenantKey != "" {
		ctx.Request.Header.Set(tenantctx.HeaderTenantKey, hTenantKey)
	}
	if hChannelKey != "" {
		ctx.Request.Header.Set(tenantctx.HeaderChannelKey, hChannelKey)
	}
	uri := "/api/v1/subscription-external/partners/status"
	params := ""
	if qTenantKey != "" {
		params += "tenant_key=" + qTenantKey + "&"
	}
	if qChannelKey != "" {
		params += "channel_key=" + qChannelKey + "&"
	}
	if params != "" {
		uri += "?" + params
	}
	ctx.Request.SetRequestURI(uri)
	// Set a minimal valid body so JSON parse passes.
	ctx.Request.SetBody([]byte(`{"userIdentifier":"233241234567","productId":1}`))
	return ctx
}

// --- table-driven tests for tenantRouteFromGatewayHeaders ---

func TestTenantRouteFromGatewayHeaders(t *testing.T) {
	repo := stubRepo()

	tests := []struct {
		name       string
		hTenantKey string
		hChannelKey string
		qTenantKey string
		qChannelKey string
		wantStatus int
		wantCode   string
		wantTenant string
		wantChannel string
	}{
		{
			name:        "happy path — header tenant+channel resolve",
			hTenantKey:  "careerify",
			hChannelKey: "web-gh-airteltigo",
			wantStatus:  fasthttp.StatusOK,
			wantTenant:  "tenant-uuid-1",
			wantChannel: "channel-uuid-1",
		},
		{
			name:        "happy path — query params resolve (GatewayTrusted=true)",
			qTenantKey:  "careerify",
			qChannelKey: "web-gh-airteltigo",
			wantStatus:  fasthttp.StatusOK,
			wantTenant:  "tenant-uuid-1",
			wantChannel: "channel-uuid-1",
		},
		{
			name:        "unknown tenant_key → 400 UNKNOWN_TENANT",
			hTenantKey:  "unknown-tenant",
			hChannelKey: "web-gh-airteltigo",
			wantStatus:  fasthttp.StatusBadRequest,
			wantCode:    "UNKNOWN_TENANT",
		},
		{
			name:        "unknown channel_key (tenant resolves) → 400 UNKNOWN_CHANNEL",
			hTenantKey:  "careerify",
			hChannelKey: "bad-channel",
			wantStatus:  fasthttp.StatusBadRequest,
			wantCode:    "UNKNOWN_CHANNEL",
		},
		{
			name:       "missing channel_key → 400 TENANT_CONTEXT_REQUIRED",
			hTenantKey: "careerify",
			// channel key absent
			wantStatus: fasthttp.StatusBadRequest,
			wantCode:   "TENANT_CONTEXT_REQUIRED",
		},
		{
			name:       "missing tenant_key entirely → 400 TENANT_CONTEXT_REQUIRED",
			// both absent
			wantStatus: fasthttp.StatusBadRequest,
			wantCode:   "TENANT_CONTEXT_REQUIRED",
		},
		{
			name:        "header/query conflict → 409 TENANT_KEY_CONFLICT",
			hTenantKey:  "careerify",
			hChannelKey: "web-gh-airteltigo",
			qTenantKey:  "other-tenant",
			wantStatus:  fasthttp.StatusConflict,
			wantCode:    "TENANT_KEY_CONFLICT",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := buildCtx(tc.hTenantKey, tc.hChannelKey, tc.qTenantKey, tc.qChannelKey)
			route, err := tenantRouteFromGatewayHeaders(ctx, repo)

			if tc.wantCode == "" {
				// Expect success
				if err != nil {
					t.Fatalf("expected success, got error: %v", err)
				}
				if route.TenantID != tc.wantTenant {
					t.Errorf("TenantID: want %q got %q", tc.wantTenant, route.TenantID)
				}
				if route.ChannelID != tc.wantChannel {
					t.Errorf("ChannelID: want %q got %q", tc.wantChannel, route.ChannelID)
				}
			} else {
				// Expect error
				if err == nil {
					t.Fatalf("expected error with code %q, got success: %+v", tc.wantCode, route)
				}
				gotStatus, gotCode := gatewayRouteStatus(err)
				if gotStatus != tc.wantStatus {
					t.Errorf("HTTP status: want %d got %d (err=%v)", tc.wantStatus, gotStatus, err)
				}
				if gotCode != tc.wantCode {
					t.Errorf("error code: want %q got %q", tc.wantCode, gotCode)
				}
			}
		})
	}
}

// --- handler-level error-path tests (do not exercise service downstream) ---

// TestPartnerSubscriptionHandlers_ErrorPaths verifies that all four handlers
// return the correct error response before reaching the service layer.
func TestPartnerSubscriptionHandlers_ErrorPaths(t *testing.T) {
	tests := []struct {
		name        string
		hTenantKey  string
		hChannelKey string
		qTenantKey  string
		wantStatus  int
		wantCode    string
	}{
		{
			name:        "unknown tenant → 400 UNKNOWN_TENANT",
			hTenantKey:  "bogus",
			hChannelKey: "some-channel", // channel must be present to reach tenant lookup
			wantStatus:  fasthttp.StatusBadRequest,
			wantCode:    "UNKNOWN_TENANT",
		},
		{
			name:        "conflict → 409 TENANT_KEY_CONFLICT",
			hTenantKey:  "careerify",
			hChannelKey: "web-gh-airteltigo",
			qTenantKey:  "other",
			wantStatus:  fasthttp.StatusConflict,
			wantCode:    "TENANT_KEY_CONFLICT",
		},
		{
			name:       "missing both keys → 400 TENANT_CONTEXT_REQUIRED",
			wantStatus: fasthttp.StatusBadRequest,
			wantCode:   "TENANT_CONTEXT_REQUIRED",
		},
	}

	handlers := []struct {
		name string
		fn   func(*PartnerHandler) func(*fasthttp.RequestCtx)
	}{
		{"status", func(h *PartnerHandler) func(*fasthttp.RequestCtx) { return h.PartnerSubscriptionStatus }},
		{"optout", func(h *PartnerHandler) func(*fasthttp.RequestCtx) { return h.PartnerSubscriptionOptout }},
		{"confirm", func(h *PartnerHandler) func(*fasthttp.RequestCtx) { return h.PartnerSubscriptionConfirm }},
		{"optin", func(h *PartnerHandler) func(*fasthttp.RequestCtx) { return h.PartnerSubscriptionOptin }},
	}

	for _, hh := range handlers {
		for _, tc := range tests {
			t.Run(hh.name+"/"+tc.name, func(t *testing.T) {
				h := newTestPartnerHandler(stubRepo())
				ctx := buildCtx(tc.hTenantKey, tc.hChannelKey, tc.qTenantKey, "")
				hh.fn(h)(ctx)

				got := ctx.Response.StatusCode()
				code := errorCode(ctx.Response.Body())
				if code != tc.wantCode {
					t.Errorf("error code: want %q got %q (body=%s)", tc.wantCode, code, ctx.Response.Body())
				}
				if got != tc.wantStatus {
					t.Errorf("HTTP status: want %d got %d", tc.wantStatus, got)
				}
			})
		}
	}
}

func TestPartnerSubscriptionHandlers_TenantRepoNil(t *testing.T) {
	h := newTestPartnerHandler(nil) // no repo set
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetBody([]byte(`{}`))

	handlers := []struct {
		name string
		fn   func(*fasthttp.RequestCtx)
	}{
		{"optin", h.PartnerSubscriptionOptin},
		{"confirm", h.PartnerSubscriptionConfirm},
		{"optout", h.PartnerSubscriptionOptout},
		{"status", h.PartnerSubscriptionStatus},
	}
	for _, tc := range handlers {
		t.Run(tc.name, func(t *testing.T) {
			ctx2 := &fasthttp.RequestCtx{}
			ctx2.Request.SetBody([]byte(`{}`))
			tc.fn(ctx2)
			if ctx2.Response.StatusCode() != fasthttp.StatusInternalServerError {
				t.Errorf("want 500 when tenantRepo is nil, got %d", ctx2.Response.StatusCode())
			}
		})
	}
}

// TestPartnerSubscriptionOptin_UUIDsInRoute verifies that when tenant+channel resolve,
// the TenantRouteContext is populated with UUIDs (audit trail requirement).
func TestPartnerSubscriptionOptin_UUIDsInRoute(t *testing.T) {
	repo := stubRepo()
	ctx := buildCtx("careerify", "web-gh-airteltigo", "", "")

	route, err := tenantRouteFromGatewayHeaders(ctx, repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if route.TenantID == "" {
		t.Error("TenantID must be non-empty (audit requirement)")
	}
	if route.ChannelID == "" {
		t.Error("ChannelID must be non-empty (audit requirement)")
	}
	if route.TenantKey != "careerify" {
		t.Errorf("TenantKey: want %q got %q", "careerify", route.TenantKey)
	}
	if route.ChannelKey != "web-gh-airteltigo" {
		t.Errorf("ChannelKey: want %q got %q", "web-gh-airteltigo", route.ChannelKey)
	}
}

// TestGatewayRouteStatus_ConflictMapsTo409 is a unit test for the error-to-status mapper.
func TestGatewayRouteStatus_ConflictMapsTo409(t *testing.T) {
	err := fmt.Errorf("%w: X-Tenant-Key header=%q query=%q", tenantctx.ErrTenantKeyConflict, "a", "b")
	status, code := gatewayRouteStatus(err)
	if status != fasthttp.StatusConflict {
		t.Errorf("want 409, got %d", status)
	}
	if code != "TENANT_KEY_CONFLICT" {
		t.Errorf("want TENANT_KEY_CONFLICT, got %q", code)
	}
}

// TestPartnerSubscriptionOptout_MissingChannelKey tests that a missing channel_key
// returns 400 TENANT_CONTEXT_REQUIRED (mirrors TMP-071 CHANNEL_REQUIRED behaviour).
func TestPartnerSubscriptionOptout_MissingChannelKey(t *testing.T) {
	repo := stubRepo()
	ctx := buildCtx("careerify", "", "", "") // tenant present, channel absent
	h := newTestPartnerHandler(repo)
	h.PartnerSubscriptionOptout(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusBadRequest {
		t.Errorf("want 400, got %d", ctx.Response.StatusCode())
	}
	code := errorCode(ctx.Response.Body())
	if code != "TENANT_CONTEXT_REQUIRED" {
		t.Errorf("want TENANT_CONTEXT_REQUIRED, got %q", code)
	}
}

// TestPartnerSubscriptionConfirm_UnknownChannel tests that an unknown channel_key
// returns 400 UNKNOWN_CHANNEL when the tenant resolves but the channel doesn't.
func TestPartnerSubscriptionConfirm_UnknownChannel(t *testing.T) {
	repo := stubRepo()
	ctx := buildCtx("careerify", "nonexistent-channel", "", "")
	h := newTestPartnerHandler(repo)
	h.PartnerSubscriptionConfirm(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusBadRequest {
		t.Errorf("want 400, got %d", ctx.Response.StatusCode())
	}
	code := errorCode(ctx.Response.Body())
	if code != "UNKNOWN_CHANNEL" {
		t.Errorf("want UNKNOWN_CHANNEL, got %q", code)
	}
}

// Ensure domain.TenantRouteContext is usable (compile-time check).
var _ domain.TenantRouteContext = domain.TenantRouteContext{}
