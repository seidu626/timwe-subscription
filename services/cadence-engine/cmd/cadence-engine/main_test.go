package main

import (
	"database/sql"
	"testing"

	"github.com/seidu626/subscription-manager/common/config"
	"go.uber.org/zap"
)

// main applies config.ConfigureDatabasePool, which rejects a missing limit or
// lifetime, so the shipped config.yaml plus the compose PGMAX_* vars must
// yield a finite, valid pool.
func TestShippedConfigBoundsDatabasePool(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want int
	}{
		{name: "shipped default", env: "", want: 25},
		{name: "compose PGMAX_OPEN_CONNS", env: "3", want: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PGMAX_OPEN_CONNS", tt.env)
			t.Setenv("PGMAX_IDLE_CONNS", "1")
			cfg := config.InitConfig(zap.NewNop(), "../..", []string{"config.yaml"})
			db, err := sql.Open("postgres", "host=127.0.0.1 port=1 sslmode=disable")
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer db.Close()
			if err := config.ConfigureDatabasePool(db, cfg); err != nil {
				t.Fatalf("ConfigureDatabasePool: %v", err)
			}
			if got := db.Stats().MaxOpenConnections; got != tt.want {
				t.Fatalf("MaxOpenConnections = %d, want %d", got, tt.want)
			}
		})
	}
}
