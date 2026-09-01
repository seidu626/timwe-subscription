package catalog

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestMADAPIClientVerifiedAndTokenCached(t *testing.T) {
	var tokenCalls atomic.Int32
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		tokenCalls.Add(1)
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Basic ") {
			t.Error("missing basic auth")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"test-token","expires_in":3600}`)
	})
	mux.HandleFunc("/kyc/233551234567", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing bearer token")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"transactionId":"tx-1","data":{"registrationStatus":"REGISTERED"}}`)
	})

	client := NewMADAPIClient(MADAPIConfig{BaseURL: server.URL, TokenURL: server.URL + "/token", KYCPath: "/kyc/{customerId}", ClientID: "id", ClientSecret: "secret"}, server.Client())
	for i := 0; i < 2; i++ {
		result := client.Verify(context.Background(), "233551234567")
		if result.Status != StatusVerified || result.Reference != "tx-1" {
			t.Fatalf("unexpected result: %+v", result)
		}
	}
	if tokenCalls.Load() != 1 {
		t.Fatalf("token calls = %d, want 1", tokenCalls.Load())
	}
}

func TestMADAPIClientNotFoundIsInvalid(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"access_token":"token","expires_in":3600}`)
	})
	mux.HandleFunc("/kyc/233551234567", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })
	client := NewMADAPIClient(MADAPIConfig{BaseURL: server.URL, TokenURL: server.URL + "/token", KYCPath: "/kyc/{customerId}", ClientID: "id", ClientSecret: "secret"}, server.Client())
	result := client.Verify(context.Background(), "233551234567")
	if result.Status != StatusInvalid {
		t.Fatalf("status = %q, want INVALID", result.Status)
	}
}

func TestMADAPIClientFailsClosedWhenUnconfigured(t *testing.T) {
	result := NewMADAPIClient(MADAPIConfig{}, nil).Verify(context.Background(), "233551234567")
	if result.Status != StatusError || !strings.Contains(result.Reason, "not configured") {
		t.Fatalf("unexpected result: %+v", result)
	}
}
