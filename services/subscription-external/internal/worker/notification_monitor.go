package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"sync"

	"math/rand"

	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/seidu626/subscription-manager/subscription-external/internal/repository"
	"github.com/seidu626/subscription-manager/subscription-external/internal/service"
)

// NotificationMonitorConfig controls scan windows, batch sizes and concurrency
type NotificationMonitorConfig struct {
	BatchSize           int
	MaxInFlightBatches  int
	ScanLookbackDays    int
	RenewalWindowMonths int // include current and previous N-1 months; default 2 => prev+current
	IdleSleep           time.Duration
	LeaseTTL            time.Duration
	RedisKeyPrefix      string
	// Product and entry channel configuration for opt-out processing
	ProductIds          []string // List of product IDs to process for opt-outs
	EntryChannels       []string // List of entry channels to use for opt-ins
	DefaultEntryChannel string   // Default entry channel if none specified

	// Enhanced: Resilience and recovery configuration
	MaxRetries              int           // Maximum retry attempts for failed operations
	InitialBackoff          time.Duration // Initial backoff duration
	MaxBackoff              time.Duration // Maximum backoff duration
	BackoffMultiplier       float64       // Multiplier for exponential backoff
	CircuitBreakerThreshold int           // Number of failures before circuit breaker opens
	CircuitBreakerTimeout   time.Duration // Time to wait before attempting recovery
	GracefulDegradation     bool          // Enable graceful degradation mode
	HealthCheckInterval     time.Duration // Interval for health checks

	// Enhanced: Entry channel filtering configuration
	InvalidEntryChannels []string // List of entry channels to filter out (e.g., "CCTOOL", "INTERNAL")
}

// NotificationMonitor scans notifications and reconciles subscription state.
// It is idempotent and safe to run multiple instances with Redis locks and offsets.
type NotificationMonitor struct {
	logger  *zap.Logger
	repo    repository.SubscriptionRepositoryInterface
	userSvc *service.SubscriptionService
	redis   *redis.Client
	cfg     NotificationMonitorConfig
	ctx     context.Context

	// Enhanced: Resilience and recovery state
	circuitBreakerState    string       // "CLOSED", "OPEN", "HALF_OPEN"
	circuitBreakerFailures int          // Count of consecutive failures
	lastFailureTime        time.Time    // Timestamp of last failure
	healthLastCheck        time.Time    // Last health check timestamp
	consecutiveErrors      int          // Count of consecutive errors
	gracefulMode           bool         // Whether graceful degradation is active
	mu                     sync.RWMutex // Mutex for thread-safe state updates
}

func NewNotificationMonitor(logger *zap.Logger, repo repository.SubscriptionRepositoryInterface, userSvc *service.SubscriptionService, redis *redis.Client, cfg NotificationMonitorConfig) *NotificationMonitor {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 1000
	}
	if cfg.MaxInFlightBatches <= 0 {
		cfg.MaxInFlightBatches = 10
	}
	if cfg.ScanLookbackDays <= 0 {
		cfg.ScanLookbackDays = 60
	}
	if cfg.RenewalWindowMonths <= 0 {
		cfg.RenewalWindowMonths = 2
	}
	if cfg.IdleSleep <= 0 {
		cfg.IdleSleep = 2 * time.Second
	}
	if cfg.LeaseTTL <= 0 {
		cfg.LeaseTTL = 30 * time.Second
	}
	if cfg.RedisKeyPrefix == "" {
		cfg.RedisKeyPrefix = "notifmon"
	}
	// Set default product IDs if none provided
	if len(cfg.ProductIds) == 0 {
		cfg.ProductIds = []string{"8509"} // Default product ID
	}
	// Set default entry channels if none provided
	if len(cfg.EntryChannels) == 0 {
		cfg.EntryChannels = []string{"USSD"} // Default entry channel
	}
	if cfg.DefaultEntryChannel == "" {
		cfg.DefaultEntryChannel = cfg.EntryChannels[0] // Use first channel as default
	}

	// Enhanced: Set default invalid entry channels if none provided
	if len(cfg.InvalidEntryChannels) == 0 {
		cfg.InvalidEntryChannels = []string{
			"CCTOOL",   // Customer care tool
			"INTERNAL", // Internal system
			"ADMIN",    // Administrative tool
			"SYSTEM",   // System-generated
			"BATCH",    // Batch processing
			"API",      // API calls (not user-initiated)
		}
	}

	// Enhanced: Set default resilience values if not provided
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.InitialBackoff <= 0 {
		cfg.InitialBackoff = 1 * time.Second
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 30 * time.Second
	}
	if cfg.BackoffMultiplier <= 0 {
		cfg.BackoffMultiplier = 2.0
	}
	if cfg.CircuitBreakerThreshold <= 0 {
		cfg.CircuitBreakerThreshold = 5
	}
	if cfg.CircuitBreakerTimeout <= 0 {
		cfg.CircuitBreakerTimeout = 60 * time.Second
	}
	if cfg.HealthCheckInterval <= 0 {
		cfg.HealthCheckInterval = 30 * time.Second
	}

	return &NotificationMonitor{
		logger:  logger,
		repo:    repo,
		userSvc: userSvc,
		redis:   redis,
		cfg:     cfg,
		ctx:     context.Background(),

		// Enhanced: Initialize resilience state
		circuitBreakerState:    "CLOSED",
		circuitBreakerFailures: 0,
		lastFailureTime:        time.Time{},
		healthLastCheck:        time.Now(),
		consecutiveErrors:      0,
		gracefulMode:           false,
		mu:                     sync.RWMutex{},
	}
}

