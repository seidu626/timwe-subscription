package tenantctx

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// HeaderGatewayTrust is the header KrakenD injects on every backend request to
// prove the request originated at the gateway.
//
// Mechanism: HMAC-SHA256 over the fixed message "gateway-trust-marker" using
// the shared GATEWAY_TRUST_SECRET. This produces a static value that KrakenD
// can inject via the martian header.Modifier without per-request scripting.
//
// ⚠ MECHANISM DOWNGRADE — READ BEFORE ENABLING ENFORCEMENT:
// The KrakenD build used here does NOT include Lua scripting (krakend-lua) or
// a custom HMAC plugin. The martian header.Modifier only supports static header
// values, so a per-request HMAC (timestamp + path) is impossible without a
// build upgrade. The deployed mechanism is therefore a static-shared-secret
// header — equivalent to a bearer token — rather than a true per-request HMAC.
//
// Operational mitigation: nginx + Docker bridge network isolation (services are
// not directly internet-exposed) plus flag-gated enforcement below.
//
// To upgrade to per-request HMAC: add krakend-lua or a custom plugin to the
// KrakenD build, then update VerifyGatewayTrust to validate
// HMAC-SHA256(timestamp + path) with a clock-skew window.
const HeaderGatewayTrust = "X-Gateway-Trust"

// gatewayTrustClockSkew is reserved for a future per-request HMAC upgrade.
const gatewayTrustClockSkew = 5 * time.Minute //nolint:unused

var (
	// ErrMissingGatewayTrustHeader is returned when X-Gateway-Trust is absent.
	ErrMissingGatewayTrustHeader = errors.New("missing gateway trust header")
	// ErrInvalidGatewayTrustHeader is returned when the header value does not match.
	ErrInvalidGatewayTrustHeader = errors.New("invalid gateway trust header")
)

// GatewayTrustOptions configures VerifyGatewayTrust.
type GatewayTrustOptions struct {
	// Secret is the shared GATEWAY_TRUST_SECRET. Must not be empty.
	Secret string
	// ClockSkew is reserved for a future per-request HMAC upgrade; unused now.
	ClockSkew time.Duration
}

// VerifyGatewayTrust verifies the X-Gateway-Trust header injected by KrakenD.
//
// Returns nil on success, ErrMissingGatewayTrustHeader when the header is
// absent, or ErrInvalidGatewayTrustHeader when the value does not match.
func VerifyGatewayTrust(headers HeaderGetter, opts GatewayTrustOptions) error {
	if headers == nil {
		return ErrMissingGatewayTrustHeader
	}
	secret := strings.TrimSpace(opts.Secret)
	if secret == "" {
		return fmt.Errorf("%w: gateway trust secret not configured", ErrInvalidGatewayTrustHeader)
	}

	value := strings.TrimSpace(headers.Get(HeaderGatewayTrust))
	if value == "" {
		return ErrMissingGatewayTrustHeader
	}

	expected := GatewayTrustToken(secret)
	if !hmac.Equal([]byte(value), []byte(expected)) {
		return ErrInvalidGatewayTrustHeader
	}
	return nil
}

// GatewayTrustToken returns the hex-encoded HMAC-SHA256 that KrakenD injects as
// the X-Gateway-Trust header. Services compare the incoming header against this.
//
// The fixed message "gateway-trust-marker" is intentional: with the current
// KrakenD build the header must be a static value derived from the secret.
func GatewayTrustToken(secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("gateway-trust-marker"))
	return hex.EncodeToString(mac.Sum(nil))
}
