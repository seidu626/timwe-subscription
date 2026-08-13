package dispatcher

import (
	"context"
	"errors"
	"testing"

	"github.com/seidu626/subscription-manager/notification/internal/domain"
	"go.uber.org/zap"
)

type fakePushRepo struct {
	token       string
	hasDevice   bool
	deviceErr   error
	preferred   string
	prefErr     error
	deviceCalls int
	prefCalls   int
}

func (f *fakePushRepo) DeviceToken(ctx context.Context, msisdn string) (string, bool, error) {
	f.deviceCalls++
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
