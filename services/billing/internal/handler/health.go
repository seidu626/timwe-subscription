package handler

import (
	"github.com/valyala/fasthttp"
)

// HealthHandler handles health check requests for fasthttp
func HealthHandler(ctx *fasthttp.RequestCtx) {
	ctx.SetContentType("text/plain")
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetBodyString("OK")
}

// HealthCheck handles health check requests for net/http (legacy)
// Deprecated: Use HealthHandler for fasthttp
func HealthCheck(ctx *fasthttp.RequestCtx) {
	HealthHandler(ctx)
}
