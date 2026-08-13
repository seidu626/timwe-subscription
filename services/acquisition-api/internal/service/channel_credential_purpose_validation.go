package service

import (
	"encoding/json"
	"fmt"
	"strings"
)

// validateCredentialSecretValue rejects a credential blob that the runtime
// consumer of its purpose would reject later. Binding happens in the admin
// console, where an operator can act on "sender_id must be at most 11
// characters"; without this check the same blob is accepted silently and
// surfaces days later as an opaque login failure for every user of the tenant.
//
// It runs the consumer's own validator rather than a copy of its rules, so the
// console and the runtime can never disagree about what a valid config is.
// Purposes with no runtime schema pass through: the credential table is
// deliberately open to new purposes, and an unknown one is not an error.
//
// Only direct secret values can be checked. A secret_ref points at material
// this process may not hold (env:// is read by acquisition-api at OTP time,
// vault:// by nothing here), so those binds stay unvalidated by construction.
func validateCredentialSecretValue(purpose, secretValue string) error {
	switch purpose {
	case tenantSMSCredentialPurpose:
		cfg, err := decodeCredentialBlob[smsGatewayConfig](secretValue, "sms gateway config")
		if err != nil {
			return err
		}
		return cfg.validate()
	case tenantOTPCredentialPurpose:
		cfg, err := decodeCredentialBlob[arkeselOTPConfig](secretValue, "otp gateway config")
		if err != nil {
			return err
		}
		// Defaults are applied by the resolver too, so an operator only has to
		// supply the URLs, key and sender id here.
		cfg.withDefaults()
		return cfg.validate()
	default:
		return nil
	}
}

func decodeCredentialBlob[T any](secretValue, label string) (*T, error) {
	var cfg T
	decoder := json.NewDecoder(strings.NewReader(secretValue))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	return &cfg, nil
}
