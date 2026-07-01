package main

import (
	"testing"

	"github.com/seidu626/subscription-manager/common/config"
)

func TestBuildTIMWEConfigLoadsTrustedServiceSecretFromJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "jwt-shared-secret")

	cfg := buildTIMWEConfig(&config.Config{})

	if cfg.TrustedServiceSecret != "jwt-shared-secret" {
		t.Fatalf("expected JWT_SECRET to configure trusted service signing secret, got %q", cfg.TrustedServiceSecret)
	}
	if cfg.ServiceID != "acquisition-api" {
		t.Fatalf("expected default service ID, got %q", cfg.ServiceID)
	}
}

func TestBuildTIMWEConfigPrefersExplicitTrustedServiceSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "jwt-shared-secret")
	t.Setenv("TRUSTED_SERVICE_TENANT_SECRET", "explicit-shared-secret")
	t.Setenv("TRUSTED_SERVICE_ID", "custom-acquisition")

	cfg := buildTIMWEConfig(&config.Config{})

	if cfg.TrustedServiceSecret != "explicit-shared-secret" {
		t.Fatalf("expected explicit trusted service secret to win, got %q", cfg.TrustedServiceSecret)
	}
	if cfg.ServiceID != "custom-acquisition" {
		t.Fatalf("expected configured service ID, got %q", cfg.ServiceID)
	}
}
