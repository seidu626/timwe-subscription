package tenantctx

import (
	"errors"
	"fmt"
	"strings"
)

// ErrTenantKeyConflict is returned when a header and query parameter carry
// different (non-empty) values for the same key after case normalisation.
var ErrTenantKeyConflict = errors.New("tenant key conflict: header and query parameter disagree")

// KeyPair holds a normalised tenant_key / channel_key pair.
type KeyPair struct {
	TenantKey  string
	ChannelKey string
}

// ResolveKeyPairOptions controls resolution behaviour.
type ResolveKeyPairOptions struct {
	// GatewayTrusted must be true when the request has crossed the gateway
	// trust boundary (verified by IdentityFromTrustedRequest / Middleware).
	// When false, query-only resolution is refused.
	GatewayTrusted bool
}

// ResolveKeyPair implements the four-rule precedence contract:
//
//  1. Header wins when both header and query agree (after normalisation).
//  2. Header-vs-query conflict → ErrTenantKeyConflict with the conflicting key
//     names embedded in the message.
//  3. Query alone is accepted ONLY when the header is absent AND opts.GatewayTrusted
//     is true (i.e. the request crossed the gateway trust boundary).
//  4. Mixed-case keys are normalised to lowercase before comparison.
//
// The caller is responsible for setting GatewayTrusted based on a prior call
// to IdentityFromTrustedRequest (the existing HMAC-signed service-to-service
// trust mechanism defined in trusted_service.go).
func ResolveKeyPair(headers HeaderGetter, query KeyPair, opts ResolveKeyPairOptions) (KeyPair, error) {
	hTenant := normalise(headers.Get(HeaderTenantKey))
	hChannel := normalise(headers.Get(HeaderChannelKey))
	qTenant := normalise(query.TenantKey)
	qChannel := normalise(query.ChannelKey)

	if err := detectConflict(hTenant, qTenant, HeaderTenantKey); err != nil {
		return KeyPair{}, err
	}
	if err := detectConflict(hChannel, qChannel, HeaderChannelKey); err != nil {
		return KeyPair{}, err
	}

	// After conflict check, header value is authoritative when present.
	resolvedTenant := coalesce(hTenant, qTenant)
	resolvedChannel := coalesce(hChannel, qChannel)

	// Query-only path requires gateway trust.
	if hTenant == "" && hChannel == "" && (qTenant != "" || qChannel != "") && !opts.GatewayTrusted {
		return KeyPair{}, fmt.Errorf(
			"tenant key from query parameter is only accepted when the request originates from the gateway trust boundary",
		)
	}

	return KeyPair{TenantKey: resolvedTenant, ChannelKey: resolvedChannel}, nil
}

// normalise trims whitespace and lowercases the value.
func normalise(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

// coalesce returns the first non-empty value.
func coalesce(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// detectConflict returns ErrTenantKeyConflict when both values are non-empty
// and differ after normalisation.
func detectConflict(header, query, headerName string) error {
	if header != "" && query != "" && header != query {
		return fmt.Errorf("%w: %s header=%q query=%q", ErrTenantKeyConflict, headerName, header, query)
	}
	return nil
}
