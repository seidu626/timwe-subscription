package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func TestNotifyChargeSuccessPropagatesTenantChannel(t *testing.T) {
	var payload ChargeSuccessRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if r.Header.Get("X-Internal-Signature") == "" {
			t.Fatal("expected internal signature header")
		}
		_ = json.NewEncoder(w).Encode(ChargeSuccessResponse{Success: true, Message: "ok"})
	}))
	defer server.Close()

	t.Setenv("ACQUISITION_API_URL", server.URL)
	t.Setenv("INTERNAL_API_SECRET", "test-secret")
	client := NewAcquisitionClient(zap.NewNop())

	err := client.NotifyChargeSuccess(&ChargeSuccessRequest{
		TimweTransactionID: "charge-tx-1",
		TenantID:           "tenant-1",
		ChannelID:          "channel-1",
		MSISDN:             "233241234567",
		ProductID:          14397,
		ChargedAt:          "2026-05-08T07:40:00Z",
	})
	if err != nil {
		t.Fatalf("NotifyChargeSuccess: %v", err)
	}
	if payload.TenantID != "tenant-1" || payload.ChannelID != "channel-1" {
		t.Fatalf("expected tenant/channel propagated, got %+v", payload)
	}
}

func TestNotifyChargeSuccessReturnsErrorOnAcquisitionFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	t.Setenv("ACQUISITION_API_URL", server.URL)
	t.Setenv("INTERNAL_API_SECRET", "test-secret")
	client := NewAcquisitionClient(zap.NewNop())

	err := client.NotifyChargeSuccess(&ChargeSuccessRequest{
		TimweTransactionID: "charge-tx-1",
		TenantID:           "tenant-1",
		ChannelID:          "channel-1",
	})
	if err == nil {
		t.Fatal("expected acquisition failure to return an error")
	}
}

func TestNotifyPartnerSubscriptionPropagatesTenantChannelAndSignsRequest(t *testing.T) {
	var payload PartnerSubscriptionRequest
	var signature, timestamp string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/acquisition/partner-subscription" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		signature = r.Header.Get("X-Internal-Signature")
		timestamp = r.Header.Get("X-Internal-Timestamp")
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(PartnerSubscriptionResponse{Success: true, Message: "ok"})
	}))
	defer server.Close()

	t.Setenv("ACQUISITION_API_URL", server.URL)
	t.Setenv("INTERNAL_API_SECRET", "test-secret")
	client := NewAcquisitionClient(zap.NewNop())

	err := client.NotifyPartnerSubscription(&PartnerSubscriptionRequest{
		Action:             "optin",
		TenantID:           "tenant-1",
		ChannelID:          "channel-1",
		ChannelKey:         "web-gh-airteltigo",
		MSISDN:             "233241234567",
		ProductID:          14397,
		TimweTransactionID: "timwe-tx-1",
		SubscriptionResult: "OPTIN_ACTIVE_WAIT_CHARGING",
		EntryChannel:       "web",
	})
	if err != nil {
		t.Fatalf("NotifyPartnerSubscription: %v", err)
	}
	if payload.TenantID != "tenant-1" || payload.ChannelID != "channel-1" || payload.Action != "optin" {
		t.Fatalf("expected tenant/channel/action propagated, got %+v", payload)
	}
	if signature == "" || timestamp == "" {
		t.Fatal("expected internal auth headers to be set")
	}
}

func TestNotifyPartnerSubscriptionRequiresTenantAndMSISDN(t *testing.T) {
	t.Setenv("ACQUISITION_API_URL", "http://unused.invalid")
	t.Setenv("INTERNAL_API_SECRET", "test-secret")
	client := NewAcquisitionClient(zap.NewNop())

	tests := []struct {
		name          string
		req           *PartnerSubscriptionRequest
		wantErrSubstr string
	}{
		{"missing tenant_id", &PartnerSubscriptionRequest{MSISDN: "233241234567"}, "tenant_id is required"},
		{"missing msisdn", &PartnerSubscriptionRequest{TenantID: "tenant-1"}, "msisdn is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.NotifyPartnerSubscription(tt.req)
			if err == nil || !strings.Contains(err.Error(), tt.wantErrSubstr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErrSubstr, err)
			}
		})
	}
}

func TestNotifyPartnerSubscriptionReturnsErrorOnAcquisitionFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	t.Setenv("ACQUISITION_API_URL", server.URL)
	t.Setenv("INTERNAL_API_SECRET", "test-secret")
	client := NewAcquisitionClient(zap.NewNop())

	err := client.NotifyPartnerSubscription(&PartnerSubscriptionRequest{
		TenantID: "tenant-1",
		MSISDN:   "233241234567",
	})
	if err == nil {
		t.Fatal("expected acquisition failure to return an error")
	}
}
