package handler

import (
	"database/sql"
	"testing"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

func TestClickOutHandler_InvalidDestination(t *testing.T) {
	logger := zap.NewNop()

	// repo not used for invalid destination path; safe to pass nil DB
	repo := repository.NewOutboundClickRepository(&sql.DB{}, logger)
	h := NewClickOutHandler(repo, &ClickOutConfig{
		Destinations: map[string]DestinationConfig{
			"allowlisted": {BaseURL: "https://example.com/click"},
		},
		DefaultClickIDParam:   "click_id",
		RateLimitPerIPPerHour: 0,
		CookieSecure:          false,
	}, logger)

	var ctx fasthttp.RequestCtx
	ctx.Request.SetRequestURI("/v1/click/out?partner=test&dest=not-allowed")
	ctx.Request.Header.SetMethod(fasthttp.MethodGet)

	h.HandleClickOut(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusBadRequest {
		t.Fatalf("expected %d, got %d", fasthttp.StatusBadRequest, ctx.Response.StatusCode())
	}
}

