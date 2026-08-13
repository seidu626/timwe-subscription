// Package appauth implements the Dayline mobile app's HS256 bearer-token
// validation, per docs/dayline-app-api-contract.md.
//
// This validator is intentionally local to subscription-external. It is NOT
// shared via common/ - acquisition-api implements the same contract
// independently in its own service; deduplication happens later at
// integration, not in this lane.
package appauth

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Issuer is the required `iss` claim for Dayline app tokens.
const Issuer = "dayline-app"

// EnvSecretKey is the environment variable holding the shared HS256 secret.
const EnvSecretKey = "DAYLINE_APP_JWT_SECRET"

var (
	// ErrNotConfigured is returned when the validator has no secret (env
	// unset). Callers MUST fail closed: reject every request rather than
	// skip authentication.
	ErrNotConfigured = errors.New("appauth: validator not configured")
	ErrMissingToken  = errors.New("appauth: missing bearer token")
	ErrInvalidToken  = errors.New("appauth: invalid token")
)

// Claims are the Dayline app JWT claims: sub = msisdn, tenant = tenant_key,
// iss = "dayline-app", exp (24h), iat.
type Claims struct {
	jwt.RegisteredClaims
	Tenant string `json:"tenant"`
}

// Validator validates HS256 Dayline app bearer tokens.
type Validator struct {
	secret []byte
	parser *jwt.Parser
}

// New builds a Validator from an explicit secret. Returns ErrNotConfigured
// for an empty secret.
func New(secret string) (*Validator, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, ErrNotConfigured
	}
	return &Validator{
		secret: []byte(secret),
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{"HS256"}),
			jwt.WithIssuer(Issuer),
			jwt.WithIssuedAt(),
			// Tolerate minor clock skew between token issuer and validator.
			jwt.WithLeeway(60*time.Second),
		),
	}, nil
}

// NewFromEnv builds a Validator from DAYLINE_APP_JWT_SECRET. Returns
// ErrNotConfigured when the env var is unset or blank so callers can fail
// closed instead of silently accepting unauthenticated requests.
func NewFromEnv() (*Validator, error) {
	return New(os.Getenv(EnvSecretKey))
}

// ValidateBearer validates an `Authorization: Bearer <jwt>` header value and
// returns the decoded claims on success.
func (v *Validator) ValidateBearer(authorizationHeader string) (*Claims, error) {
	if v == nil || len(v.secret) == 0 || v.parser == nil {
		return nil, ErrNotConfigured
	}

	auth := strings.TrimSpace(authorizationHeader)
	if auth == "" {
		return nil, ErrMissingToken
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return nil, ErrMissingToken
	}
	tokenString := strings.TrimSpace(strings.TrimPrefix(auth, prefix))
	if tokenString == "" {
		return nil, ErrMissingToken
	}

	claims := &Claims{}
	token, err := v.parser.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		return v.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if token == nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	if strings.TrimSpace(claims.Subject) == "" {
		return nil, fmt.Errorf("%w: missing sub claim", ErrInvalidToken)
	}
	if strings.TrimSpace(claims.Tenant) == "" {
		return nil, fmt.Errorf("%w: missing tenant claim", ErrInvalidToken)
	}
	if token.Method == nil || token.Method.Alg() != "HS256" {
		return nil, fmt.Errorf("%w: unsupported signing algorithm", ErrInvalidToken)
	}

	return claims, nil
}
