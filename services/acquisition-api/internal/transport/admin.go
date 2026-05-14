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

// MemberTenant is a minimal active-membership record used to stamp tenant
// context for non-platform admins whose JWT carries no tenant claim.
type MemberTenant struct {
	ID        string
	TenantKey string
}

// MemberTenantLookup returns the active tenant memberships for an Auth0
// subject and email. The membership table is the source of truth — the
// gate never trusts a tenant header for non-platform identities.
type MemberTenantLookup func(auth0Subject, email string) ([]MemberTenant, error)

type adminAccess struct {
	validator                 *auth0jwt.Validator
	allowedOrigins            []string
	bootstrapPlatformEmails   map[string]struct{}
	bootstrapPlatformSubjects map[string]struct{}
	memberLookup              MemberTenantLookup
}

func newAdminAccess(memberLookup MemberTenantLookup) *adminAccess {
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
		validator:                 validator,
		allowedOrigins:            allowed,
		bootstrapPlatformEmails:   bootstrapPlatformEmailSet(os.Getenv("ADMIN_BOOTSTRAP_PLATFORM_EMAILS")),
		bootstrapPlatformSubjects: bootstrapPlatformSubjectSet(os.Getenv("ADMIN_BOOTSTRAP_PLATFORM_SUBJECTS")),
		memberLookup:              memberLookup,
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
	ctx.Response.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Tenant-Id, X-Tenant-Key")
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
	identity := claims.Identity()
	identity = a.applyBootstrapPlatformScope(identity)
	identity = a.applyMembershipTenantContext(ctx, identity)
	identity = a.applySelectedTenantContext(ctx, identity)
	ctx.SetUserValue(tenantctx.FastHTTPUserValueKey, identity)
	return true
}

// applyMembershipTenantContext stamps Identity.TenantID/TenantKey from the
// active-membership table when a non-platform identity arrives without a
// tenant claim. Single membership is auto-stamped; multiple memberships
// require the request's X-Tenant-Key header to match one of them, which
// the membership table validates — the header itself is never trusted.
func (a *adminAccess) applyMembershipTenantContext(ctx *fasthttp.RequestCtx, identity tenantctx.Identity) tenantctx.Identity {
	if identity.PlatformScoped || identity.HasTenant() || a.memberLookup == nil {
		return identity
	}
	subject := strings.TrimSpace(identity.Subject)
	email := strings.TrimSpace(strings.ToLower(identity.Email))
	if identity.EmailVerifiedSet && !identity.EmailVerified {
		email = ""
	}
	if subject == "" && email == "" {
		return identity
	}
	memberships, err := a.memberLookup(subject, email)
	if err != nil {
		log.Printf("admin tenant resolution failed (acquisition-api): subject=%s err=%v", subject, err)
		return identity
	}
	if len(memberships) == 0 {
		return identity
	}
	if len(memberships) == 1 {
		identity.TenantID = memberships[0].ID
		identity.TenantKey = memberships[0].TenantKey
		return identity
	}
	selected := strings.TrimSpace(string(ctx.Request.Header.Peek(tenantctx.HeaderTenantKey)))
	if selected == "" {
		return identity
	}
	for _, m := range memberships {
		if strings.EqualFold(m.TenantKey, selected) {
			identity.TenantID = m.ID
			identity.TenantKey = m.TenantKey
			return identity
		}
	}
	return identity
}

func (a *adminAccess) applyBootstrapPlatformScope(identity tenantctx.Identity) tenantctx.Identity {
	if identity.PlatformScoped {
		return identity
	}

	if len(a.bootstrapPlatformSubjects) > 0 {
		subject := strings.TrimSpace(identity.Subject)
		if _, ok := a.bootstrapPlatformSubjects[subject]; ok {
			return grantPlatformScope(identity)
		}
	}

	if len(a.bootstrapPlatformEmails) > 0 {
		if identity.EmailVerifiedSet && !identity.EmailVerified {
			return identity
		}
		email := strings.TrimSpace(strings.ToLower(identity.Email))
		if _, ok := a.bootstrapPlatformEmails[email]; ok {
			return grantPlatformScope(identity)
		}
	}

	return identity
}

func grantPlatformScope(identity tenantctx.Identity) tenantctx.Identity {
	identity.PlatformScoped = true
	if !identity.HasPermission("platform:all_tenants") {
		identity.Permissions = append(identity.Permissions, "platform:all_tenants")
	}
	return identity
}

func (a *adminAccess) applySelectedTenantContext(ctx *fasthttp.RequestCtx, identity tenantctx.Identity) tenantctx.Identity {
	if !identity.PlatformScoped {
		return identity
	}
	if tenantID := strings.TrimSpace(string(ctx.Request.Header.Peek(tenantctx.HeaderTenantID))); tenantID != "" {
		identity.TenantID = tenantID
	}
	if tenantKey := strings.TrimSpace(string(ctx.Request.Header.Peek(tenantctx.HeaderTenantKey))); tenantKey != "" {
		identity.TenantKey = tenantKey
	}
	return identity
}

func bootstrapPlatformEmailSet(raw string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, email := range strings.Split(raw, ",") {
		normalized := strings.TrimSpace(strings.ToLower(email))
		if normalized != "" {
			out[normalized] = struct{}{}
		}
	}
	return out
}

func bootstrapPlatformSubjectSet(raw string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, subject := range strings.Split(raw, ",") {
		normalized := strings.TrimSpace(subject)
		if normalized != "" {
			out[normalized] = struct{}{}
		}
	}
	return out
}

// errorWithCORS sends an error response with CORS headers preserved
func (a *adminAccess) errorWithCORS(ctx *fasthttp.RequestCtx, msg string, statusCode int) {
	ctx.Response.Reset()
	a.setCORS(ctx)
	ctx.SetContentType("text/plain; charset=utf-8")
	ctx.SetStatusCode(statusCode)
	ctx.SetBodyString(msg)
}
