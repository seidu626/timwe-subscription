package config

import (
	"fmt"
	"os"
	"time"

	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"gopkg.in/yaml.v3"
)

// LoadRenewalConfig loads renewal configuration from file
func LoadRenewalConfig(configPath string) (*domain.RenewalConfig, error) {
	// Default configuration
	defaultConfig := &domain.RenewalConfig{
		Strategy: domain.StrategyOptOutOptIn,
		Enabled:  true,
		ChurnPolicy: domain.ChurnPolicy{
			MaxHoursWithoutPayment: 720, // 30 days * 24 hours
			MaxRenewalAttempts:     3,
			RetryIntervalHours:     24,
			GracePeriodHours:       168, // 7 days * 24 hours
			SafeMode:               true,
		},
		OptOutOptIn: struct {
			WaitBetweenMs int `json:"wait_between_ms" yaml:"wait_between_ms"`
			BatchSize     int `json:"batch_size" yaml:"batch_size"`
			MaxConcurrent int `json:"max_concurrent" yaml:"max_concurrent"`
			RateLimitMs   int `json:"rate_limit_ms" yaml:"rate_limit_ms"`
			BatchDelayMs  int `json:"batch_delay_ms" yaml:"batch_delay_ms"`
		}{
			WaitBetweenMs: 5000, // 5 seconds
			BatchSize:     100,
			MaxConcurrent: 10,
			RateLimitMs:   1000, // 1 second
			BatchDelayMs:  5000, // 5 seconds between batches
		},
		Worker: struct {
			Enabled           bool          `json:"enabled" yaml:"enabled"`
			DailyRunTime      string        `json:"daily_run_time" yaml:"daily_run_time"`
			Timezone          string        `json:"timezone" yaml:"timezone"`
			TimeoutPerRenewal time.Duration `json:"timeout_per_renewal" yaml:"timeout_per_renewal"`
			MaxRetries        int           `json:"max_retries" yaml:"max_retries"`
		}{
			Enabled:           true,
			DailyRunTime:      "02:00", // 2 AM
			Timezone:          "UTC",
			TimeoutPerRenewal: 30 * time.Second,
			MaxRetries:        3,
		},
		Monitoring: struct {
			AlertOnFailureRate float64 `json:"alert_on_failure_rate" yaml:"alert_on_failure_rate"`
			AlertOnChurnRate   float64 `json:"alert_on_churn_rate" yaml:"alert_on_churn_rate"`
			MetricsPort        int     `json:"metrics_port" yaml:"metrics_port"`
		}{
			AlertOnFailureRate: 0.1,  // 10%
			AlertOnChurnRate:   0.05, // 5%
			MetricsPort:        9090,
		},
	}

	// Try to load from file if it exists
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			data, err := os.ReadFile(configPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read renewal config file: %w", err)
			}

			if err := yaml.Unmarshal(data, defaultConfig); err != nil {
				return nil, fmt.Errorf("failed to parse renewal config file: %w", err)
			}
		}
	}

	return defaultConfig, nil
}
