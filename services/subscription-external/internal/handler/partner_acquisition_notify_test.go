package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/seidu626/subscription-manager/subscription-external/internal/service"
	"go.uber.org/zap"
)

// notifyPartnerSubscriptionTestServer starts an httptest server standing in
// for acquisition-api's /internal/acquisition/partner-subscription endpoint
// and returns the received payloads over a channel, since
// notifyAcquisitionPartnerSubscription dispatches asynchronously and the
// test must synchronize on the goroutine's HTTP call rather than its return.
func notifyPartnerSubscriptionTestServer(t *testing.T) (*httptest.Server, chan service.PartnerSubscriptionRequest) {
	received := make(chan service.PartnerSubscriptionRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload service.PartnerSubscriptionRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode partner subscription request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		received <- payload
		_ = json.NewEncoder(w).Encode(service.PartnerSubscriptionResponse{Success: true, Message: "ok"})
	}))
	return server, received
}

func awaitPartnerSubscriptionNotification(t *testing.T, received chan service.PartnerSubscriptionRequest) service.PartnerSubscriptionRequest {
	select {
	case payload := <-received:
		return payload
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for async partner subscription notification")
		return service.PartnerSubscriptionRequest{}
	}
}

func TestNotifyAcquisitionPartnerSubscription_OptinSendsMappedPayload(t *testing.T) {
	server, received := notifyPartnerSubscriptionTestServer(t)
	defer server.Close()

	t.Setenv("ACQUISITION_API_URL", server.URL)
	t.Setenv("INTERNAL_API_SECRET", "test-secret")
	acqClient := service.NewAcquisitionClient(zap.NewNop())

	h := NewPartnerHandler(zap.NewNop(), nil, nil).WithAcquisitionClient(acqClient)

	route := domain.TenantRouteContext{
		TenantID:   "11111111-1111-1111-1111-111111111111",
		ChannelID:  "22222222-2222-2222-2222-222222222222",
		ChannelKey: "web-gh-airteltigo",
	}
	resp := &domain.MTResponse{
		ResponseData: map[string]interface{}{
			"subscriptionResult": "OPTIN_ACTIVE_WAIT_CHARGING",
			"transactionId":      "timwe-tx-1",
		},
	}

	h.notifyAcquisitionPartnerSubscription(partnerAcquisitionActionOptin, route, "233241234567", 14397, "web", resp)

	payload := awaitPartnerSubscriptionNotification(t, received)
	if payload.Action != partnerAcquisitionActionOptin {
		t.Fatalf("expected action=%q, got %q", partnerAcquisitionActionOptin, payload.Action)
	}
	if payload.TenantID != route.TenantID || payload.ChannelID != route.ChannelID || payload.ChannelKey != route.ChannelKey {
		t.Fatalf("expected route context propagated, got %+v", payload)
	}
	if payload.MSISDN != "233241234567" || payload.ProductID != 14397 || payload.EntryChannel != "web" {
		t.Fatalf("expected msisdn/product_id/entry_channel propagated, got %+v", payload)
	}
	if payload.TimweTransactionID != "timwe-tx-1" || payload.SubscriptionResult != "OPTIN_ACTIVE_WAIT_CHARGING" {
		t.Fatalf("expected timwe transaction id/subscription result extracted from MTResponse, got %+v", payload)
	}
}

func TestNotifyAcquisitionPartnerSubscription_ConfirmSendsAction(t *testing.T) {
	server, received := notifyPartnerSubscriptionTestServer(t)
	defer server.Close()

	t.Setenv("ACQUISITION_API_URL", server.URL)
	t.Setenv("INTERNAL_API_SECRET", "test-secret")
	acqClient := service.NewAcquisitionClient(zap.NewNop())

	h := NewPartnerHandler(zap.NewNop(), nil, nil).WithAcquisitionClient(acqClient)

	route := domain.TenantRouteContext{
		TenantID:   "11111111-1111-1111-1111-111111111111",
		ChannelKey: "web-gh-airteltigo",
	}
	resp := &domain.MTResponse{ResponseData: map[string]interface{}{}}

	h.notifyAcquisitionPartnerSubscription(partnerAcquisitionActionConfirm, route, "233241234567", 14397, "", resp)

	payload := awaitPartnerSubscriptionNotification(t, received)
	if payload.Action != partnerAcquisitionActionConfirm {
		t.Fatalf("expected action=%q, got %q", partnerAcquisitionActionConfirm, payload.Action)
	}
}

func TestNotifyAcquisitionPartnerSubscription_NilClientIsNoop(t *testing.T) {
	h := NewPartnerHandler(zap.NewNop(), nil, nil)
	route := domain.TenantRouteContext{TenantID: "11111111-1111-1111-1111-111111111111"}
	resp := &domain.MTResponse{ResponseData: map[string]interface{}{}}

	// Must not panic when no acquisition client is wired (e.g. tests or a
	// deployment that hasn't configured ACQUISITION_API_URL yet).
	h.notifyAcquisitionPartnerSubscription(partnerAcquisitionActionOptin, route, "233241234567", 14397, "", resp)
}
