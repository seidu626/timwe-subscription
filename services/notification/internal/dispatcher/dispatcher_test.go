package dispatcher

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/seidu626/subscription-manager/notification/internal/domain"
	"go.uber.org/zap"
)

type fakePushRepo struct {
	token        string
	hasDevice    bool
	deviceErr    error
	preferred    string
	prefErr      error
	deviceCalls  int
	prefCalls    int
	lastTenantID string
}

func (f *fakePushRepo) DeviceToken(ctx context.Context, msisdn, tenantID string) (string, bool, error) {
	f.deviceCalls++
	f.lastTenantID = tenantID
	if f.deviceErr != nil {
		return "", false, f.deviceErr
	}
	return f.token, f.hasDevice, nil
}

func (f *fakePushRepo) PreferredChannel(ctx context.Context, msisdn string, partnerRoleID, productID int) (string, error) {
	f.prefCalls++
	if f.prefErr != nil {
		return "", f.prefErr
	}
	return f.preferred, nil
}

type fakePushSender struct {
	sent bool
	err  error
}

func (f *fakePushSender) Send(ctx context.Context, deviceToken, body string) error {
	f.sent = true
	return f.err
}

func testJob() domain.OutboxJob {
	return domain.OutboxJob{JobID: "job-1", MSISDN: "233241234567", PartnerRoleID: 7, ProductID: 42}
}

func TestResolveChannel_PushWhenPreferredDeviceAndSenderConfigured(t *testing.T) {
	d := NewDispatcher(nil, zap.NewNop(), Config{}).WithPush(
		&fakePushRepo{token: "device-token", hasDevice: true, preferred: "PUSH"},
		&fakePushSender{},
	)

	channel, token := d.resolveChannel(context.Background(), testJob())
	if channel != channelPush || token != "device-token" {
		t.Errorf("got channel=%q token=%q, want PUSH/device-token", channel, token)
	}
}

func TestResolveChannel_FallsBackToSMSWhenSenderNotConfigured(t *testing.T) {
	d := NewDispatcher(nil, zap.NewNop(), Config{}).WithPush(
		&fakePushRepo{token: "device-token", hasDevice: true, preferred: "PUSH"},
		nil,
	)

	channel, token := d.resolveChannel(context.Background(), testJob())
	if channel != channelSMS || token != "" {
		t.Errorf("got channel=%q token=%q, want SMS/empty", channel, token)
	}

	// Calling again must stay deterministic even though the once-guarded
	// warning has already fired.
	channel, token = d.resolveChannel(context.Background(), testJob())
	if channel != channelSMS || token != "" {
		t.Errorf("second call: got channel=%q token=%q, want SMS/empty", channel, token)
	}
}

func TestResolveChannel_FallsBackToSMSWhenNoDevice(t *testing.T) {
	d := NewDispatcher(nil, zap.NewNop(), Config{}).WithPush(
		&fakePushRepo{hasDevice: false, preferred: "PUSH"},
		&fakePushSender{},
	)

	channel, _ := d.resolveChannel(context.Background(), testJob())
	if channel != channelSMS {
		t.Errorf("channel = %q, want SMS", channel)
	}
}

func TestResolveChannel_FallsBackToSMSOnDeviceLookupError(t *testing.T) {
	d := NewDispatcher(nil, zap.NewNop(), Config{}).WithPush(
		&fakePushRepo{deviceErr: errors.New("db down")},
		&fakePushSender{},
	)

	channel, token := d.resolveChannel(context.Background(), testJob())
	if channel != channelSMS || token != "" {
		t.Errorf("got channel=%q token=%q, want SMS/empty", channel, token)
	}
}

func TestResolveChannel_FallsBackToSMSOnPreferenceLookupError(t *testing.T) {
	d := NewDispatcher(nil, zap.NewNop(), Config{}).WithPush(
		&fakePushRepo{hasDevice: true, token: "t", prefErr: errors.New("db down")},
		&fakePushSender{},
	)

	channel, token := d.resolveChannel(context.Background(), testJob())
	if channel != channelSMS || token != "" {
		t.Errorf("got channel=%q token=%q, want SMS/empty", channel, token)
	}
}

func TestResolveChannel_SMSWhenPushNotWired(t *testing.T) {
	d := NewDispatcher(nil, zap.NewNop(), Config{})

	channel, token := d.resolveChannel(context.Background(), testJob())
	if channel != channelSMS || token != "" {
		t.Errorf("got channel=%q token=%q, want SMS/empty", channel, token)
	}
}

