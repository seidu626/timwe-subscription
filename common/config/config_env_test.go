package config

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestInitConfig_BindsLegacyTIMWEEnvNames(t *testing.T) {
	cfgPath := writeMinimalConfig(t)

	t.Setenv("TIMWE_API_KEY", "legacy-api-key")
	t.Setenv("TIMWE_PSK", "1234567890123456")
	t.Setenv("TIMWE_PARTNER_SERVICE_ID", "2170")
	t.Setenv("TIMWE_AUTHENTICATION_KEY", "legacy-auth-token")

	loaded := InitConfig(zap.NewNop(), filepath.Dir(cfgPath), []string{filepath.Base(cfgPath)})

	if loaded.Application.TIMWE.APIKey != "legacy-api-key" {
		t.Fatalf("expected API key from legacy env, got %q", loaded.Application.TIMWE.APIKey)
	}
	if loaded.Application.TIMWE.Psk != "1234567890123456" {
		t.Fatalf("expected PSK from legacy env, got %q", loaded.Application.TIMWE.Psk)
	}
	if loaded.Application.TIMWE.PartnerServiceID != "2170" {
		t.Fatalf("expected partner service id from legacy env, got %q", loaded.Application.TIMWE.PartnerServiceID)
	}
	if loaded.Application.TIMWE.AuthenticationKey != "legacy-auth-token" {
		t.Fatalf("expected auth key from legacy env, got %q", loaded.Application.TIMWE.AuthenticationKey)
	}
}

func TestInitConfig_BindsAppPrefixedTIMWEEnvNames(t *testing.T) {
	cfgPath := writeMinimalConfig(t)

	t.Setenv("APP_APPLICATION_TIMWE_MA_API_KEY", "app-api-key")
	t.Setenv("APP_APPLICATION_TIMWE_MA_PSK", "123456789012345678901234")
	t.Setenv("APP_APPLICATION_TIMWE_MA_PARTNER_SERVICE_ID", "3300")
	t.Setenv("APP_APPLICATION_TIMWE_MA_AUTHENTICATION_KEY", "app-auth-token")

	loaded := InitConfig(zap.NewNop(), filepath.Dir(cfgPath), []string{filepath.Base(cfgPath)})

	if loaded.Application.TIMWE.APIKey != "app-api-key" {
		t.Fatalf("expected API key from APP env, got %q", loaded.Application.TIMWE.APIKey)
	}
	if loaded.Application.TIMWE.Psk != "123456789012345678901234" {
		t.Fatalf("expected PSK from APP env, got %q", loaded.Application.TIMWE.Psk)
	}
	if loaded.Application.TIMWE.PartnerServiceID != "3300" {
		t.Fatalf("expected partner service id from APP env, got %q", loaded.Application.TIMWE.PartnerServiceID)
	}
	if loaded.Application.TIMWE.AuthenticationKey != "app-auth-token" {
		t.Fatalf("expected auth key from APP env, got %q", loaded.Application.TIMWE.AuthenticationKey)
	}
}

func writeMinimalConfig(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte(`
APPLICATION:
  TIMWE_MA: {}
  HTTP: {}
DATABASE:
  POSTGRESQL: {}
CACHE:
  REDIS: {}
AUTH:
  JWT_TOKEN: {}
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	return path
}
