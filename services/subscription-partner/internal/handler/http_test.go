package handler

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription/internal/domain"
	"github.com/seidu626/subscription-manager/subscription/internal/service"
	"github.com/valyala/fasthttp"
)

// handlerRepoStub satisfies the subscriptionRepository interface used by the service layer.
type handlerRepoStub struct {
	fetchResp        *domain.ListResponse
	fetchErr         error
	createdSub       *domain.SubscriptionRequest
	createSubErr     error
}

func (h *handlerRepoStub) FetchSubscriptions(tenantID, tenantKey string, startDate, endDate time.Time, productID int, shortcode, userIdentifier, entryChannel, sortBy, sortDir string, page, pageSize int) (*domain.ListResponse, error) {
	if h.fetchErr != nil {
		return nil, h.fetchErr
	}
	if h.fetchResp != nil {
		return h.fetchResp, nil
	}
	return &domain.ListResponse{}, nil
}

func (h *handlerRepoStub) ConfirmSubscription(request *domain.SubscriptionConfirmationRequest) error {
	return nil
}

func (h *handlerRepoStub) CreateSubscription(request *domain.SubscriptionRequest) error {
	h.createdSub = request
	return h.createSubErr
}

func (h *handlerRepoStub) CreateNotification(notification *domain.NotificationRequest) error {
	return nil
}

func (h *handlerRepoStub) OptOutSubscription(request *domain.UnsubscriptionRequest) error {
	return nil
}

func (h *handlerRepoStub) GetSubscriptionStatus(request *domain.GetStatusRequest) (*domain.SubscriptionStatus, error) {
	return nil, nil
}

// tenantRepoStub satisfies gatewayTenantLookup for handler tests.
type tenantRepoStub struct {
	tenantID   string
	tenantErr  error
	channelID  string
	channelErr error
}

func (t *tenantRepoStub) TenantIDByKey(tenantKey string) (string, error) {
	return t.tenantID, t.tenantErr
}

func (t *tenantRepoStub) ChannelIDByKeys(tenantID, channelKey string) (string, error) {
	return t.channelID, t.channelErr
}

// resolvedTenantRepo returns a stub that resolves successfully to known IDs.
func resolvedTenantRepo() *tenantRepoStub {
	return &tenantRepoStub{
		tenantID:  "tenant-uuid-1",
		channelID: "channel-uuid-1",
	}
}

func TestListSubscriptions_ReturnsInternalServerError(t *testing.T) {
	svc := service.NewSubscriptionService(&handlerRepoStub{fetchErr: errors.New("query failed")}, &config.Config{})
	h := NewSubscriptionHandler(svc, &config.Config{})

	ctx := newListRequestContext("/api/v1/subscription/list?page=1&pageSize=10")
	ctx.Request.Header.Set("X-Tenant-Key", "nrg")
	h.ListSubscriptions(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", ctx.Response.StatusCode())
	}
	if !strings.Contains(string(ctx.Response.Body()), "Error fetching listResponse") {
		t.Fatalf("unexpected response body: %s", string(ctx.Response.Body()))
	}
}

