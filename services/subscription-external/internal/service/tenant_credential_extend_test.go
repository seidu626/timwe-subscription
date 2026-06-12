// Tests for the extended ProviderCredentialSecret / TenantProviderConfig fields
// introduced to carry full per-tenant account config (TMP-cred-extend).
package service

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/sony/gobreaker"
)

// ─── (i) Resolve round-trips all new credential fields ────────────────────────

func TestResolveRoundTripsExtendedCredentialFields(t *testing.T) {
	const envName = "CRED_EXTEND_FULL"
	t.Setenv(envName, `{
		"base_url":               "https://api.timwe.test",
		"api_key":                "ext-api-key",
		"authentication_key":     "ext-auth-key",
		"partner_role_id":        "4242",
		"realm":                  "ext-realm",
		"mt_api_key":             "ext-mt-api-key",
		"mcc":                    "621",
		"mnc":                    "07",
		"large_account":          "EXT-SHORT",
		"service_name":           "ext-service",
		"free_mt_pricepoint_id":  "pp-free-001",
		"mo_pricepoint_ids":      "pp-mo-001,pp-mo-002",
		"billing_pricepoint_ids": "pp-bill-001",
		"he_iv_param_spec_key":   "he-iv-key-abc"
	}`)

	row := []driver.Value{
		"channel-ext-001",
		"tenant-ext-001",
		"timwe",
		"{optin,mt}",
		"env://" + envName,
		"cred-extend-full",
	}
	db := openFakeDB(t, row)
	defer db.Close()

	cfg := tenantRoutingTestConfig("https://fallback.invalid")
	cfg.Application.TIMWE.MCC = "global-mcc"
	cfg.Application.TIMWE.MNC = "global-mnc"
	cfg.Application.TIMWE.MTAPIKey = "global-mt-api-key"

	resolved, err := NewTenantProviderRouter(db, cfg, EnvProviderCredentialResolver{}).Resolve(
		context.Background(), ChannelOperationMT,
		domain.TenantRouteContext{TenantID: "tenant-ext-001", ChannelID: "channel-ext-001"},
	)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	checks := []struct {
		name string
		got  string
		want string
	}{
		{"MTAPIKey", resolved.MTAPIKey, "ext-mt-api-key"},
		{"MCC", resolved.MCC, "621"},
		{"MNC", resolved.MNC, "07"},
		{"LargeAccount", resolved.LargeAccount, "EXT-SHORT"},
		{"ServiceName", resolved.ServiceName, "ext-service"},
		{"FreeMTPricepointID", resolved.FreeMTPricepointID, "pp-free-001"},
		{"MOPricepointIDs", resolved.MOPricepointIDs, "pp-mo-001,pp-mo-002"},
		{"BillingPricepointIDs", resolved.BillingPricepointIDs, "pp-bill-001"},
		{"HEIVParamSpecKey", resolved.HEIVParamSpecKey, "he-iv-key-abc"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, c.got, c.want)
		}
	}
}

