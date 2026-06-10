package tenantctx

import (
	"net/http"
	"testing"
)

// Note: httpHeaderGetter is declared in resolver_test.go (same package test build).

func TestVerifyGatewayTrust_Valid(t *testing.T) {
	secret := "test-gateway-secret"
	token := GatewayTrustToken(secret)
	h := http.Header{}
	h.Set(HeaderGatewayTrust, token)
	if err := VerifyGatewayTrust(httpHeaderGetter{h: h}, GatewayTrustOptions{Secret: secret}); err != nil {
		t.Fatalf("expected valid token to be accepted: %v", err)
	}
}

func TestVerifyGatewayTrust_MissingHeader(t *testing.T) {
	err := VerifyGatewayTrust(httpHeaderGetter{h: http.Header{}}, GatewayTrustOptions{Secret: "secret"})
	if err != ErrMissingGatewayTrustHeader {
		t.Fatalf("expected ErrMissingGatewayTrustHeader, got %v", err)
	}
}

func TestVerifyGatewayTrust_WrongSecret(t *testing.T) {
	token := GatewayTrustToken("correct-secret")
	h := http.Header{}
	h.Set(HeaderGatewayTrust, token)
	err := VerifyGatewayTrust(httpHeaderGetter{h: h}, GatewayTrustOptions{Secret: "wrong-secret"})
	if err != ErrInvalidGatewayTrustHeader {
		t.Fatalf("expected ErrInvalidGatewayTrustHeader for wrong secret, got %v", err)
	}
}

func TestVerifyGatewayTrust_TamperedValue(t *testing.T) {
	h := http.Header{}
	h.Set(HeaderGatewayTrust, "000000000000000000000000000000000000000000000000000000000000dead")
	err := VerifyGatewayTrust(httpHeaderGetter{h: h}, GatewayTrustOptions{Secret: "secret"})
	if err != ErrInvalidGatewayTrustHeader {
		t.Fatalf("expected ErrInvalidGatewayTrustHeader for tampered value, got %v", err)
	}
}

func TestVerifyGatewayTrust_EmptySecret(t *testing.T) {
	h := http.Header{}
	h.Set(HeaderGatewayTrust, "somevalue")
	err := VerifyGatewayTrust(httpHeaderGetter{h: h}, GatewayTrustOptions{Secret: ""})
	if err == nil {
		t.Fatal("expected error for empty secret")
	}
}

func TestVerifyGatewayTrust_NilHeaders(t *testing.T) {
	err := VerifyGatewayTrust(nil, GatewayTrustOptions{Secret: "secret"})
	if err != ErrMissingGatewayTrustHeader {
		t.Fatalf("expected ErrMissingGatewayTrustHeader for nil headers, got %v", err)
	}
}

func TestVerifyGatewayTrust_MalformedHex(t *testing.T) {
	h := http.Header{}
	h.Set(HeaderGatewayTrust, "not-valid-hex!!!")
	err := VerifyGatewayTrust(httpHeaderGetter{h: h}, GatewayTrustOptions{Secret: "secret"})
	if err != ErrInvalidGatewayTrustHeader {
		t.Fatalf("expected ErrInvalidGatewayTrustHeader for malformed hex, got %v", err)
	}
}

func TestGatewayTrustToken_Deterministic(t *testing.T) {
	a := GatewayTrustToken("secret")
	b := GatewayTrustToken("secret")
	if a != b {
		t.Fatal("GatewayTrustToken is not deterministic")
	}
}

func TestGatewayTrustToken_DifferentSecrets(t *testing.T) {
	a := GatewayTrustToken("secret-a")
	b := GatewayTrustToken("secret-b")
	if a == b {
		t.Fatal("different secrets must produce different tokens")
	}
}
