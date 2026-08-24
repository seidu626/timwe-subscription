package transport

import (
	"testing"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/handler"
)

func TestSetAppCORSEchoesOriginAndAllowsAppMethods(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("Origin", "http://localhost:8090")

	setAppCORS(ctx)

	if got := string(ctx.Response.Header.Peek("Access-Control-Allow-Origin")); got != "http://localhost:8090" {
		t.Errorf("Allow-Origin = %q", got)
	}
	if got := string(ctx.Response.Header.Peek("Access-Control-Allow-Methods")); got != "GET, POST, DELETE, OPTIONS" {
		t.Errorf("Allow-Methods = %q", got)
	}
	if got := string(ctx.Response.Header.Peek("Access-Control-Allow-Headers")); got != "Content-Type, Authorization" {
		t.Errorf("Allow-Headers = %q", got)
	}
}

func TestSetAppCORSWildcardWithoutOrigin(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	setAppCORS(ctx)
	if got := string(ctx.Response.Header.Peek("Access-Control-Allow-Origin")); got != "*" {
		t.Errorf("Allow-Origin = %q, want *", got)
	}
}

// appTestRouter builds the real router with only the app handler populated.
// The AppHandler carries a nil JWT validator, so authenticated app routes that
// the router actually dispatches respond 401 (fail-closed) without touching
// any service; that 401 is the proof the route reached the handler.
func appTestRouter() fasthttp.RequestHandler {
	appHandler := handler.NewAppHandler(nil, nil, nil, nil, zap.NewNop())
	return NewRouter(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, appHandler, nil)
}

func runAppRequest(t *testing.T, router fasthttp.RequestHandler, method, uri string) *fasthttp.RequestCtx {
	t.Helper()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI(uri)
	ctx.Request.Header.SetMethod(method)
	router(ctx)
	return ctx
}

func TestRouter_AppPreflightReturns204WithCORS(t *testing.T) {
	router := appTestRouter()
	ctx := runAppRequest(t, router, fasthttp.MethodOptions, "/v1/app/subscriptions")
	if got := ctx.Response.StatusCode(); got != fasthttp.StatusNoContent {
		t.Fatalf("status = %d, want 204", got)
	}
	if got := string(ctx.Response.Header.Peek("Access-Control-Allow-Methods")); got == "" {
		t.Error("missing Access-Control-Allow-Methods on preflight")
	}
}

func TestRouter_AppRoutesRejectWrongMethodsWithCORS(t *testing.T) {
	router := appTestRouter()
	cases := []struct {
		name, method, uri string
	}{
		{"otp request GET", fasthttp.MethodGet, "/v1/app/auth/otp/request"},
		{"otp verify DELETE", fasthttp.MethodDelete, "/v1/app/auth/otp/verify"},
		{"catalog POST", fasthttp.MethodPost, "/v1/app/catalog"},
		{"subscriptions PUT", fasthttp.MethodPut, "/v1/app/subscriptions"},
		{"confirm GET", fasthttp.MethodGet, "/v1/app/subscriptions/tx-1/confirm"},
		{"subscription item POST", fasthttp.MethodPost, "/v1/app/subscriptions/tx-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := runAppRequest(t, router, tc.method, tc.uri)
			if got := ctx.Response.StatusCode(); got != fasthttp.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want 405", got)
			}
			if got := string(ctx.Response.Header.Peek("Access-Control-Allow-Origin")); got == "" {
				t.Error("405 response missing app CORS headers")
			}
		})
	}
}

func TestRouter_AppAuthenticatedRoutesDispatchToHandler(t *testing.T) {
	// 401 (nil-validator fail-closed) rather than 404/405 proves the router
	// matched the route and invoked the app handler.
	router := appTestRouter()
	cases := []struct {
		name, method, uri string
	}{
		{"create subscription", fasthttp.MethodPost, "/v1/app/subscriptions"},
		{"list subscriptions", fasthttp.MethodGet, "/v1/app/subscriptions"},
		{"confirm subscription", fasthttp.MethodPost, "/v1/app/subscriptions/tx-1/confirm"},
		{"cancel subscription", fasthttp.MethodDelete, "/v1/app/subscriptions/tx-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := runAppRequest(t, router, tc.method, tc.uri)
			if got := ctx.Response.StatusCode(); got != fasthttp.StatusUnauthorized {
				t.Fatalf("status = %d, want 401 (dispatched, fail-closed)", got)
			}
		})
	}
}

func TestRouter_UnknownAppPathFallsThroughTo404(t *testing.T) {
	router := appTestRouter()
	ctx := runAppRequest(t, router, fasthttp.MethodGet, "/v1/app/unknown")
	if got := ctx.Response.StatusCode(); got != fasthttp.StatusNotFound {
		t.Fatalf("status = %d, want 404", got)
	}
}
