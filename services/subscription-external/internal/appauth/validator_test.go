package appauth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func signToken(t *testing.T, secret string, claims Claims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func validClaims(msisdn, tenant string) Claims {
	now := time.Now()
	return Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   msisdn,
			Issuer:    Issuer,
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		Tenant: tenant,
	}
}

func TestNewFromEnv_FailsClosedWhenSecretUnset(t *testing.T) {
	t.Setenv(EnvSecretKey, "")
	if _, err := NewFromEnv(); err != ErrNotConfigured {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestNewFromEnv_SucceedsWhenSecretSet(t *testing.T) {
	t.Setenv(EnvSecretKey, "top-secret")
	v, err := NewFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v == nil {
		t.Fatal("expected non-nil validator")
	}
}

func TestValidateBearer_ValidToken(t *testing.T) {
	const secret = "shared-secret"
	v, err := New(secret)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	claims := validClaims("233241234567", "careerify")
	token := signToken(t, secret, claims)

	got, err := v.ValidateBearer("Bearer " + token)
	if err != nil {
		t.Fatalf("ValidateBearer: %v", err)
	}
	if got.Subject != "233241234567" {
		t.Errorf("Subject = %q, want %q", got.Subject, "233241234567")
	}
	if got.Tenant != "careerify" {
		t.Errorf("Tenant = %q, want %q", got.Tenant, "careerify")
	}
}

func TestValidateBearer_MissingHeader(t *testing.T) {
	v, _ := New("shared-secret")
	if _, err := v.ValidateBearer(""); err != ErrMissingToken {
		t.Fatalf("expected ErrMissingToken, got %v", err)
	}
}

func TestValidateBearer_NotBearerScheme(t *testing.T) {
	v, _ := New("shared-secret")
	if _, err := v.ValidateBearer("Basic abc123"); err != ErrMissingToken {
		t.Fatalf("expected ErrMissingToken, got %v", err)
	}
}

func TestValidateBearer_WrongSecret(t *testing.T) {
	v, _ := New("shared-secret")
	token := signToken(t, "different-secret", validClaims("233241234567", "careerify"))
	if _, err := v.ValidateBearer("Bearer " + token); err == nil {
		t.Fatal("expected error for wrong signing secret")
	}
}

func TestValidateBearer_ExpiredToken(t *testing.T) {
	const secret = "shared-secret"
	v, _ := New(secret)
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "233241234567",
			Issuer:    Issuer,
			ExpiresAt: jwt.NewNumericDate(now.Add(-2 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now.Add(-3 * time.Hour)),
		},
		Tenant: "careerify",
	}
	token := signToken(t, secret, claims)
	if _, err := v.ValidateBearer("Bearer " + token); err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestValidateBearer_WrongIssuer(t *testing.T) {
	const secret = "shared-secret"
	v, _ := New(secret)
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "233241234567",
			Issuer:    "some-other-issuer",
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		Tenant: "careerify",
	}
	token := signToken(t, secret, claims)
	if _, err := v.ValidateBearer("Bearer " + token); err == nil {
		t.Fatal("expected error for wrong issuer")
	}
}

func TestValidateBearer_MissingTenantClaim(t *testing.T) {
	const secret = "shared-secret"
	v, _ := New(secret)
	claims := validClaims("233241234567", "")
	token := signToken(t, secret, claims)
	if _, err := v.ValidateBearer("Bearer " + token); err == nil {
		t.Fatal("expected error for missing tenant claim")
	}
}

func TestValidateBearer_UnconfiguredValidator(t *testing.T) {
	var v *Validator
	if _, err := v.ValidateBearer("Bearer whatever"); err != ErrNotConfigured {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestValidateBearer_RejectsAlgNone(t *testing.T) {
	const secret = "shared-secret"
	v, _ := New(secret)
	// alg=none tokens must never validate even without a signature check bug;
	// the parser's WithValidMethods should reject this outright.
	noneToken := jwt.NewWithClaims(jwt.SigningMethodNone, validClaims("233241234567", "careerify"))
	signed, err := noneToken.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign none token: %v", err)
	}
	if _, err := v.ValidateBearer("Bearer " + signed); err == nil {
		t.Fatal("expected error for alg=none token")
	}
}
