package domain

import (
	"time"
)

// RenewalStrategy defines how renewals are processed
type RenewalStrategy string

const (
	StrategyOptOutOptIn  RenewalStrategy = "opt_out_opt_in"
	StrategyDirectCharge RenewalStrategy = "direct_charge" // Not working
)

// ChurnAction defines what to do with a subscription
type ChurnAction string

const (
	ActionAttemptRenewal ChurnAction = "attempt_renewal"
	ActionChurn          ChurnAction = "churn"
	ActionNoAction       ChurnAction = "no_action"
	ActionGracePeriod    ChurnAction = "grace_period"
)

// Subscription result constants from TIMWE API responses
const (
	SubscriptionResultOptinAlreadyActive      = "OPTIN_ALREADY_ACTIVE"
	SubscriptionResultOptinActiveWaitCharging = "OPTIN_ACTIVE_WAIT_CHARGING"
)

// RenewalCycle tracks an opt-out/opt-in renewal attempt
// @Description Tracks each step of the opt-out/opt-in renewal process
type RenewalCycle struct {
	ID             int64      `json:"id" db:"id"`
	SubscriptionID int64      `json:"subscription_id" db:"subscription_id"`
	MSISDN         string     `json:"msisdn" db:"msisdn"`
	ProductID      string     `json:"product_id" db:"product_id"`
	CycleNumber    int        `json:"cycle_number" db:"cycle_number"`
	OptOutTime     *time.Time `json:"opt_out_time" db:"opt_out_time"`
	OptOutStatus   string     `json:"opt_out_status" db:"opt_out_status"`
	OptOutResponse string     `json:"opt_out_response" db:"opt_out_response"`
	OptInTime      *time.Time `json:"opt_in_time" db:"opt_in_time"`
	OptInStatus    string     `json:"opt_in_status" db:"opt_in_status"`
	OptInResponse  string     `json:"opt_in_response" db:"opt_in_response"`
	BillingStatus  string     `json:"billing_status" db:"billing_status"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}

// ChurnPolicy defines when to churn subscriptions
type ChurnPolicy struct {
	MaxHoursWithoutPayment int  `json:"max_hours_without_payment" yaml:"max_hours_without_payment"`
	MaxRenewalAttempts     int  `json:"max_renewal_attempts" yaml:"max_renewal_attempts"`
	RetryIntervalHours     int  `json:"retry_interval_hours" yaml:"retry_interval_hours"`
	GracePeriodHours       int  `json:"grace_period_hours" yaml:"grace_period_hours"`
	SafeMode               bool `json:"safe_mode" yaml:"safe_mode"`
}

// ChurnRecord tracks churned subscriptions
type ChurnRecord struct {
	ID                   int64      `json:"id" db:"id"`
	SubscriptionID       int64      `json:"subscription_id" db:"subscription_id"`
	MSISDN               string     `json:"msisdn" db:"msisdn"`
	ProductID            string     `json:"product_id" db:"product_id"`
	Reason               string     `json:"reason" db:"reason"`
	ChurnedAt            time.Time  `json:"churned_at" db:"churned_at"`
	LastPaymentDate      *time.Time `json:"last_payment_date" db:"last_payment_date"`
	HoursWithoutPayment  int        `json:"hours_without_payment" db:"hours_without_payment"`
	TotalRenewalAttempts int        `json:"total_renewal_attempts" db:"total_renewal_attempts"`
	CreatedAt            time.Time  `json:"created_at" db:"created_at"`
}

// PriorityRetryQueue tracks failed opt-ins that need immediate retry
// @Description Manages failed opt-ins that require immediate retry with exponential backoff
type PriorityRetryQueue struct {
	ID            int64      `json:"id" db:"id"`
	MSISDN        string     `json:"msisdn" db:"msisdn"`
	ProductID     string     `json:"product_id" db:"product_id"`
	Reason        string     `json:"reason" db:"reason"`
	Priority      int        `json:"priority" db:"priority"`
	RetryCount    int        `json:"retry_count" db:"retry_count"`
	NextRetryAt   *time.Time `json:"next_retry_at" db:"next_retry_at"`
	LastAttemptAt *time.Time `json:"last_attempt_at" db:"last_attempt_at"`
	Status        string     `json:"status" db:"status"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
}

