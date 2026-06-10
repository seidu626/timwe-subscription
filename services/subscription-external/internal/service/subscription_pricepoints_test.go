// Tests for per-tenant pricepoint and LargeAccount wiring (wave2/tenant-pricepoints).
//
// Precedence rule (documented at each wiring site):
//   product-level value (non-zero) wins → tenant providerCfg value is fallback → absent-both keeps legacy behaviour.
//
// Coverage per field:
//   FreeMTPricepointID  — optin/MT payload pricepointId
//   BillingPricepointIDs / MOPricepointIDs — charge PricepointID
//   LargeAccount (charge path) — charge ShortCode
//
// Each field has three cases: tenant-only, product-wins, absent-both.
// Edge cases: empty-string is treated as unset (same as absent).
package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/sony/gobreaker"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

// captureOptinPayload runs a single SendMT call against a fake server and
// returns the decoded JSON body sent to TIMWE.
func captureOptinPayload(t *testing.T, mtReq domain.MTRequest, providerCfg *TenantProviderConfig) map[string]interface{} {
	t.Helper()
	var captured map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"responseData": map[string]interface{}{
				"transactionId": "tx-pp", "subscriptionResult": "OPTIN_PREACTIVE_WAIT_CONF",
			},
			"message": "ok", "inError": false, "requestId": "req-pp", "code": "SUCCESS",
		})
	}))
	t.Cleanup(server.Close)

	svc := newSubscriptionServiceForExternalTxIDTest(server.URL)
	svc.circuitBreaker = gobreaker.NewTwoStepCircuitBreaker(gobreaker.Settings{})
	svc.bulkhead = make(chan struct{}, 1)

	cfg := &TenantProviderConfig{
		TenantID: "t-pp", ChannelID: "c-pp", Provider: "timwe",
		BaseURL: server.URL, APIKey: "k", Authentication: "auth-key-long",
		PartnerRoleID: "2117", Realm: "realm",
		FreeMTPricepointID:   providerCfg.FreeMTPricepointID,
		MOPricepointIDs:      providerCfg.MOPricepointIDs,
		BillingPricepointIDs: providerCfg.BillingPricepointIDs,
		LargeAccount:         providerCfg.LargeAccount,
		MCC:                  providerCfg.MCC,
		MNC:                  providerCfg.MNC,
	}
	mtReq.TenantRoute = domain.TenantRouteContext{TenantID: "t-pp", ChannelID: "c-pp"}
	svc.SetTenantProviderRouter(&fakeTenantProviderResolver{cfg: cfg})

	_, _ = svc.SendMT(mtReq, "realm", "WEB")
	return captured
}

// captureChargePayload runs RequestCharge and returns the decoded JSON body.
func captureChargePayload(t *testing.T, chargeReq domain.ChargeRequest, providerCfg *TenantProviderConfig) map[string]interface{} {
	t.Helper()
	var captured map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("failed to decode charge body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"responseData": map[string]interface{}{},
			"message": "ok", "inError": false, "requestId": "req-chg", "code": "SUCCESS",
		})
	}))
	t.Cleanup(server.Close)

	svc := newSubscriptionServiceForExternalTxIDTest(server.URL)
	svc.circuitBreaker = gobreaker.NewTwoStepCircuitBreaker(gobreaker.Settings{})
	svc.bulkhead = make(chan struct{}, 1)

	cfg := &TenantProviderConfig{
		TenantID: "t-chg", ChannelID: "c-chg", Provider: "timwe",
		BaseURL: server.URL, APIKey: "k", Authentication: "auth-key-long",
		PartnerRoleID: "2117", Realm: "realm",
		BillingPricepointIDs: providerCfg.BillingPricepointIDs,
		MOPricepointIDs:      providerCfg.MOPricepointIDs,
		LargeAccount:         providerCfg.LargeAccount,
	}
	chargeReq.TenantRoute = domain.TenantRouteContext{TenantID: "t-chg", ChannelID: "c-chg"}
	svc.SetTenantProviderRouter(&fakeTenantProviderResolver{cfg: cfg})

	_, _ = svc.RequestCharge(chargeReq)
	return captured
}

// ─── FreeMTPricepointID ───────────────────────────────────────────────────────

// TestFreeMTPricepointID_TenantUsedWhenProductAbsent: tenant value reaches
// outbound pricepointId when the request carries no product-level pricepoint.
func TestFreeMTPricepointID_TenantUsedWhenProductAbsent(t *testing.T) {
	body := captureOptinPayload(t,
		domain.MTRequest{UserIdentifier: "233501234567", ProductID: 1, PricepointID: 0},
		&TenantProviderConfig{FreeMTPricepointID: "7001"},
	)
	assertNumberField(t, body, "pricepointId", 7001)
}

