package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
)

// newTestOTPProvider returns a provider whose credential resolution is stubbed
// to cfg (nil meaning the tenant is not in delegated mode).
func newTestOTPProvider(cfg *arkeselOTPConfig, resolveErr error) *ArkeselOTPProvider {
	p := NewArkeselOTPProvider(nil, zap.NewNop())
	p.resolveConfig = func(ctx context.Context, tenantKey string) (*arkeselOTPConfig, error) {
		return cfg, resolveErr
	}
	return p
}

// otpTestServer replies with body for every request and records the last
// decoded request payload.
func otpTestServer(t *testing.T, body string) (*httptest.Server, *map[string]any) {
	t.Helper()
	captured := map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &captured)
		w.Header().Set("X-Api-Key-Seen", r.Header.Get("api-key"))
		if _, err := io.WriteString(w, body); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &captured
}

func testOTPConfig(url string) *arkeselOTPConfig {
	cfg := &arkeselOTPConfig{
		GenerateURL: url,
		VerifyURL:   url,
		Headers:     map[string]string{"api-key": "test-key"},
		SenderID:    "Dayline",
	}
	cfg.withDefaults()
	return cfg
}

func TestArkeselOTPGenerateSuccessSendsDocumentedPayload(t *testing.T) {
	srv, captured := otpTestServer(t, `{"code":"1000","ussd_code":"*928*01#","message":"Successful"}`)
	p := newTestOTPProvider(testOTPConfig(srv.URL), nil)

	if err := p.Generate(context.Background(), "233241234567", "careerify"); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	got := *captured
	if got["number"] != "233241234567" {
		t.Errorf("number = %v", got["number"])
	}
	// The provider substitutes the code into this slot, so losing it silently
	// downgrades every message to one with no code in it.
	if msg, _ := got["message"].(string); !strings.Contains(msg, arkeselOTPCodeSlot) {
		t.Errorf("message %q lost the code slot", msg)
	}
	if got["sender_id"] != "Dayline" {
		t.Errorf("sender_id = %v", got["sender_id"])
	}
	if got["medium"] != "sms" || got["type"] != "numeric" {
		t.Errorf("medium/type = %v/%v", got["medium"], got["type"])
	}
	if got["length"] != float64(6) || got["expiry"] != float64(5) {
		t.Errorf("length/expiry = %v/%v", got["length"], got["expiry"])
	}
}

func TestArkeselOTPGenerateFailureCodeIsAnError(t *testing.T) {
	srv, _ := otpTestServer(t, `{"code":"1007","message":"Insufficient balance"}`)
	p := newTestOTPProvider(testOTPConfig(srv.URL), nil)

	err := p.Generate(context.Background(), "233241234567", "careerify")
	if err == nil || !strings.Contains(err.Error(), "1007") {
		t.Fatalf("want generate failure carrying the provider code, got %v", err)
	}
}

func TestArkeselOTPVerifyOutcomes(t *testing.T) {
	cases := []struct {
		name string
		body string
		want error
	}{
		{"success", `{"code":"1100","message":"Successful"}`, nil},
		{"invalid code", `{"code":"1104","message":"Invalid code"}`, ErrDelegatedOTPCodeInvalid},
		{"expired", `{"code":"1105","message":"Code has expired"}`, ErrDelegatedOTPExpired},
		// The spec documents string codes but the 500 example emits a bare
		// number, so unquoted codes must map the same way.
		{"unquoted code", `{"code":1104,"message":"Invalid code"}`, ErrDelegatedOTPCodeInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, _ := otpTestServer(t, tc.body)
			p := newTestOTPProvider(testOTPConfig(srv.URL), nil)

			err := p.Verify(context.Background(), "233241234567", "careerify", "482913")
			if tc.want == nil && err != nil {
				t.Fatalf("want success, got %v", err)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
		})
	}
}

func TestArkeselOTPVerifyUnknownCodeIsOpaqueError(t *testing.T) {
	srv, _ := otpTestServer(t, `{"code":"1106","message":"Internal error"}`)
	p := newTestOTPProvider(testOTPConfig(srv.URL), nil)

	err := p.Verify(context.Background(), "233241234567", "careerify", "482913")
	if err == nil || errors.Is(err, ErrDelegatedOTPCodeInvalid) || errors.Is(err, ErrDelegatedOTPExpired) {
		t.Fatalf("provider error must not be mistaken for a verdict on the code, got %v", err)
	}
}

func TestArkeselOTPConfiguredReflectsCredentialPresence(t *testing.T) {
	on, err := newTestOTPProvider(testOTPConfig("https://example.test"), nil).Configured(context.Background(), "careerify")
	if err != nil || !on {
		t.Fatalf("configured tenant: got %v, %v", on, err)
	}

	off, err := newTestOTPProvider(nil, nil).Configured(context.Background(), "nrg")
	if err != nil || off {
		t.Fatalf("unconfigured tenant: got %v, %v", off, err)
	}

	// A broken credential must surface, never read as "not delegated": that
	// would silently move the tenant to a different authentication path.
	broken, err := newTestOTPProvider(nil, errors.New("decrypt failed")).Configured(context.Background(), "careerify")
	if err == nil || broken {
		t.Fatalf("broken credential: got %v, %v", broken, err)
	}
}

func TestArkeselOTPConfigValidation(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*arkeselOTPConfig)
		wantErr string
	}{
		{"defaults are valid", func(*arkeselOTPConfig) {}, ""},
		{"generate_url required", func(c *arkeselOTPConfig) { c.GenerateURL = "" }, "generate_url is required"},
		{"verify_url required", func(c *arkeselOTPConfig) { c.VerifyURL = "" }, "verify_url is required"},
		{"url scheme", func(c *arkeselOTPConfig) { c.GenerateURL = "ftp://x" }, "must be http(s)"},
		{"message needs the code slot",
			func(c *arkeselOTPConfig) { c.MessageTemplate = "no slot here" }, "must contain %otp_code%"},
		{"length lower bound", func(c *arkeselOTPConfig) { c.Length = 5 }, "length must be 6..15"},
		{"length upper bound", func(c *arkeselOTPConfig) { c.Length = 16 }, "length must be 6..15"},
		{"expiry upper bound", func(c *arkeselOTPConfig) { c.ExpiryMinutes = 11 }, "expiry_minutes must be 1..10"},
		{"sender id cap", func(c *arkeselOTPConfig) { c.SenderID = "TwelveCharsX" }, "at most 11 characters"},
		{"medium enum", func(c *arkeselOTPConfig) { c.Medium = "carrier-pigeon" }, "medium must be"},
		{"type enum", func(c *arkeselOTPConfig) { c.Type = "roman" }, "type must be"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testOTPConfig("https://sms.example.test/otp")
			tc.mutate(cfg)
			err := cfg.validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("got %v, want %q", err, tc.wantErr)
			}
		})
	}
}
