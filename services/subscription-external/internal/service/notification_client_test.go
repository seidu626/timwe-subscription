package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
)

func TestNotificationClientNotifyUserOptin(t *testing.T) {
	var got domain.NotificationRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/notification/user-optin/2117" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("tenant_key") != "careerify" || r.URL.Query().Get("channel_key") != "web-gh-airteltigo" {
			t.Errorf("unexpected tenant route query: %s", r.URL.RawQuery)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewNotificationClient()
	client.baseURL = server.URL
	notification := &domain.NotificationRequest{
		ExternalTxID: "external-tx-1",
		ProductID:    32535,
		MSISDN:       "233572503330",
		Type:         "USER_OPTIN",
	}
	err := client.NotifyUserOptin(context.Background(), domain.TenantRouteContext{
		TenantKey:  "careerify",
		ChannelKey: "web-gh-airteltigo",
	}, 2117, notification)
	if err != nil {
		t.Fatalf("NotifyUserOptin: %v", err)
	}
	if got.ExternalTxID != notification.ExternalTxID || got.ProductID != notification.ProductID || got.MSISDN != notification.MSISDN {
		t.Fatalf("unexpected notification payload: %+v", got)
	}
}

func TestNotificationClientNotifyUserOptinRequiresIdempotencyInput(t *testing.T) {
	client := NewNotificationClient()
	err := client.NotifyUserOptin(context.Background(), domain.TenantRouteContext{
		TenantKey:  "careerify",
		ChannelKey: "web-gh-airteltigo",
	}, 2117, &domain.NotificationRequest{})
	if err == nil {
		t.Fatal("expected missing externalTxId error")
	}
}
