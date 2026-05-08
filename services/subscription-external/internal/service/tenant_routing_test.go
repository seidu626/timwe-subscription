// slice-harness: allow-new-canonical-path: TMP-007 tests the tenant routing module.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"go.uber.org/zap"
)

type fakeTenantProviderResolver struct {
	cfg       *TenantProviderConfig
	err       error
	seenOp    ChannelOperation
	seenRoute domain.TenantRouteContext
}

func (r *fakeTenantProviderResolver) Resolve(ctx context.Context, operation ChannelOperation, route domain.TenantRouteContext) (*TenantProviderConfig, error) {
	_ = ctx
	r.seenOp = operation
	r.seenRoute = route
	if r.err != nil {
		return nil, r.err
	}
	return r.cfg, nil
}

func TestTenantRoutingOperationAllowedRequiresExplicitCapability(t *testing.T) {
	policy := map[ChannelOperation][]string{
		ChannelOperationCharge: []string{"charge"},
		ChannelOperationMT:     []string{"mt", "optin"},
	}
	if !operationAllowed(ChannelOperationMT, []string{"optin"}, policy) {
		t.Fatal("MT should be allowed by optin compatibility capability")
	}
	if operationAllowed(ChannelOperationCharge, []string{"optin"}, policy) {
		t.Fatal("charge must not be allowed by optin-only channel")
	}
}

func TestEnvProviderCredentialResolver(t *testing.T) {
	t.Setenv("TMP007_TIMWE_CREDENTIAL", `{"base_url":"http://timwe.test","api_key":"api","authentication_key":"auth","partner_role_id":"2117","realm":"realm"}`)
	secret, err := (EnvProviderCredentialResolver{}).ResolveProviderCredential(context.Background(), "env://TMP007_TIMWE_CREDENTIAL")
	if err != nil {
		t.Fatalf("expected env credential to resolve: %v", err)
	}
	if secret.BaseURL != "http://timwe.test" || secret.APIKey != "api" || secret.PartnerRoleID != "2117" {
		t.Fatalf("unexpected secret: %+v", secret)
	}

	_, err = (EnvProviderCredentialResolver{}).ResolveProviderCredential(context.Background(), "env://TMP007_MISSING")
	if !errors.Is(err, ErrTenantCredentialMissing) {
		t.Fatalf("expected missing credential error, got %v", err)
	}
}

func TestRedactProviderHeaders(t *testing.T) {
	headers := redactProviderHeaders(map[string]string{
		"apikey":         "api-secret",
		"authentication": "auth-secret",
		"external-tx-id": "tx-1",
	})
	if headers["apikey"] != "[REDACTED]" || headers["authentication"] != "[REDACTED]" {
		t.Fatalf("expected provider credentials redacted, got %+v", headers)
	}
	if headers["external-tx-id"] != "tx-1" {
		t.Fatalf("expected non-secret header preserved, got %+v", headers)
	}
}

func TestSendMTRoutesThroughTenantProviderConfig(t *testing.T) {
	var providerPath string
	var providerAPIKey string
	var providerAuth string
	var payload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providerPath = r.URL.Path
		providerAPIKey = r.Header.Get("apikey")
		providerAuth = r.Header.Get("authentication")
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode provider payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"responseData": map[string]interface{}{
				"transactionId":      "tenant-tx-1",
				"subscriptionResult": "OPTIN_PREACTIVE_WAIT_CONF",
			},
			"message":   "ok",
			"inError":   false,
			"requestId": "req-tenant-mt",
			"code":      "SUCCESS",
		})
	}))
	defer server.Close()

	cfg := tenantRoutingTestConfig("http://legacy.invalid")
	svc := NewSubscriptionService(zap.NewNop(), nil, nil, nil, cfg, nil)
	router := &fakeTenantProviderResolver{
		cfg: &TenantProviderConfig{
			TenantID:       "tenant-1",
			ChannelID:      "channel-1",
			Provider:       "timwe",
			BaseURL:        server.URL,
			APIKey:         "tenant-api-key",
			Authentication: "tenant-auth-key",
			PartnerRoleID:  "9090",
			Realm:          "tenant-realm",
		},
	}
	svc.SetTenantProviderRouter(router)

	resp, err := svc.SendMT(domain.MTRequest{
		ProductID:          14397,
		UserIdentifier:     "233241234567",
		UserIdentifierType: "MSISDN",
		EntryChannel:       "WEB",
		TenantRoute: domain.TenantRouteContext{
			TenantID:  "tenant-1",
			ChannelID: "channel-1",
		},
	}, "legacy-realm", "WEB")
	if err != nil {
		t.Fatalf("expected tenant-routed MT success, got %v", err)
	}
	if resp == nil || resp.Code != ResponseCodeSuccess {
		t.Fatalf("expected SUCCESS response, got %+v", resp)
	}
	if router.seenOp != ChannelOperationMT {
		t.Fatalf("expected MT operation, got %s", router.seenOp)
	}
	if router.seenRoute.TenantID != "tenant-1" || router.seenRoute.ChannelID != "channel-1" {
		t.Fatalf("resolver saw wrong route: %+v", router.seenRoute)
	}
	if providerPath != "/subscription/optin/9090" {
		t.Fatalf("expected tenant partner role path, got %s", providerPath)
	}
	if providerAPIKey != "tenant-api-key" || providerAuth != "tenant-auth-key" {
		t.Fatalf("expected tenant credentials, got api=%q auth=%q", providerAPIKey, providerAuth)
	}
	if got := payload["userIdentifier"]; got != "233241234567" {
		t.Fatalf("expected MSISDN in provider payload, got %#v", got)
	}
}

