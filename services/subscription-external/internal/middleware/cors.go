package middleware

import (
	"github.com/valyala/fasthttp"
)

// CORSMiddleware enables Cross-Origin Resource Sharing for HTTP requests.
func CORSMiddleware(next fasthttp.RequestHandler, allowedOrigins []string) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		origin := string(ctx.Request.Header.Peek("Origin"))

		if isAllowedOrigin(origin, allowedOrigins) {
			ctx.Response.Header.Set("Access-Control-Allow-Origin", origin)
			ctx.Response.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			ctx.Response.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, X-RequestId, Accept, Origin, Access-Control-Allow-Origin, Access-Control-Allow-Methods, Cache-Control, X-Forwarded-For, User-Agent, Referer")
			ctx.Response.Header.Set("Access-Control-Allow-Credentials", "true")
		}

		// Handle preflight request for OPTIONS method
		if string(ctx.Method()) == fasthttp.MethodOptions {
			ctx.SetStatusCode(fasthttp.StatusOK)
			return
		}

		next(ctx)
	}
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
