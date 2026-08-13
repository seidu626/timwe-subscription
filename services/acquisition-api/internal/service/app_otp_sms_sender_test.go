package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func newTestSMSSender(cfg *smsGatewayConfig, resolveErr error) *TenantSMSSender {
	s := &TenantSMSSender{
		client: http.DefaultClient,
		logger: zap.NewNop(),
	}
	s.resolveConfig = func(ctx context.Context, tenantKey string) (*smsGatewayConfig, error) {
		if resolveErr != nil {
			return nil, resolveErr
		}
		return cfg, nil
	}
	return s
}

func TestSendLoginOTPPostsRenderedBody(t *testing.T) {
	var gotBody map[string]interface{}
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("api-key")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("gateway received invalid JSON: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender := newTestSMSSender(&smsGatewayConfig{
		URL:          srv.URL,
		Headers:      map[string]string{"api-key": "k-123"},
		BodyTemplate: `{"sender":"{{sender}}","message":"{{text}}","recipients":["{{msisdn}}"]}`,
		SenderID:     "Dayline",
	}, nil)

	if err := sender.SendLoginOTP("233241234567", "careerify", "482913"); err != nil {
		t.Fatalf("SendLoginOTP: %v", err)
	}
	if gotAuth != "k-123" {
		t.Errorf("api-key header = %q, want k-123", gotAuth)
	}
	msg, _ := gotBody["message"].(string)
	if !strings.Contains(msg, "482913") {
		t.Errorf("message %q does not contain the otp code", msg)
	}
	if gotBody["sender"] != "Dayline" {
		t.Errorf("sender = %v, want Dayline", gotBody["sender"])
	}
	recipients, _ := gotBody["recipients"].([]interface{})
	if len(recipients) != 1 || recipients[0] != "233241234567" {
		t.Errorf("recipients = %v, want [233241234567]", gotBody["recipients"])
	}
}

func TestSendLoginOTPCustomMessageTemplate(t *testing.T) {
	var msg string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		msg = body["message"]
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender := newTestSMSSender(&smsGatewayConfig{
		URL:             srv.URL,
		BodyTemplate:    `{"message":"{{text}}"}`,
		MessageTemplate: `Code "{{code}}" expires soon`,
	}, nil)

	if err := sender.SendLoginOTP("233241234567", "careerify", "111222"); err != nil {
		t.Fatalf("SendLoginOTP: %v", err)
	}
	// The template's embedded quotes must survive JSON-escaping intact.
	if msg != `Code "111222" expires soon` {
		t.Errorf("message = %q", msg)
	}
}

func TestSendLoginOTPGatewayFailureStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	sender := newTestSMSSender(&smsGatewayConfig{
		URL:          srv.URL,
		BodyTemplate: `{"message":"{{text}}"}`,
	}, nil)

	err := sender.SendLoginOTP("233241234567", "careerify", "111222")
	if err == nil || !strings.Contains(err.Error(), "status 401") {
		t.Fatalf("want gateway status error, got %v", err)
	}
}

func TestSendLoginOTPResolveFailureIsTerminal(t *testing.T) {
	sender := newTestSMSSender(nil, fmt.Errorf("no ACTIVE sms_api credential for tenant"))
	err := sender.SendLoginOTP("233241234567", "careerify", "111222")
	if err == nil || !strings.Contains(err.Error(), "sms_api") {
		t.Fatalf("want resolve error, got %v", err)
	}
}

func TestSMSGatewayConfigValidate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     smsGatewayConfig
		wantErr string
	}{
		{"missing url", smsGatewayConfig{BodyTemplate: "{}"}, "url is required"},
		{"bad scheme", smsGatewayConfig{URL: "ftp://x", BodyTemplate: "{}"}, "http(s)"},
		{"missing body", smsGatewayConfig{URL: "https://x"}, "body_template is required"},
		{"valid", smsGatewayConfig{URL: "https://x", BodyTemplate: "{}"}, ""},
	}
	for _, tc := range cases {
		err := tc.cfg.validate()
		if tc.wantErr == "" && err != nil {
			t.Errorf("%s: unexpected error %v", tc.name, err)
		}
		if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
			t.Errorf("%s: got %v, want %q", tc.name, err, tc.wantErr)
		}
	}
}

func TestRenderJSONTemplateEscapesValues(t *testing.T) {
	out := renderJSONTemplate(`{"a":"{{text}}"}`, map[string]string{"text": `he said "hi"` + "\nline2"})
	var parsed map[string]string
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("rendered template is not valid JSON: %v (%s)", err, out)
	}
	if parsed["a"] != "he said \"hi\"\nline2" {
		t.Errorf("round-trip = %q", parsed["a"])
	}
}
