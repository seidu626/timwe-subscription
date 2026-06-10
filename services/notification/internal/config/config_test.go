package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"go.uber.org/zap"
)

func TestInitConfigReadsYamlWhenEnvFileMissing(t *testing.T) {
	cfg = Config{}
	doOnce = sync.Once{}

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`
APPLICATION:
  ENVIRONMENT: DEVELOPMENT
  PORT: 9090
  ALLOWED_ORIGINS:
    - http://localhost:4200
    - https://admin.example.com
DB:
  POSTGRESQL:
    HOST: db.local
    PORT: "5432"
    USER: sm_admin
    DB_NAME: subscription_manager
    SSL_MODE: disable
CACHE:
  REDIS:
    HOST: redis.local
    PORT: 6379
    DB: 0
`), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	loaded := InitConfig(zap.NewNop(), dir, []string{"config.yaml", ".env"})

	if loaded.Application.Port != 9090 {
		t.Fatalf("expected port from YAML, got %d", loaded.Application.Port)
	}
	if len(loaded.Application.AllowedOrigins) != 2 || loaded.Application.AllowedOrigins[0] != "http://localhost:4200" {
		t.Fatalf("expected allowed origins from YAML, got %#v", loaded.Application.AllowedOrigins)
	}
	if loaded.DB.Postgresql.DBHost != "db.local" {
		t.Fatalf("expected database host from YAML, got %q", loaded.DB.Postgresql.DBHost)
	}
}
func noopLogger() *zap.Logger { return zap.NewNop() }

// TestParseTenantContextFlag verifies fail-closed semantics for NOTIFICATION_REQUIRE_TENANT_CONTEXT.
// Only explicit falsy values ("false", "0", "no", case-insensitive) disable the check.
// Unset, empty, or any other value must resolve to true.
func TestParseTenantContextFlag(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
		desc string
	}{
		// Unset (empty string represents the env var not being set)
		{"", true, "unset → true (fail-closed)"},
		// Explicit true values
		{"true", true, "\"true\" → true"},
		{"True", true, "\"True\" → true"},
		{"TRUE", true, "\"TRUE\" → true"},
		{"1", true, "\"1\" → true"},
		// Explicit false values
		{"false", false, "\"false\" → false"},
		{"False", false, "\"False\" → false"},
		{"FALSE", false, "\"FALSE\" → false"},
		{"0", false, "\"0\" → false"},
		{"no", false, "\"no\" → false"},
		{"No", false, "\"No\" → false"},
		{"NO", false, "\"NO\" → false"},
		// Garbage / unparseable — must fail closed
		{"Flase", true, "\"Flase\" (typo) → true (fail-closed)"},
		{"maybe", true, "\"maybe\" → true (fail-closed)"},
		{"yes", true, "\"yes\" → true (fail-closed, not a known falsy)"},
		{" ", true, "whitespace-only → true (fail-closed)"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			got := parseTenantContextFlag(tc.raw)
			if got != tc.want {
				t.Errorf("parseTenantContextFlag(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

// TestRequireTenantContextViaEnv verifies that InitConfig picks up the env var
// and applies fail-closed semantics end-to-end.
func TestRequireTenantContextViaEnv(t *testing.T) {
	envCases := []struct {
		envVal string
		want   bool
		desc   string
	}{
		{"", true, "unset → true"},
		{"true", true, "\"true\" → true"},
		{"false", false, "\"false\" → false"},
		{"False", false, "\"False\" → false"},
		{"Flase", true, "garbage → true (fail-closed)"},
		{"maybe", true, "\"maybe\" → true (fail-closed)"},
	}

	for _, tc := range envCases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			// Reset singleton state for each sub-test.
			cfg = Config{}
			doOnce = sync.Once{}

			if tc.envVal == "" {
				_ = os.Unsetenv("NOTIFICATION_REQUIRE_TENANT_CONTEXT")
			} else {
				t.Setenv("NOTIFICATION_REQUIRE_TENANT_CONTEXT", tc.envVal)
			}

			loaded := InitConfig(noopLogger(), t.TempDir(), nil)
			if loaded.Notification.RequireTenantContext != tc.want {
				t.Errorf("RequireTenantContext = %v, want %v (env=%q)", loaded.Notification.RequireTenantContext, tc.want, tc.envVal)
			}
		})
	}
}