func TestResolveChannel_PassesJobTenantIDToDeviceLookup(t *testing.T) {
	repo := &fakePushRepo{token: "device-token", hasDevice: true, preferred: "PUSH"}
	d := NewDispatcher(nil, zap.NewNop(), Config{}).WithPush(repo, &fakePushSender{})

	tenantID := "11111111-1111-1111-1111-111111111111"
	job := testJob()
	job.TenantID = &tenantID

	d.resolveChannel(context.Background(), job)
	if repo.lastTenantID != tenantID {
		t.Errorf("lastTenantID = %q, want %q", repo.lastTenantID, tenantID)
	}
}

func TestResolveChannel_EmptyTenantIDWhenJobHasNone(t *testing.T) {
	repo := &fakePushRepo{token: "device-token", hasDevice: true, preferred: "PUSH"}
	d := NewDispatcher(nil, zap.NewNop(), Config{}).WithPush(repo, &fakePushSender{})

	d.resolveChannel(context.Background(), testJob())
	if repo.lastTenantID != "" {
		t.Errorf("lastTenantID = %q, want empty (no tenant on job)", repo.lastTenantID)
	}
}

func TestResolveChannel_SeriesSMSOverrideSkipsPushLookups(t *testing.T) {
	repo := &fakePushRepo{token: "device-token", hasDevice: true, preferred: "PUSH"}
	d := NewDispatcher(nil, zap.NewNop(), Config{}).WithPush(repo, &fakePushSender{})
	job := testJob()
	job.DeliveryChannel = channelSMS

	channel, token := d.resolveChannel(context.Background(), job)
	if channel != channelSMS || token != "" {
		t.Errorf("got channel=%q token=%q, want SMS/empty", channel, token)
	}
	if repo.deviceCalls != 0 || repo.prefCalls != 0 {
		t.Errorf("expected no push lookups, got device=%d pref=%d", repo.deviceCalls, repo.prefCalls)
	}
}

func TestResolveChannel_SeriesPUSHOverrideIgnoresSubscriberPreference(t *testing.T) {
	repo := &fakePushRepo{token: "device-token", hasDevice: true, preferred: "SMS", prefErr: errors.New("pref unavailable")}
	d := NewDispatcher(nil, zap.NewNop(), Config{}).WithPush(repo, &fakePushSender{})
	job := testJob()
	job.DeliveryChannel = channelPush

	channel, token := d.resolveChannel(context.Background(), job)
	if channel != channelPush || token != "device-token" {
		t.Errorf("got channel=%q token=%q, want PUSH/device-token", channel, token)
	}
	if repo.prefCalls != 0 {
		t.Errorf("expected preference lookup skipped, got %d calls", repo.prefCalls)
	}
}

type fakeTenantGateway struct {
	cfg          *smsGatewayConfig
	resolveErr   error
	sendErr      error
	resolveCalls int
	sendCalls    int
	lastTenantID string
}

func (f *fakeTenantGateway) Resolve(ctx context.Context, tenantID string) (*smsGatewayConfig, error) {
	f.resolveCalls++
	f.lastTenantID = tenantID
	if f.resolveErr != nil {
		return nil, f.resolveErr
	}
	return f.cfg, nil
}

func (f *fakeTenantGateway) Send(ctx context.Context, cfg *smsGatewayConfig, msisdn, text string) error {
	f.sendCalls++
	return f.sendErr
}

// mtTestServer returns an httptest server that answers the TIMWE MT contract
// sendMT expects, plus a *bool set true when it receives a request.
func mtTestServer(t *testing.T, inError bool) (*httptest.Server, *bool) {
	t.Helper()
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"inError": inError, "code": "500", "message": "boom"})
	}))
	t.Cleanup(srv.Close)
	return srv, &called
}

func TestSendSMS_RoutesToTenantGatewayWhenBound(t *testing.T) {
	mt, mtCalled := mtTestServer(t, false)
	gateway := &fakeTenantGateway{cfg: &smsGatewayConfig{URL: "https://gw.example/sms", BodyTemplate: "{}"}}
	d := NewDispatcher(nil, zap.NewNop(), Config{MTBaseURL: mt.URL}).WithTenantGateway(gateway)

	tenantID := "11111111-1111-1111-1111-111111111111"
	job := testJob()
	job.TenantID = &tenantID
	job.MessageText = "hello"

	if err := d.sendSMS(context.Background(), job); err != nil {
		t.Fatalf("sendSMS() error = %v", err)
	}
	if gateway.sendCalls != 1 {
		t.Errorf("gateway.Send called %d times, want 1", gateway.sendCalls)
	}
	if gateway.lastTenantID != tenantID {
		t.Errorf("gateway.Resolve tenantID = %q, want %q", gateway.lastTenantID, tenantID)
	}
	if *mtCalled {
		t.Error("MT endpoint was called; bound tenant should not fall back to MT")
	}
}

