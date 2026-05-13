package tenantctx

import (
	"errors"
	"net/http"
	"testing"
)

// httpHeaderGetter wraps http.Header so it satisfies HeaderGetter in tests.
type httpHeaderGetter struct {
	h http.Header
}

func (g httpHeaderGetter) Get(name string) string { return g.h.Get(name) }

// emptyHeaders returns a HeaderGetter that returns "" for every key.
func emptyHeaders() HeaderGetter { return httpHeaderGetter{h: http.Header{}} }

func headersWithTenantKey(tenantKey, channelKey string) HeaderGetter {
	h := http.Header{}
	if tenantKey != "" {
		h.Set(HeaderTenantKey, tenantKey)
	}
	if channelKey != "" {
		h.Set(HeaderChannelKey, channelKey)
	}
	return httpHeaderGetter{h: h}
}

// Case 1: Header wins when both header and query agree (case-insensitive).
func TestResolveKeyPair_HeaderWinsWhenBothAgree(t *testing.T) {
	headers := headersWithTenantKey("Careerify", "web-gh-mobplus")
	query := KeyPair{TenantKey: "careerify", ChannelKey: "WEB-GH-MOBPLUS"}
	got, err := ResolveKeyPair(headers, query, ResolveKeyPairOptions{GatewayTrusted: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Normalised header value returned
	if got.TenantKey != "careerify" {
		t.Errorf("TenantKey = %q, want %q", got.TenantKey, "careerify")
	}
	if got.ChannelKey != "web-gh-mobplus" {
		t.Errorf("ChannelKey = %q, want %q", got.ChannelKey, "web-gh-mobplus")
	}
}

// Case 2: Conflict between header and query returns ErrTenantKeyConflict.
func TestResolveKeyPair_ConflictReturnsError(t *testing.T) {
	tests := []struct {
		name    string
		headers HeaderGetter
		query   KeyPair
	}{
		{
			name:    "tenant key conflict",
			headers: headersWithTenantKey("careerify", ""),
			query:   KeyPair{TenantKey: "other-tenant"},
		},
		{
			name:    "channel key conflict",
			headers: headersWithTenantKey("", "web-gh-mobplus"),
			query:   KeyPair{ChannelKey: "sms-gh-mobplus"},
		},
		{
			name:    "both conflict",
			headers: headersWithTenantKey("careerify", "web-gh-mobplus"),
			query:   KeyPair{TenantKey: "other", ChannelKey: "other-ch"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ResolveKeyPair(tc.headers, tc.query, ResolveKeyPairOptions{GatewayTrusted: true})
			if !errors.Is(err, ErrTenantKeyConflict) {
				t.Fatalf("want ErrTenantKeyConflict, got %v", err)
			}
		})
	}
}

// Case 3: Query alone is accepted ONLY when header absent AND gateway trusted.
func TestResolveKeyPair_QueryAloneAcceptedOnlyWithGatewayTrust(t *testing.T) {
	query := KeyPair{TenantKey: "careerify", ChannelKey: "web-gh-mobplus"}

	// Trusted gateway: accepted.
	got, err := ResolveKeyPair(emptyHeaders(), query, ResolveKeyPairOptions{GatewayTrusted: true})
	if err != nil {
		t.Fatalf("expected success with GatewayTrusted=true, got %v", err)
	}
	if got.TenantKey != "careerify" {
		t.Errorf("TenantKey = %q, want %q", got.TenantKey, "careerify")
	}

	// Untrusted gateway: refused.
	_, err = ResolveKeyPair(emptyHeaders(), query, ResolveKeyPairOptions{GatewayTrusted: false})
	if err == nil {
		t.Fatal("expected error with GatewayTrusted=false, got nil")
	}
}

// Case 4: Mixed-case keys are normalised — Careerify and careerify are the same.
func TestResolveKeyPair_CaseNormalisedBeforeComparison(t *testing.T) {
	// Mixed-case header and query that agree after normalisation: no conflict.
	headers := headersWithTenantKey("CAREERIFY", "WEB-GH-MOBPLUS")
	query := KeyPair{TenantKey: "careerify", ChannelKey: "web-gh-mobplus"}
	got, err := ResolveKeyPair(headers, query, ResolveKeyPairOptions{GatewayTrusted: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TenantKey != "careerify" {
		t.Errorf("TenantKey = %q, want %q", got.TenantKey, "careerify")
	}
	if got.ChannelKey != "web-gh-mobplus" {
		t.Errorf("ChannelKey = %q, want %q", got.ChannelKey, "web-gh-mobplus")
	}
}

// Header-only path: no query params, no gateway trust required.
func TestResolveKeyPair_HeaderOnly_NoTrustRequired(t *testing.T) {
	headers := headersWithTenantKey("nrg", "sms-gh")
	got, err := ResolveKeyPair(headers, KeyPair{}, ResolveKeyPairOptions{GatewayTrusted: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TenantKey != "nrg" || got.ChannelKey != "sms-gh" {
		t.Errorf("got %+v, want {nrg sms-gh}", got)
	}
}

// Conflict error message names the conflicting keys.
func TestResolveKeyPair_ConflictErrorNamesKeys(t *testing.T) {
	headers := headersWithTenantKey("careerify", "")
	query := KeyPair{TenantKey: "other"}
	_, err := ResolveKeyPair(headers, query, ResolveKeyPairOptions{GatewayTrusted: true})
	if err == nil {
		t.Fatal("want error")
	}
	msg := err.Error()
	if msg == "" {
		t.Fatal("error message is empty")
	}
	// Message should mention the header name and both values.
	for _, want := range []string{HeaderTenantKey, "careerify", "other"} {
		found := false
		for i := 0; i+len(want) <= len(msg); i++ {
			if msg[i:i+len(want)] == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("error message %q does not contain %q", msg, want)
		}
	}
}