// Run starts the monitor loop. It acquires a lease to avoid multi-instance duplication.
func (m *NotificationMonitor) Run() error {
	leaseKey := fmt.Sprintf("%s:lease", m.cfg.RedisKeyPrefix)

	// Enhanced: Start health check goroutine
	go m.healthCheckLoop()

	for {
		// Enhanced: Check circuit breaker state before processing
		if m.isCircuitBreakerOpen() {
			m.logger.Warn("circuit breaker is OPEN, waiting for timeout",
				zap.String("state", m.getCircuitBreakerState()),
				zap.Int("failures", m.getCircuitBreakerFailures()),
				zap.Time("lastFailure", m.getLastFailureTime()))

			// Wait for circuit breaker timeout
			time.Sleep(m.cfg.CircuitBreakerTimeout)
			m.attemptCircuitBreakerRecovery()
			continue
		}

		ok, err := m.tryAcquireLease(leaseKey, m.cfg.LeaseTTL)
		if err != nil {
			m.logger.Error("lease attempt failed", zap.Error(err))
			m.recordError("lease_acquisition")
			time.Sleep(m.cfg.IdleSleep)
			continue
		}
		if !ok {
			// another worker active
			time.Sleep(m.cfg.IdleSleep)
			continue
		}

		// Enhanced: Process with resilience wrapper
		if err := m.processCycleWithResilience(); err != nil {
			m.logger.Error("processCycle error", zap.Error(err))
			m.recordError("process_cycle")
		} else {
			// Reset error count on successful processing
			m.resetConsecutiveErrors()
		}

		// renew lease
		_ = m.redis.Expire(m.ctx, leaseKey, m.cfg.LeaseTTL/2).Err()
	}
}

func (m *NotificationMonitor) tryAcquireLease(key string, ttl time.Duration) (bool, error) {
	res, err := m.redis.SetNX(m.ctx, key, time.Now().Unix(), ttl).Result()
	return res, err
}

func (m *NotificationMonitor) processCycle() error {
	// process USER_OPTOUT
	if err := m.processUserOptout(); err != nil {
		m.logger.Error("processUserOptout failed", zap.Error(err))
	}

	// Enhanced: Process ghost subscriptions (subscriptions without opt-in notifications)
	if err := m.processGhostSubscriptions(); err != nil {
		m.logger.Error("processGhostSubscriptions failed", zap.Error(err))
	}

	// process RENEWAL
	// if err := m.processRenewal(); err != nil {
	// 	m.logger.Error("processRenewal failed", zap.Error(err))
	// }
	return nil
}

func (m *NotificationMonitor) offsetKey(t string) string {
	return fmt.Sprintf("%s:offset:%s", m.cfg.RedisKeyPrefix, t)
}