// TestResolveExtendedFieldsFallToGlobalConfig verifies that mcc/mnc/mt_api_key
// fall back to global config values when absent from the tenant secret.
func TestResolveExtendedFieldsFallToGlobalConfig(t *testing.T) {
	const envName = "CRED_EXTEND_NOEXTRAS"
	t.Setenv(envName, `{
		"base_url":           "https://api.timwe.test",
		"api_key":            "api-key",
		"authentication_key": "auth-key",
		"partner_role_id":    "4242",
		"realm":              "realm"
	}`)

	row := []driver.Value{
		"channel-ext-002",
		"tenant-ext-002",
		"timwe",
		"{optin,mt}",
		"env://" + envName,
		"cred-extend-noextras",
	}
	db := openFakeDB(t, row)
	defer db.Close()

	cfg := tenantRoutingTestConfig("https://fallback.invalid")
	cfg.Application.TIMWE.MCC = "620"
	cfg.Application.TIMWE.MNC = "03"
	cfg.Application.TIMWE.MTAPIKey = "global-mt-key"

	resolved, err := NewTenantProviderRouter(db, cfg, EnvProviderCredentialResolver{}).Resolve(
		context.Background(), ChannelOperationMT,
		domain.TenantRouteContext{TenantID: "tenant-ext-002", ChannelID: "channel-ext-002"},
	)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if resolved.MCC != "620" {
		t.Errorf("MCC: got %q, want global fallback %q", resolved.MCC, "620")
	}
	if resolved.MNC != "03" {
		t.Errorf("MNC: got %q, want global fallback %q", resolved.MNC, "03")
	}
	if resolved.MTAPIKey != "global-mt-key" {
		t.Errorf("MTAPIKey: got %q, want global fallback %q", resolved.MTAPIKey, "global-mt-key")
	}
	if resolved.LargeAccount != "" {
		t.Errorf("LargeAccount: expected empty, got %q", resolved.LargeAccount)
	}
}

// ─── (ii) providerCfg.MCC/MNC override global default in optin payload ───────

func TestBuildOptinPayload_TenantMCCMNCOverrideGlobal(t *testing.T) {
	svc := &SubscriptionService{
		config: newMinimalConfigForCredExtend("620", "03"),
	}
	providerCfg := &TenantProviderConfig{MCC: "234", MNC: "10"}
	req := domain.MTRequest{UserIdentifier: "447911123456", ProductID: 1}
	payload, err := svc.buildTIMWEOptinPayload(req, providerCfg)
	if err != nil {
		t.Fatalf("buildTIMWEOptinPayload: %v", err)
	}
	if payload.MCC != "234" {
		t.Errorf("MCC: got %q, want tenant override %q", payload.MCC, "234")
	}
	if payload.MNC != "10" {
		t.Errorf("MNC: got %q, want tenant override %q", payload.MNC, "10")
	}
}

func TestBuildOptinPayload_GlobalMCCMNCUsedWhenTenantAbsent(t *testing.T) {
	svc := &SubscriptionService{config: newMinimalConfigForCredExtend("620", "03")}
	payload, err := svc.buildTIMWEOptinPayload(domain.MTRequest{
		UserIdentifier: "233501234567",
		ProductID:      1,
	}, nil)
	if err != nil {
		t.Fatalf("buildTIMWEOptinPayload: %v", err)
	}
	if payload.MCC != "620" {
		t.Errorf("MCC: got %q, want global %q", payload.MCC, "620")
	}
	if payload.MNC != "03" {
		t.Errorf("MNC: got %q, want global %q", payload.MNC, "03")
	}
}

// ─── (iii) Opt-in uses the subscription api_key even when mt_api_key is set ──

func TestSendMT_UsesSubscriptionAPIKeyForOptinWhenMTAPIKeySet(t *testing.T) {
	var capturedAPIKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAPIKey = r.Header.Get("apikey")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"responseData": map[string]interface{}{
				"transactionId": "tx-1", "subscriptionResult": "SUCCESS",
			},
			"message": "ok", "inError": false, "requestId": "req-1", "code": "SUCCESS",
		})
	}))
	defer server.Close()

	svc := newSubscriptionServiceForExternalTxIDTest(server.URL)
	svc.circuitBreaker = gobreaker.NewTwoStepCircuitBreaker(gobreaker.Settings{})
	svc.bulkhead = make(chan struct{}, 1)
	svc.SetTenantProviderRouter(&fakeTenantProviderResolver{
		cfg: &TenantProviderConfig{
			TenantID: "t1", ChannelID: "c1", Provider: "timwe",
			BaseURL: server.URL, APIKey: "generic-api-key", MTAPIKey: "mt-specific-key",
			Authentication: "auth-key-long", PartnerRoleID: "2117", Realm: "realm",
		},
	})

	_, _ = svc.SendMT(domain.MTRequest{
		ProductID: 1, UserIdentifier: "233501234567",
		TenantRoute: domain.TenantRouteContext{TenantID: "t1", ChannelID: "c1"},
	}, "realm", "WEB")

	if capturedAPIKey != "generic-api-key" {
		t.Errorf("apikey header: got %q, want subscription key %q", capturedAPIKey, "generic-api-key")
	}
}

