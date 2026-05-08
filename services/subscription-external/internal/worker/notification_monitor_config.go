package worker

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// NotificationMonitorYAMLConfig represents the YAML configuration structure
type NotificationMonitorYAMLConfig struct {
	Monitor struct {
		BatchSize           int    `yaml:"batch_size"`
		MaxInFlightBatches  int    `yaml:"max_in_flight_batches"`
		ScanLookbackDays    int    `yaml:"scan_lookback_days"`
		RenewalWindowMonths int    `yaml:"renewal_window_months"`
		IdleSleep           string `yaml:"idle_sleep"`
		LeaseTTL            string `yaml:"lease_ttl"`
		RedisKeyPrefix      string `yaml:"redis_key_prefix"`

		Products struct {
			ProductIds []string `yaml:"product_ids"`
		} `yaml:"products"`

		EntryChannels struct {
			Channels []string `yaml:"channels"`
			Default  string   `yaml:"default"`
			Strategy string   `yaml:"strategy"`
		} `yaml:"entry_channels"`

		Behavior struct {
			SkipUnconfiguredProducts bool `yaml:"skip_unconfigured_products"`
			TryOriginalChannelFirst  bool `yaml:"try_original_channel_first"`
			MaxOptinAttempts         int  `yaml:"max_optin_attempts"`
			DelayBetweenAttempts     int  `yaml:"delay_between_attempts"`
		} `yaml:"behavior"`

		Metrics struct {
			Enabled            bool     `yaml:"enabled"`
			CollectionInterval string   `yaml:"collection_interval"`
			Labels             []string `yaml:"labels"`
		} `yaml:"metrics"`

		Logging struct {
			Level                   string `yaml:"level"`
			LogSuccessfulOptins     bool   `yaml:"log_successful_optins"`
			LogSkippedNotifications bool   `yaml:"log_skipped_notifications"`
			LogChannelFailures      bool   `yaml:"log_channel_failures"`
		} `yaml:"logging"`

		ErrorHandling struct {
			MaxConsecutiveErrors  int    `yaml:"max_consecutive_errors"`
			BackoffDuration       string `yaml:"backoff_duration"`
			RetryFailedOperations bool   `yaml:"retry_failed_operations"`
			MaxRetryAttempts      int    `yaml:"max_retry_attempts"`
		} `yaml:"error_handling"`
		// Enhanced: Resilience configuration
		Resilience struct {
			MaxRetries              int     `yaml:"max_retries"`
			InitialBackoff          string  `yaml:"initial_backoff"`
			MaxBackoff              string  `yaml:"max_backoff"`
			BackoffMultiplier       float64 `yaml:"backoff_multiplier"`
			CircuitBreakerThreshold int     `yaml:"circuit_breaker_threshold"`
			CircuitBreakerTimeout   string  `yaml:"circuit_breaker_timeout"`
			GracefulDegradation     bool    `yaml:"graceful_degradation"`
			HealthCheckInterval     string  `yaml:"health_check_interval"`
		} `yaml:"resilience"`

		// Enhanced: Invalid entry channel filtering
		InvalidEntryChannels []string `yaml:"invalid_entry_channels"`
	} `yaml:"monitor"`
}

