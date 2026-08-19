package dispatcher

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestSmsGatewayConfigValidate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     smsGatewayConfig
		wantErr bool
	}{
		{"missing url", smsGatewayConfig{BodyTemplate: "{}"}, true},
		{"non-http url", smsGatewayConfig{URL: "ftp://x", BodyTemplate: "{}"}, true},
		{"no body and no url placeholders", smsGatewayConfig{URL: "https://gw.example/sms"}, true},
		{"url placeholders instead of body", smsGatewayConfig{URL: "https://gw.example/sms?to={{msisdn}}"}, false},
		{"body template ok", smsGatewayConfig{URL: "https://gw.example/sms", BodyTemplate: `{"to":"{{msisdn}}"}`}, false},
		{"success field without value", smsGatewayConfig{URL: "https://gw.example/sms", BodyTemplate: "{}", SuccessField: "status"}, true},
		{"success field and value ok", smsGatewayConfig{URL: "https://gw.example/sms", BodyTemplate: "{}", SuccessField: "status", SuccessValue: "success"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestSmsGatewayConfigCheckSuccessBody(t *testing.T) {
	cases := []struct {
		name    string
		cfg     smsGatewayConfig
		body    string
		wantErr bool
	}{
		{"no markers trusts status", smsGatewayConfig{}, "anything", false},
		{"body contains marker", smsGatewayConfig{SuccessBodyContains: "OK"}, "code=OK", false},
		{"body missing marker", smsGatewayConfig{SuccessBodyContains: "OK"}, "code=FAIL", true},
		{"field match", smsGatewayConfig{SuccessField: "status", SuccessValue: "success"}, `{"status":"success"}`, false},
		{"field mismatch", smsGatewayConfig{SuccessField: "status", SuccessValue: "success"}, `{"status":"failed"}`, true},
		{"field missing", smsGatewayConfig{SuccessField: "status", SuccessValue: "success"}, `{"other":"x"}`, true},
		{"non-json body", smsGatewayConfig{SuccessField: "status", SuccessValue: "success"}, "not json", true},
		{"numeric field compares as string", smsGatewayConfig{SuccessField: "code", SuccessValue: "0"}, `{"code":0}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.checkSuccessBody([]byte(tc.body))
			if (err != nil) != tc.wantErr {
				t.Errorf("checkSuccessBody() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestRenderJSONTemplateEscapesQuotesAndNewlines(t *testing.T) {
	tmpl := `{"to":"{{msisdn}}","message":"{{text}}"}`
	values := map[string]string{
		"msisdn": "233241234567",
		"text":   "Hello \"friend\"\nsee you soon",
	}

	got := renderJSONTemplate(tmpl, values)

	want := `{"to":"233241234567","message":"Hello \"friend\"\nsee you soon"}`
	if got != want {
		t.Errorf("renderJSONTemplate() = %q, want %q", got, want)
	}
}

func TestRenderURLTemplateEncodesValues(t *testing.T) {
	tmpl := "https://gw.example/sms?to={{msisdn}}&sms={{text}}"
	values := map[string]string{"msisdn": "233241234567", "text": "hi there & bye"}

	got := renderURLTemplate(tmpl, values)

	want := "https://gw.example/sms?to=233241234567&sms=hi+there+%26+bye"
	if got != want {
		t.Errorf("renderURLTemplate() = %q, want %q", got, want)
	}
}

func TestRedactURLInErrorStripsQueryString(t *testing.T) {
	reqURL := "https://gw.example/sms?api_key=SECRET&to=233241234567"
	err := errors.New("dial failed: " + reqURL)

	got := redactURLInError(err, reqURL)

	if strings.Contains(got, "SECRET") {
		t.Errorf("redactURLInError() leaked api key: %q", got)
	}
	if !strings.Contains(got, "REDACTED") {
		t.Errorf("redactURLInError() = %q, want it to contain REDACTED", got)
	}
}

func TestTenantGatewaySenderResolveCachesFoundAndAbsentConfigs(t *testing.T) {
	calls := 0
	s := &TenantGatewaySender{logger: zap.NewNop(), cache: make(map[string]cachedGatewayConfig)}
	cfg := &smsGatewayConfig{URL: "https://gw.example/sms", BodyTemplate: "{}"}
	s.fetchConfig = func(ctx context.Context, tenantID string) (*smsGatewayConfig, error) {
		calls++
		if tenantID == "bound-tenant" {
			return cfg, nil
		}
		return nil, nil
	}

	// Bound tenant: first call fetches, second call hits cache.
	got, err := s.Resolve(context.Background(), "bound-tenant")
	if err != nil || got != cfg {
		t.Fatalf("Resolve() = %v, %v, want cfg, nil", got, err)
	}
	if _, err := s.Resolve(context.Background(), "bound-tenant"); err != nil {
		t.Fatalf("second Resolve() error = %v", err)
	}
	if calls != 1 {
		t.Errorf("fetchConfig called %d times, want 1 (second call should hit cache)", calls)
	}

	// Unbound tenant: nil result is also cached, not re-fetched.
	got, err = s.Resolve(context.Background(), "unbound-tenant")
	if err != nil || got != nil {
		t.Fatalf("Resolve() = %v, %v, want nil, nil", got, err)
	}
	if _, err := s.Resolve(context.Background(), "unbound-tenant"); err != nil {
		t.Fatalf("second Resolve() error = %v", err)
	}
	if calls != 2 {
		t.Errorf("fetchConfig called %d times, want 2 (one fetch per tenant)", calls)
	}
}

func TestTenantGatewaySenderResolveDoesNotCacheErrors(t *testing.T) {
	calls := 0
	s := &TenantGatewaySender{logger: zap.NewNop(), cache: make(map[string]cachedGatewayConfig)}
	s.fetchConfig = func(ctx context.Context, tenantID string) (*smsGatewayConfig, error) {
		calls++
		return nil, errors.New("decrypt failed")
	}

	if _, err := s.Resolve(context.Background(), "tenant-1"); err == nil {
		t.Fatal("Resolve() error = nil, want decrypt failed")
	}
	if _, err := s.Resolve(context.Background(), "tenant-1"); err == nil {
		t.Fatal("second Resolve() error = nil, want decrypt failed")
	}
	if calls != 2 {
		t.Errorf("fetchConfig called %d times, want 2 (errors must not be cached)", calls)
	}
}

func TestTenantGatewaySenderResolveExpiresCacheEntry(t *testing.T) {
	calls := 0
	s := &TenantGatewaySender{logger: zap.NewNop(), cache: make(map[string]cachedGatewayConfig)}
	s.fetchConfig = func(ctx context.Context, tenantID string) (*smsGatewayConfig, error) {
		calls++
		return nil, nil
	}

	if _, err := s.Resolve(context.Background(), "tenant-1"); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	// Force the cached entry to look expired without sleeping the TTL.
	s.mu.Lock()
	entry := s.cache["tenant-1"]
	entry.expiresAt = time.Now().Add(-time.Second)
	s.cache["tenant-1"] = entry
	s.mu.Unlock()

	if _, err := s.Resolve(context.Background(), "tenant-1"); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if calls != 2 {
		t.Errorf("fetchConfig called %d times, want 2 (expired entry must re-fetch)", calls)
	}
}

func TestTenantGatewaySenderSendSuccessAndFailure(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		gotBody = string(buf)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))
	defer srv.Close()

	s := NewTenantGatewaySender(nil, zap.NewNop())
	cfg := &smsGatewayConfig{
		URL:          srv.URL,
		BodyTemplate: `{"to":"{{msisdn}}","message":"{{text}}"}`,
		SuccessField: "status",
		SuccessValue: "success",
	}

	if _, err := s.Send(context.Background(), cfg, "233241234567", "hello"); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if want := `{"to":"233241234567","message":"hello"}`; gotBody != want {
		t.Errorf("gateway received body %q, want %q", gotBody, want)
	}

	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failSrv.Close()
	cfg.URL = failSrv.URL
	if _, err := s.Send(context.Background(), cfg, "233241234567", "hello"); err == nil {
		t.Fatal("Send() error = nil, want gateway 500 to surface an error")
	}
}

func TestExtractJSONPath(t *testing.T) {
	body := []byte(`{"status":"success","data":[{"id":"msg-123","recipient":"233241234567"}],"code":200}`)
	cases := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{"nested array element", "data.0.id", "msg-123", false},
		{"top-level string", "status", "success", false},
		{"numeric leaf", "code", "200", false},
		{"missing key", "data.0.missing", "", true},
		{"index out of range", "data.5.id", "", true},
		{"non-numeric index into array", "data.id", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractJSONPath(body, tc.path)
			if (err != nil) != tc.wantErr {
				t.Fatalf("extractJSONPath() error = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("extractJSONPath() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClassifyDeliveryStatus(t *testing.T) {
	cfg := &smsGatewayConfig{
		StatusDeliveredValues: []string{"delivered"},
		StatusFailedValues:    []string{"failed", "rejected"},
	}
	cases := []struct {
		raw  string
		want string
	}{
		{"delivered", deliveryStatusDelivered},
		{"DELIVERED", deliveryStatusDelivered},
		{"Failed", deliveryStatusFailed},
		{"rejected", deliveryStatusFailed},
		{"submitted", deliveryStatusPending},
		{"", deliveryStatusPending},
	}
	for _, tc := range cases {
		if got := classifyDeliveryStatus(cfg, tc.raw); got != tc.want {
			t.Errorf("classifyDeliveryStatus(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestTenantGatewaySenderSendExtractsMessageID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success","data":[{"id":"msg-abc-1"}]}`))
	}))
	defer srv.Close()

	s := NewTenantGatewaySender(nil, zap.NewNop())
	cfg := &smsGatewayConfig{
		URL:           srv.URL,
		BodyTemplate:  `{"to":"{{msisdn}}"}`,
		SuccessField:  "status",
		SuccessValue:  "success",
		MessageIDPath: "data.0.id",
	}

	id, err := s.Send(context.Background(), cfg, "233241234567", "hello")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if id != "msg-abc-1" {
		t.Errorf("Send() message id = %q, want %q", id, "msg-abc-1")
	}

	// A missing id must not fail the send: the SMS already went out and a
	// send error would trigger a duplicate resend.
	cfg.MessageIDPath = "data.0.missing"
	id, err = s.Send(context.Background(), cfg, "233241234567", "hello")
	if err != nil {
		t.Fatalf("Send() with missing id path error = %v", err)
	}
	if id != "" {
		t.Errorf("Send() message id = %q, want empty when path is missing", id)
	}
}

