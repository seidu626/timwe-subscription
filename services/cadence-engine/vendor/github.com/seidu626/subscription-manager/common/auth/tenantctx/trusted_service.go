package tenantctx

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	HeaderTenantID         = "X-Tenant-Id"
	HeaderTenantKey        = "X-Tenant-Key"
	HeaderServiceID        = "X-Service-Id"
	HeaderServiceTimestamp = "X-Service-Timestamp"
	HeaderServiceNonce     = "X-Service-Nonce"
	HeaderServiceBodySHA   = "X-Service-Body-SHA256"
	HeaderServiceSignature = "X-Service-Signature"
)

var (
	ErrMissingTrustedHeader = errors.New("missing trusted service header")
	ErrInvalidTrustedHeader = errors.New("invalid trusted service header")
	ErrTrustedHeaderExpired = errors.New("trusted service header expired")
)

type HeaderGetter interface {
	Get(name string) string
}

type TrustedHeaderOptions struct {
	Secret     string
	Now        func() time.Time
	MaxSkew    time.Duration
	NonceStore NonceStore
}

type NonceStore interface {
	Use(nonce string, expiresAt time.Time) bool
}

func (o TrustedHeaderOptions) now() time.Time {
	if o.Now != nil {
		return o.Now().UTC()
	}
	return time.Now().UTC()
}

func (o TrustedHeaderOptions) maxSkew() time.Duration {
	if o.MaxSkew > 0 {
		return o.MaxSkew
	}
	return 5 * time.Minute
}

type SignInput struct {
	Method    string
	Path      string
	Timestamp string
	Nonce     string
	ServiceID string
	TenantID  string
	TenantKey string
	BodySHA   string
}

func SignServiceContext(secret, timestamp, serviceID, tenantID string, tenantKey ...string) string {
	key := ""
	if len(tenantKey) > 0 {
		key = tenantKey[0]
	}
	return SignServiceRequest(secret, SignInput{
		Timestamp: timestamp,
		ServiceID: serviceID,
		TenantID:  tenantID,
		TenantKey: key,
	})
}

func SignServiceRequest(secret string, input SignInput) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(canonicalServiceMessage(input)))
	return hex.EncodeToString(mac.Sum(nil))
}

func IdentityFromTrustedHeaders(headers HeaderGetter, opts TrustedHeaderOptions) (Identity, error) {
	return IdentityFromTrustedRequest("", "", headers, opts)
}

func IdentityFromTrustedRequest(method, path string, headers HeaderGetter, opts TrustedHeaderOptions) (Identity, error) {
	if headers == nil {
		return Identity{}, ErrMissingTrustedHeader
	}
	secret := strings.TrimSpace(opts.Secret)
	if secret == "" {
		return Identity{}, fmt.Errorf("%w: secret not configured", ErrInvalidTrustedHeader)
	}

	tenantID := strings.TrimSpace(headers.Get(HeaderTenantID))
	tenantKey := strings.TrimSpace(headers.Get(HeaderTenantKey))
	serviceID := strings.TrimSpace(headers.Get(HeaderServiceID))
	timestamp := strings.TrimSpace(headers.Get(HeaderServiceTimestamp))
	nonce := strings.TrimSpace(headers.Get(HeaderServiceNonce))
	bodySHA := strings.TrimSpace(headers.Get(HeaderServiceBodySHA))
	signature := strings.TrimSpace(headers.Get(HeaderServiceSignature))
	if (tenantID == "" && tenantKey == "") || serviceID == "" || timestamp == "" || signature == "" {
		return Identity{}, ErrMissingTrustedHeader
	}

	ts, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return Identity{}, fmt.Errorf("%w: timestamp", ErrInvalidTrustedHeader)
	}
	if opts.now().Sub(ts.UTC()).Abs() > opts.maxSkew() {
		return Identity{}, ErrTrustedHeaderExpired
	}
	if opts.NonceStore != nil {
		if nonce == "" {
			return Identity{}, ErrMissingTrustedHeader
		}
		if !opts.NonceStore.Use(nonce, ts.UTC().Add(opts.maxSkew())) {
			return Identity{}, fmt.Errorf("%w: nonce replay", ErrInvalidTrustedHeader)
		}
	}

	expected := SignServiceRequest(secret, SignInput{
		Method:    method,
		Path:      path,
		Timestamp: timestamp,
		Nonce:     nonce,
		ServiceID: serviceID,
		TenantID:  tenantID,
		TenantKey: tenantKey,
		BodySHA:   bodySHA,
	})
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return Identity{}, fmt.Errorf("%w: signature", ErrInvalidTrustedHeader)
	}

	return Identity{
		TenantID:    tenantID,
		TenantKey:   tenantKey,
		ServiceID:   serviceID,
		TrustSource: TrustSourceTrustedService,
	}, nil
}

func Middleware(opts TrustedHeaderOptions, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, err := IdentityFromTrustedRequest(r.Method, r.URL.EscapedPath(), r.Header, opts)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), identity)))
	})
}

func BodySHA256(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func canonicalServiceMessage(input SignInput) string {
	return strings.Join([]string{
		strings.ToUpper(strings.TrimSpace(input.Method)),
		strings.TrimSpace(input.Path),
		strings.TrimSpace(input.Timestamp),
		strings.TrimSpace(input.Nonce),
		strings.TrimSpace(input.ServiceID),
		strings.TrimSpace(input.TenantID),
		strings.TrimSpace(input.TenantKey),
		strings.TrimSpace(input.BodySHA),
	}, "\n")
}
