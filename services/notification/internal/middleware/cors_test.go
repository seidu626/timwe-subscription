package middleware

import (
	"strings"
	"testing"

	"github.com/valyala/fasthttp"
)

func TestCORSMiddlewareAllowsAdminPreflight(t *testing.T) {
	called := false
	handler := CORSMiddleware(func(ctx *fasthttp.RequestCtx) {
		called = true
	}, []string{"http://localhost:4200"})

	var ctx fasthttp.RequestCtx
	ctx.Request.SetRequestURI("/api/v1/notification/list?page=1&pageSize=10")
	ctx.Request.Header.SetMethod(fasthttp.MethodOptions)
	ctx.Request.Header.Set("Origin", "http://localhost:4200")
	ctx.Request.Header.Set("Access-Control-Request-Method", "GET")
	ctx.Request.Header.Set("Access-Control-Request-Headers", "authorization,content-type")

	handler(&ctx)

	if called {
		t.Fatal("preflight should not call the wrapped route handler")
	}
	if got := ctx.Response.StatusCode(); got != fasthttp.StatusOK {
		t.Fatalf("expected 200 preflight, got %d", got)
	}
	if got := string(ctx.Response.Header.Peek("Access-Control-Allow-Origin")); got != "http://localhost:4200" {
		t.Fatalf("expected localhost allow-origin, got %q", got)
	}
	if methods := string(ctx.Response.Header.Peek("Access-Control-Allow-Methods")); !strings.Contains(methods, "GET") || !strings.Contains(methods, "OPTIONS") {
		t.Fatalf("unexpected allow methods: %q", methods)
	}
	if headers := string(ctx.Response.Header.Peek("Access-Control-Allow-Headers")); !strings.Contains(headers, "Authorization") || !strings.Contains(headers, "Content-Type") {
		t.Fatalf("unexpected allow headers: %q", headers)
	}
}