// processUserOptout logic:
// - scan notifications of type USER_OPTOUT since lookback, paginated by id>
// - for each: mark subscription inactive (upsert if missing), then attempt optin using configured products and entry channels
// - Enhanced: Prevent re-processing of already processed opt-outs using composite key tracking
func (m *NotificationMonitor) processUserOptout() error {
	lastID, _ := m.redis.Get(m.ctx, m.offsetKey("USER_OPTOUT")).Int64()

	cutoff := time.Now().Add(-time.Duration(m.cfg.ScanLookbackDays) * 24 * time.Hour)
	processedCount := 0
	skippedCount := 0
	successCount := 0
	errorCount := 0
	duplicateCount := 0

	for {
		// Enhanced: Fetch only unprocessed notifications to avoid duplicates
		rows, err := m.repo.FetchUnprocessedOptoutNotifications(cutoff, lastID, m.cfg.BatchSize)
		if err != nil {
			incError("USER_OPTOUT", "fetch_notifications")
			return fmt.Errorf("failed to fetch unprocessed optout notifications: %w", err)
		}
		if len(rows) == 0 {
			break
		}

		for _, n := range rows {
			incProcessed("USER_OPTOUT")
			processedCount++

			// Defensive validation
			if n.MSISDN == "" || n.ProductID == 0 {
				m.logger.Warn("skipping notification with empty MSISDN or ProductID",
					zap.String("msisdn", n.MSISDN),
					zap.Int("productId", n.ProductID),
					zap.Int("notificationId", n.ID))
				skippedCount++
				incError("USER_OPTOUT", "invalid_notification")
				continue
			}

			// Check if this product ID is in our configured list
			productIDStr := fmt.Sprintf("%d", n.ProductID)
			if !m.isProductConfigured(productIDStr) {
				m.logger.Debug("skipping notification for unconfigured product",
					zap.String("msisdn", n.MSISDN),
					zap.Int("productId", n.ProductID),
					zap.Strings("configuredProducts", m.cfg.ProductIds))
				skippedCount++
				incError("USER_OPTOUT", "unconfigured_product")
				continue
			}

			// Enhanced: Check if the entry channel is valid for processing
			if !m.isEntryChannelValid(n.EntryChannel) {
				m.logger.Debug("skipping notification with invalid entry channel",
					zap.String("msisdn", n.MSISDN),
					zap.Int("productId", n.ProductID),
					zap.String("invalidEntryChannel", n.EntryChannel),
					zap.Strings("configuredEntryChannels", m.cfg.EntryChannels))
				skippedCount++
				incError("USER_OPTOUT", "invalid_entry_channel")
				continue
			}

			// Enhanced: Check if this opt-out has already been processed using composite key
			processingKey := m.generateOptoutProcessingKey(n.MSISDN, n.ProductID, n.CreatedAt)
			if m.isOptoutAlreadyProcessed(processingKey) {
				m.logger.Debug("skipping already processed opt-out",
					zap.String("msisdn", n.MSISDN),
					zap.Int("productId", n.ProductID),
					zap.Time("createdAt", n.CreatedAt),
					zap.String("processingKey", processingKey))
				duplicateCount++
				continue
			}

			// Enhanced: Check current subscription state to handle opt-outs after opt-ins
			currentStatus, lastOptinTime, err := m.getCurrentSubscriptionState(n.MSISDN, n.ProductID)
			if err != nil {
				m.logger.Error("failed to get current subscription state",
					zap.String("msisdn", n.MSISDN),
					zap.Int("productId", n.ProductID),
					zap.Error(err))
				incError("USER_OPTOUT", "get_subscription_state")
				errorCount++
				m.releaseOptoutProcessing(processingKey)
				continue
			}

			// Enhanced: Skip processing if opt-out is older than the last opt-in
			if lastOptinTime != nil && n.CreatedAt.Before(*lastOptinTime) {
				m.logger.Info("skipping opt-out that occurred before last opt-in",
					zap.String("msisdn", n.MSISDN),
					zap.Int("productId", n.ProductID),
					zap.Time("optoutTime", n.CreatedAt),
					zap.Time("lastOptinTime", *lastOptinTime),
					zap.String("currentStatus", currentStatus))
				// Mark as processed to avoid re-processing
				m.markOptoutAsProcessed(processingKey, true) // Consider it "successfully" handled
				m.releaseOptoutProcessing(processingKey)
				skippedCount++
				continue
			}

			// Enhanced: Handle case where subscription is already inactive
			if currentStatus == "inactive" {
				m.logger.Debug("subscription already inactive, skipping opt-out processing",
					zap.String("msisdn", n.MSISDN),
					zap.Int("productId", n.ProductID),
					zap.String("currentStatus", currentStatus))
				// Mark as processed to avoid re-processing
				m.markOptoutAsProcessed(processingKey, true)
				m.releaseOptoutProcessing(processingKey)
				skippedCount++
				continue
			}

			// Mark as being processed to prevent race conditions
			if !m.markOptoutAsProcessing(processingKey) {
				m.logger.Debug("opt-out already being processed by another instance",
					zap.String("msisdn", n.MSISDN),
					zap.Int("productId", n.ProductID),
					zap.String("processingKey", processingKey))
				duplicateCount++
				continue
			}

			// 1) upsert subscription as inactive
			if err := m.repo.UpsertSubscriptionStatus(n.MSISDN, n.ProductID, "inactive"); err != nil {
				m.logger.Error("upsert inactive failed",
					zap.String("msisdn", n.MSISDN),
					zap.Int("productId", n.ProductID),
					zap.Error(err))
				incError("USER_OPTOUT", "upsert_inactive")
				errorCount++
				// Release processing lock
				m.releaseOptoutProcessing(processingKey)
				continue
			}

			// 2) attempt opt-in using configured entry channels
			optinSuccess := m.attemptOptinWithConfiguredChannels(n.MSISDN, n.ProductID, n.EntryChannel)

			if optinSuccess {
				// 3) mark active on success
				if err := m.repo.UpsertSubscriptionStatus(n.MSISDN, n.ProductID, "active"); err != nil {
					m.logger.Error("failed to mark subscription active after successful optin",
						zap.String("msisdn", n.MSISDN),
						zap.Int("productId", n.ProductID),
						zap.Error(err))
					incError("USER_OPTOUT", "mark_active_failed")
					errorCount++
				} else {
					incOptinSuccess()
					successCount++
					m.logger.Info("successfully processed opt-out notification with opt-in",
						zap.String("msisdn", n.MSISDN),
						zap.Int("productId", n.ProductID),
						zap.String("notificationEntryChannel", n.EntryChannel),
						zap.Strings("configuredEntryChannels", m.cfg.EntryChannels))
				}
			} else {
				m.logger.Warn("optin attempt failed after optout",
					zap.String("msisdn", n.MSISDN),
					zap.Int("productId", n.ProductID),
					zap.String("notificationEntryChannel", n.EntryChannel),
					zap.Strings("configuredEntryChannels", m.cfg.EntryChannels))
				errorCount++
			}

			// Mark opt-out as successfully processed
			m.markOptoutAsProcessed(processingKey, optinSuccess)

			// Release processing lock
			m.releaseOptoutProcessing(processingKey)

			lastID = int64(n.ID)
		}

		// persist offset after batch
		if err := m.redis.Set(m.ctx, m.offsetKey("USER_OPTOUT"), lastID, 24*time.Hour).Err(); err != nil {
			m.logger.Error("failed to persist offset", zap.Error(err))
		}

		// Add small delay between batches to avoid overwhelming the system
		time.Sleep(100 * time.Millisecond)
	}

	// Log batch processing summary
	m.logger.Info("completed USER_OPTOUT processing batch",
		zap.Int("processed", processedCount),
		zap.Int("skipped", skippedCount),
		zap.Int("successful", successCount),
		zap.Int("errors", errorCount),
		zap.Int("duplicates", duplicateCount),
		zap.Int64("lastProcessedID", lastID))

	return nil
}

// isProductConfigured checks if a product ID is in the configured list
func (m *NotificationMonitor) isProductConfigured(productID string) bool {
	for _, configuredID := range m.cfg.ProductIds {
		if configuredID == productID {
			return true
		}
	}
	return false
}

// isEntryChannelValid checks if an entry channel is valid for processing
// This filters out invalid channels like "CCTOOL", "INTERNAL", etc.
func (m *NotificationMonitor) isEntryChannelValid(entryChannel string) bool {
	// Skip empty entry channels
	if entryChannel == "" {
		return false
	}

	// Check against configured invalid entry channels
	for _, invalid := range m.cfg.InvalidEntryChannels {
		if strings.EqualFold(entryChannel, invalid) {
			return false
		}
	}

	return true
}

// attemptOptinWithConfiguredChannels attempts opt-in using configured entry channels
// Returns true if any opt-in attempt succeeds
func (m *NotificationMonitor) attemptOptinWithConfiguredChannels(msisdn string, productID int, originalEntryChannel string) bool {
	productIDStr := fmt.Sprintf("%d", productID)

	// Determine which entry channels to try
	channelsToTry := m.cfg.EntryChannels

	// Enhanced: Log the original entry channel for debugging
	if originalEntryChannel != "" {
		if m.isEntryChannelConfigured(originalEntryChannel) {
			m.logger.Debug("original entry channel is configured, will try it first",
				zap.String("msisdn", msisdn),
				zap.String("productId", productIDStr),
				zap.String("originalEntryChannel", originalEntryChannel))

			// Reorder channels to try original first, then configured channels
			channelsToTry = append([]string{originalEntryChannel}, m.cfg.EntryChannels...)
			// Remove duplicates while preserving order
			channelsToTry = m.removeDuplicateChannels(channelsToTry)
		} else {
			m.logger.Debug("original entry channel is not configured, using only configured channels",
				zap.String("msisdn", msisdn),
				zap.String("productId", productIDStr),
				zap.String("originalEntryChannel", originalEntryChannel),
				zap.Strings("configuredChannels", m.cfg.EntryChannels))
		}
	} else {
		m.logger.Debug("no original entry channel specified, using configured channels",
			zap.String("msisdn", msisdn),
			zap.String("productId", productIDStr),
			zap.Strings("configuredChannels", m.cfg.EntryChannels))
	}

	// Enhanced: Try each configured entry channel with resilience and retry logic
	for _, channel := range channelsToTry {
		// Enhanced: Attempt opt-in with retry and exponential backoff
		if m.attemptOptinWithResilience(msisdn, productIDStr, channel) {
			return true
		}
	}

	// If we get here, all attempts failed
	incError("USER_OPTOUT", "optin_all_channels_failed")
	return false
}

