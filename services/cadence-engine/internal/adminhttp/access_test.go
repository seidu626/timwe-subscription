package adminhttp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/seidu626/subscription-manager/common/auth/auth0jwt"
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
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
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
		requireAllowedCORSHeader(t, allowedHeaders, header)
	}
}

func TestRequireAppliesBootstrapPlatformSubjectAndSelectedTenant(t *testing.T) {
	privateKey := mustAdminRSAKey(t)
	validator, err := auth0jwt.NewWithKeyfunc("example.auth0.com", "api", func(token *jwt.Token) (any, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}
	adminAccess := &access{
		validator: validator,
		bootstrapPlatformSubjects: map[string]struct{}{
			"google-oauth2|bootstrap-admin": {},
		},
	}
	token := mustAdminToken(t, privateKey, jwt.MapClaims{
		"iss": "https://example.auth0.com/",
		"aud": []string{"api"},
		"sub": "google-oauth2|bootstrap-admin",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/cadence/series", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set(tenantctx.HeaderTenantKey, "nrg")
	rr := httptest.NewRecorder()

	if !adminAccess.require(rr, req) {
		t.Fatalf("expected admin auth to pass, status=%d body=%q", rr.Code, rr.Body.String())
	}
	identity, ok := tenantctx.FromContext(req.Context())
	if !ok {
		t.Fatal("tenant identity missing from request context")
	}
	if !identity.PlatformScoped || !identity.HasPermission("platform:all_tenants") || identity.TenantKey != "nrg" {
		t.Fatalf("identity = %#v", identity)
	}
	if identity.Email != "" {
		t.Fatalf("test token should not rely on email claim, identity = %#v", identity)
	}
}

func TestRequireDoesNotApplySelectedTenantForUnscopedIdentity(t *testing.T) {
	privateKey := mustAdminRSAKey(t)
	validator, err := auth0jwt.NewWithKeyfunc("example.auth0.com", "api", func(token *jwt.Token) (any, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}
	adminAccess := &access{
		validator: validator,
		bootstrapPlatformEmails: map[string]struct{}{
			"bootstrap@example.com": {},
		},
	}
	token := mustAdminToken(t, privateKey, jwt.MapClaims{
		"iss":   "https://example.auth0.com/",
		"aud":   []string{"api"},
		"sub":   "auth0|ordinary-admin",
		"email": "ordinary@example.com",
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(time.Hour).Unix(),
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/cadence/series", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set(tenantctx.HeaderTenantKey, "nrg")
	rr := httptest.NewRecorder()

	if !adminAccess.require(rr, req) {
		t.Fatalf("expected admin auth to pass, status=%d body=%q", rr.Code, rr.Body.String())
	}
	identity, ok := tenantctx.FromContext(req.Context())
	if !ok {
		t.Fatal("tenant identity missing from request context")
	}
	if identity.PlatformScoped || identity.TenantKey != "" {
		t.Fatalf("identity = %#v", identity)
	}
}

func TestRequireAppliesSelectedTenantWhenMembershipMatches(t *testing.T) {
	privateKey := mustAdminRSAKey(t)
	validator, err := auth0jwt.NewWithKeyfunc("example.auth0.com", "api", func(token *jwt.Token) (any, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}
	adminAccess := &access{
		validator: validator,
		memberLookup: func(_ context.Context, subject, email string) ([]MemberTenant, error) {
			if subject != "auth0|ordinary-admin" || email != "ordinary@example.com" {
				t.Fatalf("membership lookup subject=%q email=%q", subject, email)
			}
			return []MemberTenant{
				{ID: "tenant-a-id", TenantKey: "tenant-a"},
				{ID: "tenant-b-id", TenantKey: "tenant-b"},
			}, nil
		},
	}
	token := mustAdminToken(t, privateKey, jwt.MapClaims{
		"iss":   "https://example.auth0.com/",
		"aud":   []string{"api"},
		"sub":   "auth0|ordinary-admin",
		"email": "ordinary@example.com",
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(time.Hour).Unix(),
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/cadence/series", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set(tenantctx.HeaderTenantKey, "Tenant-B")
	rr := httptest.NewRecorder()

	if !adminAccess.require(rr, req) {
		t.Fatalf("expected admin auth to pass, status=%d body=%q", rr.Code, rr.Body.String())
	}
	identity, ok := tenantctx.FromContext(req.Context())
	if !ok {
		t.Fatal("tenant identity missing from request context")
	}
	if identity.PlatformScoped || identity.TenantID != "tenant-b-id" || identity.TenantKey != "tenant-b" {
		t.Fatalf("identity = %#v", identity)
	}
}

func TestRequireRejectsUnmatchedSelectedTenantForMembershipBackedIdentity(t *testing.T) {
	privateKey := mustAdminRSAKey(t)
	validator, err := auth0jwt.NewWithKeyfunc("example.auth0.com", "api", func(token *jwt.Token) (any, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}
	adminAccess := &access{
		validator: validator,
		memberLookup: func(_ context.Context, _, _ string) ([]MemberTenant, error) {
			return []MemberTenant{
				{ID: "tenant-a-id", TenantKey: "tenant-a"},
				{ID: "tenant-b-id", TenantKey: "tenant-b"},
			}, nil
		},
	}
	token := mustAdminToken(t, privateKey, jwt.MapClaims{
		"iss":   "https://example.auth0.com/",
		"aud":   []string{"api"},
		"sub":   "auth0|ordinary-admin",
		"email": "ordinary@example.com",
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(time.Hour).Unix(),
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/cadence/series", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set(tenantctx.HeaderTenantKey, "tenant-c")
	rr := httptest.NewRecorder()

	if !adminAccess.require(rr, req) {
		t.Fatalf("expected admin auth to pass, status=%d body=%q", rr.Code, rr.Body.String())
	}
	identity, ok := tenantctx.FromContext(req.Context())
	if !ok {
		t.Fatal("tenant identity missing from request context")
	}
	if identity.HasTenant() {
		t.Fatalf("unmatched selected tenant must not stamp membership context, identity = %#v", identity)
	}
}

func TestBootstrapPlatformSubjectSetDefaultsClosed(t *testing.T) {
	if got := bootstrapPlatformSubjectSet(""); len(got) != 0 {
		t.Fatalf("empty bootstrap subject config must not grant platform scope, got %#v", got)
	}
}

func mustAdminRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	return key
}

func mustAdminToken(t *testing.T, privateKey *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(privateKey)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

func requireAllowedCORSHeader(t *testing.T, allowedHeaders string, want string) {
	t.Helper()
	for _, header := range strings.Split(allowedHeaders, ",") {
		if strings.EqualFold(strings.TrimSpace(header), want) {
			return
		}
	}
	t.Fatalf("expected allowed headers %q to include %q", allowedHeaders, want)
}
