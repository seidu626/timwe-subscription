package tenantctx

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIdentityFromTrustedHeadersAcceptsSignedTenantContext(t *testing.T) {
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	headers := http.Header{}
	headers.Set(HeaderTenantID, "tenant-123")
	headers.Set(HeaderServiceID, "subscription-external")
	headers.Set(HeaderServiceTimestamp, now.Format(time.RFC3339))
	headers.Set(HeaderServiceSignature, SignServiceContext("secret", now.Format(time.RFC3339), "subscription-external", "tenant-123"))

	identity, err := IdentityFromTrustedHeaders(headers, TrustedHeaderOptions{
		Secret: "secret",
		Now:    func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("trusted headers rejected: %v", err)
	}
	if identity.TenantID != "tenant-123" || identity.ServiceID != "subscription-external" {
		t.Fatalf("identity = %#v", identity)
	}
	if identity.TrustSource != TrustSourceTrustedService {
		t.Fatalf("trust source = %q", identity.TrustSource)
	}
}

func TestIdentityFromTrustedHeadersRejectsForgedSignature(t *testing.T) {
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	headers := http.Header{}
	headers.Set(HeaderTenantID, "tenant-123")
	headers.Set(HeaderServiceID, "subscription-external")
	headers.Set(HeaderServiceTimestamp, now.Format(time.RFC3339))
	headers.Set(HeaderServiceSignature, "bad-signature")

	if _, err := IdentityFromTrustedHeaders(headers, TrustedHeaderOptions{
		Secret: "secret",
		Now:    func() time.Time { return now },
	}); err == nil {
		t.Fatal("expected forged signature to be rejected")
	}
}

func TestIdentityFromTrustedHeadersRejectsMissingTenant(t *testing.T) {
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	headers := http.Header{}
	headers.Set(HeaderServiceID, "subscription-external")
	headers.Set(HeaderServiceTimestamp, now.Format(time.RFC3339))
	headers.Set(HeaderServiceSignature, SignServiceContext("secret", now.Format(time.RFC3339), "subscription-external", ""))

	if _, err := IdentityFromTrustedHeaders(headers, TrustedHeaderOptions{
		Secret: "secret",
		Now:    func() time.Time { return now },
	}); err == nil {
		t.Fatal("expected missing tenant context to be rejected")
	}
}

func TestIdentityFromTrustedHeadersBindsTenantKeyToSignature(t *testing.T) {
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	headers := http.Header{}
	headers.Set(HeaderTenantKey, "tenant-a")
	headers.Set(HeaderServiceID, "gateway")
	headers.Set(HeaderServiceTimestamp, now.Format(time.RFC3339))
	headers.Set(HeaderServiceSignature, SignServiceContext("secret", now.Format(time.RFC3339), "gateway", "", "tenant-a"))

	if _, err := IdentityFromTrustedHeaders(headers, TrustedHeaderOptions{
		Secret: "secret",
		Now:    func() time.Time { return now },
	}); err != nil {
		t.Fatalf("trusted tenant key header rejected: %v", err)
	}

	headers.Set(HeaderTenantKey, "tenant-b")
	if _, err := IdentityFromTrustedHeaders(headers, TrustedHeaderOptions{
		Secret: "secret",
		Now:    func() time.Time { return now },
	}); err == nil {
		t.Fatal("expected tenant-key tampering to be rejected")
	}
}

func TestIdentityFromTrustedHeadersRejectsExpiredTimestamp(t *testing.T) {
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	old := now.Add(-10 * time.Minute)
	headers := http.Header{}
	headers.Set(HeaderTenantID, "tenant-123")
	headers.Set(HeaderServiceID, "subscription-external")
	headers.Set(HeaderServiceTimestamp, old.Format(time.RFC3339))
	headers.Set(HeaderServiceSignature, SignServiceContext("secret", old.Format(time.RFC3339), "subscription-external", "tenant-123"))

	if _, err := IdentityFromTrustedHeaders(headers, TrustedHeaderOptions{
		Secret:  "secret",
		Now:     func() time.Time { return now },
		MaxSkew: time.Minute,
	}); err == nil {
		t.Fatal("expected expired timestamp to be rejected")
	}
}

func TestMiddlewareAttachesIdentityToRequestContext(t *testing.T) {
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	req := httptest.NewRequest(http.MethodGet, "/internal", nil)
	req.Header.Set(HeaderTenantID, "tenant-123")
	req.Header.Set(HeaderServiceID, "gateway")
	req.Header.Set(HeaderServiceTimestamp, now.Format(time.RFC3339))
	req.Header.Set(HeaderServiceNonce, "nonce-1")
	req.Header.Set(HeaderServiceBodySHA, BodySHA256(nil))
	req.Header.Set(HeaderServiceSignature, SignServiceRequest("secret", SignInput{
		Method:    http.MethodGet,
		Path:      "/internal",
		Timestamp: now.Format(time.RFC3339),
		Nonce:     "nonce-1",
		ServiceID: "gateway",
		TenantID:  "tenant-123",
		BodySHA:   BodySHA256(nil),
	}))
	rr := httptest.NewRecorder()

	Middleware(TrustedHeaderOptions{
		Secret:     "secret",
		Now:        func() time.Time { return now },
		NonceStore: NewMemoryNonceStore(),
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := FromContext(r.Context())
		if !ok {
			t.Fatal("identity missing from context")
		}
		if identity.TenantID != "tenant-123" || identity.ServiceID != "gateway" {
			t.Fatalf("identity = %#v", identity)
		}
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestMiddlewareRejectsReplayNonce(t *testing.T) {
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	store := NewMemoryNonceStore()
	newRequest := func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/internal", nil)
		req.Header.Set(HeaderTenantID, "tenant-123")
		req.Header.Set(HeaderServiceID, "gateway")
		req.Header.Set(HeaderServiceTimestamp, now.Format(time.RFC3339))
		req.Header.Set(HeaderServiceNonce, "nonce-1")
		req.Header.Set(HeaderServiceBodySHA, BodySHA256(nil))
		req.Header.Set(HeaderServiceSignature, SignServiceRequest("secret", SignInput{
			Method:    http.MethodPost,
			Path:      "/internal",
			Timestamp: now.Format(time.RFC3339),
			Nonce:     "nonce-1",
			ServiceID: "gateway",
			TenantID:  "tenant-123",
			BodySHA:   BodySHA256(nil),
		}))
		return req
	}
	handler := Middleware(TrustedHeaderOptions{
		Secret:     "secret",
		Now:        func() time.Time { return now },
		NonceStore: store,
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, newRequest())
	if first.Code != http.StatusNoContent {
		t.Fatalf("first status = %d", first.Code)
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, newRequest())
	if second.Code != http.StatusUnauthorized {
		t.Fatalf("second status = %d, want %d", second.Code, http.StatusUnauthorized)
	}
}
