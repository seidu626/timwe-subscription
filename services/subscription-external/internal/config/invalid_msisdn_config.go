package config

import "time"

// InvalidMSISDNConfig holds configuration for INVALID_MSISDN handling
type InvalidMSISDNConfig struct {
	// Cleanup settings
	EnableAsyncCleanup    bool          `yaml:"enable_async_cleanup" json:"enable_async_cleanup"`
	EnableBatchProcessing bool          `yaml:"enable_batch_processing" json:"enable_batch_processing"`
	MaxRetries            int           `yaml:"max_retries" json:"max_retries"`
	RetryBackoffBase      time.Duration `yaml:"retry_backoff_base" json:"retry_backoff_base"`

	// Batch processing settings
	BatchSize      int `yaml:"batch_size" json:"batch_size"`
	MaxConcurrency int `yaml:"max_concurrency" json:"max_concurrency"`

	// Logging settings
	EnableDetailedLogging bool `yaml:"enable_detailed_logging" json:"enable_detailed_logging"`
	LogCleanupMetrics     bool `yaml:"log_cleanup_metrics" json:"log_cleanup_metrics"`

	// Database settings
	EnableSubscriptionCheck bool          `yaml:"enable_subscription_check" json:"enable_subscription_check"`
	CleanupTimeout          time.Duration `yaml:"cleanup_timeout" json:"cleanup_timeout"`

	// Monitoring settings
	EnableMetrics   bool          `yaml:"enable_metrics" json:"enable_metrics"`
	MetricsInterval time.Duration `yaml:"metrics_interval" json:"metrics_interval"`
}

// DefaultInvalidMSISDNConfig returns the default configuration
func DefaultInvalidMSISDNConfig() *InvalidMSISDNConfig {
	return &InvalidMSISDNConfig{
		// Cleanup settings
		EnableAsyncCleanup:    true,
		EnableBatchProcessing: true,
		MaxRetries:            3,
		RetryBackoffBase:      100 * time.Millisecond,

		// Batch processing settings
		BatchSize:      100,
		MaxConcurrency: 10,

		// Logging settings
		EnableDetailedLogging: true,
		LogCleanupMetrics:     true,

		// Database settings
		EnableSubscriptionCheck: true,
		CleanupTimeout:          30 * time.Second,

		// Monitoring settings
		EnableMetrics:   true,
		MetricsInterval: 5 * time.Minute,
	}
}

// Validate validates the configuration
func (c *InvalidMSISDNConfig) Validate() error {
	if c.MaxRetries < 0 {
		c.MaxRetries = 0
	}
	if c.MaxRetries > 10 {
		c.MaxRetries = 10
	}

	if c.BatchSize < 1 {
		c.BatchSize = 1
	}
	if c.BatchSize > 1000 {
		c.BatchSize = 1000
	}

	if c.MaxConcurrency < 1 {
		c.MaxConcurrency = 1
	}
	if c.MaxConcurrency > 100 {
		c.MaxConcurrency = 100
	}

	if c.RetryBackoffBase < 10*time.Millisecond {
		c.RetryBackoffBase = 10 * time.Millisecond
	}
	if c.RetryBackoffBase > 1*time.Second {
		c.RetryBackoffBase = 1 * time.Second
	}

	if c.CleanupTimeout < 1*time.Second {
		c.CleanupTimeout = 1 * time.Second
	}
	if c.CleanupTimeout > 5*time.Minute {
		c.CleanupTimeout = 5 * time.Minute
	}

	if c.MetricsInterval < 1*time.Minute {
		c.MetricsInterval = 1 * time.Minute
	}
	if c.MetricsInterval > 1*time.Hour {
		c.MetricsInterval = 1 * time.Hour
	}

	return nil
}
