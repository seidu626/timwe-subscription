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
