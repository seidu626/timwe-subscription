package transport

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/seidu626/subscription-manager/common/auth/auth0jwt"
	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/valyala/fasthttp"
)

func TestAdminRequireStoresTenantIdentity(t *testing.T) {
	privateKey := mustAdminRSAKey(t)
	validator, err := auth0jwt.NewWithKeyfunc("example.auth0.com", "api", func(token *jwt.Token) (any, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}
	access := &adminAccess{validator: validator}
	token := mustAdminToken(t, privateKey, jwt.MapClaims{
		"iss":                    "https://example.auth0.com/",
		"aud":                    []string{"api"},
		"sub":                    "auth0|admin",
		"iat":                    time.Now().Unix(),
		"exp":                    time.Now().Add(time.Hour).Unix(),
		"tenant_id":              "tenant-123",
		"https://platform/roles": []string{"tenant_admin"},
	})

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.Set("Authorization", "Bearer "+token)

	if !access.require(&ctx) {
		t.Fatalf("expected admin auth to pass, status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	value := ctx.UserValue(tenantctx.FastHTTPUserValueKey)
	identity, ok := value.(tenantctx.Identity)
	if !ok {
		t.Fatalf("expected tenant identity in user values, got %#v", value)
	}
	if identity.TenantID != "tenant-123" || !identity.HasRole("tenant_admin") || identity.TrustSource != tenantctx.TrustSourceJWT {
		t.Fatalf("identity = %#v", identity)
	}
}

func TestAdminRequireRejectsAudienceMismatchBeforeIdentity(t *testing.T) {
	privateKey := mustAdminRSAKey(t)
	validator, err := auth0jwt.NewWithKeyfunc("example.auth0.com", "api", func(token *jwt.Token) (any, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}
	access := &adminAccess{validator: validator}
	token := mustAdminToken(t, privateKey, jwt.MapClaims{
		"iss": "https://example.auth0.com/",
		"aud": []string{"wrong"},
		"sub": "auth0|admin",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.Set("Authorization", "Bearer "+token)

	if access.require(&ctx) {
		t.Fatal("expected admin auth to reject audience mismatch")
	}
	if ctx.UserValue(tenantctx.FastHTTPUserValueKey) != nil {
		t.Fatal("tenant identity should not be attached on auth failure")
	}
	if ctx.Response.StatusCode() != fasthttp.StatusUnauthorized {
		t.Fatalf("status = %d", ctx.Response.StatusCode())
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
