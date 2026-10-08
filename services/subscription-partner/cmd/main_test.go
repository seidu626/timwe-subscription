package main

import (
	"testing"

	"github.com/seidu626/subscription-manager/common/config"
	"go.uber.org/zap"
)

// main switches to the production logger only when APPLICATION.ENVIRONMENT is
// PRODUCTION, so the shipped config.yaml must define that key for
// APP_APPLICATION_ENVIRONMENT to override it.
func TestShippedConfigEnvironment(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want config.Environment
	}{
		{name: "defaults to development", env: "", want: config.DEVELOPMENT},
		{name: "env override", env: "PRODUCTION", want: config.PRODUCTION},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("APP_APPLICATION_ENVIRONMENT", tt.env)
			cfg := config.InitConfig(zap.NewNop(), "..", []string{"config.yaml"})
			if cfg.Application.Environment != tt.want {
				t.Fatalf("environment = %q, want %q", cfg.Application.Environment, tt.want)
			}
		})
	}
}
