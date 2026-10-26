package config

import (
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
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

func TestDatabasePoolLimits(t *testing.T) {
	cases := []struct {
		desc               string
		env                map[string]string
		wantOpen, wantIdle int
		wantLifetime       time.Duration
	}{
		{"defaults", nil, 4, 4, 5 * time.Minute},
		{"compose PGMAX vars", map[string]string{"PGMAX_OPEN_CONNS": "6", "PGMAX_IDLE_CONNS": "2"}, 6, 2, 5 * time.Minute},
		{"APP vars", map[string]string{"APP_DATABASE_POSTGRESQL_MAX_OPEN_CONNS": "3", "APP_DATABASE_POSTGRESQL_MAX_IDLE_CONNS": "1", "APP_DATABASE_POSTGRESQL_CONN_MAX_LIFETIME": "10m"}, 3, 1, 10 * time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			cfg = Config{}
			doOnce = sync.Once{}
			for _, key := range []string{"PGMAX_OPEN_CONNS", "PGMAX_IDLE_CONNS", "APP_DATABASE_POSTGRESQL_MAX_OPEN_CONNS", "APP_DATABASE_POSTGRESQL_MAX_IDLE_CONNS", "APP_DATABASE_POSTGRESQL_CONN_MAX_LIFETIME"} {
				t.Setenv(key, "")
				_ = os.Unsetenv(key)
			}
			for key, value := range tc.env {
				t.Setenv(key, value)
			}

			loaded := InitConfig(noopLogger(), t.TempDir(), nil)
			pool := loaded.DB.Postgresql
			if pool.MaxOpenConns != tc.wantOpen || pool.MaxIdleConns != tc.wantIdle || pool.ConnMaxLifetime != tc.wantLifetime {
				t.Fatalf("pool = %d/%d/%s, want %d/%d/%s", pool.MaxOpenConns, pool.MaxIdleConns, pool.ConnMaxLifetime, tc.wantOpen, tc.wantIdle, tc.wantLifetime)
			}

			db, err := sql.Open("postgres", "host=127.0.0.1 port=1 sslmode=disable")
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer db.Close()
			if err := ConfigureDatabasePool(db, loaded); err != nil {
				t.Fatalf("ConfigureDatabasePool: %v", err)
			}
			if got := db.Stats().MaxOpenConnections; got != tc.wantOpen {
				t.Fatalf("MaxOpenConnections = %d, want %d", got, tc.wantOpen)
			}
		})
	}
}

func TestConfigureDatabasePoolRejectsInvalidLimits(t *testing.T) {
	db, err := sql.Open("postgres", "host=127.0.0.1 port=1 sslmode=disable")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	for _, tc := range []struct {
		desc       string
		open, idle int
		lifetime   time.Duration
	}{
		{"unbounded", 0, 0, time.Minute},
		{"idle above open", 2, 3, time.Minute},
		{"no lifetime", 2, 1, 0},
	} {
		var c Config
		c.DB.Postgresql.MaxOpenConns = tc.open
		c.DB.Postgresql.MaxIdleConns = tc.idle
		c.DB.Postgresql.ConnMaxLifetime = tc.lifetime
		if err := ConfigureDatabasePool(db, c); err == nil {
			t.Errorf("%s: expected an error", tc.desc)
		}
	}
	if got := db.Stats().MaxOpenConnections; got != 0 {
		t.Fatalf("rejected limits were applied: MaxOpenConnections = %d", got)
	}
}