func TestSendSMS_FallsBackToMTWhenNoGatewayBinding(t *testing.T) {
	mt, mtCalled := mtTestServer(t, false)
	gateway := &fakeTenantGateway{cfg: nil} // no ACTIVE sms_api credential
	d := NewDispatcher(nil, zap.NewNop(), Config{MTBaseURL: mt.URL}).WithTenantGateway(gateway)

	tenantID := "22222222-2222-2222-2222-222222222222"
	job := testJob()
	job.TenantID = &tenantID

	if err := d.sendSMS(context.Background(), job); err != nil {
		t.Fatalf("sendSMS() error = %v", err)
	}
	if gateway.sendCalls != 0 {
		t.Errorf("gateway.Send called %d times, want 0 (no binding)", gateway.sendCalls)
	}
	if !*mtCalled {
		t.Error("MT endpoint was not called; unbound tenant must fall back to MT")
	}
}

func TestSendSMS_FallsBackToMTWhenJobHasNoTenantID(t *testing.T) {
	mt, mtCalled := mtTestServer(t, false)
	gateway := &fakeTenantGateway{cfg: &smsGatewayConfig{URL: "https://gw.example/sms", BodyTemplate: "{}"}}
	d := NewDispatcher(nil, zap.NewNop(), Config{MTBaseURL: mt.URL}).WithTenantGateway(gateway)

	job := testJob() // TenantID left nil

	if err := d.sendSMS(context.Background(), job); err != nil {
		t.Fatalf("sendSMS() error = %v", err)
	}
	if gateway.resolveCalls != 0 {
		t.Errorf("gateway.Resolve called %d times, want 0 (job has no tenant)", gateway.resolveCalls)
	}
	if !*mtCalled {
		t.Error("MT endpoint was not called for a job with no TenantID")
	}
}

func TestSendSMS_GatewayResolveErrorSurfacesWithoutMTFallback(t *testing.T) {
	mt, mtCalled := mtTestServer(t, false)
	gateway := &fakeTenantGateway{resolveErr: errors.New("decrypt failed")}
	d := NewDispatcher(nil, zap.NewNop(), Config{MTBaseURL: mt.URL}).WithTenantGateway(gateway)

	tenantID := "33333333-3333-3333-3333-333333333333"
	job := testJob()
	job.TenantID = &tenantID

	err := d.sendSMS(context.Background(), job)
	if err == nil {
		t.Fatal("sendSMS() error = nil, want resolve error to surface")
	}
	if *mtCalled {
		t.Error("MT endpoint was called; a resolve error must not silently fall back to MT")
	}
}

func TestSendSMS_GatewaySendErrorSurfacesWithoutMTFallback(t *testing.T) {
	mt, mtCalled := mtTestServer(t, false)
	gateway := &fakeTenantGateway{
		cfg:     &smsGatewayConfig{URL: "https://gw.example/sms", BodyTemplate: "{}"},
		sendErr: errors.New("sms gateway status 500"),
	}
	d := NewDispatcher(nil, zap.NewNop(), Config{MTBaseURL: mt.URL}).WithTenantGateway(gateway)

	tenantID := "44444444-4444-4444-4444-444444444444"
	job := testJob()
	job.TenantID = &tenantID

	err := d.sendSMS(context.Background(), job)
	if err == nil {
		t.Fatal("sendSMS() error = nil, want gateway send error to surface for the retry path")
	}
	if *mtCalled {
		t.Error("MT endpoint was called; a gateway send failure must not silently fall back to MT")
	}
}

func TestSendSMS_UnwiredGatewayKeepsMTPath(t *testing.T) {
	mt, mtCalled := mtTestServer(t, false)
	d := NewDispatcher(nil, zap.NewNop(), Config{MTBaseURL: mt.URL})

	tenantID := "55555555-5555-5555-5555-555555555555"
	job := testJob()
	job.TenantID = &tenantID

	if err := d.sendSMS(context.Background(), job); err != nil {
		t.Fatalf("sendSMS() error = %v", err)
	}
	if !*mtCalled {
		t.Error("MT endpoint was not called; a dispatcher without WithTenantGateway must keep the MT path")
	}
}
