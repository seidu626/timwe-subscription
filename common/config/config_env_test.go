package config

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"
)

type noConnectConnector struct{}

func (noConnectConnector) Connect(context.Context) (driver.Conn, error) {
	return nil, errors.New("test connector does not connect")
}
func (noConnectConnector) Driver() driver.Driver { return noConnectDriver{} }

type noConnectDriver struct{}

func (noConnectDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("test driver does not connect")
}

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

func TestInitConfig_BindsDatabasePoolEnv(t *testing.T) {
	cfgPath := writeMinimalConfig(t)

	t.Setenv("APP_DATABASE_POSTGRESQL_MAX_OPEN_CONNS", "23")
	t.Setenv("APP_DATABASE_POSTGRESQL_MAX_IDLE_CONNS", "4")
	t.Setenv("APP_DATABASE_POSTGRESQL_CONN_MAX_LIFETIME", "7m")
	t.Setenv("APP_DATABASE_POSTGRESQL_CONNECTION_TIMEOUT", "11s")
	t.Setenv("APP_APPLICATION_BATCH_MAX_WORKERS_PER_JOB", "13")
	t.Setenv("APP_APPLICATION_BATCH_MAX_CONCURRENT_OPTINS", "17")

	loaded := InitConfig(zap.NewNop(), filepath.Dir(cfgPath), []string{filepath.Base(cfgPath)})
	if loaded.Database.Postgresql.MaxOpenConns != 23 {
		t.Fatalf("MaxOpenConns = %d, want 23", loaded.Database.Postgresql.MaxOpenConns)
	}
	if loaded.Database.Postgresql.MaxIdleConns != 4 {
		t.Fatalf("MaxIdleConns = %d, want 4", loaded.Database.Postgresql.MaxIdleConns)
	}
	if loaded.Database.Postgresql.ConnMaxLifetime != 7*time.Minute {
		t.Fatalf("ConnMaxLifetime = %s, want 7m", loaded.Database.Postgresql.ConnMaxLifetime)
	}
	if loaded.Database.Postgresql.ConnectionTimeout != 11*time.Second {
		t.Fatalf("ConnectionTimeout = %s, want 11s", loaded.Database.Postgresql.ConnectionTimeout)
	}
	if loaded.Application.Batch.MaxWorkersPerJob != 13 {
		t.Fatalf("MaxWorkersPerJob = %d, want 13", loaded.Application.Batch.MaxWorkersPerJob)
	}
	if loaded.Application.Batch.MaxConcurrentOptins != 17 {
		t.Fatalf("MaxConcurrentOptins = %d, want 17", loaded.Application.Batch.MaxConcurrentOptins)
	}
}

func TestConfigureDatabasePoolRequiresFiniteValidBounds(t *testing.T) {
	db := sql.OpenDB(noConnectConnector{})
	t.Cleanup(func() { _ = db.Close() })

	tests := []struct {
		name    string
		db      *sql.DB
		config  *Config
		wantErr bool
	}{
		{name: "nil pool", config: databasePoolTestConfig(3, 1, time.Minute), wantErr: true},
		{name: "nil config", db: db, wantErr: true},
		{name: "unbounded open connections", db: db, config: databasePoolTestConfig(0, 0, time.Minute), wantErr: true},
		{name: "idle exceeds open", db: db, config: databasePoolTestConfig(2, 3, time.Minute), wantErr: true},
		{name: "unbounded lifetime", db: db, config: databasePoolTestConfig(2, 1, 0), wantErr: true},
		{name: "valid", db: db, config: databasePoolTestConfig(3, 1, time.Minute)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ConfigureDatabasePool(tt.db, tt.config)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ConfigureDatabasePool() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
	if got := db.Stats().MaxOpenConnections; got != 3 {
		t.Fatalf("MaxOpenConnections = %d, want 3", got)
	}
}

func databasePoolTestConfig(maxOpen, maxIdle int, lifetime time.Duration) *Config {
	cfg := &Config{}
	cfg.Database.Postgresql.MaxOpenConns = maxOpen
	cfg.Database.Postgresql.MaxIdleConns = maxIdle
	cfg.Database.Postgresql.ConnMaxLifetime = lifetime
	return cfg
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
