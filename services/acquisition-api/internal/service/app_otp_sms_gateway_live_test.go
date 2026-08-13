package service

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"go.uber.org/zap"
)

// TestTenantSMSSenderAgainstLiveGateway sends through a REAL aggregator using
// the same blob shape the sms_api credential stores, closing the gap the
// httptest-backed unit tests leave: that the aggregator accepts our rendered
// body, auth headers and success markers. Skipped unless both env vars are
// set:
//
//	TEST_SMS_GATEWAY_CONFIG='{"url":"https://sms.arkesel.com/api/v2/sms/send",...}' \
//	TEST_SMS_RECIPIENT=233XXXXXXXXX go test ./internal/service/ -run LiveGateway -v
//
// Add "sandbox":true to the blob's body_template to exercise the full path
// without billing or delivering the message (Arkesel v2).
func TestTenantSMSSenderAgainstLiveGateway(t *testing.T) {
	blob := os.Getenv("TEST_SMS_GATEWAY_CONFIG")
	recipient := os.Getenv("TEST_SMS_RECIPIENT")
	if blob == "" || recipient == "" {
		t.Skip("TEST_SMS_GATEWAY_CONFIG or TEST_SMS_RECIPIENT not set")
	}

	var cfg smsGatewayConfig
	if err := json.Unmarshal([]byte(blob), &cfg); err != nil {
		t.Fatalf("parse TEST_SMS_GATEWAY_CONFIG: %v", err)
	}
	if err := cfg.validate(); err != nil {
		t.Fatalf("invalid gateway config: %v", err)
	}

	// A development logger surfaces the gateway's own error message, which is
	// the only useful diagnostic when an aggregator rejects a live send.
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	sender := NewTenantSMSSender(nil, logger)
	sender.resolveConfig = func(ctx context.Context, tenantKey string) (*smsGatewayConfig, error) {
		return &cfg, nil
	}

	if err := sender.SendLoginOTP(recipient, "careerify", "424242"); err != nil {
		t.Fatalf("live gateway send failed: %v", err)
	}
}
