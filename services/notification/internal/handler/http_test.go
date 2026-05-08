package handler

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/seidu626/subscription-manager/notification/internal/domain"
	"github.com/seidu626/subscription-manager/notification/internal/service"
	"github.com/valyala/fasthttp"
)

type handlerRepoStub struct {
	fetchResp      *domain.ListResponse
	fetchErr       error
	fetchTenantID  string
	fetchChannelID string
	saved          *domain.NotificationRequest
}

func (h *handlerRepoStub) FetchNotifications(startDate, endDate time.Time, tenantID, channelID, partnerRole, msisdn, entryChannel, notificationType string, page, pageSize int) (*domain.ListResponse, error) {
	h.fetchTenantID = tenantID
	h.fetchChannelID = channelID
	if h.fetchErr != nil {
		return nil, h.fetchErr
	}
	if h.fetchResp != nil {
		return h.fetchResp, nil
	}
	return &domain.ListResponse{}, nil
}

func (h *handlerRepoStub) Save(notification *domain.NotificationRequest) error {
	h.saved = notification
	return nil
}

func TestListNotifications_ReturnsInternalServerError(t *testing.T) {
	svc := service.NewNotificationService(&handlerRepoStub{fetchErr: errors.New("query failed")})
	h := NewNotificationHandler(svc)
	ctx := newListRequestContext("/api/v1/notification/list?page=1&pageSize=10")
	setTenantIdentity(ctx, "tenant-1")

	h.ListNotifications(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", ctx.Response.StatusCode())
	}
	if !strings.Contains(string(ctx.Response.Body()), "Error fetching listResponse") {
		t.Fatalf("unexpected response body: %s", string(ctx.Response.Body()))
	}
}

func TestListNotifications_ReturnsPaginationHeaderAndBody(t *testing.T) {
	listResponse := &domain.ListResponse{
		Data: []*domain.Notification{
			{
				ID:           1,
				PartnerRole:  2117,
				MSISDN:       "233241234567",
				ProductID:    8509,
				EntryChannel: "SMS",
				CreatedAt:    time.Date(2026, time.February, 12, 22, 0, 0, 0, time.UTC),
			},
		},
		Page:        1,
		PageSize:    10,
		TotalCount:  1,
		TotalPages:  1,
		HasNextPage: false,
		HasPrevPage: false,
	}

	svc := service.NewNotificationService(&handlerRepoStub{fetchResp: listResponse})
	h := NewNotificationHandler(svc)
	ctx := newListRequestContext("/api/v1/notification/list?page=1&pageSize=10")
	setTenantIdentity(ctx, "tenant-1")

	h.ListNotifications(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected status 200, got %d", ctx.Response.StatusCode())
	}

	headerValue := string(ctx.Response.Header.Peek("X-Pagination"))
	if headerValue == "" {
		t.Fatalf("expected X-Pagination header")
	}

	var pagination map[string]interface{}
	if err := json.Unmarshal([]byte(headerValue), &pagination); err != nil {
		t.Fatalf("failed to parse X-Pagination header: %v", err)
	}
	if pagination["totalCount"] != float64(1) || pagination["page"] != float64(1) {
		t.Fatalf("unexpected pagination header values: %+v", pagination)
	}

	var body domain.ListResponse
	if err := json.Unmarshal(ctx.Response.Body(), &body); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	if body.TotalCount != 1 || len(body.Data) != 1 {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestListNotifications_RequiresTenantContext(t *testing.T) {
	svc := service.NewNotificationService(&handlerRepoStub{})
	h := NewNotificationHandler(svc)
	ctx := newListRequestContext("/api/v1/notification/list?page=1&pageSize=10")

	h.ListNotifications(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusForbidden {
		t.Fatalf("expected status 403 without tenant context, got %d", ctx.Response.StatusCode())
	}
	if !strings.Contains(string(ctx.Response.Body()), "tenant context required") {
		t.Fatalf("unexpected response body: %s", ctx.Response.Body())
	}
}

func TestListNotifications_RejectsSpoofedTenantHeaderWithoutVerifiedIdentity(t *testing.T) {
	svc := service.NewNotificationService(&handlerRepoStub{})
	h := NewNotificationHandler(svc)
	ctx := newListRequestContext("/api/v1/notification/list?page=1&pageSize=10")
	ctx.Request.Header.Set("X-Tenant-Id", "tenant-evil")

	h.ListNotifications(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusForbidden {
		t.Fatalf("expected status 403 for spoofed tenant header, got %d", ctx.Response.StatusCode())
	}
}

func TestListNotifications_AllowsPlatformScopedTenantSelection(t *testing.T) {
	svc := service.NewNotificationService(&handlerRepoStub{})
	h := NewNotificationHandler(svc)
	ctx := newListRequestContext("/api/v1/notification/list?page=1&pageSize=10")
	ctx.SetUserValue(tenantctx.FastHTTPUserValueKey, tenantctx.Identity{PlatformScoped: true})
	ctx.Request.Header.Set("X-Tenant-Id", "tenant-1")

	h.ListNotifications(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected status 200 for platform tenant selection, got %d", ctx.Response.StatusCode())
	}
}

func TestListNotifications_UsesVerifiedTenantOverSpoofedTenantHeader(t *testing.T) {
	repo := &handlerRepoStub{}
	svc := service.NewNotificationService(repo)
	h := NewNotificationHandler(svc)
	ctx := newListRequestContext("/api/v1/notification/list?page=1&pageSize=10")
	setTenantIdentity(ctx, "tenant-a")
	ctx.Request.Header.Set("X-Tenant-Id", "tenant-b")

	h.ListNotifications(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected status 200, got %d", ctx.Response.StatusCode())
	}
	if repo.fetchTenantID != "tenant-a" {
		t.Fatalf("expected verified tenant tenant-a to be used, got %q", repo.fetchTenantID)
	}
}

func TestNotificationInboundPersistsTenantChannelContext(t *testing.T) {
	repo := &handlerRepoStub{}
	svc := service.NewNotificationService(repo)
	h := NewNotificationHandler(svc)
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/api/v1/notification/user-optin/2117")
	ctx.SetUserValue("partnerRole", "2117")
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.Header.Set("X-Tenant-Id", "tenant-1")
	ctx.Request.Header.Set("X-Tenant-Channel-Id", "channel-1")
	ctx.Request.SetBodyString(`{"msisdn":"233241234567","productId":8509,"message":"ok"}`)

	h.UserOptinHandler(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	if repo.saved == nil {
		t.Fatal("expected notification to be saved")
	}
	if repo.saved.TenantID == nil || *repo.saved.TenantID != "tenant-1" {
		t.Fatalf("expected tenant persisted, got %#v", repo.saved.TenantID)
	}
	if repo.saved.ChannelID == nil || *repo.saved.ChannelID != "channel-1" {
		t.Fatalf("expected channel persisted, got %#v", repo.saved.ChannelID)
	}
}

func newListRequestContext(uri string) *fasthttp.RequestCtx {
	req := fasthttp.AcquireRequest()
	req.Header.SetMethod(fasthttp.MethodGet)
	req.SetRequestURI(uri)

	ctx := &fasthttp.RequestCtx{}
	ctx.Init(req, nil, nil)

	return ctx
}

func setTenantIdentity(ctx *fasthttp.RequestCtx, tenantID string) {
	ctx.SetUserValue(tenantctx.FastHTTPUserValueKey, tenantctx.Identity{TenantID: tenantID})
}
