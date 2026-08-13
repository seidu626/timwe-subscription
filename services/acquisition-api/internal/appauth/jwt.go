// Package appauth implements the Dayline mobile app's JWT session credential.
//
// This is deliberately local to acquisition-api rather than a shared
// common/ package: subscription-external needs the same HS256
// verification logic against the same DAYLINE_APP_JWT_SECRET, but is being
// implemented independently in a parallel lane. Deduplication into a shared
// package happens later at integration, not here.
package appauth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Issuer is the fixed JWT "iss" claim for Dayline app session tokens.
const Issuer = "dayline-app"

// TokenTTL is the session token lifetime (24h per the app API contract).
const TokenTTL = 24 * time.Hour

// ErrSecretNotConfigured is returned by NewValidator/IssueToken when the
// DAYLINE_APP_JWT_SECRET is empty. Callers must fail closed, never fall back
// to an unsigned or default-secret token.
var ErrSecretNotConfigured = errors.New("DAYLINE_APP_JWT_SECRET is not configured")

// ErrInvalidToken is returned for any malformed, expired, or wrong-issuer token.
var ErrInvalidToken = errors.New("invalid or expired token")

// Claims is the Dayline app session JWT payload.
type Claims struct {
	MSISDN string `json:"sub"`
	Tenant string `json:"tenant"`
	jwt.RegisteredClaims
}

// Validator issues and verifies Dayline app session JWTs against a single
// shared secret loaded once at startup.
type Validator struct {
	secret []byte
}

// NewValidator builds a Validator from the raw secret. Returns
// ErrSecretNotConfigured if secret is empty so callers fail closed at
// startup rather than silently accepting unsigned/forged tokens.
func NewValidator(secret string) (*Validator, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, ErrSecretNotConfigured
	}
	return &Validator{secret: []byte(secret)}, nil
}

// IssueToken mints a signed session token for the given msisdn/tenant pair.
func (v *Validator) IssueToken(msisdn, tenantKey string, now time.Time) (string, error) {
	claims := Claims{
		MSISDN: msisdn,
		Tenant: tenantKey,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(TokenTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(v.secret)
	if err != nil {
		return "", fmt.Errorf("failed to sign app session token: %w", err)
	}
	return signed, nil
}

// Parse validates a bearer token string and returns its claims.
// It enforces HS256, issuer, and expiry; any failure collapses to
// ErrInvalidToken so callers never leak parsing internals to clients.
func (v *Validator) Parse(tokenString string) (*Claims, error) {
	tokenString = strings.TrimSpace(tokenString)
	if tokenString == "" {
		return nil, ErrInvalidToken
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return v.secret, nil
	}, jwt.WithIssuer(Issuer), jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}))
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	if strings.TrimSpace(claims.MSISDN) == "" || strings.TrimSpace(claims.Tenant) == "" {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// BearerToken extracts the token from an "Authorization: Bearer <token>" header value.
func BearerToken(headerValue string) string {
	const prefix = "Bearer "
	headerValue = strings.TrimSpace(headerValue)
	if len(headerValue) > len(prefix) && strings.EqualFold(headerValue[:len(prefix)], prefix) {
		return strings.TrimSpace(headerValue[len(prefix):])
	}
	return ""
}