// attemptOptinWithResilience attempts opt-in with comprehensive resilience mechanisms
func (m *NotificationMonitor) attemptOptinWithResilience(msisdn, productIDStr, channel string) bool {
	// Enhanced: Check circuit breaker state before attempting
	if m.isCircuitBreakerOpen() {
		m.logger.Debug("circuit breaker is OPEN, skipping opt-in attempt",
			zap.String("msisdn", msisdn),
			zap.String("productId", productIDStr),
			zap.String("entryChannel", channel))
		return false
	}

	// Enhanced: Implement exponential backoff with retry logic
	backoff := m.cfg.InitialBackoff
	maxBackoff := m.cfg.MaxBackoff

	for attempt := 1; attempt <= m.cfg.MaxRetries; attempt++ {
		// Enhanced: Check if we should continue based on error patterns
		if m.shouldSkipOptinAttempt(msisdn, productIDStr, channel) {
			m.logger.Debug("skipping opt-in attempt due to recent failures",
				zap.String("msisdn", msisdn),
				zap.String("productId", productIDStr),
				zap.String("entryChannel", channel),
				zap.Int("attempt", attempt))
			return false
		}

		// Enhanced: Create opt-in request with timeout context
		optin := &domain.OptinRequest{
			Msisdn:       msisdn,
			EntryChannel: channel,
			ProductIds:   []string{productIDStr},
		}

		// Enhanced: Attempt opt-in with timeout and error handling
		err := m.attemptOptinWithTimeout(optin, 30*time.Second)
		if err == nil {
			// Success! Log and return
			m.logger.Info("optin successful with configured channel",
				zap.String("msisdn", msisdn),
				zap.String("productId", productIDStr),
				zap.String("entryChannel", channel),
				zap.Int("attempt", attempt))

			// Record success to potentially close circuit breaker
			m.recordSuccess()
			return true
		}

		// Enhanced: Analyze error type and handle accordingly
		errorType := m.classifyOptinError(err)
		m.logger.Debug("optin attempt failed with channel",
			zap.String("msisdn", msisdn),
			zap.String("productId", productIDStr),
			zap.String("entryChannel", channel),
			zap.Int("attempt", attempt),
			zap.String("errorType", errorType),
			zap.Error(err))

		// Enhanced: Record error for circuit breaker management
		m.recordError(fmt.Sprintf("optin_%s", errorType))

		// Enhanced: Check if this is a permanent failure that shouldn't be retried
		if m.isPermanentOptinFailure(err) {
			m.logger.Warn("permanent opt-in failure detected, not retrying",
				zap.String("msisdn", msisdn),
				zap.String("productId", productIDStr),
				zap.String("entryChannel", channel),
				zap.String("errorType", errorType),
				zap.Error(err))
			return false
		}

		// Enhanced: Implement exponential backoff with jitter
		if attempt < m.cfg.MaxRetries {
			// Add jitter to prevent thundering herd
			jitter := time.Duration(rand.Intn(100)) * time.Millisecond
			sleepDuration := backoff + jitter

			if sleepDuration > maxBackoff {
				sleepDuration = maxBackoff
			}

			m.logger.Debug("waiting before retry",
				zap.String("msisdn", msisdn),
				zap.String("productId", productIDStr),
				zap.String("entryChannel", channel),
				zap.Int("attempt", attempt),
				zap.Duration("sleepDuration", sleepDuration))

			time.Sleep(sleepDuration)

			// Calculate next backoff
			backoff = time.Duration(float64(backoff) * m.cfg.BackoffMultiplier)
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}

	// Enhanced: Log final failure with comprehensive details
	m.logger.Warn("all opt-in attempts failed after retries",
		zap.String("msisdn", msisdn),
		zap.String("productId", productIDStr),
		zap.String("entryChannel", channel),
		zap.Int("maxRetries", m.cfg.MaxRetries),
		zap.Duration("totalBackoff", backoff))

	return false
}

// attemptOptinWithTimeout attempts opt-in with a timeout context
func (m *NotificationMonitor) attemptOptinWithTimeout(optin *domain.OptinRequest, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(m.ctx, timeout)
	defer cancel()

	// Create a channel for the result
	resultChan := make(chan error, 1)

	go func() {
		resultChan <- m.userSvc.ProcessOptin(optin)
	}()

	// Wait for result or timeout
	select {
	case err := <-resultChan:
		return err
	case <-ctx.Done():
		return fmt.Errorf("opt-in attempt timed out after %v", timeout)
	}
}

// classifyOptinError classifies opt-in errors for better handling
func (m *NotificationMonitor) classifyOptinError(err error) string {
	if err == nil {
		return "none"
	}

	errStr := err.Error()

	// Network-related errors
	if strings.Contains(errStr, "dial tcp") ||
		strings.Contains(errStr, "no route to host") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "timeout") {
		return "network"
	}

	// DNS-related errors
	if strings.Contains(errStr, "lookup") ||
		strings.Contains(errStr, "name resolution") {
		return "dns"
	}

	// Circuit breaker errors
	if strings.Contains(errStr, "circuit breaker") ||
		strings.Contains(errStr, "Circuit breaker classified failure") {
		return "circuit_breaker"
	}

	// Business logic errors
	if strings.Contains(errStr, "MSISDN") ||
		strings.Contains(errStr, "product") ||
		strings.Contains(errStr, "validation") {
		return "business_logic"
	}

	// Default to unknown
	return "unknown"
}

