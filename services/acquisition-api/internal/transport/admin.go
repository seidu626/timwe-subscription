package transport

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/seidu626/subscription-manager/common/auth/auth0jwt"
	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/valyala/fasthttp"
)

type adminAccess struct {
	validator      *auth0jwt.Validator
	allowedOrigins []string
}

func newAdminAccess() *adminAccess {
	domain := os.Getenv("ADMIN_AUTH0_DOMAIN")
	audience := os.Getenv("ADMIN_AUTH0_AUDIENCE")

	// If empty, admin endpoints should refuse access.
	validator, err := auth0jwt.New(domain, audience)
	if err != nil {
		validator = nil
	}

	originsEnv := os.Getenv("ACQUISITION_ADMIN_CORS_ORIGINS")
	allowed := []string{"http://localhost:4200"}
	if strings.TrimSpace(originsEnv) != "" {
		parts := strings.Split(originsEnv, ",")
		allowed = allowed[:0]
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				allowed = append(allowed, p)
			}
		}
		if len(allowed) == 0 {
			allowed = []string{"http://localhost:4200"}
		}
	}

	return &adminAccess{
		validator:      validator,
		allowedOrigins: allowed,
	}
}

func (a *adminAccess) setCORS(ctx *fasthttp.RequestCtx) {
	origin := string(ctx.Request.Header.Peek("Origin"))
	if origin == "" {
		return
	}

	allowOrigin := ""
	for _, o := range a.allowedOrigins {
		if o == "*" {
			allowOrigin = "*"
			break
		}
		if o == origin {
			allowOrigin = origin
			break
		}
	}
	if allowOrigin == "" {
		return
	}

	ctx.Response.Header.Set("Access-Control-Allow-Origin", allowOrigin)
	ctx.Response.Header.Set("Vary", "Origin")
	ctx.Response.Header.Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,OPTIONS")
	ctx.Response.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	ctx.Response.Header.Set("Access-Control-Max-Age", "600")
}

func (a *adminAccess) handlePreflight(ctx *fasthttp.RequestCtx) bool {
	if string(ctx.Method()) != fasthttp.MethodOptions {
		return false
	}
	a.setCORS(ctx)
	ctx.SetStatusCode(fasthttp.StatusNoContent)
	return true
}

func (a *adminAccess) require(ctx *fasthttp.RequestCtx) bool {
	// Auth must be configured server-side.
	if a.validator == nil {
		a.errorWithCORS(ctx, "Admin access not configured", fasthttp.StatusServiceUnavailable)
		return false
	}

	authHeader := string(ctx.Request.Header.Peek("Authorization"))
	claims, err := a.validator.ValidateBearer(context.Background(), authHeader)
	if err != nil {
		// Do not log the Authorization header/token. Log only the failure reason.
		log.Printf("admin auth failed (acquisition-api): remote_ip=%s err=%v", ctx.RemoteIP(), err)
		a.errorWithCORS(ctx, "Unauthorized", fasthttp.StatusUnauthorized)
		return false
	}
	ctx.SetUserValue(tenantctx.FastHTTPUserValueKey, claims.Identity())
	return true
}

// errorWithCORS sends an error response with CORS headers preserved
func (a *adminAccess) errorWithCORS(ctx *fasthttp.RequestCtx, msg string, statusCode int) {
	ctx.Response.Reset()
	a.setCORS(ctx)
	ctx.SetContentType("text/plain; charset=utf-8")
	ctx.SetStatusCode(statusCode)
	ctx.SetBodyString(msg)
}
