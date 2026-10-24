package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/sony/gobreaker"
	"go.uber.org/zap"
)

// newBreakerTestService routes every TIMWE call to a stub whose opt-out endpoint answers optoutStatus.
// The breaker trips on the first counted failure.
func newBreakerTestService(t *testing.T, optoutStatus int) (*SubscriptionService, *atomic.Int32) {
	t.Helper()
	var optoutCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/subscription/optout/") {
			optoutCalls.Add(1)
			w.WriteHeader(optoutStatus)
			_, _ = w.Write([]byte(`{"code":"INVALID_ENTRY_FLOW_CHANNEL","inError":true,"message":"invalid entry channel"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"responseData": map[string]interface{}{"transactionId": "tx-1", "subscriptionResult": "OPTIN_PREACTIVE_WAIT_CONF"},
			"inError":      false,
			"code":         "SUCCESS",
		})
	}))
	t.Cleanup(server.Close)
	cfg := tenantRoutingTestConfig("http://legacy.invalid")
	cfg.Application.TIMWE.CBConsecutiveFailures = 1
	svc := NewSubscriptionService(zap.NewNop(), &MockSubscriptionRepository{}, nil, nil, cfg, nil)
	svc.SetTenantProviderRouter(&fakeTenantProviderResolver{cfg: &TenantProviderConfig{
		TenantID: "tenant-1", ChannelID: "channel-1", Provider: "timwe", BaseURL: server.URL,
		APIKey: "tenant-api-key", Authentication: "tenant-auth-key", PartnerRoleID: "9090", Realm: "tenant-realm",
	}})
	return svc, &optoutCalls
}

func breakerTestOptout() domain.UnsubscriptionRequest {
	return domain.UnsubscriptionRequest{
		UserIdentifier: "233261234567", UserIdentifierType: "MSISDN", ProductId: 33261,
		TenantRoute: domain.TenantRouteContext{TenantID: "tenant-1", ChannelID: "channel-1"},
	}
}

func breakerTestOptin() domain.MTRequest {
	return domain.MTRequest{
		ProductID: 33261, UserIdentifier: "233261234567", UserIdentifierType: "MSISDN", EntryChannel: "USSD",
		TenantRoute: domain.TenantRouteContext{TenantID: "tenant-1", ChannelID: "channel-1"},
	}
}

// A TIMWE 400 is a business rejection, not an outage: it must report its real status and leave the breaker closed.
func TestSendOptoutRejectionDoesNotTripBreaker(t *testing.T) {
	svc, optoutCalls := newBreakerTestService(t, http.StatusBadRequest)

	for i := 0; i < 2; i++ {
		_, err := svc.SendOptout(breakerTestOptout(), "tenant-realm")
		if err == nil || !strings.Contains(err.Error(), "status code: 400") {
			t.Fatalf("call %d: SendOptout error = %v; want the provider's 400", i+1, err)
		}
	}
	if n := optoutCalls.Load(); n != 6 {
		t.Fatalf("provider saw %d opt-out attempts; want 6 (two calls, three attempts each)", n)
	}
}

// Opt-outs failing for real open their own breaker only; opt-ins keep flowing.
func TestOptoutOutageDoesNotRefuseOptins(t *testing.T) {
	svc, _ := newBreakerTestService(t, http.StatusServiceUnavailable)

	if _, err := svc.SendOptout(breakerTestOptout(), "tenant-realm"); err == nil {
		t.Fatal("SendOptout succeeded against a 503 provider")
	}
	if _, err := svc.SendOptout(breakerTestOptout(), "tenant-realm"); !errors.Is(err, gobreaker.ErrOpenState) {
		t.Fatalf("second SendOptout error = %v; want the opt-out breaker open", err)
	}
	resp, err := svc.SendMT(breakerTestOptin(), "tenant-realm", "USSD")
	if err != nil {
		t.Fatalf("SendMT refused after opt-out outage: %v", err)
	}
	if resp == nil || resp.Code != ResponseCodeSuccess {
		t.Fatalf("SendMT response = %+v; want SUCCESS", resp)
	}
}
