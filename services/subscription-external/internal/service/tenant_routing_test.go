// slice-harness: allow-new-canonical-path: TMP-007 tests the tenant routing module.
package service

import (
	"context"
	"errors"
	"os"
	"testing"
)

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

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