// isPermanentOptinFailure checks if an error indicates a permanent failure
func (m *NotificationMonitor) isPermanentOptinFailure(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Business logic errors that won't be fixed by retrying
	if strings.Contains(errStr, "MSISDN") &&
		(strings.Contains(errStr, "excluded") || strings.Contains(errStr, "invalid")) {
		return true
	}

	// Product configuration errors
	if strings.Contains(errStr, "OPTIN_CONFIG_NOT_FOUND") {
		return true
	}

	return false
}

// shouldSkipOptinAttempt checks if we should skip an opt-in attempt based on recent failures
func (m *NotificationMonitor) shouldSkipOptinAttempt(msisdn, productID, channel string) bool {
	// Check if this specific combination has failed recently
	recentFailureKey := fmt.Sprintf("%s:optin_failure:%s:%s:%s",
		m.cfg.RedisKeyPrefix, msisdn, productID, channel)

	exists, _ := m.redis.Exists(m.ctx, recentFailureKey).Result()
	if exists > 0 {
		return true
	}

	// Check circuit breaker state
	if m.isCircuitBreakerOpen() {
		return true
	}

	return false
}

// isEntryChannelConfigured checks if an entry channel is in the configured list
func (m *NotificationMonitor) isEntryChannelConfigured(channel string) bool {
	for _, configuredChannel := range m.cfg.EntryChannels {
		if configuredChannel == channel {
			return true
		}
	}
	return false
}

// removeDuplicateChannels removes duplicate channels while preserving order
func (m *NotificationMonitor) removeDuplicateChannels(channels []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(channels))

	for _, channel := range channels {
		if !seen[channel] {
			seen[channel] = true
			result = append(result, channel)
		}
	}

	return result
}

// Enhanced opt-out processing tracking methods

// generateOptoutProcessingKey creates a unique key for tracking opt-out processing
// Format: "optout:processed:{msisdn}:{productId}:{createdAtUnix}"
func (m *NotificationMonitor) generateOptoutProcessingKey(msisdn string, productID int, createdAt time.Time) string {
	return fmt.Sprintf("%s:optout:processed:%s:%d:%d",
		m.cfg.RedisKeyPrefix, msisdn, productID, createdAt.Unix())
}

// generateOptoutProcessingLockKey creates a key for preventing concurrent processing
// Format: "optout:processing:{msisdn}:{productId}:{createdAtUnix}"
func (m *NotificationMonitor) generateOptoutProcessingLockKey(msisdn string, productID int, createdAt time.Time) string {
	return fmt.Sprintf("%s:optout:processing:%s:%d:%d",
		m.cfg.RedisKeyPrefix, msisdn, productID, createdAt.Unix())
}

// isOptoutAlreadyProcessed checks if an opt-out has already been processed
func (m *NotificationMonitor) isOptoutAlreadyProcessed(processingKey string) bool {
	exists, err := m.redis.Exists(m.ctx, processingKey).Result()
	if err != nil {
		m.logger.Warn("failed to check if opt-out already processed",
			zap.String("processingKey", processingKey), zap.Error(err))
		return false // Assume not processed on error
	}
	return exists > 0
}

// markOptoutAsProcessing marks an opt-out as being processed to prevent race conditions
// Returns true if successfully marked, false if already being processed
func (m *NotificationMonitor) markOptoutAsProcessing(processingKey string) bool {
	lockKey := strings.Replace(processingKey, ":processed:", ":processing:", 1)

	// Use SET NX with TTL to prevent race conditions
	// TTL of 5 minutes should be enough for processing
	result, err := m.redis.SetNX(m.ctx, lockKey, time.Now().Unix(), 5*time.Minute).Result()
	if err != nil {
		m.logger.Warn("failed to mark opt-out as processing",
			zap.String("processingKey", processingKey), zap.Error(err))
		return false
	}
	return result
}

// markOptoutAsProcessed marks an opt-out as successfully processed
func (m *NotificationMonitor) markOptoutAsProcessed(processingKey string, optinSuccess bool) {
	// Store processing result with metadata
	processingResult := map[string]interface{}{
		"processed_at":  time.Now().Unix(),
		"optin_success": optinSuccess,
		"status":        "completed",
	}

	// Convert to JSON for storage
	if resultJSON, err := json.Marshal(processingResult); err == nil {
		// Store with 30-day TTL to allow for monitoring and debugging
		m.redis.Set(m.ctx, processingKey, resultJSON, 30*24*time.Hour)
	} else {
		m.logger.Warn("failed to marshal processing result",
			zap.String("processingKey", processingKey), zap.Error(err))
	}
}

// releaseOptoutProcessing releases the processing lock for an opt-out
func (m *NotificationMonitor) releaseOptoutProcessing(processingKey string) {
	lockKey := strings.Replace(processingKey, ":processed:", ":processing:", 1)
	m.redis.Del(m.ctx, lockKey)
}

// getOptoutProcessingStats returns statistics about opt-out processing
func (m *NotificationMonitor) getOptoutProcessingStats() map[string]interface{} {
	// Count processed opt-outs
	processedPattern := fmt.Sprintf("%s:optout:processed:*", m.cfg.RedisKeyPrefix)
	processedKeys, _ := m.redis.Keys(m.ctx, processedPattern).Result()

	// Count currently processing opt-outs
	processingPattern := fmt.Sprintf("%s:optout:processing:*", m.cfg.RedisKeyPrefix)
	processingKeys, _ := m.redis.Keys(m.ctx, processingPattern).Result()

	return map[string]interface{}{
		"total_processed":      len(processedKeys),
		"currently_processing": len(processingKeys),
		"processing_pattern":   processingPattern,
		"processed_pattern":    processedPattern,
	}
}

