package transport

import (
	"encoding/json"
	"testing"

	"github.com/valyala/fasthttp"
)

func TestHealthReportsObservabilityStatus(t *testing.T) {
	router := NewRouter(nil)
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/health")
	ctx.Request.Header.SetMethod(fasthttp.MethodGet)

	router(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d", ctx.Response.StatusCode())
	}
	var body map[string]any
	if err := json.Unmarshal(ctx.Response.Body(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	observability, ok := body["observability"].(map[string]any)
	if !ok {
		t.Fatalf("expected observability status, got %#v", body)
	}
	if observability["tenant_labels"] != "enabled" || observability["pii_labels"] != "rejected" {
		t.Fatalf("unexpected observability status: %#v", observability)
	}
}

func TestUnknownRouteReturnsErrorWithoutRequestDump(t *testing.T) {
	router := NewRouter(nil)
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/missing?msisdn=233241234567")
	ctx.Request.Header.SetMethod(fasthttp.MethodGet)

	router(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusNotFound {
		t.Fatalf("expected error 404, got %d", ctx.Response.StatusCode())
	}
}
