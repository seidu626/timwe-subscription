package adminhttp

import (
	"net/http"
	"net/http/httptest"
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