// getCurrentSubscriptionState retrieves the current subscription status and last opt-in time
// Returns: (currentStatus, lastOptinTime, error)
func (m *NotificationMonitor) getCurrentSubscriptionState(msisdn string, productID int) (string, *time.Time, error) {
	// Get current subscription status
	subscription, err := m.repo.GetSubscriptionByMSISDNAndProduct(msisdn, productID)
	if err != nil {
		// If subscription doesn't exist, consider it as "inactive"
		if strings.Contains(err.Error(), "no rows") || strings.Contains(err.Error(), "not found") {
			return "inactive", nil, nil
		}
		return "", nil, fmt.Errorf("failed to get subscription: %w", err)
	}

	// Get last opt-in notification time for this MSISDN + product
	lastOptinTime, err := m.repo.GetLastOptinNotificationTime(msisdn, productID)
	if err != nil {
		// Log error but don't fail - we can still process without this info
		m.logger.Warn("failed to get last opt-in time",
			zap.String("msisdn", msisdn),
			zap.Int("productId", productID),
			zap.Error(err))
	}

	return subscription.Status, lastOptinTime, nil
}

// processRenewal logic:
// - identify subscriptions needing renewal based on missing CHARGE/USER_RENEWED notifications
// - use RenewalService to process renewals via opt-out/opt-in strategy
// - track progress with Redis offset for subscription IDs processed
func (m *NotificationMonitor) processRenewal() error {
	lastProcessedID, _ := m.redis.Get(m.ctx, m.offsetKey("RENEWAL_SUBS")).Int64()

	// Get subscriptions needing renewal using the improved renewal system
	// Use a reasonable threshold (7 days) and batch size
	daysThreshold := 7
	limit := m.cfg.BatchSize

	// We need access to the renewal service, but it's not available in this monitor
	// For now, we'll identify subscriptions that need renewal and delegate to the renewal worker
	// This is a transitional approach until the renewal worker fully takes over

	// Query subscriptions that haven't had charges recently and need renewal
	cutoff := time.Now().Add(-time.Duration(daysThreshold) * 24 * time.Hour)

	for {
		// Get subscriptions that need renewal (those without recent CHARGE/USER_RENEWED)
		rows, err := m.repo.FetchSubscriptionsNeedingRenewal(cutoff, lastProcessedID, limit)
		if err != nil {
			return fmt.Errorf("failed to fetch subscriptions needing renewal: %w", err)
		}
		if len(rows) == 0 {
			return nil
		}

		for _, sub := range rows {
			incProcessed("RENEWAL")

			// Defensive validation
			if sub.MSISDN == "" || sub.ProductID == 0 {
				continue
			}

			// Check if this subscription was recently processed to avoid duplicates
			recentProcessKey := fmt.Sprintf("%s:recent:%s:%d", m.cfg.RedisKeyPrefix, sub.MSISDN, sub.ProductID)
			exists, _ := m.redis.Exists(m.ctx, recentProcessKey).Result()
			if exists > 0 {
				// Skip if processed recently (within last 24 hours)
				continue
			}

			// Mark as recently processed (24 hour TTL)
			_ = m.redis.Set(m.ctx, recentProcessKey, time.Now().Unix(), 24*time.Hour).Err()

			// Ensure subscription record exists and is active
			if err := m.repo.UpsertSubscriptionStatus(sub.MSISDN, sub.ProductID, "active"); err != nil {
				m.logger.Error("upsert active failed for renewal",
					zap.String("msisdn", sub.MSISDN),
					zap.Int("productId", sub.ProductID),
					zap.Error(err))
				incError("RENEWAL", "upsert_active")
				continue
			}

			// Attempt resubscribe using the existing service
			// This will trigger the opt-out/opt-in renewal cycle
			if err := m.userSvc.ResubscribeUser(sub.MSISDN, sub.EntryChannel, []string{fmt.Sprintf("%d", sub.ProductID)}); err != nil {
				m.logger.Warn("resubscribe failed for renewal candidate",
					zap.String("msisdn", sub.MSISDN),
					zap.Int("productId", sub.ProductID),
					zap.String("entryChannel", sub.EntryChannel),
					zap.Error(err))
				incError("RENEWAL", "resubscribe")
			} else {
				incResubscribeSuccess()
				m.logger.Info("renewal resubscribe initiated",
					zap.String("msisdn", sub.MSISDN),
					zap.Int("productId", sub.ProductID))
			}

			lastProcessedID = int64(sub.ID)
		}

		// Persist offset after batch
		_ = m.redis.Set(m.ctx, m.offsetKey("RENEWAL_SUBS"), lastProcessedID, 24*time.Hour).Err()

		// Add small delay between batches to avoid overwhelming the system
		time.Sleep(100 * time.Millisecond)
	}
}

// processGhostSubscriptions logic:
// - identify subscriptions that exist in the database but have no opt-in notifications
// - for each: attempt to re-opt-in using the configured entry channels
// - track progress with Redis offset for subscription IDs processed
func (m *NotificationMonitor) processGhostSubscriptions() error {
	lastProcessedID, _ := m.redis.Get(m.ctx, m.offsetKey("GHOST_SUBS")).Int64()

	// Get subscriptions that exist but have no opt-in notifications
	// Use a reasonable threshold (30 days) and batch size
	daysThreshold := 30
	limit := m.cfg.BatchSize

	// Query subscriptions that haven't had any opt-in notifications in the last 30 days
	cutoff := time.Now().Add(-time.Duration(daysThreshold) * 24 * time.Hour)

	for {
		// Get subscriptions that are "active" but have no opt-in notifications
		rows, err := m.repo.FetchGhostSubscriptions(cutoff, lastProcessedID, limit)
		if err != nil {
			return fmt.Errorf("failed to fetch ghost subscriptions: %w", err)
		}
		if len(rows) == 0 {
			return nil
		}

		for _, sub := range rows {
			incProcessed("GHOST_SUBS")

			// Defensive validation
			if sub.MSISDN == "" || sub.ProductID == 0 {
				continue
			}

			// Check if this subscription was recently processed to avoid duplicates
			recentProcessKey := fmt.Sprintf("%s:recent:%s:%d", m.cfg.RedisKeyPrefix, sub.MSISDN, sub.ProductID)
			exists, _ := m.redis.Exists(m.ctx, recentProcessKey).Result()
			if exists > 0 {
				// Skip if processed recently (within last 24 hours)
				continue
			}

			// Mark as recently processed (24 hour TTL)
			_ = m.redis.Set(m.ctx, recentProcessKey, time.Now().Unix(), 24*time.Hour).Err()

			// Attempt re-opt-in using the configured entry channels
			optinSuccess := m.attemptOptinWithConfiguredChannels(sub.MSISDN, sub.ProductID, sub.EntryChannel)

			if optinSuccess {
				// Mark as active on success
				if err := m.repo.UpsertSubscriptionStatus(sub.MSISDN, sub.ProductID, "active"); err != nil {
					m.logger.Error("failed to mark ghost subscription active after re-optin",
						zap.String("msisdn", sub.MSISDN),
						zap.Int("productId", sub.ProductID),
						zap.Error(err))
					incError("GHOST_SUBS", "mark_active_failed")
				} else {
					incOptinSuccess()
					m.logger.Info("successfully processed ghost subscription re-optin",
						zap.String("msisdn", sub.MSISDN),
						zap.Int("productId", sub.ProductID))
				}
			} else {
				m.logger.Warn("re-optin attempt failed for ghost subscription",
					zap.String("msisdn", sub.MSISDN),
					zap.Int("productId", sub.ProductID),
					zap.String("entryChannel", sub.EntryChannel))
				incError("GHOST_SUBS", "re_optin_failed")
			}

			lastProcessedID = int64(sub.ID)
		}

		// Persist offset after batch
		_ = m.redis.Set(m.ctx, m.offsetKey("GHOST_SUBS"), lastProcessedID, 24*time.Hour).Err()

		// Add small delay between batches to avoid overwhelming the system
		time.Sleep(100 * time.Millisecond)
	}
}

