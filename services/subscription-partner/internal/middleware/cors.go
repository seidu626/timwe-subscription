package middleware

import (
	"github.com/valyala/fasthttp"
)

// CORSMiddleware enables Cross-Origin Resource Sharing for HTTP requests.
// CORS headers are set AFTER the handler runs to ensure they're included in error responses.
func CORSMiddleware(next fasthttp.RequestHandler, allowedOrigins []string) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		origin := string(ctx.Request.Header.Peek("Origin"))

		// Handle preflight request for OPTIONS method
		if string(ctx.Method()) == fasthttp.MethodOptions {
			if isAllowedOrigin(origin, allowedOrigins) {
				setCORSHeaders(ctx, origin)
			}
			ctx.SetStatusCode(fasthttp.StatusOK)
			return
		}

		// Call the next handler
		next(ctx)

		// Set CORS headers AFTER handler completes (ensures error responses also get CORS headers)
		if isAllowedOrigin(origin, allowedOrigins) {
			setCORSHeaders(ctx, origin)
		}
	}
}

// setCORSHeaders sets the standard CORS response headers.
func setCORSHeaders(ctx *fasthttp.RequestCtx, origin string) {
	ctx.Response.Header.Set("Access-Control-Allow-Origin", origin)
	ctx.Response.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	ctx.Response.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Tenant-Id, X-Tenant-Key, X-Requested-With, X-RequestId, Accept, Origin, Access-Control-Allow-Origin, Access-Control-Allow-Methods, Cache-Control, X-Forwarded-For, User-Agent, Referer")
	ctx.Response.Header.Set("Access-Control-Allow-Credentials", "true")
}

// isAllowedOrigin checks if the request origin is in the list of allowed origins.
func isAllowedOrigin(origin string, allowedOrigins []string) bool {
	for _, allowedOrigin := range allowedOrigins {
		if origin == allowedOrigin {
			return true
		}
	}
	return false
}
