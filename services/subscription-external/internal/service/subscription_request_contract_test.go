package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"

	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/sony/gobreaker"
)

func TestSendMT_UsesPostmanOptinContract(t *testing.T) {
	var capturedMethod string
	var capturedPath string
	var capturedHeaders http.Header
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		capturedHeaders = r.Header.Clone()
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"responseData": map[string]interface{}{
				"transactionId":      "tx-optin",
				"subscriptionResult": "OPTIN_PREACTIVE_WAIT_CONF",
			},
			"message":   "ok",
			"inError":   false,
			"requestId": "req-optin",
			"code":      "SUCCESS",
		})
	}))
	defer server.Close()

	service := newSubscriptionServiceForExternalTxIDTest(server.URL)
	service.circuitBreaker = gobreaker.NewTwoStepCircuitBreaker(gobreaker.Settings{})
	service.bulkhead = make(chan struct{}, 1)

	reqData := domain.MTRequest{
		ProductID:          14397,
		PricepointID:       999,
		UserIdentifier:     "233572503330",
		UserIdentifierType: "",
		EntryChannel:       "",
		SubKeyword:         "RST",
		LargeAccount:       "8509",
		CampaignUrl:        "",
		SendDate:           "2026-02-23T00:00:00Z",
		Priority:           "NORMAL",
		Timezone:           "UTC",
		Context:            "Subscription",
		MoTransactionUUID:  "track-123",
	}

	_, err := service.SendMT(reqData, "realm", "WEB")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertRequestBasics(t, capturedMethod, capturedPath, "POST", "/subscription/optin/2117")
	assertCommonHeaders(t, capturedHeaders)

	expectedKeys := []string{
		"campaignUrl",
		"clientIp",
		"entryChannel",
		"largeAccount",
		"mcc",
		"mnc",
		"productId",
		"subKeyword",
		"trackingId",
		"userIdentifier",
		"userIdentifierType",
	}
	assertExactJSONKeys(t, capturedBody, expectedKeys)

	assertStringField(t, capturedBody, "userIdentifier", "233572503330")
	assertStringField(t, capturedBody, "userIdentifierType", "MSISDN")
	assertNumberField(t, capturedBody, "productId", 14397)
	assertStringField(t, capturedBody, "mcc", "620")
	assertStringField(t, capturedBody, "mnc", "03")
	assertStringField(t, capturedBody, "entryChannel", "INTERNAL")
	assertStringField(t, capturedBody, "clientIp", "INTERNAL")
	assertStringField(t, capturedBody, "campaignUrl", "INTERNAL")
	assertStringField(t, capturedBody, "trackingId", "track-123")
}

func TestSendOptoutWithRetry_UsesPostmanOptoutContract(t *testing.T) {
	var capturedMethod string
	var capturedPath string
	var capturedHeaders http.Header
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		capturedHeaders = r.Header.Clone()
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
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
		UserIdentifier: "233572503330",
		ProductId:      14397,
	}

	_, err := service.sendOptoutWithRetry(reqData, "realm")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertRequestBasics(t, capturedMethod, capturedPath, "POST", "/subscription/optout/2117")
	assertCommonHeaders(t, capturedHeaders)

	expectedKeys := []string{
		"cancelReason",
		"cancelSource",
		"clientIp",
		"controlKeyword",
		"controlServiceKeyword",
		"entryChannel",
		"largeAccount",
		"mcc",
		"mnc",
		"productId",
		"subId",
		"subKeyword",
		"trackingId",
		"userIdentifier",
		"userIdentifierType",
	}
	assertExactJSONKeys(t, capturedBody, expectedKeys)

	assertStringField(t, capturedBody, "userIdentifierType", "MSISDN")
	assertStringField(t, capturedBody, "entryChannel", "INTERNAL")
	assertStringField(t, capturedBody, "clientIp", "INTERNAL")
	assertStringField(t, capturedBody, "mcc", "620")
	assertStringField(t, capturedBody, "mnc", "03")
	assertNonEmptyStringField(t, capturedBody, "trackingId")
}

