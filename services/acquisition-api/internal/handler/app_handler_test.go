package handler

import (
	"testing"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// TestAppHandler_NilValidator_FailsClosed proves that when
// DAYLINE_APP_JWT_SECRET is unset, appauth.NewValidator returns a nil
// *Validator, and AppHandler rejects every /v1/app/* route with 401
// UNAUTHORIZED instead of panicking. This is the fix-closed counterpart to
// acquisition-api/cmd/main.go no longer calling logger.Fatal on that error.
func TestAppHandler_NilValidator_FailsClosed(t *testing.T) {
	h := NewAppHandler(nil, nil, nil, nil, zap.NewNop())

	t.Run("CreateSubscription", func(t *testing.T) {
		var ctx fasthttp.RequestCtx
		ctx.Request.SetRequestURI("/v1/app/subscriptions")
		ctx.Request.Header.SetMethod(fasthttp.MethodPost)
		ctx.Request.SetBodyString(`{"campaign_slug":"demo"}`)

		h.CreateSubscription(&ctx)

		if ctx.Response.StatusCode() != fasthttp.StatusUnauthorized {
			t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
		}
	})

	t.Run("ListSubscriptions", func(t *testing.T) {
		var ctx fasthttp.RequestCtx
		ctx.Request.SetRequestURI("/v1/app/subscriptions")
		ctx.Request.Header.SetMethod(fasthttp.MethodGet)

		h.ListSubscriptions(&ctx)

		if ctx.Response.StatusCode() != fasthttp.StatusUnauthorized {
			t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
		}
	})

	t.Run("ConfirmSubscription", func(t *testing.T) {
		var ctx fasthttp.RequestCtx
		ctx.Request.SetRequestURI("/v1/app/subscriptions/abc/confirm")
		ctx.Request.Header.SetMethod(fasthttp.MethodPost)
		ctx.Request.SetBodyString(`{"pin":"1234"}`)

		h.ConfirmSubscription(&ctx)

		if ctx.Response.StatusCode() != fasthttp.StatusUnauthorized {
			t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
		}
	})

	t.Run("CancelSubscription", func(t *testing.T) {
		var ctx fasthttp.RequestCtx
		ctx.Request.SetRequestURI("/v1/app/subscriptions/abc")
		ctx.Request.Header.SetMethod(fasthttp.MethodDelete)

		h.CancelSubscription(&ctx)

		if ctx.Response.StatusCode() != fasthttp.StatusUnauthorized {
			t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
		}
	})
}
