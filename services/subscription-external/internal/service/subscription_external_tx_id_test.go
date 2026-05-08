package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

func TestSendMTWithRetry_UsesUniqueExternalTxIDPerAttempt(t *testing.T) {
	attempt := 0
	externalIDs := make([]string, 0, 2)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		externalIDs = append(externalIDs, r.Header.Get("external-tx-id"))
		w.Header().Set("Content-Type", "application/json")

		if attempt == 1 {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"responseData": map[string]interface{}{},
				"message":      "retry",
				"inError":      false,
				"requestId":    "req-1",
				"code":         "INTERNAL_ERROR",
			})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"responseData": map[string]interface{}{
				"transactionId":      "tx-2",
				"subscriptionResult": "OPTIN_PREACTIVE_WAIT_CONF",
			},
			"message":   "ok",
			"inError":   false,
			"requestId": "req-2",
			"code":      "SUCCESS",
		})
	}))
	defer server.Close()

	service := newSubscriptionServiceForExternalTxIDTest(server.URL)
	reqData := domain.MTRequest{
		ProductID:          14397,
		UserIdentifier:     "233572503330",
		UserIdentifierType: "MSISDN",
		EntryChannel:       "WEB",
	}
	requestBody, err := json.Marshal(reqData)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	resp, err := service.sendMTWithRetry(reqData, server.URL, "api-key", "auth-key", requestBody, 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp == nil || resp.Code != "SUCCESS" {
		t.Fatalf("expected SUCCESS response, got %+v", resp)
	}
	assertUniqueIDs(t, externalIDs, 2)
}

func TestSendOptoutWithRetry_UsesUniqueExternalTxIDPerAttempt(t *testing.T) {
	attempt := 0
	externalIDs := make([]string, 0, 2)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		externalIDs = append(externalIDs, r.Header.Get("external-tx-id"))
		w.Header().Set("Content-Type", "application/json")

		if attempt == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":"INTERNAL_ERROR","message":"retry"}`))
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"responseData": map[string]interface{}{},
			"message":      "ok",
			"inError":      false,
			"requestId":    "req-optout",
			"code":         "SUCCESS",
		})
	}))
	defer server.Close()

	service := newSubscriptionServiceForExternalTxIDTest(server.URL)
	reqData := domain.UnsubscriptionRequest{
		UserIdentifier:     "233572503330",
		UserIdentifierType: "MSISDN",
		ProductId:          14397,
	}

	resp, err := service.sendOptoutWithRetry(reqData, "WEB")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp == nil || resp.Code != "SUCCESS" {
		t.Fatalf("expected SUCCESS response, got %+v", resp)
	}
	assertUniqueIDs(t, externalIDs, 2)
}

func TestSendOptinConfirmWithRetry_UsesUniqueExternalTxIDPerAttempt(t *testing.T) {
	attempt := 0
	externalIDs := make([]string, 0, 2)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		externalIDs = append(externalIDs, r.Header.Get("external-tx-id"))
		w.Header().Set("Content-Type", "application/json")

		if attempt == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":"INTERNAL_ERROR","message":"retry"}`))
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"responseData": map[string]interface{}{
				"transactionId":      "tx-confirm",
				"subscriptionResult": "OPTIN_PREACTIVE_WAIT_CONF",
			},
			"message":   "ok",
			"inError":   false,
			"requestId": "req-confirm",
			"code":      "SUCCESS",
		})
	}))
	defer server.Close()

	service := newSubscriptionServiceForExternalTxIDTest(server.URL)
	reqData := domain.SubscriptionConfirmationRequest{
		UserIdentifier:      "233572503330",
		UserIdentifierType:  "MSISDN",
		ProductId:           14397,
		TransactionAuthCode: "000",
	}

	resp, err := service.sendOptinConfirmWithRetry(reqData, "WEB")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp == nil || resp.Code != "SUCCESS" {
		t.Fatalf("expected SUCCESS response, got %+v", resp)
	}
	assertUniqueIDs(t, externalIDs, 2)
}

func TestSendStatusCheckWithRetry_UsesUniqueExternalTxIDPerAttempt(t *testing.T) {
	attempt := 0
	externalIDs := make([]string, 0, 2)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		externalIDs = append(externalIDs, r.Header.Get("external-tx-id"))
		w.Header().Set("Content-Type", "application/json")

		if attempt == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":"INTERNAL_ERROR","message":"retry"}`))
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"responseData": map[string]interface{}{},
			"message":      "ok",
			"inError":      false,
			"requestId":    "req-status",
			"code":         "SUCCESS",
		})
	}))
	defer server.Close()

	service := newSubscriptionServiceForExternalTxIDTest(server.URL)
	reqData := domain.GetStatusRequest{
		UserIdentifier:     "233572503330",
		UserIdentifierType: "MSISDN",
		ProductId:          14397,
	}

	resp, err := service.sendStatusCheckWithRetry(reqData, "WEB")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp == nil || resp.Code != "SUCCESS" {
		t.Fatalf("expected SUCCESS response, got %+v", resp)
	}
	assertUniqueIDs(t, externalIDs, 2)
}

func assertUniqueIDs(t *testing.T, ids []string, expected int) {
	t.Helper()

	if len(ids) != expected {
		t.Fatalf("expected %d requests, got %d", expected, len(ids))
	}
	if ids[0] == "" || ids[1] == "" {
		t.Fatalf("expected non-empty external-tx-id headers, got %+v", ids)
	}
	if ids[0] == ids[1] {
		t.Fatalf("expected unique external-tx-id per attempt, got %+v", ids)
	}
}

func newSubscriptionServiceForExternalTxIDTest(baseURL string) *SubscriptionService {
	cfg := &config.Config{}
	cfg.Application.TIMWE.BaseURL = baseURL
	cfg.Application.TIMWE.APIKey = "test-api-key"
	cfg.Application.TIMWE.AuthenticationKey = "test-auth-key-long-enough"
	cfg.Application.TIMWE.PartnerRoleID = "2117"
	cfg.Application.TIMWE.Timeout = time.Second

	return &SubscriptionService{
		logger: zap.NewNop(),
		config: cfg,
		client: &fasthttp.Client{},
	}
}
