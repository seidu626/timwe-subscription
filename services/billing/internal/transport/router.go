package transport

import (
	"github.com/seidu626/subscription-manager/billing/internal/handler"
	"github.com/valyala/fasthttp"
	"log"
	"strings"
)

func NewRouter(h *handler.BillingHandler) fasthttp.RequestHandler {
	router := func(ctx *fasthttp.RequestCtx) {
		path := string(ctx.Path())

		switch {
		case strings.EqualFold(path, "/health"):
			handler.HealthHandler(ctx)
		case strings.EqualFold(path, "/metrics"):
			handler.MetricsHandler(ctx)
		case strings.EqualFold(path, "/api/v1/billing/transactions"):
			if string(ctx.Method()) == fasthttp.MethodGet {
				h.ListTransactions(ctx)
			} else if string(ctx.Method()) == fasthttp.MethodPost {
				h.CreateTransaction(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
		case strings.HasPrefix(path, "/api/v1/billing/transaction/"):
			if string(ctx.Method()) == fasthttp.MethodGet {
				h.GetTransaction(ctx)
			} else {
				ctx.Error("Method not allowed", fasthttp.StatusMethodNotAllowed)
			}
		default:
			log.Printf("Processing unknown request: %s", ctx.Request.String())
			ctx.Error("Not Found", fasthttp.StatusNotFound)
		}
	}
	return router
}
