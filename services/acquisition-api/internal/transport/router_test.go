package transport

import (
	"testing"

	"github.com/valyala/fasthttp"
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
