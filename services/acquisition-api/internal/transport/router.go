package transport

import (
	"strings"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/handler"
	"github.com/valyala/fasthttp"
)

// NewRouter creates a new HTTP router for the acquisition API
func NewRouter(
	campaignHandler *handler.CampaignHandler,
	transactionHandler *handler.TransactionHandler,
	callbackHandler *handler.CallbackHandler,
	internalHandler *handler.InternalHandler,
	analyticsHandler *handler.AnalyticsHandler,
	reportsHandler *handler.ReportsHandler,
	postbackAdminHandler *handler.PostbackAdminHandler,
	transactionAdminHandler *handler.TransactionAdminHandler,
	clickOutHandler *handler.ClickOutHandler,
	heBootstrapHandler *handler.HEBootstrapHandler,
) fasthttp.RequestHandler {
	admin := newAdminAccess()

	router := func(ctx *fasthttp.RequestCtx) {
		path := string(ctx.Path())
		method := string(ctx.Method())

		// Admin endpoints (token-protected) + CORS preflight
		if strings.HasPrefix(path, "/v1/admin/") {
			admin.setCORS(ctx)
			if admin.handlePreflight(ctx) {
				return
			}
			if !admin.require(ctx) {
				return
			}
		}

		// Public analytics endpoints need CORS for landing-web
		if strings.HasPrefix(path, "/v1/analytics/") {
			setPublicCORS(ctx)
			if method == fasthttp.MethodOptions {
				ctx.SetStatusCode(fasthttp.StatusNoContent)
				return
			}
		}

		switch {
		// Health check
		case strings.EqualFold(path, "/health"):
			if method == fasthttp.MethodGet {
				ctx.SetContentType("application/json")
				ctx.SetStatusCode(fasthttp.StatusOK)
				ctx.WriteString(`{"status":"healthy"}`)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// HE Bootstrap endpoints (HTTP-only Header Enrichment capture)
		// These are called from HTTP (port 80) by NGINX when operator HE headers are detected
		case strings.EqualFold(path, "/v1/he/bootstrap"):
			if method == fasthttp.MethodGet {
				if heBootstrapHandler != nil {
					heBootstrapHandler.HandleBootstrap(ctx)
				} else {
					ctx.Error("HE bootstrap not configured", fasthttp.StatusNotImplemented)
				}
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		case strings.HasPrefix(path, "/v1/he/bootstrap/campaign/"):
			if method == fasthttp.MethodGet {
				if heBootstrapHandler != nil {
					heBootstrapHandler.HandleBootstrapWithCampaign(ctx)
				} else {
					ctx.Error("HE bootstrap not configured", fasthttp.StatusNotImplemented)
				}
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// HE Token Exchange endpoint (HTTPS - exchange bootstrap token for identity)
		case strings.EqualFold(path, "/v1/he/token/exchange"):
			// Allow CORS for token exchange from landing-web
			setPublicCORS(ctx)
			if method == fasthttp.MethodOptions {
				ctx.SetStatusCode(fasthttp.StatusNoContent)
				return
			}
			if method == fasthttp.MethodGet || method == fasthttp.MethodPost {
				if heBootstrapHandler != nil {
					heBootstrapHandler.HandleTokenExchange(ctx)
				} else {
					ctx.Error("HE bootstrap not configured", fasthttp.StatusNotImplemented)
				}
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Admin campaign endpoints
		case strings.EqualFold(path, "/v1/admin/campaigns"):
			switch method {
			case fasthttp.MethodGet:
				campaignHandler.AdminList(ctx)
			case fasthttp.MethodPost:
				campaignHandler.AdminCreate(ctx)
			default:
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Admin postback diagnostics
		case strings.EqualFold(path, "/v1/admin/postbacks"):
			if method == fasthttp.MethodGet {
				postbackAdminHandler.GetByTransactionID(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Admin transactions list
		case strings.EqualFold(path, "/v1/admin/transactions"):
			if method == fasthttp.MethodGet {
				transactionAdminHandler.ListTransactions(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Admin transactions stats
		case strings.EqualFold(path, "/v1/admin/transactions/stats"):
			if method == fasthttp.MethodGet {
				transactionAdminHandler.GetTransactionStats(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Admin transaction detail
		case strings.HasPrefix(path, "/v1/admin/transactions/"):
			if method == fasthttp.MethodGet {
				transactionAdminHandler.GetTransaction(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		case strings.HasPrefix(path, "/v1/admin/campaigns/") && strings.HasSuffix(path, "/enabled"):
			if method == fasthttp.MethodPatch {
				campaignHandler.AdminSetEnabled(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		case strings.HasPrefix(path, "/v1/admin/campaigns/"):
			switch method {
			case fasthttp.MethodGet:
				campaignHandler.AdminGetBySlug(ctx)
			case fasthttp.MethodPut:
				campaignHandler.AdminUpdate(ctx)
			default:
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Campaign endpoints
		case strings.HasPrefix(path, "/v1/campaigns/"):
			if method == fasthttp.MethodGet {
				campaignHandler.GetBySlug(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		case strings.EqualFold(path, "/v1/campaigns"):
			if method == fasthttp.MethodGet {
				campaignHandler.ListEnabled(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Click-out redirect endpoint (public, for outbound click tracking)
		case strings.EqualFold(path, "/v1/click/out"):
			if method == fasthttp.MethodGet {
				if clickOutHandler != nil {
					clickOutHandler.HandleClickOut(ctx)
				} else {
					ctx.Error("Click-out not configured", fasthttp.StatusNotImplemented)
				}
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Transaction endpoints
		case strings.EqualFold(path, "/v1/acquisition/transactions"):
			if method == fasthttp.MethodPost {
				transactionHandler.CreateTransaction(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		case strings.HasPrefix(path, "/v1/acquisition/transactions/") && strings.HasSuffix(path, "/confirm"):
			if method == fasthttp.MethodPost {
				transactionHandler.ConfirmTransaction(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		case strings.HasPrefix(path, "/v1/acquisition/transactions/") && strings.HasSuffix(path, "/status"):
			if method == fasthttp.MethodGet {
				transactionHandler.GetTransactionStatus(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Callback endpoint (for telco callbacks)
		case strings.HasPrefix(path, "/v1/callbacks/"):
			if method == fasthttp.MethodPost {
				callbackHandler.HandleCallback(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Internal endpoints (for subscription-external service-to-service calls)
		// These require HMAC authentication via X-Internal-Signature header
		case strings.EqualFold(path, "/internal/acquisition/charge-success"):
			if method == fasthttp.MethodPost {
				internalHandler.HandleChargeSuccess(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Analytics endpoints (public, for landing page event ingestion)
		case strings.EqualFold(path, "/v1/analytics/landing/events"):
			if method == fasthttp.MethodPost {
				analyticsHandler.CreateLandingEvent(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		// Admin reporting endpoints (token-protected)
		case strings.EqualFold(path, "/v1/admin/reports/kpis"):
			if method == fasthttp.MethodGet {
				reportsHandler.GetKPIs(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		case strings.EqualFold(path, "/v1/admin/reports/acquisition-funnel"):
			if method == fasthttp.MethodGet {
				reportsHandler.GetAcquisitionFunnel(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		case strings.EqualFold(path, "/v1/admin/reports/campaign-performance"):
			if method == fasthttp.MethodGet {
				reportsHandler.GetCampaignPerformance(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		case strings.EqualFold(path, "/v1/admin/reports/campaign-performance/export"):
			if method == fasthttp.MethodGet {
				reportsHandler.ExportCampaignPerformanceCSV(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		case strings.EqualFold(path, "/v1/admin/reports/timeseries"):
			if method == fasthttp.MethodGet {
				reportsHandler.GetTimeSeries(ctx)
			} else {
				ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
			}
			return

		default:
			ctx.Error("Not Found", fasthttp.StatusNotFound)
		}
	}
	return router
}

// setPublicCORS sets permissive CORS headers for public endpoints (like analytics)
func setPublicCORS(ctx *fasthttp.RequestCtx) {
	origin := string(ctx.Request.Header.Peek("Origin"))
	if origin == "" {
		origin = "*"
	}
	ctx.Response.Header.Set("Access-Control-Allow-Origin", origin)
	ctx.Response.Header.Set("Vary", "Origin")
	ctx.Response.Header.Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	ctx.Response.Header.Set("Access-Control-Allow-Headers", "Content-Type")
	ctx.Response.Header.Set("Access-Control-Max-Age", "600")
}