// RenewalConfig holds renewal configuration
type RenewalConfig struct {
	Strategy    RenewalStrategy `json:"strategy" yaml:"strategy"`
	Enabled     bool            `json:"enabled" yaml:"enabled"`
	ChurnPolicy ChurnPolicy     `json:"churn_policy" yaml:"churn_policy"`

	OptOutOptIn struct {
		WaitBetweenMs int `json:"wait_between_ms" yaml:"wait_between_ms"`
		BatchSize     int `json:"batch_size" yaml:"batch_size"`
		MaxConcurrent int `json:"max_concurrent" yaml:"max_concurrent"`
		RateLimitMs   int `json:"rate_limit_ms" yaml:"rate_limit_ms"`
		BatchDelayMs  int `json:"batch_delay_ms" yaml:"batch_delay_ms"`
	} `json:"opt_out_opt_in" yaml:"opt_out_opt_in"`

	Worker struct {
		Enabled           bool          `json:"enabled" yaml:"enabled"`
		DailyRunTime      string        `json:"daily_run_time" yaml:"daily_run_time"`
		Timezone          string        `json:"timezone" yaml:"timezone"`
		TimeoutPerRenewal time.Duration `json:"timeout_per_renewal" yaml:"timeout_per_renewal"`
		MaxRetries        int           `json:"max_retries" yaml:"max_retries"`
	} `json:"worker" yaml:"worker"`

	Monitoring struct {
		AlertOnFailureRate float64 `json:"alert_on_failure_rate" yaml:"alert_on_failure_rate"`
		AlertOnChurnRate   float64 `json:"alert_on_churn_rate" yaml:"alert_on_churn_rate"`
		MetricsPort        int     `json:"metrics_port" yaml:"metrics_port"`
	} `json:"monitoring" yaml:"monitoring"`
}

// SubscriptionWithRenewalInfo extends Subscription with renewal tracking
type SubscriptionWithRenewalInfo struct {
	*Subscription
	RenewalStatus              string     `json:"renewal_status" db:"renewal_status"`
	LastRenewalAttempt         *time.Time `json:"last_renewal_attempt" db:"last_renewal_attempt"`
	TotalRenewalAttempts       int        `json:"total_renewal_attempts" db:"total_renewal_attempts"`
	LastSuccessfulPayment      *time.Time `json:"last_successful_payment" db:"last_successful_payment"`
	ConsecutivePaymentFailures int        `json:"consecutive_payment_failures" db:"consecutive_payment_failures"`
}

// RenewalMetrics tracks renewal performance
// @Description Performance metrics for the renewal system
type RenewalMetrics struct {
	TotalProcessed       int64     `json:"total_processed"`
	SuccessfulRenewals   int64     `json:"successful_renewals"`
	FailedRenewals       int64     `json:"failed_renewals"`
	ChurnedSubscriptions int64     `json:"churned_subscriptions"`
	SuccessRate          float64   `json:"success_rate"`
	AverageCycleTime     float64   `json:"average_cycle_time"`
	LastRunTime          time.Time `json:"last_run_time"`
}

// RenewalRequest represents a renewal request
// @Description Request structure for subscription renewal
type RenewalRequest struct {
	MSISDN       string `json:"msisdn"`
	ProductID    string `json:"product_id"`
	EntryChannel string `json:"entry_channel"`
	ForceRenewal bool   `json:"force_renewal"`
	Priority     int    `json:"priority"`
}

// RenewalResponse represents the result of a renewal attempt
// @Description Response structure for renewal operations
type RenewalResponse struct {
	MSISDN        string `json:"msisdn"`
	ProductID     string `json:"product_id"`
	Success       bool   `json:"success"`
	Status        string `json:"status"`
	Message       string `json:"message"`
	CycleID       int64  `json:"cycle_id,omitempty"`
	OptOutStatus  string `json:"opt_out_status,omitempty"`
	OptInStatus   string `json:"opt_in_status,omitempty"`
	BillingStatus string `json:"billing_status,omitempty"`
	Error         string `json:"error,omitempty"`
}