// Repo-facing lightweight DTO to avoid cross-package import cycles from notification service domain
type NotificationRow struct {
	ID           int
	MSISDN       string
	ProductID    int
	EntryChannel string
	CreatedAt    time.Time
	Type         string
}

// Repository extension points used by monitor
// We intentionally extend SubscriptionRepositoryInterface here for cohesion.
// Implementations must be idempotent and index-friendly for large volumes.
type NotificationRepositoryExtensions interface {
	FetchNotificationsWindow(ntype string, since time.Time, afterId int64, limit int) ([]repository.NotificationRow, error)
	FetchSubscriptionsNeedingRenewal(cutoff time.Time, afterId int64, limit int) ([]repository.NotificationRow, error)
	UpsertSubscriptionStatus(msisdn string, productId int, status string) error
	FetchUnprocessedOptoutNotifications(since time.Time, afterId int64, limit int) ([]repository.NotificationRow, error)
	FetchGhostSubscriptions(cutoff time.Time, afterId int64, limit int) ([]repository.NotificationRow, error)
}

// Compile-time assertion to ensure the concrete repo implements extension methods
var _ NotificationRepositoryExtensions = (*repository.SubscriptionRepository)(nil)

// GetConfigurationSummary returns a summary of the current configuration
func (m *NotificationMonitor) GetConfigurationSummary() map[string]interface{} {
	return map[string]interface{}{
		"batch_size":                m.cfg.BatchSize,
		"max_in_flight_batches":     m.cfg.MaxInFlightBatches,
		"scan_lookback_days":        m.cfg.ScanLookbackDays,
		"renewal_window_months":     m.cfg.RenewalWindowMonths,
		"idle_sleep":                m.cfg.IdleSleep.String(),
		"lease_ttl":                 m.cfg.LeaseTTL.String(),
		"redis_key_prefix":          m.cfg.RedisKeyPrefix,
		"product_ids":               m.cfg.ProductIds,
		"entry_channels":            m.cfg.EntryChannels,
		"default_entry_channel":     m.cfg.DefaultEntryChannel,
		"max_retries":               m.cfg.MaxRetries,
		"initial_backoff":           m.cfg.InitialBackoff.String(),
		"max_backoff":               m.cfg.MaxBackoff.String(),
		"backoff_multiplier":        m.cfg.BackoffMultiplier,
		"circuit_breaker_threshold": m.cfg.CircuitBreakerThreshold,
		"circuit_breaker_timeout":   m.cfg.CircuitBreakerTimeout.String(),
		"graceful_degradation":      m.cfg.GracefulDegradation,
		"health_check_interval":     m.cfg.HealthCheckInterval.String(),
		"invalid_entry_channels":    m.cfg.InvalidEntryChannels,
	}
}

// IsProductSupported checks if a product ID is supported by this monitor
func (m *NotificationMonitor) IsProductSupported(productID int) bool {
	productIDStr := fmt.Sprintf("%d", productID)
	return m.isProductConfigured(productIDStr)
}

// GetSupportedProducts returns the list of supported product IDs
func (m *NotificationMonitor) GetSupportedProducts() []string {
	return append([]string{}, m.cfg.ProductIds...)
}

// GetSupportedEntryChannels returns the list of supported entry channels
func (m *NotificationMonitor) GetSupportedEntryChannels() []string {
	return append([]string{}, m.cfg.EntryChannels...)
}

// GetDefaultEntryChannel returns the default entry channel
func (m *NotificationMonitor) GetDefaultEntryChannel() string {
	return m.cfg.DefaultEntryChannel
}

// UpdateConfiguration updates the monitor configuration at runtime
func (m *NotificationMonitor) UpdateConfiguration(newConfig NotificationMonitorConfig) error {
	// Validate the new configuration
	if err := newConfig.ValidateConfig(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Update the configuration
	m.cfg = newConfig

	m.logger.Info("configuration updated",
		zap.Strings("newProductIds", m.cfg.ProductIds),
		zap.Strings("newEntryChannels", m.cfg.EntryChannels),
		zap.String("newDefaultEntryChannel", m.cfg.DefaultEntryChannel))

	return nil
}

// Enhanced resilience and recovery methods

// healthCheckLoop runs periodic health checks to monitor system health
func (m *NotificationMonitor) healthCheckLoop() {
	ticker := time.NewTicker(m.cfg.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.performHealthCheck()
		case <-m.ctx.Done():
			return
		}
	}
}

