package appauth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestNewValidatorFailsClosedOnEmptySecret(t *testing.T) {
	if _, err := NewValidator(""); err != ErrSecretNotConfigured {
		t.Fatalf("expected ErrSecretNotConfigured, got %v", err)
	}
	if _, err := NewValidator("   "); err != ErrSecretNotConfigured {
		t.Fatalf("expected ErrSecretNotConfigured for whitespace-only secret, got %v", err)
	}
}

func TestIssueAndParseRoundTrip(t *testing.T) {
	v, err := NewValidator("test-secret")
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	// Uses the real clock rather than a fixed past date: Parse validates
	// expiry against wall-clock time (no clock injection in the
	// production Parse path), and this test only checks the
	// issue-then-immediately-parse round trip, not expiry behavior.
	now := time.Now()
	token, err := v.IssueToken("233241234567", "nrg", now)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	claims, err := v.Parse(token)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if claims.MSISDN != "233241234567" {
		t.Fatalf("expected msisdn 233241234567, got %q", claims.MSISDN)
	}
	if claims.Tenant != "nrg" {
		t.Fatalf("expected tenant nrg, got %q", claims.Tenant)
	}
	if claims.Issuer != Issuer {
		t.Fatalf("expected issuer %q, got %q", Issuer, claims.Issuer)
	}
}

func TestParseRejectsExpiredToken(t *testing.T) {
	v, err := NewValidator("test-secret")
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	// Issue a token whose TTL has already elapsed relative to "now".
	issuedAt := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	token, err := v.IssueToken("233241234567", "nrg", issuedAt)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	if _, err := v.Parse(token); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken for expired token, got %v", err)
	}
}

func TestParseRejectsWrongIssuer(t *testing.T) {
	v, err := NewValidator("test-secret")
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	claims := Claims{
		MSISDN: "233241234567",
		Tenant: "nrg",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "some-other-issuer",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	if _, err := v.Parse(signed); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken for wrong issuer, got %v", err)
	}
}

func TestParseRejectsWrongSigningMethod(t *testing.T) {
	v, err := NewValidator("test-secret")
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	claims := Claims{
		MSISDN: "233241234567",
		Tenant: "nrg",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    Issuer,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	// Sign with alg "none" to ensure Parse rejects anything that isn't
	// HS256, not just tokens signed with the wrong key.
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	signed, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	if _, err := v.Parse(signed); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken for alg=none token, got %v", err)
	}
}

func TestParseRejectsTokenSignedWithDifferentSecret(t *testing.T) {
	issuer, err := NewValidator("secret-a")
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	verifier, err := NewValidator("secret-b")
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	token, err := issuer.IssueToken("233241234567", "nrg", time.Now())
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	if _, err := verifier.Parse(token); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken for token signed with a different secret, got %v", err)
	}
}

func TestParseRejectsEmptyToken(t *testing.T) {
	v, err := NewValidator("test-secret")
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	if _, err := v.Parse(""); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken for empty token, got %v", err)
	}
	if _, err := v.Parse("   "); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken for whitespace token, got %v", err)
	}
}

func TestBearerTokenExtractsToken(t *testing.T) {
	cases := []struct {
		header string
		want   string
	}{
		{"Bearer abc.def.ghi", "abc.def.ghi"},
		{"bearer abc.def.ghi", "abc.def.ghi"},
		{"  Bearer   abc.def.ghi  ", "abc.def.ghi"},
		{"", ""},
		{"Basic abc.def.ghi", ""},
		{"Bearer", ""},
	}
	for _, tc := range cases {
		got := BearerToken(tc.header)
		if got != tc.want {
			t.Fatalf("BearerToken(%q) = %q, want %q", tc.header, got, tc.want)
		}
	}
}

func TestBearerTokenTrimsInternalWhitespaceOnly(t *testing.T) {
	got := BearerToken("Bearer " + strings.Repeat("a", 10))
	if got != strings.Repeat("a", 10) {
		t.Fatalf("unexpected token: %q", got)
	}
}
