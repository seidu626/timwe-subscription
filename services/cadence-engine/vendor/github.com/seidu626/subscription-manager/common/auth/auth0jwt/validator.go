package auth0jwt

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v2"
	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidConfig  = errors.New("auth0 jwt validator not configured")
	ErrInvalidToken   = errors.New("invalid token")
	ErrMissingToken   = errors.New("missing token")
	ErrUnsupportedAlg = errors.New("unsupported token signing algorithm")
)

type Validator struct {
	keyFunc  jwt.Keyfunc
	issuer   string
	audience string
	parser   *jwt.Parser
}

func New(domain, audience string) (*Validator, error) {
	domain = strings.TrimSpace(domain)
	audience = strings.TrimSpace(audience)
	if domain == "" || audience == "" {
		return nil, ErrInvalidConfig
	}

	issuer := fmt.Sprintf("https://%s/", domain)
	jwksURL := fmt.Sprintf("https://%s/.well-known/jwks.json", domain)

	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	jwks, err := keyfunc.Get(jwksURL, keyfunc.Options{
		Client:            httpClient,
		RefreshInterval:   12 * time.Hour,
		RefreshRateLimit:  5 * time.Minute,
		RefreshTimeout:    10 * time.Second,
		RefreshUnknownKID: true,
	})
	if err != nil {
		return nil, fmt.Errorf("init jwks: %w", err)
	}

	return &Validator{
		keyFunc:  jwks.Keyfunc,
		issuer:   issuer,
		audience: audience,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{"RS256"}),
			jwt.WithIssuedAt(),
		),
	}, nil
}

// ValidateBearer validates an `Authorization: Bearer <token>` header value.
// It returns registered claims on success.
func (v *Validator) ValidateBearer(ctx context.Context, authorizationHeader string) (*jwt.RegisteredClaims, error) {
	if v == nil || v.keyFunc == nil || v.parser == nil || v.issuer == "" || v.audience == "" {
		return nil, ErrInvalidConfig
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

	claims := &jwt.RegisteredClaims{}

	// The jwt library doesn't currently accept context directly for ParseWithClaims,
	// so we pre-check cancellation here and keep parsing bounded by keyfunc/http timeouts.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	token, err := v.parser.ParseWithClaims(tokenString, claims, v.keyFunc)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if token == nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	// Enforce issuer and audience explicitly.
	if claims.Issuer != v.issuer {
		return nil, ErrInvalidToken
	}
	audienceOK := false
	for _, aud := range claims.Audience {
		if aud == v.audience {
			audienceOK = true
			break
		}
	}
	if !audienceOK {
		return nil, ErrInvalidToken
	}

	// Ensure alg is RS256 (defense-in-depth; ParseWithClaims already enforces valid methods).
	if token.Method == nil || token.Method.Alg() != "RS256" {
		return nil, ErrUnsupportedAlg
	}

	return claims, nil
}