func TestTenantGatewaySenderDeliveryStatus(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("api-key")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"status":"delivered"}}`))
	}))
	defer srv.Close()

	s := NewTenantGatewaySender(nil, zap.NewNop())
	cfg := &smsGatewayConfig{
		URL:                   "https://gw.example/sms",
		BodyTemplate:          "{}",
		Headers:               map[string]string{"api-key": "k1"},
		StatusURL:             srv.URL + "/sms/{{message_id}}",
		StatusPath:            "data.status",
		StatusDeliveredValues: []string{"delivered"},
	}

	status, raw, err := s.DeliveryStatus(context.Background(), cfg, "msg-abc-1")
	if err != nil {
		t.Fatalf("DeliveryStatus() error = %v", err)
	}
	if status != deliveryStatusDelivered || raw != "delivered" {
		t.Errorf("DeliveryStatus() = %q, %q, want DELIVERED, delivered", status, raw)
	}
	if gotPath != "/sms/msg-abc-1" {
		t.Errorf("status endpoint path = %q, want /sms/msg-abc-1", gotPath)
	}
	if gotAuth != "k1" {
		t.Errorf("status request api-key header = %q, want k1", gotAuth)
	}

	// No status endpoint configured is an explicit error so the poller can
	// mark the job UNTRACKED instead of retrying forever.
	if _, _, err := s.DeliveryStatus(context.Background(), &smsGatewayConfig{}, "msg-abc-1"); err == nil {
		t.Fatal("DeliveryStatus() error = nil, want error when status endpoint unconfigured")
	}
}
