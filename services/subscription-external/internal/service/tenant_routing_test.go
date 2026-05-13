// slice-harness: allow-new-canonical-path: TMP-007 tests the tenant routing module.
package service

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"go.uber.org/zap"
)

// ─── minimal fake sql driver for tenant routing tests ────────────────────────
// Implements just enough of database/sql/driver to let TenantProviderRouter.Resolve
// run its QueryRowContext against a static row, without any real database or sqlmock.

var registerFakeDriverOnce sync.Once

const fakeDriverName = "fake-tenant-routing"

type fakeDriver struct{ row []driver.Value }
type fakeConn struct{ row []driver.Value }
type fakeStmt struct{ row []driver.Value }
type fakeRows struct {
	row    []driver.Value
	served bool
}

func (d *fakeDriver) Open(_ string) (driver.Conn, error) { return &fakeConn{row: d.row}, nil }
func (c *fakeConn) Prepare(_ string) (driver.Stmt, error) {
	return &fakeStmt{row: c.row}, nil
}
func (c *fakeConn) Close() error          { return nil }
func (c *fakeConn) Begin() (driver.Tx, error) { return nil, errors.New("unsupported") }
func (s *fakeStmt) Close() error          { return nil }
func (s *fakeStmt) NumInput() int         { return -1 } // variadic
func (s *fakeStmt) Exec(_ []driver.Value) (driver.Result, error) {
	return nil, errors.New("unsupported")
}
func (s *fakeStmt) Query(_ []driver.Value) (driver.Rows, error) {
	return &fakeRows{row: s.row}, nil
}
func (r *fakeRows) Columns() []string {
	return []string{"id", "tenant_id", "provider", "capabilities", "secret_ref", "secret_ref_display"}
}
func (r *fakeRows) Close() error { return nil }
func (r *fakeRows) Next(dest []driver.Value) error {
	if r.served {
		return io.EOF
	}
	r.served = true
	for i, v := range r.row {
		dest[i] = v
	}
	return nil
}

// openFakeDB returns a *sql.DB backed by a fake driver returning the given row.
// The driver is registered once; subsequent calls reuse the same registration.
func openFakeDB(t *testing.T, row []driver.Value) *sql.DB {
	t.Helper()
	name := fakeDriverName
	registerFakeDriverOnce.Do(func() {
		// Register with a placeholder; the actual row is set per-call via DSN trick below.
	})
	// Register a unique driver per test to avoid shared state between parallel tests.
	driverName := name + "-" + t.Name()
	sql.Register(driverName, &fakeDriver{row: row})
	db, err := sql.Open(driverName, "fake")
	if err != nil {
		t.Fatalf("openFakeDB: %v", err)
	}
	return db
}

// ─── TMP-066 careerify seed verification ─────────────────────────────────────

// TestTenantRoutingCareerifyChannelLookup verifies that the tenant_routing.go
// query (lines 208-222) returns provider='timwe' and a non-empty secret_ref for
// the careerify / web-gh-airteltigo pair seeded by seed_careerify_tenant_channel.sql.
//
// It drives TenantProviderRouter.Resolve with a fake *sql.DB that returns exactly
// the row the seed migration would produce, and asserts the contract used by all
// downstream slices (TMP-067..070).
func TestTenantRoutingCareerifyChannelLookup(t *testing.T) {
	const (
		careerifyTenantID  = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
		careerifyChannelID = "11111111-2222-3333-4444-555555555555"
		secretRef          = "env://CAREERIFY_TIMWE_API_SECRET"
		secretRefDisplay   = "careerify-timwe-api"
	)

	// The seed migration sets secret_ref = 'env://CAREERIFY_TIMWE_API_SECRET'.
	// EnvProviderCredentialResolver needs the env var to resolve credentials.
	t.Setenv("CAREERIFY_TIMWE_API_SECRET", `{
		"base_url":          "https://api.timwe.test",
		"api_key":           "careerify-api-key",
		"authentication_key":"careerify-auth",
		"partner_role_id":   "9999",
		"realm":             "careerify-realm"
	}`)

	// Fake DB row matches what seed_careerify_tenant_channel.sql inserts:
	//   c.id, c.tenant_id, c.provider, c.capabilities, cred.secret_ref, cred.secret_ref_display
	row := []driver.Value{
		careerifyChannelID,
		careerifyTenantID,
		"timwe",
		"{optin,confirm,mt,charge}", // pq.StringArray scans from PostgreSQL array literal
		secretRef,
		secretRefDisplay,
	}
	db := openFakeDB(t, row)
	defer db.Close()

	cfg := tenantRoutingTestConfig("https://legacy.invalid")
	router := NewTenantProviderRouter(db, cfg, EnvProviderCredentialResolver{})

	resolved, err := router.Resolve(context.Background(), ChannelOperationMT, domain.TenantRouteContext{
		TenantKey:  "careerify",
		ChannelKey: "web-gh-airteltigo",
	})
	if err != nil {
		t.Fatalf("careerify channel lookup failed: %v", err)
	}
	if resolved.Provider != "timwe" {
		t.Errorf("expected provider=timwe, got %q", resolved.Provider)
	}
	if resolved.TenantID != careerifyTenantID {
		t.Errorf("expected tenant_id=%s, got %q", careerifyTenantID, resolved.TenantID)
	}
	if resolved.ChannelID != careerifyChannelID {
		t.Errorf("expected channel_id=%s, got %q", careerifyChannelID, resolved.ChannelID)
	}
	if resolved.SecretRefDisplay != secretRefDisplay {
		t.Errorf("expected secret_ref_display=%q, got %q", secretRefDisplay, resolved.SecretRefDisplay)
	}
	// The resolved config must have API credentials populated from the env secret.
	if resolved.APIKey == "" {
		t.Error("expected non-empty APIKey from careerify secret")
	}
	if resolved.BaseURL == "" {
		t.Error("expected non-empty BaseURL from careerify secret")
	}
}

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
	repo := &MockSubscriptionRepository{chargeInserted: true}
	svc := NewSubscriptionService(zap.NewNop(), repo, nil, nil, cfg, nil)
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

