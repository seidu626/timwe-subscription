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
	audiences map[string]struct{}
	parser   *jwt.Parser
}

func New(domain, audience string) (*Validator, error) {
	domain = strings.TrimSpace(domain)
	audience = strings.TrimSpace(audience)
	if domain == "" || audience == "" {
		return nil, ErrInvalidConfig
	}

	// Support comma-separated audiences to allow safe migration between API identifiers.
	// Example: "https://dev-xxx.auth0.com/api/v2/,https://api.example.com"
	audiences := make(map[string]struct{})
	for _, part := range strings.Split(audience, ",") {
		a := strings.TrimSpace(part)
		if a == "" {
			continue
		}
		audiences[a] = struct{}{}
	}
	if len(audiences) == 0 {
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
		keyFunc:   jwks.Keyfunc,
		issuer:    issuer,
		audiences: audiences,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{"RS256"}),
			jwt.WithIssuedAt(),
			// Allow 60 seconds of clock skew between token issuer and validator.
			// This prevents "token used before issued" errors due to minor clock differences.
			jwt.WithLeeway(60*time.Second),
		),
	}, nil
}

// ValidateBearer validates an `Authorization: Bearer <token>` header value.
// It returns registered claims on success.
func (v *Validator) ValidateBearer(ctx context.Context, authorizationHeader string) (*jwt.RegisteredClaims, error) {
	if v == nil || v.keyFunc == nil || v.parser == nil || v.issuer == "" || len(v.audiences) == 0 {
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
		return nil, fmt.Errorf("%w: issuer mismatch (got %q want %q)", ErrInvalidToken, claims.Issuer, v.issuer)
	}
	audienceOK := false
	for _, aud := range claims.Audience {
		if _, ok := v.audiences[aud]; ok {
			audienceOK = true
			break
		}
	}
	if !audienceOK {
		// Keep this non-sensitive: report the token audiences + expected set keys.
		expected := make([]string, 0, len(v.audiences))
		for a := range v.audiences {
			expected = append(expected, a)
		}
		return nil, fmt.Errorf("%w: audience mismatch (got %v want one of %v)", ErrInvalidToken, claims.Audience, expected)
	}

	// Ensure alg is RS256 (defense-in-depth; ParseWithClaims already enforces valid methods).
	if token.Method == nil || token.Method.Alg() != "RS256" {
		return nil, ErrUnsupportedAlg
	}

	return claims, nil
}
