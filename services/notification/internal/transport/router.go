package transport

import (
	"log"
	"strconv"
	"strings"

	"github.com/seidu626/subscription-manager/notification/internal/handler"
	"github.com/valyala/fasthttp"
)

func NewRouter(handler *handler.NotificationHandler) fasthttp.RequestHandler {
	router := func(ctx *fasthttp.RequestCtx) {
		path := string(ctx.Path())

		// Check for paths with partnerRole as a parameter
		switch {
		case strings.EqualFold(path, "/health"):
			ctx.SetContentType("application/json")
			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.WriteString(`{"status":"healthy","service":"notification"}`)
			return
		case strings.EqualFold(path, "/api/v1/notification/list"):
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
			log.Printf("Processing unknown request: %s", ctx.Request.String())
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
