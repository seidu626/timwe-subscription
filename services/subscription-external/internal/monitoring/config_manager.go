package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// MonitorConfig holds all configuration for the monitoring system
type MonitorConfig struct {
	Thresholds      *AlertThresholds      `json:"thresholds" yaml:"thresholds"`
	SyncInterval    time.Duration         `json:"sync_interval" yaml:"sync_interval"`
	RetentionPeriod time.Duration         `json:"retention_period" yaml:"retention_period"`
	AlertRules      []*AlertRule          `json:"alert_rules" yaml:"alert_rules"`
	Notification    *NotificationConfig   `json:"notification" yaml:"notification"`
	CircuitBreaker  *CircuitBreakerConfig `json:"circuit_breaker" yaml:"circuit_breaker"`
	RealTime        *RealTimeConfig       `json:"real_time" yaml:"real_time"`
	Health          *HealthConfig         `json:"health" yaml:"health"`
}

// AlertRule defines a configurable alert rule
type AlertRule struct {
	ID                   string                 `json:"id" yaml:"id"`
	Name                 string                 `json:"name" yaml:"name"`
	Condition            string                 `json:"condition" yaml:"condition"`
	Severity             string                 `json:"severity" yaml:"severity"`
	Escalation           []EscalationStep       `json:"escalation" yaml:"escalation"`
	NotificationChannels []NotificationChannel  `json:"notification_channels" yaml:"notification_channels"`
	Cooldown             time.Duration          `json:"cooldown" yaml:"cooldown"`
	Enabled              bool                   `json:"enabled" yaml:"enabled"`
	Metadata             map[string]interface{} `json:"metadata" yaml:"metadata"`
}

// EscalationStep defines alert escalation behavior
type EscalationStep struct {
	Delay      time.Duration `json:"delay" yaml:"delay"`
	Recipients []string      `json:"recipients" yaml:"recipients"`
	Actions    []string      `json:"actions" yaml:"actions"`
	Level      int           `json:"level" yaml:"level"`
}

// NotificationChannel defines notification delivery methods
type NotificationChannel struct {
	Type     string                 `json:"type" yaml:"type"`
	Config   map[string]interface{} `json:"config" yaml:"config"`
	Enabled  bool                   `json:"enabled" yaml:"enabled"`
	Priority int                    `json:"priority" yaml:"priority"`
}

// NotificationConfig holds notification system configuration
type NotificationConfig struct {
	DefaultChannels []string       `json:"default_channels" yaml:"default_channels"`
	Email           *EmailConfig   `json:"email" yaml:"email"`
	Slack           *SlackConfig   `json:"slack" yaml:"slack"`
	Webhook         *WebhookConfig `json:"webhook" yaml:"webhook"`
	RetryAttempts   int            `json:"retry_attempts" yaml:"retry_attempts"`
	RetryDelay      time.Duration  `json:"retry_delay" yaml:"retry_delay"`
}

// EmailConfig holds email notification settings
type EmailConfig struct {
	SMTPHost    string   `json:"smtp_host" yaml:"smtp_host"`
	SMTPPort    int      `json:"smtp_port" yaml:"smtp_port"`
	Username    string   `json:"username" yaml:"username"`
	Password    string   `json:"password" yaml:"password"`
	FromAddress string   `json:"from_address" yaml:"from_address"`
	ToAddresses []string `json:"to_addresses" yaml:"to_addresses"`
	UseTLS      bool     `json:"use_tls" yaml:"use_tls"`
}

// SlackConfig holds Slack notification settings
type SlackConfig struct {
	WebhookURL string `json:"webhook_url" yaml:"webhook_url"`
	Channel    string `json:"channel" yaml:"channel"`
	Username   string `json:"username" yaml:"username"`
	IconEmoji  string `json:"icon_emoji" yaml:"icon_emoji"`
}

// WebhookConfig holds webhook notification settings
type WebhookConfig struct {
	URL          string            `json:"url" yaml:"url"`
	Method       string            `json:"method" yaml:"method"`
	Headers      map[string]string `json:"headers" yaml:"headers"`
	Timeout      time.Duration     `json:"timeout" yaml:"timeout"`
	RetryOnError bool              `json:"retry_on_error" yaml:"retry_on_error"`
}

// RealTimeConfig holds real-time monitoring configuration
type RealTimeConfig struct {
	WebSocketPath     string        `json:"websocket_path" yaml:"websocket_path"`
	BroadcastInterval time.Duration `json:"broadcast_interval" yaml:"broadcast_interval"`
	MaxConnections    int           `json:"max_connections" yaml:"max_connections"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval" yaml:"heartbeat_interval"`
	EnableCompression bool          `json:"enable_compression" yaml:"enable_compression"`
}

