package worker

import (
	"time"
)

// Config holds worker configuration
type Config struct {
	// Processing settings
	BatchSize         int           `json:"batch_size"`         // Number of subscriptions per batch
	MaxConcurrency    int           `json:"max_concurrency"`    // Maximum concurrent workers
	RetryAttempts     int           `json:"retry_attempts"`     // Maximum retry attempts
	RetryDelay        time.Duration `json:"retry_delay"`        // Delay between retries
	ProcessingTimeout time.Duration `json:"processing_timeout"` // Timeout per subscription
	BatchDelay        time.Duration `json:"batch_delay"`        // Delay between batches

	// Priority and filtering
	PriorityProcessing   bool `json:"priority_processing"`    // Process by priority order
	SkipProcessed        bool `json:"skip_processed"`         // Skip already processed
	UpdateChargingHealth bool `json:"update_charging_health"` // Update charging health status

	// Performance tuning
	MaxQueueSize           int           `json:"max_queue_size"`           // Maximum items in processing queue
	ProgressReportInterval time.Duration `json:"progress_report_interval"` // How often to report progress
	ResultsBufferSize      int           `json:"results_buffer_size"`      // Buffer size for results

	// Monitoring and logging
	EnableMetrics bool   `json:"enable_metrics"` // Enable detailed metrics collection
	LogLevel      string `json:"log_level"`      // Logging level (debug, info, warn, error)
	EnableTracing bool   `json:"enable_tracing"` // Enable request tracing

	// Resubscription specific
	ResubscriptionDelay       time.Duration `json:"resubscription_delay"`        // Delay before attempting resubscription
	MaxResubscriptionAttempts int           `json:"max_resubscription_attempts"` // Max resubscription attempts
	ResubscriptionBackoff     time.Duration `json:"resubscription_backoff"`      // Exponential backoff base

	// External service integration
	EnableExternalValidation bool          `json:"enable_external_validation"` // Validate with external services
	ExternalServiceTimeout   time.Duration `json:"external_service_timeout"`   // Timeout for external calls
	ExternalServiceRetries   int           `json:"external_service_retries"`   // Retries for external calls

	// Database settings
	MaxDBConnections        int           `json:"max_db_connections"`        // Maximum database connections
	DBQueryTimeout          time.Duration `json:"db_query_timeout"`          // Database query timeout
	EnableConnectionPooling bool          `json:"enable_connection_pooling"` // Enable DB connection pooling

	// Circuit breaker settings
	EnableCircuitBreaker    bool          `json:"enable_circuit_breaker"`    // Enable circuit breaker pattern
	CircuitBreakerThreshold int           `json:"circuit_breaker_threshold"` // Failures before opening circuit
	CircuitBreakerTimeout   time.Duration `json:"circuit_breaker_timeout"`   // Time to wait before half-open
}

// DefaultConfig returns default worker configuration
func DefaultConfig() *Config {
	return &Config{
		BatchSize:                 100,
		MaxConcurrency:            5,
		RetryAttempts:             3,
		RetryDelay:                30 * time.Second,
		ProcessingTimeout:         2 * time.Minute,
		BatchDelay:                100 * time.Millisecond,
		PriorityProcessing:        true,
		SkipProcessed:             true,
		UpdateChargingHealth:      true,
		MaxQueueSize:              1000,
		ProgressReportInterval:    10 * time.Second,
		ResultsBufferSize:         1000,
		EnableMetrics:             true,
		LogLevel:                  "info",
		EnableTracing:             false,
		ResubscriptionDelay:       1 * time.Minute,
		MaxResubscriptionAttempts: 5,
		ResubscriptionBackoff:     2 * time.Minute,
		EnableExternalValidation:  false,
		ExternalServiceTimeout:    30 * time.Second,
		ExternalServiceRetries:    2,
		MaxDBConnections:          10,
		DBQueryTimeout:            30 * time.Second,
		EnableConnectionPooling:   true,
		EnableCircuitBreaker:      true,
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     1 * time.Minute,
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.BatchSize < 1 {
		c.BatchSize = 1
	}
	if c.BatchSize > 10000 {
		c.BatchSize = 10000
	}
	if c.MaxConcurrency < 1 {
		c.MaxConcurrency = 1
	}
	if c.MaxConcurrency > 100 {
		c.MaxConcurrency = 100
	}
	if c.RetryAttempts < 0 {
		c.RetryAttempts = 0
	}
	if c.RetryAttempts > 10 {
		c.RetryAttempts = 10
	}
	if c.ProcessingTimeout < time.Second {
		c.ProcessingTimeout = time.Second
	}
	if c.MaxQueueSize < 100 {
		c.MaxQueueSize = 100
	}
	if c.MaxQueueSize > 100000 {
		c.MaxQueueSize = 100000
	}
	if c.ProgressReportInterval < time.Second {
		c.ProgressReportInterval = time.Second
	}
	if c.ResultsBufferSize < 100 {
		c.ResultsBufferSize = 100
	}
	if c.ResultsBufferSize > 100000 {
		c.ResultsBufferSize = 100000
	}
	return nil
}

// ToProcessingConfig converts worker config to processing config
func (c *Config) ToProcessingConfig() *ProcessingConfig {
	return &ProcessingConfig{
		BatchSize:            c.BatchSize,
		MaxConcurrency:       c.MaxConcurrency,
		RetryAttempts:        c.RetryAttempts,
		RetryDelay:           c.RetryDelay,
		ProcessingTimeout:    c.ProcessingTimeout,
		PriorityProcessing:   c.PriorityProcessing,
		SkipProcessed:        c.SkipProcessed,
		UpdateChargingHealth: c.UpdateChargingHealth,
	}
}
