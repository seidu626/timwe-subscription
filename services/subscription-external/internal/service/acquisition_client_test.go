package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
