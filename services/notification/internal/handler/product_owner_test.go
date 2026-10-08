package handler

import (
	"testing"

	"github.com/seidu626/subscription-manager/notification/internal/service"
	"github.com/valyala/fasthttp"
)

// TestHandleNotification_ProductOwnerFallback covers TIMWE callbacks registered
// before tenancy: they POST /notification/{type}/{partnerRole} with no
// tenant_key/channel_key, so the tenant comes from productId ownership.
func TestHandleNotification_ProductOwnerFallback(t *testing.T) {
	const (
		nrgTenantID       = "11111111-1111-1111-1111-111111111111"
		careerifyTenantID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	)
	owners := map[int]string{8509: nrgTenantID, 32535: careerifyTenantID}

	makeCtx := func(uri, body string) *fasthttp.RequestCtx {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetRequestURI(uri)
		ctx.Request.Header.SetMethod(fasthttp.MethodPost)
		ctx.SetUserValue("partnerRole", "2117")
		ctx.Request.SetBodyString(body)
		return ctx
	}

	t.Run("no tenant context, owned product → 200 stamped with owner", func(t *testing.T) {
		repo := &handlerRepoStub{productOwners: owners}
		h := NewNotificationHandler(service.NewNotificationService(repo))

		ctx := makeCtx("/api/v1/notification/charge/2117", `{"msisdn":"233241234567","productId":32535,"transactionUUID":"tx-1"}`)
		h.ChargeHandler(ctx)

		if ctx.Response.StatusCode() != fasthttp.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", ctx.Response.StatusCode(), ctx.Response.Body())
		}
		if repo.saved == nil || repo.saved.TenantID == nil || *repo.saved.TenantID != careerifyTenantID {
			t.Fatalf("expected TenantID=%q, got %#v", careerifyTenantID, repo.saved)
		}
		if repo.saved.Type != "CHARGE" || repo.saved.PartnerRole != 2117 {
			t.Fatalf("expected CHARGE for role 2117, got type=%q role=%d", repo.saved.Type, repo.saved.PartnerRole)
		}
	})

	t.Run("body tenantId/channelId are ignored on the product-owner path", func(t *testing.T) {
		repo := &handlerRepoStub{productOwners: owners}
		h := NewNotificationHandler(service.NewNotificationService(repo))

		ctx := makeCtx("/api/v1/notification/user-renewed/2117",
			`{"tenantId":"`+careerifyTenantID+`","channelId":"spoofed-channel","msisdn":"233241234567","productId":8509}`)
		h.UserRenewedHandler(ctx)

		if ctx.Response.StatusCode() != fasthttp.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", ctx.Response.StatusCode(), ctx.Response.Body())
		}
		if repo.saved == nil || repo.saved.TenantID == nil || *repo.saved.TenantID != nrgTenantID {
			t.Fatalf("expected product owner %q, got %#v", nrgTenantID, repo.saved)
		}
		if repo.saved.ChannelID != nil {
			t.Fatalf("expected body channelId to be dropped, got %q", *repo.saved.ChannelID)
		}
	})

	t.Run("unowned product → 422 TENANT_CONTEXT_REQUIRED", func(t *testing.T) {
		repo := &handlerRepoStub{productOwners: owners}
		h := NewNotificationHandler(service.NewNotificationService(repo))

		ctx := makeCtx("/api/v1/notification/charge/2117", `{"msisdn":"233241234567","productId":99999}`)
		h.ChargeHandler(ctx)

		if ctx.Response.StatusCode() != fasthttp.StatusUnprocessableEntity {
			t.Fatalf("expected 422, got %d body=%s", ctx.Response.StatusCode(), ctx.Response.Body())
		}
		if repo.saved != nil {
			t.Fatalf("expected nothing persisted, got %#v", repo.saved)
		}
	})

	t.Run("missing or malformed productId → 422", func(t *testing.T) {
		for _, body := range []string{`{"msisdn":"233241234567"}`, `{"productId":0}`, `not-json`} {
			repo := &handlerRepoStub{productOwners: owners}
			h := NewNotificationHandler(service.NewNotificationService(repo))

			ctx := makeCtx("/api/v1/notification/user-optout/2117", body)
			h.UserOptoutHandler(ctx)

			if ctx.Response.StatusCode() != fasthttp.StatusUnprocessableEntity {
				t.Fatalf("body %q: expected 422, got %d", body, ctx.Response.StatusCode())
			}
		}
	})

	t.Run("explicit tenant_key context wins over product owner", func(t *testing.T) {
		const careerifyChannelID = "00000000-0000-0000-0000-000000000002"
		repo := &handlerRepoStub{
			productOwners:   owners,
			tenantIDByKey:   map[string]string{"careerify": careerifyTenantID},
			channelIDByKeys: map[string]string{careerifyTenantID + "|web-gh-airteltigo": careerifyChannelID},
		}
		h := NewNotificationHandler(service.NewNotificationService(repo))

		ctx := makeCtx("/api/v1/notification/charge/2117?tenant_key=careerify&channel_key=web-gh-airteltigo",
			`{"msisdn":"233241234567","productId":8509}`)
		h.ChargeHandler(ctx)

		if ctx.Response.StatusCode() != fasthttp.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", ctx.Response.StatusCode(), ctx.Response.Body())
		}
		if repo.saved == nil || repo.saved.TenantID == nil || *repo.saved.TenantID != careerifyTenantID {
			t.Fatalf("expected explicit tenant %q, got %#v", careerifyTenantID, repo.saved)
		}
		if repo.saved.ChannelID == nil || *repo.saved.ChannelID != careerifyChannelID {
			t.Fatalf("expected channel %q, got %#v", careerifyChannelID, repo.saved.ChannelID)
		}
	})
}
