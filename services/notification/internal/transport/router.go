package transport

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/seidu626/subscription-manager/common/auth/auth0jwt"
	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/seidu626/subscription-manager/notification/internal/handler"
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

func NewRouter(handler *handler.NotificationHandler, memberTenantLookup MemberTenantLookup) fasthttp.RequestHandler {
	admin := newAdminAccess(memberTenantLookup)
	router := func(ctx *fasthttp.RequestCtx) {
		path := string(ctx.Path())

		// Check for paths with partnerRole as a parameter
		switch {
		case strings.EqualFold(path, "/health"):
			ctx.SetContentType("application/json")
			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.WriteString(`{"status":"healthy","service":"notification","observability":{"tenant_labels":"enabled","pii_labels":"rejected"}}`)
			return
		case strings.EqualFold(path, "/api/v1/notification/list"):
			if !admin.require(ctx) {
				return
			}
			handler.ListNotifications(ctx)
		case strings.HasPrefix(path, "/api/v1/notification/mo/"):
			partnerRole := extractPartnerRole(path, "/api/v1/notification/mo/")
			if partnerRole != "" {
				ctx.SetUserValue("partnerRole", partnerRole)
				handler.MOHandler(ctx)
			} else {
				ctx.Error("PartnerRole parameter missing", fasthttp.StatusBadRequest)
			}
		case strings.HasPrefix(path, "/api/v1/notification/mt/dn/"):
			partnerRole := extractPartnerRole(path, "/api/v1/notification/mt/dn/")
			if partnerRole != "" {
				ctx.SetUserValue("partnerRole", partnerRole)
				handler.MTDNHandler(ctx)
			} else {
				ctx.Error("PartnerRole parameter missing", fasthttp.StatusBadRequest)
			}
		case strings.HasPrefix(path, "/api/v1/notification/user-optin/"):
			partnerRole := extractPartnerRole(path, "/api/v1/notification/user-optin/")
			if partnerRole != "" {
				ctx.SetUserValue("partnerRole", partnerRole)
				handler.UserOptinHandler(ctx)
			} else {
				ctx.Error("PartnerRole parameter missing", fasthttp.StatusBadRequest)
			}
		case strings.HasPrefix(path, "/api/v1/notification/user-renewed/"):
			partnerRole := extractPartnerRole(path, "/api/v1/notification/user-renewed/")
			if partnerRole != "" {
				ctx.SetUserValue("partnerRole", partnerRole)
				handler.UserRenewedHandler(ctx)
			} else {
				ctx.Error("PartnerRole parameter missing", fasthttp.StatusBadRequest)
			}
		case strings.HasPrefix(path, "/api/v1/notification/user-optout/"):
			partnerRole := extractPartnerRole(path, "/api/v1/notification/user-optout/")
			if partnerRole != "" {
				ctx.SetUserValue("partnerRole", partnerRole)
				handler.UserOptoutHandler(ctx)
			} else {
				ctx.Error("PartnerRole parameter missing", fasthttp.StatusBadRequest)
			}
		case strings.HasPrefix(path, "/api/v1/notification/charge/"):
			partnerRole := extractPartnerRole(path, "/api/v1/notification/charge/")
			if partnerRole != "" {
				ctx.SetUserValue("partnerRole", partnerRole)
				handler.ChargeHandler(ctx)
			} else {
				ctx.Error("PartnerRole parameter missing", fasthttp.StatusBadRequest)
			}
		default:
			log.Printf("Processing unknown request: method=%s path=%s", ctx.Method(), ctx.Path())
			ctx.Error("Not Found", fasthttp.StatusNotFound)
		}
	}
	return router
}

// Helper function to extract the partnerRole from the path
func extractPartnerRole(path, prefix string) string {
	if len(path) > len(prefix) {
		// Extract the partnerRole directly after the prefix
		partnerRole := path[len(prefix):]
		// Check if the extracted part is a valid number
		if _, err := strconv.Atoi(partnerRole); err == nil {
			return partnerRole
		}
	}
	return ""
}

type adminAccess struct {
	validator                 *auth0jwt.Validator
	bootstrapPlatformSubjects map[string]struct{}
	memberLookup              MemberTenantLookup
}

func newAdminAccess(memberLookup MemberTenantLookup) *adminAccess {
	validator, err := auth0jwt.New(os.Getenv("ADMIN_AUTH0_DOMAIN"), os.Getenv("ADMIN_AUTH0_AUDIENCE"))
	if err != nil {
		validator = nil
	}
	return &adminAccess{
		validator:                 validator,
		bootstrapPlatformSubjects: bootstrapPlatformSubjectSet(os.Getenv("ADMIN_BOOTSTRAP_PLATFORM_SUBJECTS")),
		memberLookup:              memberLookup,
	}
}

func (a *adminAccess) require(ctx *fasthttp.RequestCtx) bool {
	if a.validator == nil {
		ctx.Error("Admin access not configured", fasthttp.StatusServiceUnavailable)
		return false
	}
	claims, err := a.validator.ValidateBearer(context.Background(), string(ctx.Request.Header.Peek("Authorization")))
	if err != nil {
		log.Printf("admin auth failed (notification): remote_ip=%s err=%v", ctx.RemoteIP(), err)
		ctx.Error("Unauthorized", fasthttp.StatusUnauthorized)
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
		log.Printf("admin tenant resolution failed (notification): subject=%s err=%v", subject, err)
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
	if len(a.bootstrapPlatformSubjects) == 0 {
		return identity
	}
	subject := strings.TrimSpace(identity.Subject)
	if _, ok := a.bootstrapPlatformSubjects[subject]; !ok {
		return identity
	}
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
