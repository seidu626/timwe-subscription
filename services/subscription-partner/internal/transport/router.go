package transport

import (
	"github.com/seidu626/subscription-manager/subscription/internal/handler"
	"github.com/valyala/fasthttp"
	"log"
	"strconv"
	"strings"
)

func NewRouter(handler *handler.SubscriptionHandler, productHandler *handler.ProductHandler) fasthttp.RequestHandler {
	router := func(ctx *fasthttp.RequestCtx) {
		path := string(ctx.Path())

		switch {
		case strings.EqualFold(path, "/health"):
			ctx.SetContentType("application/json")
			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.WriteString(`{"status":"healthy","service":"subscription-partner"}`)
			return
		case strings.EqualFold(path, "/api/v1/products/list"):
			if string(ctx.Method()) == fasthttp.MethodGet {
				productHandler.ListProducts(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
		case strings.EqualFold(path, "/api/v1/products/batch"):
			if string(ctx.Method()) == fasthttp.MethodPost {
				productHandler.BatchCreateProducts(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
		case strings.EqualFold(path, "/api/v1/products"):
			if string(ctx.Method()) == fasthttp.MethodPost {
				productHandler.CreateProduct(ctx)
			} else if string(ctx.Method()) == fasthttp.MethodGet {
				productHandler.GetProduct(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
		case strings.EqualFold(path, "/api/v1/subscription/list"):
			handler.ListSubscriptions(ctx)
		case strings.HasPrefix(path, "/api/v1/subscription/optin/"):
			partnerRoleId := extractPartnerRoleId(path, "/api/v1/subscription/optin/")
			if partnerRoleId != "" {
				ctx.SetUserValue("partnerRoleId", partnerRoleId)
				handler.OptinHandler(ctx)
			} else {
				ctx.Error("PartnerRoleId parameter missing", fasthttp.StatusBadRequest)
			}
		case strings.HasPrefix(path, "/api/v1/subscription/optin/confirm/"):
			partnerRoleId := extractPartnerRoleId(path, "/api/v1/subscription/optin/confirm/")
			if partnerRoleId != "" {
				ctx.SetUserValue("partnerRoleId", partnerRoleId)
				handler.ConfirmHandler(ctx)
			} else {
				ctx.Error("PartnerRoleId parameter missing", fasthttp.StatusBadRequest)
			}
		case strings.HasPrefix(path, "/api/v1/subscription/optout/"):
			partnerRoleId := extractPartnerRoleId(path, "/api/v1/subscription/optout/")
			if partnerRoleId != "" {
				ctx.SetUserValue("partnerRoleId", partnerRoleId)
				handler.OptoutHandler(ctx)
			} else {
				ctx.Error("PartnerRoleId parameter missing", fasthttp.StatusBadRequest)
			}
		case strings.HasPrefix(path, "/api/v1/subscription/status/"):
			partnerRoleId := extractPartnerRoleId(path, "/api/v1/subscription/status/")
			if partnerRoleId != "" {
				ctx.SetUserValue("partnerRoleId", partnerRoleId)
				handler.StatusHandler(ctx)
			} else {
				ctx.Error("PartnerRoleId parameter missing", fasthttp.StatusBadRequest)
			}
		default:
			log.Printf("Processing unknown request: %s", ctx.Request.String())
			ctx.Error("Not Found", fasthttp.StatusNotFound)
		}
	}
	return router
}

// Helper function to extract the partnerRoleId from the path
func extractPartnerRoleId(path, prefix string) string {
	if len(path) > len(prefix) {
		partnerRoleId := path[len(prefix):]
		// Check if the extracted part is a valid number
		if _, err := strconv.Atoi(partnerRoleId); err == nil {
			return partnerRoleId
		}
	}
	return ""
}
