package config

import (
	"fmt"
	"time"
)

// BlacklistedConfig defines configuration for enhanced BLACKLISTED user handling
type BlacklistedConfig struct {
	EnableAsyncProcessing    bool          `yaml:"enable_async_processing" json:"enable_async_processing"`
	EnableBatchProcessing    bool          `yaml:"enable_batch_processing" json:"enable_batch_processing"`
	MaxRetries               int           `yaml:"max_retries" json:"max_retries"`
	RetryBackoffBase         time.Duration `yaml:"retry_backoff_base" json:"retry_backoff_base"`
	BatchSize                int           `yaml:"batch_size" json:"batch_size"`
	MaxConcurrency           int           `yaml:"max_concurrency" json:"max_concurrency"`
	EnableDetailedLogging    bool          `yaml:"enable_detailed_logging" json:"enable_detailed_logging"`
	LogBlacklistMetrics      bool          `yaml:"log_blacklist_metrics" json:"log_blacklist_metrics"`
	EnableSubscriptionCheck  bool          `yaml:"enable_subscription_check" json:"enable_subscription_check"`
	EnableUserbaseInsertion  bool          `yaml:"enable_userbase_insertion" json:"enable_userbase_insertion"`
	CleanupTimeout           time.Duration `yaml:"cleanup_timeout" json:"cleanup_timeout"`
	EnableMetrics            bool          `yaml:"enable_metrics" json:"enable_metrics"`
	MetricsInterval          time.Duration `yaml:"metrics_interval" json:"metrics_interval"`
	UserbaseInsertionTimeout time.Duration `yaml:"userbase_insertion_timeout" json:"userbase_insertion_timeout"`
	EnableAuditLogging       bool          `yaml:"enable_audit_logging" json:"enable_audit_logging"`
	AuditLogRetentionDays    int           `yaml:"audit_log_retention_days" json:"audit_log_retention_days"`
}

// DefaultBlacklistedConfig returns the default configuration for BLACKLISTED handling
func DefaultBlacklistedConfig() *BlacklistedConfig {
	return &BlacklistedConfig{
		EnableAsyncProcessing:    true,
		EnableBatchProcessing:    true,
		MaxRetries:               3,
		RetryBackoffBase:         100 * time.Millisecond,
		BatchSize:                50,
		MaxConcurrency:           10,
		EnableDetailedLogging:    true,
		LogBlacklistMetrics:      true,
		EnableSubscriptionCheck:  true,
		EnableUserbaseInsertion:  true,
		CleanupTimeout:           30 * time.Second,
		EnableMetrics:            true,
		MetricsInterval:          1 * time.Minute,
		UserbaseInsertionTimeout: 10 * time.Second,
		EnableAuditLogging:       true,
		AuditLogRetentionDays:    90,
	}
}

// Validate validates the BLACKLISTED configuration
func (c *BlacklistedConfig) Validate() error {
	if c.MaxRetries < 0 {
		return fmt.Errorf("max_retries must be non-negative")
	}
	if c.RetryBackoffBase < 0 {
		return fmt.Errorf("retry_backoff_base must be non-negative")
	}
	if c.BatchSize <= 0 {
		return fmt.Errorf("batch_size must be positive")
	}
	if c.MaxConcurrency <= 0 {
		return fmt.Errorf("max_concurrency must be positive")
	}
	if c.CleanupTimeout <= 0 {
		return fmt.Errorf("cleanup_timeout must be positive")
	}
	if c.UserbaseInsertionTimeout <= 0 {
		return fmt.Errorf("userbase_insertion_timeout must be positive")
	}
	if c.AuditLogRetentionDays <= 0 {
		return fmt.Errorf("audit_log_retention_days must be positive")
	}
	return nil
}
