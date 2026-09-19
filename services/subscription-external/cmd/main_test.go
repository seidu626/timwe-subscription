package main

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/seidu626/subscription-manager/common/config"
)

func TestValidateTIMWEStartupConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Config
		wantErr bool
	}{
		{
			name:    "missing api key",
			cfg:     timweTestConfig("", "1234567890123456", "2170", ""),
			wantErr: true,
		},
		{
			name:    "auth key bypasses psk and partner service requirements",
			cfg:     timweTestConfig("api-key", "", "", "precomputed-auth-key"),
			wantErr: false,
		},
		{
			name:    "missing partner service id when auth key absent",
			cfg:     timweTestConfig("api-key", "1234567890123456", "", ""),
			wantErr: true,
		},
		{
			name:    "invalid psk length when auth key absent",
			cfg:     timweTestConfig("api-key", "short", "2170", ""),
			wantErr: true,
		},
		{
			name:    "valid psk length and partner service id",
			cfg:     timweTestConfig("api-key", "1234567890123456", "2170", ""),
			wantErr: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := validateTIMWEStartupConfig(tc.cfg)
			if (err != nil) != tc.wantErr {
				t.Fatalf("unexpected error state: err=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestOpenDatabaseBoundsConcurrentConnections(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	poolConfig := &config.Config{}
	poolConfig.Database.Postgresql.MaxOpenConns = 3
	poolConfig.Database.Postgresql.MaxIdleConns = 1
	poolConfig.Database.Postgresql.ConnMaxLifetime = time.Minute
	poolConfig.Database.Postgresql.ConnectionTimeout = 2 * time.Second
	db, err := config.OpenDatabase(context.Background(), "postgres", dsn, poolConfig)
	if err != nil {
		t.Fatalf("NewSQLDB() error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	start := make(chan struct{})
	errs := make(chan error, 12)
	var wg sync.WaitGroup
	for i := 0; i < cap(errs); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			var one int
			errs <- db.QueryRowContext(context.Background(), "SELECT 1 FROM pg_sleep(0.1)").Scan(&one)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent query error: %v", err)
		}
	}

	stats := db.Stats()
	if stats.MaxOpenConnections != 3 {
		t.Fatalf("MaxOpenConnections = %d, want 3", stats.MaxOpenConnections)
	}
	if stats.OpenConnections > 3 {
		t.Fatalf("OpenConnections = %d, exceeds configured maximum 3", stats.OpenConnections)
	}
	if stats.WaitCount == 0 {
		t.Fatal("WaitCount = 0, expected excess concurrent work to queue")
	}
}

func TestIsValidAESKeyLength(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{key: "1234567890123456", want: true},
		{key: "123456789012345678901234", want: true},
		{key: "12345678901234567890123456789012", want: true},
		{key: "123456789012345", want: false},
		{key: "12345678901234567", want: false},
	}

	for _, tc := range tests {
		if got := isValidAESKeyLength(tc.key); got != tc.want {
			t.Fatalf("isValidAESKeyLength(%q)=%v want=%v", tc.key, got, tc.want)
		}
	}
}

func timweTestConfig(apiKey, psk, partnerServiceID, authKey string) *config.Config {
	cfg := &config.Config{}
	cfg.Application.TIMWE.APIKey = apiKey
	cfg.Application.TIMWE.Psk = psk
	cfg.Application.TIMWE.PartnerServiceID = partnerServiceID
	cfg.Application.TIMWE.AuthenticationKey = authKey
	return cfg
}
