package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// newInternalTestHandler builds an InternalHandler with a fixed secret so
// tests can compute valid signatures deterministically. A nil
// *service.TransactionService is safe here because auth is validated
// before the service is ever touched.
func newInternalTestHandler(t *testing.T) *InternalHandler {
	t.Setenv("INTERNAL_API_SECRET", "test-internal-secret")
	return NewInternalHandler(nil, zap.NewNop())
}

func signInternalRequest(secret, timestamp, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + body))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestHandlePartnerSubscription_RejectsMissingAuthHeaders(t *testing.T) {
	h := newInternalTestHandler(t)
	var ctx fasthttp.RequestCtx
	ctx.Request.SetRequestURI("/internal/acquisition/partner-subscription")
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetBodyString(`{"tenant_id":"11111111-1111-1111-1111-111111111111","msisdn":"233241234567","action":"optin"}`)

	h.HandlePartnerSubscription(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusUnauthorized {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
}

func TestHandlePartnerSubscription_RejectsInvalidSignature(t *testing.T) {
	h := newInternalTestHandler(t)
	body := `{"tenant_id":"11111111-1111-1111-1111-111111111111","msisdn":"233241234567","action":"optin"}`
	timestamp := time.Now().UTC().Format(time.RFC3339)

	var ctx fasthttp.RequestCtx
	ctx.Request.SetRequestURI("/internal/acquisition/partner-subscription")
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.Header.Set("X-Internal-Timestamp", timestamp)
	ctx.Request.Header.Set("X-Internal-Signature", "not-the-right-signature")
	ctx.Request.SetBodyString(body)

	h.HandlePartnerSubscription(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusUnauthorized {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
}

func TestHandlePartnerSubscription_RejectsExpiredTimestamp(t *testing.T) {
	h := newInternalTestHandler(t)
	body := `{"tenant_id":"11111111-1111-1111-1111-111111111111","msisdn":"233241234567","action":"optin"}`
	timestamp := time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339)
	signature := signInternalRequest("test-internal-secret", timestamp, body)

	var ctx fasthttp.RequestCtx
	ctx.Request.SetRequestURI("/internal/acquisition/partner-subscription")
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.Header.Set("X-Internal-Timestamp", timestamp)
	ctx.Request.Header.Set("X-Internal-Signature", signature)
	ctx.Request.SetBodyString(body)

	h.HandlePartnerSubscription(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusUnauthorized {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
}

func TestHandlePartnerSubscription_ValidAuthRejectsMissingRequiredFields(t *testing.T) {
	h := newInternalTestHandler(t)
	body := `{"action":"optin","msisdn":"233241234567"}` // missing tenant_id
	timestamp := time.Now().UTC().Format(time.RFC3339)
	signature := signInternalRequest("test-internal-secret", timestamp, body)

	var ctx fasthttp.RequestCtx
	ctx.Request.SetRequestURI("/internal/acquisition/partner-subscription")
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.Header.Set("X-Internal-Timestamp", timestamp)
	ctx.Request.Header.Set("X-Internal-Signature", signature)
	ctx.Request.SetBodyString(body)

	h.HandlePartnerSubscription(&ctx)

	// Authentication passes; the missing tenant_id must be caught before
	// the (nil in this test) transaction service is ever invoked.
	if ctx.Response.StatusCode() != fasthttp.StatusBadRequest {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	if !strings.Contains(string(ctx.Response.Body()), "tenant_id is required") {
		t.Fatalf("expected tenant_id required error, got %q", ctx.Response.Body())
	}
}