// HealthConfig holds health monitoring configuration
type HealthConfig struct {
	CheckInterval   time.Duration `json:"check_interval" yaml:"check_interval"`
	Timeout         time.Duration `json:"timeout" yaml:"timeout"`
	MaxFailures     int           `json:"max_failures" yaml:"max_failures"`
	EnableMetrics   bool          `json:"enable_metrics" yaml:"enable_metrics"`
	EnableAlerts    bool          `json:"enable_alerts" yaml:"enable_alerts"`
	HealthCheckPath string        `json:"health_check_path" yaml:"health_check_path"`
}

// ConfigManager handles dynamic configuration management
type ConfigManager struct {
	configPath  string
	config      *MonitorConfig
	mu          sync.RWMutex
	logger      *zap.Logger
	watcher     *ConfigWatcher
	subscribers []ConfigSubscriber
	stopChan    chan struct{}
	isRunning   bool
}

// ConfigSubscriber interface for components that need config updates
type ConfigSubscriber interface {
	OnConfigUpdate(config *MonitorConfig) error
}

// NewConfigManager creates a new configuration manager
func NewConfigManager(configPath string, logger *zap.Logger) *ConfigManager {
	return &ConfigManager{
		configPath:  configPath,
		logger:      logger,
		subscribers: make([]ConfigSubscriber, 0),
		stopChan:    make(chan struct{}),
	}
}

// Start begins configuration monitoring
func (cm *ConfigManager) Start(ctx context.Context) error {
	if cm.isRunning {
		return nil
	}

	cm.isRunning = true
	cm.logger.Info("Starting configuration manager")

	// Load initial configuration
	if err := cm.loadConfig(); err != nil {
		cm.logger.Error("Failed to load initial configuration", zap.Error(err))
		return err
	}

	// Start file watcher
	cm.watcher = NewConfigWatcher(cm.configPath, cm.logger)
	if err := cm.watcher.Start(ctx, cm.onConfigChange); err != nil {
		cm.logger.Error("Failed to start config watcher", zap.Error(err))
		return err
	}

	cm.logger.Info("Configuration manager started successfully")
	return nil
}

// Stop stops configuration monitoring
func (cm *ConfigManager) Stop() {
	if !cm.isRunning {
		return
	}

	cm.logger.Info("Stopping configuration manager")
	close(cm.stopChan)

	if cm.watcher != nil {
		cm.watcher.Stop()
	}

	cm.isRunning = false
}

// loadConfig loads configuration from file
func (cm *ConfigManager) loadConfig() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check if config file exists
	if _, err := os.Stat(cm.configPath); os.IsNotExist(err) {
		cm.logger.Warn("Configuration file not found, using defaults", zap.String("path", cm.configPath))
		cm.config = cm.getDefaultConfig()
		return nil
	}

	// Read and parse config file
	data, err := ioutil.ReadFile(cm.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Determine file type and parse accordingly
	ext := filepath.Ext(cm.configPath)
	var config MonitorConfig

	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse JSON config: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse YAML config: %w", err)
		}
	default:
		return fmt.Errorf("unsupported config file format: %s", ext)
	}

	// Validate configuration
	if err := cm.validateConfig(&config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	cm.config = &config
	cm.logger.Info("Configuration loaded successfully", zap.String("path", cm.configPath))
	return nil
}

// getDefaultConfig returns default configuration
func (cm *ConfigManager) getDefaultConfig() *MonitorConfig {
	return &MonitorConfig{
		Thresholds: &AlertThresholds{
			HighFailureRate:    80.0,
			LowSuccessRate:     60.0,
			HighQueueSize:      10000,
			ProcessingDelay:    5.0,
			DatabaseErrors:     10,
			ServiceUnavailable: false,
		},
		SyncInterval:    30 * time.Second,
		RetentionPeriod: 24 * time.Hour,
		AlertRules:      []*AlertRule{},
		Notification: &NotificationConfig{
			DefaultChannels: []string{"log"},
			RetryAttempts:   3,
			RetryDelay:      5 * time.Second,
		},
		CircuitBreaker: &CircuitBreakerConfig{
			Name:         "default",
			MaxFailures:  3,
			Timeout:      30 * time.Second,
			ResetTimeout: 60 * time.Second,
		},
		RealTime: &RealTimeConfig{
			WebSocketPath:     "/api/v1/subscription-external/monitoring/ws",
			BroadcastInterval: 5 * time.Second,
			MaxConnections:    100,
			HeartbeatInterval: 30 * time.Second,
			EnableCompression: true,
		},
		Health: &HealthConfig{
			CheckInterval:   30 * time.Second,
			Timeout:         10 * time.Second,
			MaxFailures:     3,
			EnableMetrics:   true,
			EnableAlerts:    true,
			HealthCheckPath: "/health",
		},
	}
}

