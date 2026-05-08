package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"go.uber.org/zap"
)

func TestTIMWEClient_OptInUsesSubscriptionExternalEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/external/v1/WEB/mt" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode payload: %v", err)
		}
		if payload["msisdn"] != "233241234567" {
			t.Fatalf("expected msisdn payload field, got %+v", payload)
		}
		if _, has := payload["userIdentifier"]; has {
			t.Fatalf("unexpected userIdentifier field in payload: %+v", payload)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"responseData": map[string]interface{}{
				"transactionId": "tx-optin-1",
			},
			"message":   "waiting for confirm",
			"inError":   false,
			"requestId": "req-1",
			"code":      "OPTIN_WAITING",
		})
	}))
	defer server.Close()

	client := newTIMWEClientForTest(server.URL)
	resp, err := client.OptIn("233241234567", 8509, "WEB", map[string]string{"click_id": "abc"}, "2117")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !resp.RequiresConfirm {
		t.Fatalf("expected RequiresConfirm=true, got %+v", resp)
	}
	if resp.TransactionID != "tx-optin-1" {
		t.Fatalf("expected transaction ID from responseData, got %s", resp.TransactionID)
	}
}

func TestTIMWEClient_OptInWithTenantSignsTenantChannelContext(t *testing.T) {
	const secret = "trusted-secret"
	var capturedHeaders http.Header
	var capturedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		if _, err := tenantctx.IdentityFromTrustedRequest(r.Method, r.URL.EscapedPath(), r.Header, tenantctx.TrustedHeaderOptions{Secret: secret}); err != nil {
			t.Fatalf("trusted tenant headers did not verify: %v", err)
		}
		if err := json.NewDecoder(r.Body).Decode(&capturedPayload); err != nil {
			t.Fatalf("failed to decode payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"responseData": map[string]interface{}{"transactionId": "tx-tenant"},
			"message":      "ok",
			"inError":      false,
			"requestId":    "req-tenant",
			"code":         "SUCCESS",
		})
	}))
	defer server.Close()

	client := newTIMWEClientForTest(server.URL)
	client.config.TrustedServiceSecret = secret
	client.config.ServiceID = "acquisition-api"

	_, err := client.OptInWithTenant(
		"233241234567",
		8509,
		"WEB",
		map[string]string{"click_id": "abc"},
		"2117",
		TenantSubscriptionContext{
			TenantID:  "11111111-1111-1111-1111-111111111111",
			ChannelID: "22222222-2222-2222-2222-222222222222",
		},
	)
	if err != nil {
		t.Fatalf("expected tenant optin to succeed, got %v", err)
	}
	if capturedHeaders.Get(tenantctx.HeaderTenantID) != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("expected tenant header, got %q", capturedHeaders.Get(tenantctx.HeaderTenantID))
	}
	if capturedHeaders.Get("X-Tenant-Channel-Id") != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("expected channel header, got %q", capturedHeaders.Get("X-Tenant-Channel-Id"))
	}
	if capturedPayload["channelId"] != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("expected channelId in subscription-external payload, got %+v", capturedPayload)
	}
}

func TestTIMWEClient_ConfirmTreatsPendingSuccessAsNonFinal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/external/v1/subscription/optin/confirm" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"responseData": map[string]interface{}{},
			"message":      "accepted but pending",
			"inError":      false,
			"requestId":    "req-2",
			"code":         "SUCCESS",
		})
	}))
	defer server.Close()

	client := newTIMWEClientForTest(server.URL)
	resp, err := client.Confirm("233241234567", 8509, "WEB", "2117", "1234")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.Success {
		t.Fatalf("expected non-final SUCCESS to be treated as not confirmed, got %+v", resp)
	}
}

func TestTIMWEClient_ConfirmTreatsSuccessWithoutPendingHintsAsFinal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/external/v1/subscription/optin/confirm" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"responseData": map[string]interface{}{},
			"message":      "null",
			"inError":      false,
			"requestId":    "req-2b",
			"code":         "SUCCESS",
		})
	}))
	defer server.Close()

	client := newTIMWEClientForTest(server.URL)
	resp, err := client.Confirm("233241234567", 8509, "WEB", "2117", "1234")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected SUCCESS without pending indicators to be final, got %+v", resp)
	}
	if resp.Message != "" {
		t.Fatalf("expected \"null\" message to be sanitized to empty, got %q", resp.Message)
	}
}

func TestTIMWEClient_ConfirmTreatsExplicitPendingStatusAsNonFinal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/external/v1/subscription/optin/confirm" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"responseData": map[string]interface{}{
				"status": "OPTIN_WAITING",
			},
			"message":   "ok",
			"inError":   false,
			"requestId": "req-2c",
			"code":      "SUCCESS",
		})
	}))
	defer server.Close()

	client := newTIMWEClientForTest(server.URL)
	resp, err := client.Confirm("233241234567", 8509, "WEB", "2117", "1234")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.Success {
		t.Fatalf("expected explicit pending response status to remain non-final, got %+v", resp)
	}
}

func TestTIMWEClient_ConfirmTreatsInnerSuccessStatusAsFinal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/external/v1/subscription/optin/confirm" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"responseData": map[string]interface{}{
				"subscriptionResult": "SUCCESS",
			},
			"message":   "",
			"inError":   false,
			"requestId": "req-2d",
			"code":      "SUCCESS",
		})
	}))
	defer server.Close()

	client := newTIMWEClientForTest(server.URL)
	resp, err := client.Confirm("233241234567", 8509, "WEB", "2117", "1234")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected inner SUCCESS subscriptionResult to be treated as final, got %+v", resp)
	}
	if resp.Status != "SUCCESS" {
		t.Fatalf("expected status to reflect inner subscriptionResult, got %q", resp.Status)
	}
}

