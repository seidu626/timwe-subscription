package handler

import (
	"encoding/json"
	"testing"

	"github.com/valyala/fasthttp"
)

func TestUploadHandlerReturnsGoneWithoutParsingOrServiceCall(t *testing.T) {
	handler := &UserBaseHandler{}
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.Header.Set("X-Tenant-ID", "untrusted-tenant")
	ctx.Request.SetBodyString("this is not multipart data")

	handler.UploadHandler(ctx)

	if got := ctx.Response.StatusCode(); got != fasthttp.StatusGone {
		t.Fatalf("status = %d, want %d", got, fasthttp.StatusGone)
	}
	if got := string(ctx.Response.Header.ContentType()); got != "application/json" {
		t.Fatalf("content type = %q, want application/json", got)
	}

	var response map[string]string
	if err := json.Unmarshal(ctx.Response.Body(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := response["canonical_endpoint"]; got != "/v1/admin/userbase/imports" {
		t.Fatalf("canonical endpoint = %q, want /v1/admin/userbase/imports", got)
	}
	if got := response["error"]; got != "legacy userbase upload endpoint is retired" {
		t.Fatalf("error = %q, want retired endpoint message", got)
	}
}
