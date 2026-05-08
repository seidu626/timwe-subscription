package monitoring

import (
	"time"
)

// Config holds monitoring configuration
type Config struct {
	// Monitoring intervals
	MetricsUpdateInterval time.Duration `json:"metrics_update_interval"`
	HealthCheckInterval   time.Duration `json:"health_check_interval"`
	AlertCleanupInterval  time.Duration `json:"alert_cleanup_interval"`

	// Alert thresholds
	Thresholds AlertThresholds `json:"thresholds"`

	// Alert retention
	AlertRetentionHours int `json:"alert_retention_hours"`

	// Notification settings
	EnableEmailAlerts   bool `json:"enable_email_alerts"`
	EnableSlackAlerts   bool `json:"enable_slack_alerts"`
	EnableWebhookAlerts bool `json:"enable_webhook_alerts"`

	// Email configuration
	SMTPHost     string   `json:"smtp_host"`
	SMTPPort     int      `json:"smtp_port"`
	SMTPUsername string   `json:"smtp_username"`
	SMTPPassword string   `json:"smtp_password"`
	FromEmail    string   `json:"from_email"`
	ToEmails     []string `json:"to_emails"`

	// Slack configuration
	SlackWebhookURL string `json:"slack_webhook_url"`
	SlackChannel    string `json:"slack_channel"`

	// Webhook configuration
	WebhookURLs []string `json:"webhook_urls"`

	// Dashboard settings
	DashboardRefreshInterval time.Duration `json:"dashboard_refresh_interval"`
	MaxChartDataPoints       int           `json:"max_chart_data_points"`
}

// DefaultConfig returns default monitoring configuration
func DefaultConfig() *Config {
	return &Config{
		MetricsUpdateInterval:    30 * time.Second,
		HealthCheckInterval:      5 * time.Minute,
		AlertCleanupInterval:     1 * time.Hour,
		AlertRetentionHours:      24,
		EnableEmailAlerts:        false,
		EnableSlackAlerts:        false,
		EnableWebhookAlerts:      false,
		DashboardRefreshInterval: 30 * time.Second,
		MaxChartDataPoints:       100,
		Thresholds: AlertThresholds{
			HighFailureRate:    80.0,
			LowSuccessRate:     60.0,
			HighQueueSize:      10000,
			ProcessingDelay:    5.0,
			DatabaseErrors:     10,
			ServiceUnavailable: false,
		},
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.MetricsUpdateInterval < time.Second {
		c.MetricsUpdateInterval = time.Second
	}
	if c.HealthCheckInterval < time.Minute {
		c.HealthCheckInterval = time.Minute
	}
	if c.AlertRetentionHours < 1 {
		c.AlertRetentionHours = 1
	}
	if c.MaxChartDataPoints < 10 {
		c.MaxChartDataPoints = 10
	}
	return nil
}