func TestSendStatusCheckWithRetry_UsesPostmanStatusContract(t *testing.T) {
	var capturedMethod string
	var capturedPath string
	var capturedHeaders http.Header
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		capturedHeaders = r.Header.Clone()
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
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
		UserIdentifier: "233572503330",
		ProductId:      14397,
	}

	_, err := service.sendStatusCheckWithRetry(reqData, "realm")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertRequestBasics(t, capturedMethod, capturedPath, "POST", "/subscription/status/2117")
	assertCommonHeaders(t, capturedHeaders)

	expectedKeys := []string{
		"clientIp",
		"controlKeyword",
		"controlServiceKeyword",
		"entryChannel",
		"mcc",
		"mnc",
		"productId",
		"subId",
		"userIdentifier",
		"userIdentifierType",
	}
	assertExactJSONKeys(t, capturedBody, expectedKeys)

	assertStringField(t, capturedBody, "userIdentifierType", "MSISDN")
	assertStringField(t, capturedBody, "entryChannel", "INTERNAL")
	assertStringField(t, capturedBody, "clientIp", "INTERNAL")
}

func TestSendOptinConfirmWithRetry_UsesPostmanConfirmContract(t *testing.T) {
	var capturedMethod string
	var capturedPath string
	var capturedHeaders http.Header
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		capturedHeaders = r.Header.Clone()
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
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
		ProductId:           14397,
		TransactionAuthCode: "0000",
	}

	_, err := service.sendOptinConfirmWithRetry(reqData, "realm")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	assertRequestBasics(t, capturedMethod, capturedPath, "POST", "/subscription/optin/confirm/2117")
	assertCommonHeaders(t, capturedHeaders)

	expectedKeys := []string{
		"clientIp",
		"entryChannel",
		"mcc",
		"mnc",
		"productId",
		"transactionAuthCode",
		"userIdentifier",
		"userIdentifierType",
	}
	assertExactJSONKeys(t, capturedBody, expectedKeys)

	assertStringField(t, capturedBody, "userIdentifierType", "MSISDN")
	assertStringField(t, capturedBody, "entryChannel", "INTERNAL")
	assertStringField(t, capturedBody, "clientIp", "INTERNAL")
	assertStringField(t, capturedBody, "transactionAuthCode", "0000")
}

func assertRequestBasics(t *testing.T, method string, path string, expectedMethod string, expectedPath string) {
	t.Helper()
	if method != expectedMethod {
		t.Fatalf("expected method %s, got %s", expectedMethod, method)
	}
	if path != expectedPath {
		t.Fatalf("expected path %s, got %s", expectedPath, path)
	}
}

func assertCommonHeaders(t *testing.T, headers http.Header) {
	t.Helper()
	if headers.Get("apikey") == "" {
		t.Fatalf("expected apikey header to be set")
	}
	if headers.Get("authentication") == "" {
		t.Fatalf("expected authentication header to be set")
	}
	if headers.Get("external-tx-id") == "" {
		t.Fatalf("expected external-tx-id header to be set")
	}
	if headers.Get("Content-Type") != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", headers.Get("Content-Type"))
	}
	if headers.Get("Accept") != "*/*" {
		t.Fatalf("expected Accept */*, got %q", headers.Get("Accept"))
	}
}

func assertExactJSONKeys(t *testing.T, body map[string]interface{}, expected []string) {
	t.Helper()
	actual := make([]string, 0, len(body))
	for key := range body {
		actual = append(actual, key)
	}
	sort.Strings(actual)

	expectedCopy := append([]string(nil), expected...)
	sort.Strings(expectedCopy)

	if !reflect.DeepEqual(actual, expectedCopy) {
		t.Fatalf("unexpected body keys. expected=%v actual=%v", expectedCopy, actual)
	}
}

func assertStringField(t *testing.T, body map[string]interface{}, key string, expected string) {
	t.Helper()
	value, ok := body[key]
	if !ok {
		t.Fatalf("expected key %q in body", key)
	}
	strValue, ok := value.(string)
	if !ok {
		t.Fatalf("expected key %q to be string, got %T", key, value)
	}
	if strValue != expected {
		t.Fatalf("expected key %q to be %q, got %q", key, expected, strValue)
	}
}

func assertNonEmptyStringField(t *testing.T, body map[string]interface{}, key string) {
	t.Helper()
	value, ok := body[key]
	if !ok {
		t.Fatalf("expected key %q in body", key)
	}
	strValue, ok := value.(string)
	if !ok {
		t.Fatalf("expected key %q to be string, got %T", key, value)
	}
	if strValue == "" {
		t.Fatalf("expected key %q to be non-empty", key)
	}
}

func assertNumberField(t *testing.T, body map[string]interface{}, key string, expected int) {
	t.Helper()
	value, ok := body[key]
	if !ok {
		t.Fatalf("expected key %q in body", key)
	}
	floatValue, ok := value.(float64)
	if !ok {
		t.Fatalf("expected key %q to be numeric, got %T", key, value)
	}
	if int(floatValue) != expected {
		t.Fatalf("expected key %q to be %d, got %v", key, expected, value)
	}
}
