package adminhttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
)

func TestRequireWithStaticAdminTokenAttachesPlatformIdentity(t *testing.T) {
	adminAccess := &access{staticToken: "secret-token"}
	req := httptest.NewRequest(http.MethodGet, "/admin/series", nil)
	req.Header.Set("X-Admin-Token", "secret-token")
	rr := httptest.NewRecorder()

	if !adminAccess.require(rr, req) {
		t.Fatalf("require rejected static token: status=%d", rr.Code)
	}

	identity, ok := tenantctx.FromContext(req.Context())
	if !ok {
		t.Fatal("tenant identity missing from request context")
	}
	if !identity.PlatformScoped {
		t.Fatalf("identity not platform scoped: %#v", identity)
	}
	if identity.ServiceID != "cadence-admin-token" || identity.TrustSource != tenantctx.TrustSourceTrustedService {
		t.Fatalf("unexpected identity: %#v", identity)
	}
}

func TestHandlePreflightAllowsAdminTenantHeaders(t *testing.T) {
	adminAccess := &access{allowedOrigins: []string{"http://localhost:4200"}}
	req := httptest.NewRequest(http.MethodOptions, "/v1/admin/cadence/series?limit=500", nil)
	req.Header.Set("Origin", "http://localhost:4200")
	req.Header.Set("Access-Control-Request-Headers", "x-admin-token,x-tenant-key")
	rr := httptest.NewRecorder()

	if !adminAccess.handlePreflight(rr, req) {
		t.Fatal("expected OPTIONS request to be handled as preflight")
	}
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:4200" {
		t.Fatalf("unexpected allow origin %q", got)
	}

	allowedHeaders := rr.Header().Get("Access-Control-Allow-Headers")
	for _, header := range []string{
		"Content-Type",
		"Authorization",
		"X-Admin-Token",
		"X-Tenant-Id",
		"X-Tenant-Key",
		"X-Tenant-Channel-Id",
		"X-Channel-Id",
	} {
		if !strings.Contains(allowedHeaders, header) {
			t.Fatalf("expected allowed headers %q to include %q", allowedHeaders, header)
		}
	}
}