// TestFreeMTPricepointID_ProductWinsWhenBothSet: non-zero product PricepointID
// takes precedence over the tenant fallback.
func TestFreeMTPricepointID_ProductWinsWhenBothSet(t *testing.T) {
	body := captureOptinPayload(t,
		domain.MTRequest{UserIdentifier: "233501234567", ProductID: 1, PricepointID: 999},
		&TenantProviderConfig{FreeMTPricepointID: "7001"},
	)
	assertNumberField(t, body, "pricepointId", 999)
}

// TestFreeMTPricepointID_AbsentBothKeepsLegacyBehaviour: pricepointId absent
// from outbound payload when neither product-level nor tenant value is set.
func TestFreeMTPricepointID_AbsentBothKeepsLegacyBehaviour(t *testing.T) {
	body := captureOptinPayload(t,
		domain.MTRequest{UserIdentifier: "233501234567", ProductID: 1, PricepointID: 0},
		&TenantProviderConfig{FreeMTPricepointID: ""},
	)
	if _, present := body["pricepointId"]; present {
		t.Errorf("pricepointId should be absent (omitempty zero); got %v", body["pricepointId"])
	}
}

// TestFreeMTPricepointID_EmptyStringTreatedAsAbsent: whitespace-only tenant
// FreeMTPricepointID is treated the same as absent.
func TestFreeMTPricepointID_EmptyStringTreatedAsAbsent(t *testing.T) {
	body := captureOptinPayload(t,
		domain.MTRequest{UserIdentifier: "233501234567", ProductID: 1, PricepointID: 0},
		&TenantProviderConfig{FreeMTPricepointID: "   "},
	)
	if _, present := body["pricepointId"]; present {
		t.Errorf("whitespace-only FreeMTPricepointID should be treated as absent")
	}
}

// TestFreeMTPricepointID_FirstEntryOfCSVUsed: when tenant value is CSV,
// the first entry is used.
func TestFreeMTPricepointID_FirstEntryOfCSVUsed(t *testing.T) {
	body := captureOptinPayload(t,
		domain.MTRequest{UserIdentifier: "233501234567", ProductID: 1, PricepointID: 0},
		&TenantProviderConfig{FreeMTPricepointID: "7001,7002,7003"},
	)
	assertNumberField(t, body, "pricepointId", 7001)
}

// ─── BillingPricepointIDs (charge path) ───────────────────────────────────────

// TestBillingPricepointIDs_TenantUsedWhenProductAbsent: first entry of tenant
// BillingPricepointIDs applied to charge payload when product pricepoint is 0.
func TestBillingPricepointIDs_TenantUsedWhenProductAbsent(t *testing.T) {
	body := captureChargePayload(t,
		domain.ChargeRequest{ProductID: 1, PricepointID: 0, MSISDN: "233501234567"},
		&TenantProviderConfig{BillingPricepointIDs: "8001,8002"},
	)
	assertNumberField(t, body, "pricepointId", 8001)
}

// TestBillingPricepointIDs_ProductWinsWhenBothSet: non-zero product pricepoint
// takes precedence over tenant BillingPricepointIDs.
func TestBillingPricepointIDs_ProductWinsWhenBothSet(t *testing.T) {
	body := captureChargePayload(t,
		domain.ChargeRequest{ProductID: 1, PricepointID: 5000, MSISDN: "233501234567"},
		&TenantProviderConfig{BillingPricepointIDs: "8001"},
	)
	assertNumberField(t, body, "pricepointId", 5000)
}

// TestBillingPricepointIDs_AbsentBothKeepsLegacyBehaviour: absent billing
// pricepoint keeps legacy 0 in the payload.
func TestBillingPricepointIDs_AbsentBothKeepsLegacyBehaviour(t *testing.T) {
	body := captureChargePayload(t,
		domain.ChargeRequest{ProductID: 1, PricepointID: 0, MSISDN: "233501234567"},
		&TenantProviderConfig{BillingPricepointIDs: ""},
	)
	assertNumberField(t, body, "pricepointId", 0)
}

// TestBillingPricepointIDs_EmptyStringTreatedAsAbsent: whitespace-only
// BillingPricepointIDs is treated as absent.
func TestBillingPricepointIDs_EmptyStringTreatedAsAbsent(t *testing.T) {
	body := captureChargePayload(t,
		domain.ChargeRequest{ProductID: 1, PricepointID: 0, MSISDN: "233501234567"},
		&TenantProviderConfig{BillingPricepointIDs: "  "},
	)
	assertNumberField(t, body, "pricepointId", 0)
}

// ─── MOPricepointIDs (charge path secondary fallback) ─────────────────────────

// TestMOPricepointIDs_UsedWhenBillingAbsent: MOPricepointIDs acts as secondary
// fallback when BillingPricepointIDs is empty.
func TestMOPricepointIDs_UsedWhenBillingAbsent(t *testing.T) {
	body := captureChargePayload(t,
		domain.ChargeRequest{ProductID: 1, PricepointID: 0, MSISDN: "233501234567"},
		&TenantProviderConfig{BillingPricepointIDs: "", MOPricepointIDs: "9001,9002"},
	)
	assertNumberField(t, body, "pricepointId", 9001)
}

