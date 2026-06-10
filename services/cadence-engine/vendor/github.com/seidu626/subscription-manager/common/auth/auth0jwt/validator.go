package auth0jwt

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/MicahParks/keyfunc/v2"
	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidConfig   = errors.New("auth0 jwt validator not configured")
	ErrInvalidToken    = errors.New("invalid token")
	ErrJWKSUnavailable = errors.New("jwks unavailable: keys not yet loaded")
	ErrMissingToken    = errors.New("missing token")
	ErrUnsupportedAlg  = errors.New("unsupported token signing algorithm")
)

type Validator struct {
	keyFunc   jwt.Keyfunc
	issuer    string
	audiences map[string]struct{}
	parser    *jwt.Parser
	jwksReady *atomic.Bool // nil for NewWithKeyfunc validators (always ready)
}

// defaultRetryInterval is used for the background JWKS refresh when the initial
// fetch fails. Tests may inject a shorter interval via withRetryInterval.
const defaultRetryInterval = 30 * time.Second

// newOptions holds injectable parameters for New; used only by tests.
type newOptions struct {
	retryInterval time.Duration // overrides defaultRetryInterval when non-zero
}

// Option is a functional option for New.
type Option func(*newOptions)

// withRetryInterval overrides the background-retry interval used when the
// initial JWKS fetch fails. Intentionally unexported — only for testing.
func withRetryInterval(d time.Duration) Option {
	return func(o *newOptions) { o.retryInterval = d }
}

func New(domain, audience string, opts ...Option) (*Validator, error) {
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

	cfg := &newOptions{retryInterval: defaultRetryInterval}
	for _, o := range opts {
		o(cfg)
	}

	issuer := fmt.Sprintf("https://%s/", domain)
	jwksURL := fmt.Sprintf("https://%s/.well-known/jwks.json", domain)

	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	// jwksReady is set to true once keyfunc has successfully loaded at least
	// one key set from the remote JWKS endpoint.
	ready := &atomic.Bool{}

	jwks, err := keyfunc.Get(jwksURL, keyfunc.Options{
		Client:           httpClient,
		RefreshInterval:  12 * time.Hour,
		RefreshRateLimit: 5 * time.Minute,
		RefreshTimeout:   10 * time.Second,
		// Tolerate a failed initial fetch: New succeeds and the background
		// goroutine (from RefreshInterval) will heal the key set automatically.
		TolerateInitialJWKHTTPError: true,
		RefreshErrorHandler: func(err error) {
			log.Printf("auth0jwt: jwks refresh error (url=%s): %v", jwksURL, err)
		},
		RefreshUnknownKID: true,
	})
	if err != nil {
		// keyfunc.Get only errors here when TolerateInitialJWKHTTPError is
		// false or when the URL itself is structurally invalid.
		return nil, fmt.Errorf("init jwks: %w", err)
	}

	// Wrap the keyfunc to (a) track first successful key load and (b) surface
	// ErrJWKSUnavailable while the cache is still empty.
	rawKeyfunc := jwks.Keyfunc
	wrappedKeyfunc := func(token *jwt.Token) (any, error) {
		key, err := rawKeyfunc(token)
		if err == nil {
			ready.Store(true)
			return key, nil
		}
		// If we have never successfully loaded any keys, translate the error to
		// ErrJWKSUnavailable so callers can respond with 503.
		if !ready.Load() {
			return nil, fmt.Errorf("%w: %v", ErrJWKSUnavailable, err)
		}
		return nil, err
	}

	// If the initial fetch failed, the keyfunc's refresh ticker only fires every
	// 12 h. To recover quickly after a transient startup failure, launch a
	// short-interval retry goroutine that stops once keys are available.
	retryInterval := cfg.retryInterval
	go func() {
		for {
			time.Sleep(retryInterval)
			if ready.Load() {
				return
			}
			if rerr := jwks.Refresh(context.Background(), keyfunc.RefreshOptions{IgnoreRateLimit: true}); rerr != nil {
				log.Printf("auth0jwt: jwks retry failed (url=%s): %v", jwksURL, rerr)
			} else {
				// Successful refresh; mark ready and stop.
				ready.Store(true)
				log.Printf("auth0jwt: jwks loaded after retry (url=%s)", jwksURL)
				return
			}
		}
	}()

	return &Validator{
		keyFunc:   wrappedKeyfunc,
		issuer:    issuer,
		audiences: audiences,
		jwksReady: ready,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{"RS256"}),
			jwt.WithIssuedAt(),
			// Allow 60 seconds of clock skew between token issuer and validator.
			// This prevents "token used before issued" errors due to minor clock differences.
			jwt.WithLeeway(60*time.Second),
		),
	}, nil
}

func NewWithKeyfunc(domain, audience string, keyFunc jwt.Keyfunc) (*Validator, error) {
	domain = strings.TrimSpace(domain)
	audience = strings.TrimSpace(audience)
	if domain == "" || audience == "" || keyFunc == nil {
		return nil, ErrInvalidConfig
	}
	audiences := make(map[string]struct{})
	for _, part := range strings.Split(audience, ",") {
		a := strings.TrimSpace(part)
		if a != "" {
			audiences[a] = struct{}{}
		}
	}
	if len(audiences) == 0 {
		return nil, ErrInvalidConfig
	}
	return &Validator{
		keyFunc:   keyFunc,
		issuer:    fmt.Sprintf("https://%s/", domain),
		audiences: audiences,
		// jwksReady is nil: NewWithKeyfunc callers supply their own keyfunc
		// and are always considered ready.
		jwksReady: nil,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{"RS256"}),
			jwt.WithIssuedAt(),
			jwt.WithLeeway(60*time.Second),
		),
	}, nil
}

// ValidateBearer validates an `Authorization: Bearer <token>` header value.
// It returns typed tenant/platform claims on success.
func (v *Validator) ValidateBearer(ctx context.Context, authorizationHeader string) (*Claims, error) {
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

	claims := &Claims{}

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
