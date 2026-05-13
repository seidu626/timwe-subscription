package transport

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/seidu626/subscription-manager/common/auth/auth0jwt"
	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/valyala/fasthttp"
)

func TestHealthReportsObservabilityStatus(t *testing.T) {
	router := NewRouter(nil)
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/health")
	ctx.Request.Header.SetMethod(fasthttp.MethodGet)

	router(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d", ctx.Response.StatusCode())
	}
	var body map[string]any
	if err := json.Unmarshal(ctx.Response.Body(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	observability, ok := body["observability"].(map[string]any)
	if !ok {
		t.Fatalf("expected observability status, got %#v", body)
	}
	if observability["tenant_labels"] != "enabled" || observability["pii_labels"] != "rejected" {
		t.Fatalf("unexpected observability status: %#v", observability)
	}
}

func TestUnknownRouteReturnsErrorWithoutRequestDump(t *testing.T) {
	router := NewRouter(nil)
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/missing?msisdn=233241234567")
	ctx.Request.Header.SetMethod(fasthttp.MethodGet)

	router(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusNotFound {
		t.Fatalf("expected error 404, got %d", ctx.Response.StatusCode())
	}
}

func TestAdminRequireAppliesBootstrapSubjectAndTenantKey(t *testing.T) {
	privateKey := mustAdminRSAKey(t)
	validator, err := auth0jwt.NewWithKeyfunc("example.auth0.com", "api", func(token *jwt.Token) (any, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}
	access := &adminAccess{
		validator: validator,
		bootstrapPlatformSubjects: map[string]struct{}{
			"google-oauth2|118328773120143328716": {},
		},
	}
	token := mustAdminToken(t, privateKey, jwt.MapClaims{
		"iss": "https://example.auth0.com/",
		"aud": []string{"api"},
		"sub": "google-oauth2|118328773120143328716",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("Authorization", "Bearer "+token)
	ctx.Request.Header.Set(tenantctx.HeaderTenantKey, "nrg")

	if !access.require(ctx) {
		t.Fatalf("expected admin auth to pass, status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	identity := ctx.UserValue(tenantctx.FastHTTPUserValueKey).(tenantctx.Identity)
	if !identity.PlatformScoped || !identity.HasPermission("platform:all_tenants") || identity.TenantKey != "nrg" {
		t.Fatalf("identity = %#v", identity)
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