func TestListSubscriptions_ReturnsPaginationHeaderAndBody(t *testing.T) {
	listResponse := &domain.ListResponse{
		Data: []*domain.Subscription{
			{
				Id:             1,
				PartnerRoleId:  "2117",
				UserIdentifier: "233241234567",
				ProductId:      "8509",
				Status:         "active",
				CreatedAt:      "2026-02-12T08:30:00Z",
				StartDate:      time.Date(2026, time.February, 12, 8, 30, 0, 0, time.UTC),
			},
		},
		Page:        1,
		PageSize:    10,
		TotalCount:  1,
		TotalPages:  1,
		HasNextPage: false,
		HasPrevPage: false,
	}

	svc := service.NewSubscriptionService(&handlerRepoStub{fetchResp: listResponse}, &config.Config{})
	h := NewSubscriptionHandler(svc, &config.Config{})
	ctx := newListRequestContext("/api/v1/subscription/list?page=1&pageSize=10")
	ctx.Request.Header.Set("X-Tenant-Key", "nrg")

	h.ListSubscriptions(ctx)

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

func TestListSubscriptions_RequiresTenantContext(t *testing.T) {
	svc := service.NewSubscriptionService(&handlerRepoStub{}, &config.Config{})
	h := NewSubscriptionHandler(svc, &config.Config{})

	ctx := newListRequestContext("/api/v1/subscription/list?page=1&pageSize=10")
	h.ListSubscriptions(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusForbidden {
		t.Fatalf("expected status 403, got %d", ctx.Response.StatusCode())
	}
}

// TestOptinHandler_TenantlessRejects verifies that an optin without tenant context
// is rejected with 422 (TENANT_CONTEXT_REQUIRED).
func TestOptinHandler_TenantlessRejects(t *testing.T) {
	svc := service.NewSubscriptionService(&handlerRepoStub{}, &config.Config{})
	h := NewSubscriptionHandler(svc, &config.Config{}).WithTenantRepo(resolvedTenantRepo())

	ctx := newPostRequestContext(
		"/api/v1/subscription/optin/2117",
		`{"userIdentifier":"233241234567","userIdentifierType":"MSISDN","productId":8509}`,
	)
	ctx.SetUserValue("partnerRoleId", "2117")
	// No X-Tenant-Key / X-Channel-Key / query params → must reject
	h.OptinHandler(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for tenantless optin, got %d body=%s", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(ctx.Response.Body(), &body); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if body["code"] != "TENANT_CONTEXT_REQUIRED" {
		t.Fatalf("expected code TENANT_CONTEXT_REQUIRED, got %v", body["code"])
	}
}

// TestOptinHandler_WithTenantContext verifies that optin with valid tenant_key+channel_key
// resolves tenant IDs and the request succeeds.
func TestOptinHandler_WithTenantContext(t *testing.T) {
	repo := &handlerRepoStub{}
	svc := service.NewSubscriptionService(repo, &config.Config{})
	h := NewSubscriptionHandler(svc, &config.Config{}).WithTenantRepo(resolvedTenantRepo())

	ctx := newPostRequestContext(
		"/api/v1/subscription/optin/2117?tenant_key=nrg&channel_key=web",
		`{"userIdentifier":"233241234567","userIdentifierType":"MSISDN","productId":8509}`,
	)
	ctx.SetUserValue("partnerRoleId", "2117")
	h.OptinHandler(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	if repo.createdSub == nil {
		t.Fatal("expected CreateSubscription to be called")
	}
	if repo.createdSub.TenantRoute.TenantID != "tenant-uuid-1" {
		t.Fatalf("expected tenant_id stamped, got %q", repo.createdSub.TenantRoute.TenantID)
	}
	if repo.createdSub.TenantRoute.ChannelID != "channel-uuid-1" {
		t.Fatalf("expected channel_id stamped, got %q", repo.createdSub.TenantRoute.ChannelID)
	}
}

// TestOptinHandler_WithTenantKeyHeader verifies X-Tenant-Key / X-Channel-Key headers work.
func TestOptinHandler_WithTenantKeyHeader(t *testing.T) {
	repo := &handlerRepoStub{}
	svc := service.NewSubscriptionService(repo, &config.Config{})
	h := NewSubscriptionHandler(svc, &config.Config{}).WithTenantRepo(resolvedTenantRepo())

	ctx := newPostRequestContext(
		"/api/v1/subscription/optin/2117",
		`{"userIdentifier":"233241234567","userIdentifierType":"MSISDN","productId":8509}`,
	)
	ctx.SetUserValue("partnerRoleId", "2117")
	ctx.Request.Header.Set("X-Tenant-Key", "nrg")
	ctx.Request.Header.Set("X-Channel-Key", "web")
	h.OptinHandler(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	if repo.createdSub == nil || repo.createdSub.TenantRoute.TenantID == "" {
		t.Fatal("expected TenantID stamped via headers")
	}
}

// TestConfirmHandler_ReturnsNotImplemented verifies the confirm path is still 501.
// Confirm check occurs after tenant resolution — requires valid tenant context.
func TestConfirmHandler_ReturnsNotImplemented(t *testing.T) {
	svc := service.NewSubscriptionService(&handlerRepoStub{}, &config.Config{})
	h := NewSubscriptionHandler(svc, &config.Config{}).WithTenantRepo(resolvedTenantRepo())

	ctx := newPostRequestContext(
		"/api/v1/subscription/optin/confirm/2117?tenant_key=nrg&channel_key=web",
		`{"userIdentifier":"233241234567","userIdentifierType":"MSISDN","productId":8509,"transactionAuthCode":"1234"}`,
	)
	ctx.SetUserValue("partnerRoleId", "2117")

	h.ConfirmHandler(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusNotImplemented {
		t.Fatalf("expected status 501, got %d", ctx.Response.StatusCode())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(ctx.Response.Body(), &body); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	if body["code"] != "NOT_SUPPORTED" {
		t.Fatalf("expected code NOT_SUPPORTED, got %+v", body["code"])
	}
	if body["inError"] != true {
		t.Fatalf("expected inError=true, got %+v", body["inError"])
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

func newPostRequestContext(uri, body string) *fasthttp.RequestCtx {
	req := fasthttp.AcquireRequest()
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/json")
	req.SetRequestURI(uri)
	req.SetBody([]byte(body))

	ctx := &fasthttp.RequestCtx{}
	ctx.Init(req, nil, nil)
	return ctx
}