func TestSendMTFailsClosedBeforeProviderCallWhenTenantRoutingRejects(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want error
	}{
		{name: "unsupported capability", err: ErrUnsupportedChannelOperation, want: ErrUnsupportedChannelOperation},
		{name: "missing credential", err: ErrTenantCredentialMissing, want: ErrTenantCredentialMissing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			providerCalls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				providerCalls++
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			svc := NewSubscriptionService(zap.NewNop(), nil, nil, nil, tenantRoutingTestConfig(server.URL), nil)
			svc.SetTenantProviderRouter(&fakeTenantProviderResolver{err: tc.err})

			resp, err := svc.SendMT(domain.MTRequest{
				ProductID:          14397,
				UserIdentifier:     "233241234567",
				UserIdentifierType: "MSISDN",
				EntryChannel:       "WEB",
				TenantRoute: domain.TenantRouteContext{
					TenantID:  "tenant-1",
					ChannelID: "channel-1",
				},
			}, "tenant-realm", "WEB")
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got resp=%+v err=%v", tc.want, resp, err)
			}
			if providerCalls != 0 {
				t.Fatalf("expected no provider calls, got %d", providerCalls)
			}
		})
	}
}

func TestMapMTRequestToSubscriptionRequestPreservesTenantChannel(t *testing.T) {
	got := domain.MapMTRequestToSubscriptionRequest(domain.MTRequest{
		ProductID:          14397,
		UserIdentifier:     "233241234567",
		UserIdentifierType: "MSISDN",
		EntryChannel:       "WEB",
		TenantRoute: domain.TenantRouteContext{
			TenantID:  "tenant-1",
			ChannelID: "channel-1",
		},
	}, "tx-1", 9090, "127.0.0.1", "https://campaign.example")
	if got.TenantID == nil || *got.TenantID != "tenant-1" {
		t.Fatalf("expected tenant id preserved, got %+v", got.TenantID)
	}
	if got.ChannelID == nil || *got.ChannelID != "channel-1" {
		t.Fatalf("expected channel id preserved, got %+v", got.ChannelID)
	}
}

func tenantRoutingTestConfig(baseURL string) *config.Config {
	cfg := &config.Config{}
	cfg.Application.TIMWE.BaseURL = baseURL
	cfg.Application.TIMWE.APIKey = "legacy-api-key"
	cfg.Application.TIMWE.AuthenticationKey = "legacy-auth-key"
	cfg.Application.TIMWE.PartnerRoleID = "2117"
	cfg.Application.TIMWE.Realm = "legacy-realm"
	cfg.Application.TIMWE.Timeout = 2 * time.Second
	cfg.Application.TIMWE.MaxConnections = 10
	cfg.Application.TIMWE.CBMaxRequests = 1
	cfg.Application.TIMWE.CBMinRequests = 1
	cfg.Application.TIMWE.CBTimeout = time.Second
	cfg.Application.TIMWE.CBInterval = time.Second
	return cfg
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