// validateConfig validates configuration values
func (cm *ConfigManager) validateConfig(config *MonitorConfig) error {
	if config.SyncInterval < time.Second {
		return fmt.Errorf("sync_interval must be at least 1 second")
	}

	if config.RetentionPeriod < time.Hour {
		return fmt.Errorf("retention_period must be at least 1 hour")
	}

	if config.RealTime != nil {
		if config.RealTime.BroadcastInterval < time.Second {
			return fmt.Errorf("broadcast_interval must be at least 1 second")
		}
	}

	if config.Health != nil {
		if config.Health.CheckInterval < time.Second {
			return fmt.Errorf("health_check_interval must be at least 1 second")
		}
	}

	return nil
}

// onConfigChange handles configuration file changes
func (cm *ConfigManager) onConfigChange() {
	cm.logger.Info("Configuration file changed, reloading...")

	if err := cm.loadConfig(); err != nil {
		cm.logger.Error("Failed to reload configuration", zap.Error(err))
		return
	}

	// Notify subscribers
	cm.notifySubscribers()
}

// notifySubscribers notifies all subscribers of configuration changes
func (cm *ConfigManager) notifySubscribers() {
	cm.mu.RLock()
	subscribers := make([]ConfigSubscriber, len(cm.subscribers))
	copy(subscribers, cm.subscribers)
	cm.mu.RUnlock()

	for _, subscriber := range subscribers {
		if err := subscriber.OnConfigUpdate(cm.config); err != nil {
			cm.logger.Error("Failed to notify subscriber", zap.Error(err))
		}
	}
}

// Subscribe adds a configuration subscriber
func (cm *ConfigManager) Subscribe(subscriber ConfigSubscriber) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.subscribers = append(cm.subscribers, subscriber)
}

// Unsubscribe removes a configuration subscriber
func (cm *ConfigManager) Unsubscribe(subscriber ConfigSubscriber) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for i, sub := range cm.subscribers {
		if sub == subscriber {
			cm.subscribers = append(cm.subscribers[:i], cm.subscribers[i+1:]...)
			break
		}
	}
}

// GetConfig returns the current configuration
func (cm *ConfigManager) GetConfig() *MonitorConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.config
}

// UpdateConfig updates configuration dynamically
func (cm *ConfigManager) UpdateConfig(updates map[string]interface{}) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Create a copy of current config
	configCopy := *cm.config

	// Apply updates
	for key, value := range updates {
		if err := cm.applyConfigUpdate(&configCopy, key, value); err != nil {
			return fmt.Errorf("failed to apply update for %s: %w", key, err)
		}
	}

	// Validate updated configuration
	if err := cm.validateConfig(&configCopy); err != nil {
		return fmt.Errorf("updated configuration is invalid: %w", err)
	}

	// Update configuration
	cm.config = &configCopy

	// Notify subscribers
	go cm.notifySubscribers()

	cm.logger.Info("Configuration updated successfully")
	return nil
}

// applyConfigUpdate applies a single configuration update
func (cm *ConfigManager) applyConfigUpdate(config *MonitorConfig, key string, value interface{}) error {
	// This is a simplified implementation - in production, you'd want more sophisticated
	// field mapping and validation
	switch key {
	case "sync_interval":
		if duration, ok := value.(time.Duration); ok {
			config.SyncInterval = duration
		} else if str, ok := value.(string); ok {
			if duration, err := time.ParseDuration(str); err == nil {
				config.SyncInterval = duration
			} else {
				return fmt.Errorf("invalid duration format: %s", str)
			}
		}
	case "thresholds":
		if thresholds, ok := value.(*AlertThresholds); ok {
			config.Thresholds = thresholds
		}
	default:
		return fmt.Errorf("unknown configuration key: %s", key)
	}

	return nil
}

// SaveConfig saves current configuration to file
func (cm *ConfigManager) SaveConfig() error {
	cm.mu.RLock()
	config := cm.config
	cm.mu.RUnlock()

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := ioutil.WriteFile(cm.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	cm.logger.Info("Configuration saved successfully", zap.String("path", cm.configPath))
	return nil
}
