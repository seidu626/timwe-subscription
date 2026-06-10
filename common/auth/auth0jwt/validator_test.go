package auth0jwt

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

// TestNewEmptyConfigErrors confirms that empty domain or audience still returns
// ErrInvalidConfig immediately (config validation is not lazy).
func TestNewEmptyConfigErrors(t *testing.T) {
	if _, err := New("", "audience"); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig for empty domain, got %v", err)
	}
	if _, err := New("example.auth0.com", ""); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig for empty audience, got %v", err)
	}
}

// TestNewUnreachableJWKSDoesNotErrorButFailsClosed verifies that New succeeds
// even when the JWKS endpoint is unreachable, and that ValidateBearer returns
// a non-nil, non-ErrInvalidConfig error (fails closed).
func TestNewUnreachableJWKSDoesNotErrorButFailsClosed(t *testing.T) {
	// Use a long retry so the goroutine doesn't interfere during the test.
	v, err := New("localhost:1", "test-audience", withRetryInterval(10*time.Second))
	if err != nil {
		t.Fatalf("New must not return an error for an unreachable JWKS: %v", err)
	}
	if v == nil {
		t.Fatal("New must return a non-nil Validator")
	}

	_, berr := v.ValidateBearer(context.Background(), "Bearer eyJhbGciOiJSUzI1NiIsImtpZCI6InRlc3QifQ.e30.x")
	if berr == nil {
		t.Fatal("ValidateBearer must fail when JWKS is unavailable")
	}
	if errors.Is(berr, ErrInvalidConfig) {
		t.Fatalf("ValidateBearer must not return ErrInvalidConfig when JWKS is temporarily unavailable: %v", berr)
	}
}

// TestValidatorSelfHealsAfterJWKSBecomesAvailable verifies that a validator
// created when the JWKS endpoint is down starts accepting requests once the
// endpoint comes up, without any restart.
func TestValidatorSelfHealsAfterJWKSBecomesAvailable(t *testing.T) {
	var serveCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveCount.Load() == 0 {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// Serve an empty-but-valid JWKS (no keys needed for this test — we
		// only need to confirm the unavailable state clears).
		w.Write([]byte(`{"keys":[]}`))
	}))
	t.Cleanup(srv.Close)

	domain := strings.TrimPrefix(srv.URL, "http://")
	v, err := New(domain, "test-audience", withRetryInterval(50*time.Millisecond))
	if err != nil {
		t.Fatalf("New must not fail: %v", err)
	}

	// Before the server comes up, confirm we fail closed (not ErrInvalidConfig).
	_, berr := v.ValidateBearer(context.Background(), "Bearer x.y.z")
	if berr == nil {
		t.Fatal("expected error before JWKS is available")
	}
	if errors.Is(berr, ErrInvalidConfig) {
		t.Fatalf("must not be ErrInvalidConfig before JWKS loads: %v", berr)
	}

	// Bring the server up.
	serveCount.Store(1)

	// Wait for the retry goroutine to load the (empty) JWKS. Once it does,
	// ValidateBearer will no longer return ErrJWKSUnavailable — it will fail
	// for a different reason (bad token / no matching key), confirming recovery.
	deadline := time.Now().Add(2 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		_, lastErr = v.ValidateBearer(context.Background(), "Bearer x.y.z")
		if lastErr != nil && !errors.Is(lastErr, ErrJWKSUnavailable) {
			// Error is now about token format/signature, not availability — healed.
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("validator did not self-heal within 2s; last err: %v", lastErr)
}
