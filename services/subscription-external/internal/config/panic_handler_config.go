package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// PanicHandlerConfig represents the configuration for panic handling
type PanicHandlerConfig struct {
	EnableRecovery   bool          `yaml:"enable_recovery" env:"PANIC_ENABLE_RECOVERY"`
	LogStackTraces   bool          `yaml:"log_stack_traces" env:"PANIC_LOG_STACK_TRACES"`
	LogGoroutineInfo bool          `yaml:"log_goroutine_info" env:"PANIC_LOG_GOROUTINE_INFO"`
	MaxStackDepth    int           `yaml:"max_stack_depth" env:"PANIC_MAX_STACK_DEPTH"`
	RecoveryTimeout  time.Duration `yaml:"recovery_timeout" env:"PANIC_RECOVERY_TIMEOUT"`
	ExitOnFatal      bool          `yaml:"exit_on_fatal" env:"PANIC_EXIT_ON_FATAL"`
	ExitCode         int           `yaml:"exit_code" env:"PANIC_EXIT_CODE"`
}

// DefaultPanicHandlerConfig returns sensible defaults
func DefaultPanicHandlerConfig() *PanicHandlerConfig {
	return &PanicHandlerConfig{
		EnableRecovery:   true,
		LogStackTraces:   true,
		LogGoroutineInfo: true,
		MaxStackDepth:    64,
		RecoveryTimeout:  30 * time.Second,
		ExitOnFatal:      true,
		ExitCode:         1,
	}
}

// LoadPanicHandlerConfig loads panic handler configuration from file and environment
func LoadPanicHandlerConfig(configPath string, environment string) (*PanicHandlerConfig, error) {
	// Start with defaults
	config := DefaultPanicHandlerConfig()

	// Load from YAML file if it exists
	if err := loadFromYAML(config, configPath, environment); err != nil {
		// Log warning but continue with defaults
		fmt.Printf("Warning: Failed to load panic handler config from %s: %v\n", configPath, err)
	}

	// Override with environment variables
	loadFromEnvironment(config)

	return config, nil
}

// loadFromYAML loads configuration from YAML file
func loadFromYAML(config *PanicHandlerConfig, configPath string, environment string) error {
	// Read the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var yamlConfig map[string]interface{}
	if err := yaml.Unmarshal(data, &yamlConfig); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Get environment-specific config
	envConfig, exists := yamlConfig[environment]
	if !exists {
		// Fall back to default if environment-specific config doesn't exist
		envConfig = yamlConfig["default"]
	}

	if envConfig == nil {
		return fmt.Errorf("no configuration found for environment '%s' or 'default'", environment)
	}

	// Convert to map
	envMap, ok := envConfig.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid configuration format for environment '%s'", environment)
	}

	// Apply configuration values
	if val, exists := envMap["enable_recovery"]; exists {
		if boolVal, ok := val.(bool); ok {
			config.EnableRecovery = boolVal
		}
	}

	if val, exists := envMap["log_stack_traces"]; exists {
		if boolVal, ok := val.(bool); ok {
			config.LogStackTraces = boolVal
		}
	}

	if val, exists := envMap["log_goroutine_info"]; exists {
		if boolVal, ok := val.(bool); ok {
			config.LogGoroutineInfo = boolVal
		}
	}

	if val, exists := envMap["max_stack_depth"]; exists {
		if intVal, ok := val.(int); ok {
			config.MaxStackDepth = intVal
		}
	}

	if val, exists := envMap["recovery_timeout"]; exists {
		if strVal, ok := val.(string); ok {
			if duration, err := time.ParseDuration(strVal); err == nil {
				config.RecoveryTimeout = duration
			}
		}
	}

	if val, exists := envMap["exit_on_fatal"]; exists {
		if boolVal, ok := val.(bool); ok {
			config.ExitOnFatal = boolVal
		}
	}

	if val, exists := envMap["exit_code"]; exists {
		if intVal, ok := val.(int); ok {
			config.ExitCode = intVal
		}
	}

	return nil
}

// loadFromEnvironment loads configuration from environment variables
func loadFromEnvironment(config *PanicHandlerConfig) {
	// EnableRecovery
	if val := os.Getenv("PANIC_ENABLE_RECOVERY"); val != "" {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			config.EnableRecovery = boolVal
		}
	}

	// LogStackTraces
	if val := os.Getenv("PANIC_LOG_STACK_TRACES"); val != "" {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			config.LogStackTraces = boolVal
		}
	}

	// LogGoroutineInfo
	if val := os.Getenv("PANIC_LOG_GOROUTINE_INFO"); val != "" {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			config.LogGoroutineInfo = boolVal
		}
	}

	// MaxStackDepth
	if val := os.Getenv("PANIC_MAX_STACK_DEPTH"); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			config.MaxStackDepth = intVal
		}
	}

	// RecoveryTimeout
	if val := os.Getenv("PANIC_RECOVERY_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			config.RecoveryTimeout = duration
		}
	}

	// ExitOnFatal
	if val := os.Getenv("PANIC_EXIT_ON_FATAL"); val != "" {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			config.ExitOnFatal = boolVal
		}
	}

	// ExitCode
	if val := os.Getenv("PANIC_EXIT_CODE"); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			config.ExitCode = intVal
		}
	}
}

// Validate validates the configuration
func (c *PanicHandlerConfig) Validate() error {
	if c.MaxStackDepth <= 0 {
		return fmt.Errorf("max_stack_depth must be positive, got %d", c.MaxStackDepth)
	}

	if c.RecoveryTimeout <= 0 {
		return fmt.Errorf("recovery_timeout must be positive, got %v", c.RecoveryTimeout)
	}

	if c.ExitCode < 0 || c.ExitCode > 255 {
		return fmt.Errorf("exit_code must be between 0 and 255, got %d", c.ExitCode)
	}

	return nil
}

// String returns a string representation of the configuration
func (c *PanicHandlerConfig) String() string {
	return fmt.Sprintf(
		"PanicHandlerConfig{EnableRecovery: %t, LogStackTraces: %t, LogGoroutineInfo: %t, "+
			"MaxStackDepth: %d, RecoveryTimeout: %v, ExitOnFatal: %t, ExitCode: %d}",
		c.EnableRecovery, c.LogStackTraces, c.LogGoroutineInfo,
		c.MaxStackDepth, c.RecoveryTimeout, c.ExitOnFatal, c.ExitCode,
	)
}

// GetEnvironment returns the current environment
func GetEnvironment() string {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = os.Getenv("ENVIRONMENT")
	}
	if env == "" {
		env = "development"
	}
	return strings.ToLower(env)
}
