package handler

// batch_admin_auth.go — admin identity validation for batch/backfill/resubscribe endpoints.
//
// Auth rules (FIX 4):
//  (a) Authorization: Bearer <token>  →  validate via Auth0 JWKS.
//      Tenant-scoped identity  →  may only operate on their own tenant (404 on mismatch).
//      Platform-scoped identity  →  may operate on any tenant.
//  (b) No bearer but valid X-Internal-Signature / X-Internal-Timestamp HMAC
//      (acquisition-api pattern)  →  accepted as internal caller (CLI batch-processor,
//      resubscribe-processor); tenant_key in body is accepted as-is.
//  (c) Neither  →  401 Unauthorized.

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"time"

	"github.com/seidu626/subscription-manager/common/auth/auth0jwt"
	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/valyala/fasthttp"
)

// batchAdminGuard validates caller identity for batch/backfill/resubscribe endpoints.
// It is constructed once at handler creation time.
type batchAdminGuard struct {
	jwtValidator   *auth0jwt.Validator // nil when ADMIN_AUTH0_DOMAIN/AUDIENCE not set
	internalSecret string              // INTERNAL_API_SECRET; empty disables HMAC path
}

func newBatchAdminGuard() *batchAdminGuard {
	domain := strings.TrimSpace(os.Getenv("ADMIN_AUTH0_DOMAIN"))
	audience := strings.TrimSpace(os.Getenv("ADMIN_AUTH0_AUDIENCE"))

	var v *auth0jwt.Validator
	if domain != "" && audience != "" {
		var err error
		v, err = auth0jwt.New(domain, audience)
		if err != nil {
			v = nil // misconfigured; fall through to HMAC only
		}
	}

	secret := strings.TrimSpace(os.Getenv("INTERNAL_API_SECRET"))

	return &batchAdminGuard{
		jwtValidator:   v,
		internalSecret: secret,
	}
}

// authorise validates the request and returns the resolved identity.
// On failure it writes the appropriate error to ctx and returns ok=false.
//
// requestedTenantKey is the tenant_key the caller wants to act on (from the
// request body or query string); it is validated against the identity when the
// caller is tenant-scoped.
func (g *batchAdminGuard) authorise(ctx *fasthttp.RequestCtx, requestedTenantKey string) (identity tenantctx.Identity, ok bool) {
	authHeader := strings.TrimSpace(string(ctx.Request.Header.Peek("Authorization")))

	// --- path (a): JWT bearer ---
	if strings.HasPrefix(authHeader, "Bearer ") {
		if g.jwtValidator == nil {
			ctx.Error("Admin access not configured", fasthttp.StatusServiceUnavailable)
			return identity, false
		}
		claims, err := g.jwtValidator.ValidateBearer(context.Background(), authHeader)
		if err != nil {
			ctx.Error("Unauthorized", fasthttp.StatusUnauthorized)
			return identity, false
		}
		identity = claims.Identity()
		if !identity.PlatformScoped && !matchesTenant(identity, requestedTenantKey) {
			// Do not reveal job existence to wrong tenant.
			ctx.Error("Not Found", fasthttp.StatusNotFound)
			return identity, false
		}
		return identity, true
	}

	// --- path (b): internal HMAC ---
	if g.internalSecret != "" {
		sig := strings.TrimSpace(string(ctx.Request.Header.Peek("X-Internal-Signature")))
		ts := strings.TrimSpace(string(ctx.Request.Header.Peek("X-Internal-Timestamp")))
		if sig != "" && ts != "" {
			if g.validateHMAC(sig, ts, ctx.PostBody()) {
				// Internal callers are platform-scoped; they pass tenant_key in the body.
				identity = tenantctx.Identity{
					PlatformScoped: true,
					TrustSource:    tenantctx.TrustSourceTrustedService,
				}
				return identity, true
			}
		}
	}

	// --- path (c): nothing valid ---
	ctx.Error("Unauthorized", fasthttp.StatusUnauthorized)
	return identity, false
}

// validateHMAC checks the acquisition-api pattern: HMAC-SHA256(timestamp + body).
func (g *batchAdminGuard) validateHMAC(sig, timestamp string, body []byte) bool {
	ts, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return false
	}
	if time.Since(ts).Abs() > 5*time.Minute {
		return false
	}
	message := timestamp + string(body)
	mac := hmac.New(sha256.New, []byte(g.internalSecret))
	mac.Write([]byte(message))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expected))
}

// maskMSISDN returns a masked representation of an MSISDN for safe logging.
// It keeps the first 5 and last 2 digits and replaces the middle with '*'.
// E.g. "233241234567" → "23324****67".
// If the MSISDN is too short to mask (≤7 chars) only a count is returned.
func maskMSISDN(msisdn string) string {
	const keepPrefix = 5
	const keepSuffix = 2
	n := len(msisdn)
	if n <= keepPrefix+keepSuffix {
		return "***"
	}
	masked := msisdn[:keepPrefix]
	for i := keepPrefix; i < n-keepSuffix; i++ {
		masked += "*"
	}
	masked += msisdn[n-keepSuffix:]
	return masked
}

// matchesTenant returns true when the identity's tenant key matches the
// requested tenant key (case-insensitive).
//
// Fail-closed (NF3 / S2 residual): a non-platform, non-internal identity with
// a blank TenantKey is DENIED.  An empty requestedTenantKey is always allowed
// (the job has no tenant stamp yet).
func matchesTenant(id tenantctx.Identity, requestedTenantKey string) bool {
	rk := strings.TrimSpace(requestedTenantKey)
	ik := strings.TrimSpace(id.TenantKey)
	// Blank requested key means the caller is not asserting tenant ownership; allow.
	if rk == "" {
		return true
	}
	// Blank identity key for a non-platform, non-internal identity is never allowed.
	if ik == "" {
		return false
	}
	return strings.EqualFold(ik, rk)
}
