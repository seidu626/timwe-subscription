package handler

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

func TestHandleCallbackRejectsUncorrelatablePayload(t *testing.T) {
	h := NewCallbackHandler(nil, nil, nil, nil, zap.NewNop())
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/v1/callbacks/timwe")
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetBodyString(`{"msisdn":"233241234567","status":"SUCCESS"}`)

	h.HandleCallback(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for missing transaction correlation, got %d body=%s", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	if !strings.Contains(string(ctx.Response.Body()), "transaction_id is required") {
		t.Fatalf("expected correlation error body, got %s", ctx.Response.Body())
	}
}

func TestCallbackTenantMatchesRejectsMismatch(t *testing.T) {
	tenantID := "11111111-1111-1111-1111-111111111111"
	tx := &domain.AcquisitionTransaction{ID: uuid.New(), TenantID: &tenantID}

	if !callbackTenantMatches(tx, " "+tenantID+" ") {
		t.Fatal("expected matching tenant to pass")
	}
	if callbackTenantMatches(tx, "22222222-2222-2222-2222-222222222222") {
		t.Fatal("expected mismatched tenant to be rejected")
	}
	if callbackTenantMatches(&domain.AcquisitionTransaction{ID: uuid.New()}, tenantID) {
		t.Fatal("expected callback tenant to require transaction tenant")
	}
	if !callbackTenantMatches(tx, "") {
		t.Fatal("expected legacy callback without tenant_id to remain accepted")
	}
}

func TestCallbackChannelIDOrCampaignPrefersCallbackThenCampaign(t *testing.T) {
	campaignChannel := "11111111-1111-1111-1111-111111111111"
	callbackChannel := "22222222-2222-2222-2222-222222222222"

	got := callbackChannelIDOrCampaign(" "+callbackChannel+" ", &domain.Campaign{ChannelID: &campaignChannel})
	if got == nil || *got != callbackChannel {
		t.Fatalf("expected callback channel %q, got %#v", callbackChannel, got)
	}

	got = callbackChannelIDOrCampaign("", &domain.Campaign{ChannelID: &campaignChannel})
	if got == nil || *got != campaignChannel {
		t.Fatalf("expected campaign channel %q, got %#v", campaignChannel, got)
	}

	blank := " "
	if got := callbackChannelIDOrCampaign(" ", &domain.Campaign{ChannelID: &blank}); got != nil {
		t.Fatalf("expected nil channel for blank values, got %q", *got)
	}
}
