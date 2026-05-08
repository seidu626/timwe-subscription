package auth0jwt

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestValidateBearerReturnsTypedTenantClaims(t *testing.T) {
	privateKey := mustRSAKey(t)
	validator := mustValidator(t, privateKey)
	token := mustToken(t, privateKey, jwt.MapClaims{
		"iss":                     "https://example.auth0.com/",
		"aud":                     []string{"api-audience"},
		"sub":                     "auth0|123",
		"iat":                     time.Now().Unix(),
		"exp":                     time.Now().Add(time.Hour).Unix(),
		"tenant_id":               "tenant-123",
		"https://platform/roles":  []string{"tenant_admin"},
		"https://platform/org_id": "org-123",
	})

	claims, err := validator.ValidateBearer(context.Background(), "Bearer "+token)
	if err != nil {
		t.Fatalf("ValidateBearer returned error: %v", err)
	}

	if claims.TenantID != "tenant-123" || claims.OrgID != "org-123" || claims.Subject != "auth0|123" {
		t.Fatalf("typed claims not preserved: %#v", claims)
	}
	if !claims.Identity().HasTenant() || !claims.Identity().HasRole("tenant_admin") {
		t.Fatalf("identity did not preserve tenant/roles: %#v", claims.Identity())
	}
}

func TestValidateBearerRejectsAudienceMismatch(t *testing.T) {
	privateKey := mustRSAKey(t)
	validator := mustValidator(t, privateKey)
	token := mustToken(t, privateKey, jwt.MapClaims{
		"iss": "https://example.auth0.com/",
		"aud": []string{"wrong-audience"},
		"sub": "auth0|123",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	_, err := validator.ValidateBearer(context.Background(), "Bearer "+token)
	if err == nil || !strings.Contains(err.Error(), "audience mismatch") {
		t.Fatalf("expected audience mismatch, got %v", err)
	}
}

func mustRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	return key
}

func mustValidator(t *testing.T, privateKey *rsa.PrivateKey) *Validator {
	t.Helper()
	validator, err := NewWithKeyfunc("example.auth0.com", "api-audience", func(token *jwt.Token) (any, error) {
		return &privateKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}
	return validator
}

func mustToken(t *testing.T, privateKey *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(privateKey)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}
