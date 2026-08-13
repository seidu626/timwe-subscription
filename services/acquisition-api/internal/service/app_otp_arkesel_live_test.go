package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"go.uber.org/zap"
)

// TestArkeselOTPProviderAgainstLiveGateway exercises the delegated OTP path
// against the real provider, closing the gap the httptest-backed tests leave:
// that the provider accepts our generate payload and that its body codes map
// to the sentinels callers branch on. Skipped unless both env vars are set:
//
//	TEST_OTP_GATEWAY_CONFIG='{"generate_url":"https://sms.arkesel.com/api/otp/generate",...}' \
//	TEST_SMS_RECIPIENT=233XXXXXXXXX go test ./internal/service/ -run ArkeselOTPProviderAgainstLive -v
//
// Generate bills the provider's account and sends a real message. Set
// TEST_OTP_CODE to the delivered code to also exercise the success leg of
// verify, which otherwise cannot be automated: only the handset has the code.
func TestArkeselOTPProviderAgainstLiveGateway(t *testing.T) {
	blob := os.Getenv("TEST_OTP_GATEWAY_CONFIG")
	recipient := os.Getenv("TEST_SMS_RECIPIENT")
	if blob == "" || recipient == "" {
		t.Skip("TEST_OTP_GATEWAY_CONFIG or TEST_SMS_RECIPIENT not set")
	}

	var cfg arkeselOTPConfig
	if err := json.Unmarshal([]byte(blob), &cfg); err != nil {
		t.Fatalf("parse TEST_OTP_GATEWAY_CONFIG: %v", err)
	}
	cfg.withDefaults()
	if err := cfg.validate(); err != nil {
		t.Fatalf("invalid otp gateway config: %v", err)
	}

	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	provider := NewArkeselOTPProvider(nil, logger)
	provider.resolveConfig = func(ctx context.Context, tenantKey string) (*arkeselOTPConfig, error) {
		return &cfg, nil
	}
	ctx := context.Background()

	if err := provider.Generate(ctx, recipient, "careerify"); err != nil {
		t.Fatalf("live generate failed: %v", err)
	}

	// A wrong code must come back as the invalid-code sentinel rather than an
	// opaque error, because the caller burns an attempt only on that verdict.
	if err := provider.Verify(ctx, recipient, "careerify", "000000"); !errors.Is(err, ErrDelegatedOTPCodeInvalid) {
		t.Fatalf("live verify of a wrong code: got %v, want ErrDelegatedOTPCodeInvalid", err)
	}

	if code := os.Getenv("TEST_OTP_CODE"); code != "" {
		if err := provider.Verify(ctx, recipient, "careerify", code); err != nil {
			t.Fatalf("live verify of the delivered code failed: %v", err)
		}
	}
}