// LoadNotificationMonitorConfig loads configuration from a YAML file
func LoadNotificationMonitorConfig(configPath string) (*NotificationMonitorConfig, error) {
	// Default configuration
	config := &NotificationMonitorConfig{
		BatchSize:               1000,
		MaxInFlightBatches:      10,
		ScanLookbackDays:        60,
		RenewalWindowMonths:     2,
		IdleSleep:               2 * time.Second,
		LeaseTTL:                30 * time.Second,
		RedisKeyPrefix:          "notifmon",
		ProductIds:              []string{"8509"},
		EntryChannels:           []string{"USSD"},
		DefaultEntryChannel:     "USSD",
		MaxRetries:              3,
		InitialBackoff:          1 * time.Second,
		MaxBackoff:              30 * time.Second,
		BackoffMultiplier:       2.0,
		CircuitBreakerThreshold: 5,
		CircuitBreakerTimeout:   60 * time.Second,
		GracefulDegradation:     true,
		HealthCheckInterval:     30 * time.Second,
		InvalidEntryChannels:    []string{"CCTOOL", "INTERNAL", "ADMIN", "SYSTEM", "BATCH", "API"},
	}

	// If config file is specified, load it
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			data, err := os.ReadFile(configPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}

			var yamlConfig NotificationMonitorYAMLConfig
			if err := yaml.Unmarshal(data, &yamlConfig); err != nil {
				return nil, fmt.Errorf("failed to parse YAML config: %w", err)
			}

			// Apply YAML configuration
			if yamlConfig.Monitor.BatchSize > 0 {
				config.BatchSize = yamlConfig.Monitor.BatchSize
			}
			if yamlConfig.Monitor.MaxInFlightBatches > 0 {
				config.MaxInFlightBatches = yamlConfig.Monitor.MaxInFlightBatches
			}
			if yamlConfig.Monitor.ScanLookbackDays > 0 {
				config.ScanLookbackDays = yamlConfig.Monitor.ScanLookbackDays
			}
			if yamlConfig.Monitor.RenewalWindowMonths > 0 {
				config.RenewalWindowMonths = yamlConfig.Monitor.RenewalWindowMonths
			}
			if yamlConfig.Monitor.IdleSleep != "" {
				if duration, err := time.ParseDuration(yamlConfig.Monitor.IdleSleep); err == nil {
					config.IdleSleep = duration
				}
			}
			if yamlConfig.Monitor.LeaseTTL != "" {
				if duration, err := time.ParseDuration(yamlConfig.Monitor.LeaseTTL); err == nil {
					config.LeaseTTL = duration
				}
			}
			if yamlConfig.Monitor.RedisKeyPrefix != "" {
				config.RedisKeyPrefix = yamlConfig.Monitor.RedisKeyPrefix
			}

			// Apply product configuration
			if len(yamlConfig.Monitor.Products.ProductIds) > 0 {
				config.ProductIds = yamlConfig.Monitor.Products.ProductIds
			}

			// Apply entry channel configuration
			if len(yamlConfig.Monitor.EntryChannels.Channels) > 0 {
				config.EntryChannels = yamlConfig.Monitor.EntryChannels.Channels
			}
			if yamlConfig.Monitor.EntryChannels.Default != "" {
				config.DefaultEntryChannel = yamlConfig.Monitor.EntryChannels.Default
			}

			// Enhanced: Apply resilience configuration
			if yamlConfig.Monitor.Resilience.MaxRetries > 0 {
				config.MaxRetries = yamlConfig.Monitor.Resilience.MaxRetries
			}
			if yamlConfig.Monitor.Resilience.InitialBackoff != "" {
				if duration, err := time.ParseDuration(yamlConfig.Monitor.Resilience.InitialBackoff); err == nil {
					config.InitialBackoff = duration
				}
			}
			if yamlConfig.Monitor.Resilience.MaxBackoff != "" {
				if duration, err := time.ParseDuration(yamlConfig.Monitor.Resilience.MaxBackoff); err == nil {
					config.MaxBackoff = duration
				}
			}
			if yamlConfig.Monitor.Resilience.BackoffMultiplier > 0 {
				config.BackoffMultiplier = yamlConfig.Monitor.Resilience.BackoffMultiplier
			}
			if yamlConfig.Monitor.Resilience.CircuitBreakerThreshold > 0 {
				config.CircuitBreakerThreshold = yamlConfig.Monitor.Resilience.CircuitBreakerThreshold
			}
			if yamlConfig.Monitor.Resilience.CircuitBreakerTimeout != "" {
				if duration, err := time.ParseDuration(yamlConfig.Monitor.Resilience.CircuitBreakerTimeout); err == nil {
					config.CircuitBreakerTimeout = duration
				}
			}
			config.GracefulDegradation = yamlConfig.Monitor.Resilience.GracefulDegradation
			if yamlConfig.Monitor.Resilience.HealthCheckInterval != "" {
				if duration, err := time.ParseDuration(yamlConfig.Monitor.Resilience.HealthCheckInterval); err == nil {
					config.HealthCheckInterval = duration
				}
			}

			// Enhanced: Apply invalid entry channel configuration
			if len(yamlConfig.Monitor.InvalidEntryChannels) > 0 {
				config.InvalidEntryChannels = yamlConfig.Monitor.InvalidEntryChannels
			}
		}
	}

	// Ensure we have at least one product ID and entry channel
	if len(config.ProductIds) == 0 {
		config.ProductIds = []string{"8509"}
	}
	if len(config.EntryChannels) == 0 {
		config.EntryChannels = []string{"USSD"}
	}
	if config.DefaultEntryChannel == "" {
		config.DefaultEntryChannel = config.EntryChannels[0]
	}

	return config, nil
}

// ValidateConfig validates the configuration values
func (c *NotificationMonitorConfig) ValidateConfig() error {
	if c.BatchSize <= 0 {
		return fmt.Errorf("batch_size must be positive")
	}
	if c.MaxInFlightBatches <= 0 {
		return fmt.Errorf("max_in_flight_batches must be positive")
	}
	if c.ScanLookbackDays <= 0 {
		return fmt.Errorf("scan_lookback_days must be positive")
	}
	if c.RenewalWindowMonths <= 0 {
		return fmt.Errorf("renewal_window_months must be positive")
	}
	if c.IdleSleep <= 0 {
		return fmt.Errorf("idle_sleep must be positive")
	}
	if c.LeaseTTL <= 0 {
		return fmt.Errorf("lease_ttl must be positive")
	}
	if len(c.ProductIds) == 0 {
		return fmt.Errorf("at least one product_id must be configured")
	}
	if len(c.EntryChannels) == 0 {
		return fmt.Errorf("at least one entry_channel must be configured")
	}
	if c.DefaultEntryChannel == "" {
		return fmt.Errorf("default_entry_channel must be specified")
	}

	// Enhanced: Validate resilience configuration
	if c.MaxRetries < 0 {
		return fmt.Errorf("max_retries must be non-negative")
	}
	if c.InitialBackoff <= 0 {
		return fmt.Errorf("initial_backoff must be positive")
	}
	if c.MaxBackoff <= 0 {
		return fmt.Errorf("max_backoff must be positive")
	}
	if c.BackoffMultiplier <= 0 {
		return fmt.Errorf("backoff_multiplier must be positive")
	}
	if c.CircuitBreakerThreshold <= 0 {
		return fmt.Errorf("circuit_breaker_threshold must be positive")
	}
	if c.CircuitBreakerTimeout <= 0 {
		return fmt.Errorf("circuit_breaker_timeout must be positive")
	}
	if c.HealthCheckInterval <= 0 {
		return fmt.Errorf("health_check_interval must be positive")
	}

	return nil
}