func TestSendMT_FallsBackToAPIKeyWhenMTAPIKeyAbsent(t *testing.T) {
	var capturedAPIKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAPIKey = r.Header.Get("apikey")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"responseData": map[string]interface{}{
				"transactionId": "tx-2", "subscriptionResult": "SUCCESS",
			},
			"message": "ok", "inError": false, "requestId": "req-2", "code": "SUCCESS",
		})
	}))
	defer server.Close()

	svc := newSubscriptionServiceForExternalTxIDTest(server.URL)
	svc.circuitBreaker = gobreaker.NewTwoStepCircuitBreaker(gobreaker.Settings{})
	svc.bulkhead = make(chan struct{}, 1)
	svc.SetTenantProviderRouter(&fakeTenantProviderResolver{
		cfg: &TenantProviderConfig{
			TenantID: "t2", ChannelID: "c2", Provider: "timwe",
			BaseURL: server.URL, APIKey: "generic-api-key", MTAPIKey: "",
			Authentication: "auth-key-long", PartnerRoleID: "2117", Realm: "realm",
		},
	})

	_, _ = svc.SendMT(domain.MTRequest{
		ProductID: 1, UserIdentifier: "233501234567",
		TenantRoute: domain.TenantRouteContext{TenantID: "t2", ChannelID: "c2"},
	}, "realm", "WEB")

	if capturedAPIKey != "generic-api-key" {
		t.Errorf("apikey header: got %q, want fallback %q", capturedAPIKey, "generic-api-key")
	}
}

// ─── (iv) LargeAccount fallback when product.ShortCode empty ──────────────────

func TestBuildOptinPayload_LargeAccountFallsBackToProviderCfg(t *testing.T) {
	svc := &SubscriptionService{config: newMinimalConfigForCredExtend("620", "03")}
	payload, err := svc.buildTIMWEOptinPayload(domain.MTRequest{
		UserIdentifier: "233501234567", ProductID: 1, LargeAccount: "",
	}, &TenantProviderConfig{LargeAccount: "CFG-SHORTCODE"})
	if err != nil {
		t.Fatalf("buildTIMWEOptinPayload: %v", err)
	}
	if payload.LargeAccount != "CFG-SHORTCODE" {
		t.Errorf("LargeAccount: got %q, want %q", payload.LargeAccount, "CFG-SHORTCODE")
	}
}

func TestBuildOptoutPayload_LargeAccountFallsBackToProviderCfg(t *testing.T) {
	svc := &SubscriptionService{config: newMinimalConfigForCredExtend("620", "03")}
	payload, err := svc.buildTIMWEOptoutPayload(domain.UnsubscriptionRequest{
		UserIdentifier: "233501234567", ProductId: 1, LargeAccount: nil,
	}, &TenantProviderConfig{LargeAccount: "CFG-SHORTCODE"})
	if err != nil {
		t.Fatalf("buildTIMWEOptoutPayload: %v", err)
	}
	if payload.LargeAccount != "CFG-SHORTCODE" {
		t.Errorf("LargeAccount: got %q, want %q", payload.LargeAccount, "CFG-SHORTCODE")
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// newMinimalConfigForCredExtend builds a bare *config.Config with TIMWE MCC/MNC set.
// Uses a distinct name to avoid collision with other test helpers.
func newMinimalConfigForCredExtend(mcc, mnc string) *config.Config {
	cfg := &config.Config{}
	cfg.Application.TIMWE.MCC = mcc
	cfg.Application.TIMWE.MNC = mnc
	return cfg
}