func TestTIMWEClient_ConfirmPropagatesInnerStatusToResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"responseData": map[string]interface{}{
				"status": "OPTIN_WAITING",
			},
			"message":   "ok",
			"inError":   false,
			"requestId": "req-2e",
			"code":      "SUCCESS",
		})
	}))
	defer server.Close()

	client := newTIMWEClientForTest(server.URL)
	resp, err := client.Confirm("233241234567", 8509, "WEB", "2117", "1234")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.Status != "OPTIN_WAITING" {
		t.Fatalf("expected inner status OPTIN_WAITING to be propagated, got %q", resp.Status)
	}
}

func TestTIMWEClient_OptInTreatsPreactiveWaitConfAsRequiresConfirm(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/external/v1/WEB/mt" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"responseData": map[string]interface{}{
				"transactionId":      "tx-optin-preactive",
				"subscriptionResult": "OPTIN_PREACTIVE_WAIT_CONF",
			},
			"message":   "accepted and waiting for confirmation",
			"inError":   false,
			"requestId": "req-preactive",
			"code":      "SUCCESS",
		})
	}))
	defer server.Close()

	client := newTIMWEClientForTest(server.URL)
	resp, err := client.OptIn("233241234567", 8509, "WEB", map[string]string{"click_id": "abc"}, "2117")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected Success=true, got %+v", resp)
	}
	if !resp.RequiresConfirm {
		t.Fatalf("expected RequiresConfirm=true, got %+v", resp)
	}
}

func TestSendMTRequestWithRetry_UsesUniqueExternalTxIDPerAttempt(t *testing.T) {
	attempt := 0
	externalIDs := make([]string, 0, 2)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		externalIDs = append(externalIDs, r.Header.Get("external-tx-id"))

		if attempt == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":"INTERNAL_ERROR","message":"temporary failure"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"responseData": map[string]interface{}{},
			"message":      "ok",
			"inError":      false,
			"requestId":    "req-retry",
			"code":         "SUCCESS",
		})
	}))
	defer server.Close()

	client := newTIMWEClientForTest(server.URL)
	client.config.MaxRetries = 2

	resp, err := client.sendMTRequestWithRetry(server.URL, []byte(`{"payload":"value"}`), outboundRequestMeta{
		Operation: "test",
		MSISDN:    "233241234567",
		ProductID: 8509,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp == nil || resp.Code != "SUCCESS" {
		t.Fatalf("expected SUCCESS response, got %+v", resp)
	}
	if len(externalIDs) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(externalIDs))
	}
	if externalIDs[0] == "" || externalIDs[1] == "" {
		t.Fatalf("expected non-empty external-tx-id headers, got %+v", externalIDs)
	}
	if externalIDs[0] == externalIDs[1] {
		t.Fatalf("expected unique external-tx-id per attempt, got %+v", externalIDs)
	}
}

func TestTIMWEClient_ConfirmUsesUniqueExternalTxIDPerAttempt(t *testing.T) {
	attempt := 0
	externalIDs := make([]string, 0, 2)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/external/v1/subscription/optin/confirm" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		attempt++
		externalIDs = append(externalIDs, r.Header.Get("external-tx-id"))
		if attempt == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":"INTERNAL_ERROR","message":"temporary failure"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"responseData": map[string]interface{}{
				"subscriptionResult": "SUBSCRIBED",
			},
			"message":   "ok",
			"inError":   false,
			"requestId": "req-confirm",
			"code":      "CONFIRMED",
		})
	}))
	defer server.Close()

	client := newTIMWEClientForTest(server.URL)
	client.config.MaxRetries = 2

	resp, err := client.Confirm("233241234567", 8509, "WEB", "2117", "1234")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp == nil || !resp.Success {
		t.Fatalf("expected successful response, got %+v", resp)
	}
	if len(externalIDs) != 2 {
		t.Fatalf("expected 2 confirm attempts, got %d", len(externalIDs))
	}
	if externalIDs[0] == "" || externalIDs[1] == "" {
		t.Fatalf("expected non-empty external-tx-id headers, got %+v", externalIDs)
	}
	if externalIDs[0] == externalIDs[1] {
		t.Fatalf("expected unique external-tx-id per confirm attempt, got %+v", externalIDs)
	}
}

func newTIMWEClientForTest(baseURL string) *TIMWEClientImpl {
	cfg := DefaultTIMWEConfig()
	cfg.BaseURL = baseURL
	cfg.MaxRetries = 1
	cfg.RetryBaseDelay = 1 * time.Millisecond
	cfg.RetryMaxDelay = 1 * time.Millisecond
	return NewTIMWEClientWithConfig(cfg, zap.NewNop())
}

func TestExtractUpstreamErrorDetails(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "includes code and message",
			body: `{"code":"INTERNAL_ERROR","message":"failed to generate auth key"}`,
			want: "code=INTERNAL_ERROR message=failed to generate auth key",
		},
		{
			name: "falls back to error field",
			body: `{"error":"upstream validation failed"}`,
			want: "message=upstream validation failed",
		},
		{
			name: "invalid json returns empty",
			body: `not-json`,
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractUpstreamErrorDetails([]byte(tc.body))
			if strings.TrimSpace(got) != strings.TrimSpace(tc.want) {
				t.Fatalf("unexpected details: got=%q want=%q", got, tc.want)
			}
		})
	}
}
