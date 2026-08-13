package service

import (
	"strings"
	"testing"
)

func TestValidateCredentialSecretValue(t *testing.T) {
	const validSMS = `{"url":"https://sms.arkesel.com/api/v2/sms/send","headers":{"api-key":"k"},` +
		`"body_template":"{\"sender\":\"{{sender}}\",\"message\":\"{{text}}\",\"recipients\":[\"{{msisdn}}\"]}",` +
		`"sender_id":"Dayline","success_field":"status","success_value":"success"}`
	const validOTP = `{"generate_url":"https://sms.arkesel.com/api/otp/generate",` +
		`"verify_url":"https://sms.arkesel.com/api/otp/verify","headers":{"api-key":"k"},"sender_id":"Dayline"}`

	cases := []struct {
		name    string
		purpose string
		value   string
		wantErr string
	}{
		{"sms valid", "sms_api", validSMS, ""},
		{"sms missing url", "sms_api", `{"body_template":"x"}`, "url is required"},
		{"sms non http url", "sms_api", `{"url":"ftp://x","body_template":"x"}`, "must be http(s)"},
		{"sms no body template and no url placeholders", "sms_api",
			`{"url":"https://sms.example/send"}`, "body_template or url placeholders"},
		{"sms half a success marker", "sms_api",
			`{"url":"https://sms.example/send","body_template":"x","success_field":"status"}`,
			"must be set together"},
		{"sms unknown field", "sms_api", `{"url":"https://x/y","body_template":"z","typo":1}`, "unknown field"},
		{"sms malformed json", "sms_api", `not json`, "sms gateway config"},

		// Only the URLs, key and sender id are required; the resolver's
		// defaults fill length, expiry, medium, type and the message.
		{"otp valid minimal", "otp_api", validOTP, ""},
		{"otp missing verify url", "otp_api",
			`{"generate_url":"https://x/gen","headers":{"api-key":"k"}}`, "verify_url is required"},
		{"otp message without code slot", "otp_api",
			`{"generate_url":"https://x/gen","verify_url":"https://x/ver","message_template":"no slot"}`,
			"message_template must contain"},
		{"otp length out of range", "otp_api",
			`{"generate_url":"https://x/gen","verify_url":"https://x/ver","length":20}`, "length must be 6..15"},
		{"otp expiry out of range", "otp_api",
			`{"generate_url":"https://x/gen","verify_url":"https://x/ver","expiry_minutes":30}`,
			"expiry_minutes must be 1..10"},
		{"otp sender id too long", "otp_api",
			`{"generate_url":"https://x/gen","verify_url":"https://x/ver","sender_id":"TwelveCharsX"}`,
			"at most 11 characters"},
		{"otp bad medium", "otp_api",
			`{"generate_url":"https://x/gen","verify_url":"https://x/ver","medium":"pigeon"}`,
			"medium must be sms or voice"},
		{"otp unknown field", "otp_api",
			`{"generate_url":"https://x/gen","verify_url":"https://x/ver","expiry":5}`, "unknown field"},

		// The credential table is open to purposes with no runtime schema.
		{"unknown purpose passes through", "provider_api", `{"api_key":"k"}`, ""},
		{"unknown purpose ignores malformed json", "future_thing", `not json`, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCredentialSecretValue(tc.purpose, tc.value)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected valid, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

// A blob bound through the console must be exactly what the OTP resolver would
// accept, so the console and the runtime cannot disagree about validity.
func TestValidateCredentialSecretValueMatchesOTPResolver(t *testing.T) {
	const blob = `{"generate_url":"https://sms.arkesel.com/api/otp/generate",` +
		`"verify_url":"https://sms.arkesel.com/api/otp/verify","headers":{"api-key":"k"},"sender_id":"Dayline"}`
	if err := validateCredentialSecretValue(tenantOTPCredentialPurpose, blob); err != nil {
		t.Fatalf("console rejected a blob: %v", err)
	}
	cfg, err := decodeCredentialBlob[arkeselOTPConfig](blob, "otp gateway config")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	cfg.withDefaults()
	if err := cfg.validate(); err != nil {
		t.Fatalf("resolver rejected a blob the console accepted: %v", err)
	}
}
