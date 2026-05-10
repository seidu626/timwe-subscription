package auth0jwt

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
)

func TestClaimsUnmarshalExtractsTenantRoleAndPlatformScope(t *testing.T) {
	raw := []byte(`{
		"iss":"https://example.auth0.com/",
		"sub":"auth0|123",
		"email":"admin@example.com",
		"email_verified":true,
		"aud":["api"],
		"tenant_id":"tenant-123",
		"tenant_key":"tenant-key",
		"org_id":"org-abc",
		"https://platform/roles":["tenant_admin","platform_operator"],
		"permissions":["reports:read"],
		"scope":"campaigns:write platform:all_tenants"
	}`)

	var claims Claims
	if err := json.Unmarshal(raw, &claims); err != nil {
		t.Fatalf("unmarshal claims: %v", err)
	}

	if claims.TenantID != "tenant-123" || claims.TenantKey != "tenant-key" || claims.OrgID != "org-abc" {
		t.Fatalf("tenant fields not extracted: %#v", claims)
	}
	if claims.Subject != "auth0|123" {
		t.Fatalf("subject = %q", claims.Subject)
	}
	if claims.Email != "admin@example.com" || !claims.EmailVerified {
		t.Fatalf("email fields not extracted: %#v", claims)
	}
	if !reflect.DeepEqual(claims.Roles, []string{"tenant_admin", "platform_operator"}) {
		t.Fatalf("roles = %#v", claims.Roles)
	}
	if !reflect.DeepEqual(claims.Permissions, []string{"reports:read", "campaigns:write", "platform:all_tenants"}) {
		t.Fatalf("permissions = %#v", claims.Permissions)
	}
	if !claims.PlatformScoped {
		t.Fatal("expected platform scoped claims")
	}

	identity := claims.Identity()
	if identity.TrustSource != tenantctx.TrustSourceJWT {
		t.Fatalf("trust source = %q", identity.TrustSource)
	}
	if !identity.PlatformScoped || !identity.HasRole("platform_operator") || !identity.HasPermission("platform:all_tenants") {
		t.Fatalf("identity did not preserve role/permission/platform scope: %#v", identity)
	}
	if identity.Email != "admin@example.com" || !identity.EmailVerified {
		t.Fatalf("identity email fields = %#v", identity)
	}
}