// performHealthCheck performs a comprehensive health check
func (m *NotificationMonitor) performHealthCheck() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check Redis connectivity
	_, err := m.redis.Ping(m.ctx).Result()
	if err != nil {
		m.logger.Error("health check failed: Redis connectivity issue", zap.Error(err))
		m.recordError("health_check_redis")
	} else {
		m.logger.Debug("health check passed: Redis connectivity OK")
	}

	// Check database connectivity (simple query)
	if err := m.checkDatabaseHealth(); err != nil {
		m.logger.Error("health check failed: Database connectivity issue", zap.Error(err))
		m.recordError("health_check_database")
	} else {
		m.logger.Debug("health check passed: Database connectivity OK")
	}

	// Update health check timestamp
	m.healthLastCheck = time.Now()

	// Check if we should enter graceful degradation mode
	if m.consecutiveErrors >= m.cfg.CircuitBreakerThreshold {
		m.gracefulMode = true
		m.logger.Warn("entering graceful degradation mode due to high error rate",
			zap.Int("consecutiveErrors", m.consecutiveErrors),
			zap.Int("threshold", m.cfg.CircuitBreakerThreshold))
	} else if m.consecutiveErrors == 0 {
		m.gracefulMode = false
	}
}

// checkDatabaseHealth performs a simple database health check
func (m *NotificationMonitor) checkDatabaseHealth() error {
	// Try to fetch a single notification to test connectivity
	_, err := m.repo.FetchNotificationsWindow("USER_OPTOUT", time.Now().Add(-24*time.Hour), 0, 1)
	return err
}

// Circuit breaker management methods

// isCircuitBreakerOpen checks if the circuit breaker is in OPEN state
func (m *NotificationMonitor) isCircuitBreakerOpen() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.circuitBreakerState == "OPEN"
}

// getCircuitBreakerState returns the current circuit breaker state
func (m *NotificationMonitor) getCircuitBreakerState() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.circuitBreakerState
}

// getCircuitBreakerFailures returns the current failure count
func (m *NotificationMonitor) getCircuitBreakerFailures() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.circuitBreakerFailures
}

// getLastFailureTime returns the timestamp of the last failure
func (m *NotificationMonitor) getLastFailureTime() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastFailureTime
}

// recordError records an error and updates circuit breaker state
func (m *NotificationMonitor) recordError(errorType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.consecutiveErrors++
	m.circuitBreakerFailures++
	m.lastFailureTime = time.Now()

	// Check if circuit breaker should open
	if m.circuitBreakerFailures >= m.cfg.CircuitBreakerThreshold {
		m.circuitBreakerState = "OPEN"
		m.logger.Warn("circuit breaker opened due to high failure rate",
			zap.String("errorType", errorType),
			zap.Int("failures", m.circuitBreakerFailures),
			zap.Int("threshold", m.cfg.CircuitBreakerThreshold))
	}

	m.logger.Debug("error recorded",
		zap.String("errorType", errorType),
		zap.Int("consecutiveErrors", m.consecutiveErrors),
		zap.Int("circuitBreakerFailures", m.circuitBreakerFailures),
		zap.String("circuitBreakerState", m.circuitBreakerState))
}

// resetConsecutiveErrors resets the consecutive error count on successful operations
func (m *NotificationMonitor) resetConsecutiveErrors() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.consecutiveErrors > 0 {
		m.logger.Debug("resetting consecutive error count after successful operation",
			zap.Int("previousCount", m.consecutiveErrors))
		m.consecutiveErrors = 0
	}
}

// attemptCircuitBreakerRecovery attempts to recover from circuit breaker OPEN state
func (m *NotificationMonitor) attemptCircuitBreakerRecovery() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.circuitBreakerState == "OPEN" {
		m.circuitBreakerState = "HALF_OPEN"
		m.logger.Info("circuit breaker moved to HALF_OPEN state for recovery attempt")
	}
}

// recordSuccess records a successful operation and updates circuit breaker state
func (m *NotificationMonitor) recordSuccess() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.circuitBreakerState == "HALF_OPEN" {
		m.circuitBreakerState = "CLOSED"
		m.circuitBreakerFailures = 0
		m.logger.Info("circuit breaker closed after successful recovery")
	}
}

// processCycleWithResilience wraps processCycle with resilience mechanisms
func (m *NotificationMonitor) processCycleWithResilience() error {
	// Check if we're in graceful degradation mode
	if m.gracefulMode {
		m.logger.Info("processing cycle in graceful degradation mode",
			zap.Int("consecutiveErrors", m.getCircuitBreakerFailures()))

		// In graceful mode, only process critical operations
		if err := m.processUserOptoutWithResilience(); err != nil {
			return fmt.Errorf("graceful mode processing failed: %w", err)
		}

		// Skip other processing in graceful mode
		return nil
	}

	// Normal processing mode
	if err := m.processCycle(); err != nil {
		return err
	}

	// Record success to potentially close circuit breaker
	m.recordSuccess()
	return nil
}

// processUserOptoutWithResilience processes opt-outs with enhanced resilience
func (m *NotificationMonitor) processUserOptoutWithResilience() error {
	// Implement resilient opt-out processing
	// This could include reduced batch sizes, longer delays, etc.
	return m.processUserOptout()
}

// GetResilienceStatus returns the current resilience and circuit breaker status
func (m *NotificationMonitor) GetResilienceStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"circuit_breaker_state":    m.circuitBreakerState,
		"circuit_breaker_failures": m.circuitBreakerFailures,
		"last_failure_time":        m.lastFailureTime.Format(time.RFC3339),
		"consecutive_errors":       m.consecutiveErrors,
		"graceful_mode":            m.gracefulMode,
		"health_last_check":        m.healthLastCheck.Format(time.RFC3339),
		"is_healthy":               m.isHealthy(),
	}
}

// isHealthy checks if the system is in a healthy state
func (m *NotificationMonitor) isHealthy() bool {
	// System is healthy if:
	// 1. Circuit breaker is not OPEN
	// 2. Consecutive errors are below threshold
	// 3. Recent health checks passed
	return m.circuitBreakerState != "OPEN" &&
		m.consecutiveErrors < m.cfg.CircuitBreakerThreshold &&
		time.Since(m.healthLastCheck) < m.cfg.HealthCheckInterval*2
}
