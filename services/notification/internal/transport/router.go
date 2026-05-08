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

func NewRouter(handler *handler.NotificationHandler) fasthttp.RequestHandler {
	admin := newAdminAccess()
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
	validator *auth0jwt.Validator
}

func newAdminAccess() *adminAccess {
	validator, err := auth0jwt.New(os.Getenv("ADMIN_AUTH0_DOMAIN"), os.Getenv("ADMIN_AUTH0_AUDIENCE"))
	if err != nil {
		validator = nil
	}
	return &adminAccess{validator: validator}
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
	ctx.SetUserValue(tenantctx.FastHTTPUserValueKey, claims.Identity())
	return true
}
