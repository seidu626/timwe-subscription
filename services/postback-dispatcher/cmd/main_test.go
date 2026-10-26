package main

import (
	"database/sql"
	"testing"

	"github.com/seidu626/subscription-manager/common/config"
	"go.uber.org/zap"
)

// The vendored common/config does not bind PGMAX_*, so the shipped config.yaml
// must carry finite pool limits that APP_DATABASE_POSTGRESQL_* can override.
func TestShippedConfigBoundsDatabasePool(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want int
	}{
		{name: "shipped default", env: "", want: 2},
		{name: "env override", env: "7", want: 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("APP_DATABASE_POSTGRESQL_MAX_OPEN_CONNS", tt.env)
			cfg := config.InitConfig(zap.NewNop(), "..", []string{"config.yaml"})
			db, err := sql.Open("postgres", "host=127.0.0.1 port=1 sslmode=disable")
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer db.Close()
			if err := configureDatabasePool(db, cfg); err != nil {
				t.Fatalf("configureDatabasePool: %v", err)
			}
			if got := db.Stats().MaxOpenConnections; got != tt.want {
				t.Fatalf("MaxOpenConnections = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestConfigureDatabasePoolRejectsUnboundedPool(t *testing.T) {
	db, err := sql.Open("postgres", "host=127.0.0.1 port=1 sslmode=disable")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := configureDatabasePool(db, &config.Config{}); err == nil {
		t.Fatal("expected an error for a zero open-connection limit")
	}
	if got := db.Stats().MaxOpenConnections; got != 0 {
		t.Fatalf("rejected limits were applied: MaxOpenConnections = %d", got)
	}
}