// TestMOPricepointIDs_BillingWinsOverMO: BillingPricepointIDs takes precedence
// over MOPricepointIDs when both are set.
func TestMOPricepointIDs_BillingWinsOverMO(t *testing.T) {
	body := captureChargePayload(t,
		domain.ChargeRequest{ProductID: 1, PricepointID: 0, MSISDN: "233501234567"},
		&TenantProviderConfig{BillingPricepointIDs: "8001", MOPricepointIDs: "9001"},
	)
	assertNumberField(t, body, "pricepointId", 8001)
}

// TestMOPricepointIDs_ProductWinsOverAll: product-level pricepoint wins over
// both tenant billing and MO pricepoints.
func TestMOPricepointIDs_ProductWinsOverAll(t *testing.T) {
	body := captureChargePayload(t,
		domain.ChargeRequest{ProductID: 1, PricepointID: 5000, MSISDN: "233501234567"},
		&TenantProviderConfig{BillingPricepointIDs: "8001", MOPricepointIDs: "9001"},
	)
	assertNumberField(t, body, "pricepointId", 5000)
}

// ─── LargeAccount on charge path ──────────────────────────────────────────────

// TestChargeLargeAccount_TenantUsedWhenProductAbsent: tenant LargeAccount used
// as the outbound shortCode when the request carries no product ShortCode.
func TestChargeLargeAccount_TenantUsedWhenProductAbsent(t *testing.T) {
	body := captureChargePayload(t,
		domain.ChargeRequest{ProductID: 1, MSISDN: "233501234567", ShortCode: ""},
		&TenantProviderConfig{LargeAccount: "TENANT-SC"},
	)
	assertStringField(t, body, "shortCode", "TENANT-SC")
}

// TestChargeLargeAccount_ProductWinsWhenBothSet: non-empty product ShortCode
// takes precedence over the tenant LargeAccount.
func TestChargeLargeAccount_ProductWinsWhenBothSet(t *testing.T) {
	body := captureChargePayload(t,
		domain.ChargeRequest{ProductID: 1, MSISDN: "233501234567", ShortCode: "PROD-SC"},
		&TenantProviderConfig{LargeAccount: "TENANT-SC"},
	)
	assertStringField(t, body, "shortCode", "PROD-SC")
}

// TestChargeLargeAccount_AbsentBothKeepsLegacyBehaviour: absent ShortCode and
// LargeAccount → empty shortCode in payload (legacy).
func TestChargeLargeAccount_AbsentBothKeepsLegacyBehaviour(t *testing.T) {
	body := captureChargePayload(t,
		domain.ChargeRequest{ProductID: 1, MSISDN: "233501234567", ShortCode: ""},
		&TenantProviderConfig{LargeAccount: ""},
	)
	assertStringField(t, body, "shortCode", "")
}

// TestChargeLargeAccount_EmptyStringTreatedAsAbsent: whitespace-only tenant
// LargeAccount is treated as absent.
func TestChargeLargeAccount_EmptyStringTreatedAsAbsent(t *testing.T) {
	body := captureChargePayload(t,
		domain.ChargeRequest{ProductID: 1, MSISDN: "233501234567", ShortCode: ""},
		&TenantProviderConfig{LargeAccount: "   "},
	)
	assertStringField(t, body, "shortCode", "")
}

// ─── tenantPricepointFallback unit tests ──────────────────────────────────────

func TestTenantPricepointFallback_ProductWins(t *testing.T) {
	if got := tenantPricepointFallback(999, "7001"); got != 999 {
		t.Errorf("product value should win: got %d, want 999", got)
	}
}

func TestTenantPricepointFallback_TenantUsedWhenProductZero(t *testing.T) {
	if got := tenantPricepointFallback(0, "7001"); got != 7001 {
		t.Errorf("tenant fallback should apply: got %d, want 7001", got)
	}
}

func TestTenantPricepointFallback_FirstCSVEntry(t *testing.T) {
	if got := tenantPricepointFallback(0, "7001,7002"); got != 7001 {
		t.Errorf("first CSV entry should be used: got %d, want 7001", got)
	}
}

func TestTenantPricepointFallback_EmptyCSVReturnsProductValue(t *testing.T) {
	if got := tenantPricepointFallback(0, ""); got != 0 {
		t.Errorf("empty CSV should return productValue unchanged: got %d, want 0", got)
	}
}

func TestTenantPricepointFallback_WhitespaceCSVTreatedAsAbsent(t *testing.T) {
	if got := tenantPricepointFallback(0, "  "); got != 0 {
		t.Errorf("whitespace CSV should be treated as absent: got %d, want 0", got)
	}
}

func TestTenantPricepointFallback_InvalidCSVEntryReturnsProductValue(t *testing.T) {
	if got := tenantPricepointFallback(0, "not-a-number"); got != 0 {
		t.Errorf("non-numeric CSV should return productValue unchanged: got %d, want 0", got)
	}
}