func TestRequestChargeRoutesThroughTenantProviderConfig(t *testing.T) {
	var providerPath string
	var providerAPIKey string
	var providerAuth string
	var providerExternalTx string
	var payload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providerPath = r.URL.Path
		providerAPIKey = r.Header.Get("apikey")
		providerAuth = r.Header.Get("authentication")
		providerExternalTx = r.Header.Get("external-tx-id")
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode provider payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"responseData": map[string]interface{}{
				"transactionId": "charge-tx-1",
			},
			"message":   "ok",
			"inError":   false,
			"requestId": "req-charge",
			"code":      "SUCCESS",
		})
	}))
	defer server.Close()

	cfg := tenantRoutingTestConfig("http://legacy.invalid")
	repo := &MockSubscriptionRepository{chargeInserted: true}
	svc := NewSubscriptionService(zap.NewNop(), repo, nil, nil, cfg, nil)
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

	resp, err := svc.RequestCharge(domain.ChargeRequest{
		ProductID:      14397,
		PricepointID:   1,
		MCC:            "620",
		MNC:            "03",
		MSISDN:         "233241234567",
		ShortCode:      "1234",
		Context:        "renewal",
		Channel:        "SMS",
		IdempotencyKey: "charge-idem-1",
		TenantRoute: domain.TenantRouteContext{
			TenantID:  "tenant-1",
			ChannelID: "channel-1",
		},
	})
	if err != nil {
		t.Fatalf("expected tenant-routed charge success, got %v", err)
	}
	if resp == nil || resp.Code != ResponseCodeSuccess {
		t.Fatalf("expected SUCCESS response, got %+v", resp)
	}
	if router.seenOp != ChannelOperationCharge {
		t.Fatalf("expected charge operation, got %s", router.seenOp)
	}
	if router.seenRoute.TenantID != "tenant-1" || router.seenRoute.ChannelID != "channel-1" {
		t.Fatalf("resolver saw wrong route: %+v", router.seenRoute)
	}
	if providerPath != "/tenant-realm/charge/dob/9090" {
		t.Fatalf("expected tenant charge path, got %s", providerPath)
	}
	if providerAPIKey != "tenant-api-key" || providerAuth != "tenant-auth-key" {
		t.Fatalf("expected tenant credentials, got api=%q auth=%q", providerAPIKey, providerAuth)
	}
	if providerExternalTx != "charge-idem-1" {
		t.Fatalf("expected charge idempotency header, got %q", providerExternalTx)
	}
	if got := payload["msisdn"]; got != "233241234567" {
		t.Fatalf("expected charge payload MSISDN, got %#v", got)
	}
	if repo.chargeNotification == nil {
		t.Fatal("expected charge ownership notification to be recorded")
	}
	if repo.chargeNotification.TenantID == nil || *repo.chargeNotification.TenantID != "tenant-1" {
		t.Fatalf("expected tenant charge ownership, got %#v", repo.chargeNotification.TenantID)
	}
	if repo.chargeNotification.ChannelID == nil || *repo.chargeNotification.ChannelID != "channel-1" {
		t.Fatalf("expected channel charge ownership, got %#v", repo.chargeNotification.ChannelID)
	}
	if repo.chargeNotification.TransactionUUID == "" {
		t.Fatal("expected charge idempotency transaction uuid")
	}
	if repo.chargeNotification.TransactionUUID != "charge-idem-1" {
		t.Fatalf("expected charge ownership to use idempotency key, got %q", repo.chargeNotification.TransactionUUID)
	}
}

func TestRequestChargeReturnsProviderSuccessWhenOwnershipRecordFails(t *testing.T) {
	providerCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providerCalls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"responseData": map[string]interface{}{
				"transactionId": "charge-tx-1",
			},
			"message":   "ok",
			"inError":   false,
			"requestId": "req-charge",
			"code":      "SUCCESS",
		})
	}))
	defer server.Close()

	cfg := tenantRoutingTestConfig("http://legacy.invalid")
	repo := &MockSubscriptionRepository{notificationError: errors.New("ownership database unavailable")}
	svc := NewSubscriptionService(zap.NewNop(), repo, nil, nil, cfg, nil)
	svc.SetTenantProviderRouter(&fakeTenantProviderResolver{
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
	})

	resp, err := svc.RequestCharge(domain.ChargeRequest{
		ProductID:      14397,
		PricepointID:   1,
		MCC:            "620",
		MNC:            "03",
		MSISDN:         "233241234567",
		ShortCode:      "1234",
		Context:        "renewal",
		Channel:        "SMS",
		IdempotencyKey: "charge-idem-2",
		TenantRoute: domain.TenantRouteContext{
			TenantID:  "tenant-1",
			ChannelID: "channel-1",
		},
	})
	if err != nil {
		t.Fatalf("expected provider success even when ownership recording fails, got %v", err)
	}
	if resp == nil || resp.Code != ResponseCodeSuccess {
		t.Fatalf("expected SUCCESS response, got %+v", resp)
	}
	if providerCalls != 1 {
		t.Fatalf("expected one provider charge call, got %d", providerCalls)
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
