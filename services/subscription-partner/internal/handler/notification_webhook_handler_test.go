package handler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/seidu626/subscription-manager/subscription/internal/domain"
	"github.com/seidu626/subscription-manager/subscription/internal/service"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

type webhookRepoStub struct {
	notification *domain.NotificationRequest
}

func (w *webhookRepoStub) FetchSubscriptions(tenantID, tenantKey string, startDate, endDate time.Time, productID int, shortcode, userIdentifier, entryChannel string, page, pageSize int) (*domain.ListResponse, error) {
	return &domain.ListResponse{}, nil
}

func (w *webhookRepoStub) ConfirmSubscription(request *domain.SubscriptionConfirmationRequest) error {
	return nil
}

func (w *webhookRepoStub) CreateSubscription(request *domain.SubscriptionRequest) error {
	return nil
}

func (w *webhookRepoStub) CreateNotification(notification *domain.NotificationRequest) error {
	w.notification = notification
	return nil
}

func (w *webhookRepoStub) OptOutSubscription(request *domain.UnsubscriptionRequest) error {
	return nil
}

func (w *webhookRepoStub) GetSubscriptionStatus(request *domain.GetStatusRequest) (*domain.SubscriptionStatus, error) {
	return nil, nil
}

func TestHandleNotificationWebhook_PersistsNotification(t *testing.T) {
	t.Setenv("ACQUISITION_CHARGE_CALLBACK_ENABLED", "false")
	logger := zap.NewNop()

	repo := &webhookRepoStub{}
	svc := service.NewSubscriptionService(repo, nil)
	acqClient := service.NewAcquisitionClient(logger)
	h := NewNotificationWebhookHandler(logger, svc, acqClient)

	ctx := &fasthttp.RequestCtx{}
	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/api/v1/webhooks/timwe/notification")
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/json")
	req.SetBody([]byte(`{"partnerRole":2117,"externalTxId":"ext-1","productId":8509,"msisdn":"233241234567","type":"CHARGE","transactionUuid":"tx-1"}`))
	ctx.Init(req, nil, nil)

	h.HandleNotificationWebhook(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected status 200, got %d", ctx.Response.StatusCode())
	}
	if repo.notification == nil {
		t.Fatal("expected notification to be persisted")
	}
	if repo.notification.MSISDN != "233241234567" || repo.notification.Type != "CHARGE" {
		t.Fatalf("unexpected persisted notification: %+v", repo.notification)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(ctx.Response.Body(), &body); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected response status ok, got %+v", body["status"])
	}
}

func TestHandleNotificationWebhook_InvalidPayload(t *testing.T) {
	logger := zap.NewNop()
	repo := &webhookRepoStub{}
	svc := service.NewSubscriptionService(repo, nil)
	acqClient := service.NewAcquisitionClient(logger)
	h := NewNotificationWebhookHandler(logger, svc, acqClient)

	ctx := &fasthttp.RequestCtx{}
	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/api/v1/webhooks/timwe/notification")
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/json")
	req.SetBody([]byte(`{"type"`))
	ctx.Init(req, nil, nil)

	h.HandleNotificationWebhook(ctx)
	if ctx.Response.StatusCode() != fasthttp.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", ctx.Response.StatusCode())
	}
}
