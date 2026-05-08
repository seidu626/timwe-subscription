package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/postback-dispatcher/internal/domain"
	"github.com/seidu626/subscription-manager/postback-dispatcher/internal/repository"
	"github.com/sony/gobreaker"
	"go.uber.org/zap"
	_ "github.com/lib/pq"
)

// PostbackDispatcher handles async postback delivery
type PostbackDispatcher struct {
	postbackRepo      *repository.PostbackRepository
	httpClient        *http.Client
	circuitBreaker    *gobreaker.CircuitBreaker
	logger            *zap.Logger
	batchSize         int
	pollInterval      time.Duration
	useSkipLocked     bool // Use FOR UPDATE SKIP LOCKED for safe horizontal scaling
}

// DispatcherConfig holds configurable dispatcher settings
type DispatcherConfig struct {
	BatchSize       int
	PollInterval    time.Duration
	HTTPTimeout     time.Duration
	UseSkipLocked   bool
	CBMaxRequests   uint32
	CBTimeout       time.Duration
	CBInterval      time.Duration
	CBFailThreshold uint32
}

// DefaultDispatcherConfig returns default configuration
func DefaultDispatcherConfig() *DispatcherConfig {
	return &DispatcherConfig{
		BatchSize:       10,
		PollInterval:    5 * time.Second,
		HTTPTimeout:     30 * time.Second,
		UseSkipLocked:   true, // Default to safe mode for production
		CBMaxRequests:   100,
		CBTimeout:       30 * time.Second,
		CBInterval:      60 * time.Second,
		CBFailThreshold: 10,
	}
}

// LoadDispatcherConfigFromEnv loads configuration from environment variables
func LoadDispatcherConfigFromEnv() *DispatcherConfig {
	cfg := DefaultDispatcherConfig()

	if v := os.Getenv("DISPATCHER_BATCH_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.BatchSize = n
		}
	}

	if v := os.Getenv("DISPATCHER_POLL_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.PollInterval = d
		}
	}

	if v := os.Getenv("DISPATCHER_HTTP_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.HTTPTimeout = d
		}
	}

	if v := os.Getenv("DISPATCHER_USE_SKIP_LOCKED"); v != "" {
		cfg.UseSkipLocked = v == "true" || v == "1"
	}

	return cfg
}

// NewPostbackDispatcher creates a new postback dispatcher
func NewPostbackDispatcher(
	postbackRepo *repository.PostbackRepository,
	logger *zap.Logger,
) *PostbackDispatcher {
	return NewPostbackDispatcherWithConfig(postbackRepo, logger, LoadDispatcherConfigFromEnv())
}

// NewPostbackDispatcherWithConfig creates a dispatcher with explicit configuration
func NewPostbackDispatcherWithConfig(
	postbackRepo *repository.PostbackRepository,
	logger *zap.Logger,
	cfg *DispatcherConfig,
) *PostbackDispatcher {
	// Create HTTP client with timeout
	httpClient := &http.Client{
		Timeout: cfg.HTTPTimeout,
	}

	// Create circuit breaker
	cbSettings := gobreaker.Settings{
		Name:        "PostbackDispatcher",
		MaxRequests: cfg.CBMaxRequests,
		Interval:    cfg.CBInterval,
		Timeout:     cfg.CBTimeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= cfg.CBFailThreshold
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			logger.Info("Circuit breaker state changed",
				zap.String("name", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()),
			)
		},
	}

	return &PostbackDispatcher{
		postbackRepo:   postbackRepo,
		httpClient:     httpClient,
		circuitBreaker: gobreaker.NewCircuitBreaker(cbSettings),
		logger:         logger,
		batchSize:      cfg.BatchSize,
		pollInterval:   cfg.PollInterval,
		useSkipLocked:  cfg.UseSkipLocked,
	}
}

// Run starts the dispatcher loop
func (d *PostbackDispatcher) Run(ctx context.Context) error {
	d.logger.Info("Starting postback dispatcher",
		zap.Int("batch_size", d.batchSize),
		zap.Duration("poll_interval", d.pollInterval),
		zap.Bool("use_skip_locked", d.useSkipLocked),
	)

	ticker := time.NewTicker(d.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.logger.Info("Postback dispatcher stopping")
			return ctx.Err()
		case <-ticker.C:
			if err := d.processBatch(ctx); err != nil {
				d.logger.Error("Failed to process batch", zap.Error(err))
			}
		}
	}
}

// processBatch processes a batch of pending postbacks
func (d *PostbackDispatcher) processBatch(ctx context.Context) error {
	var postbacks []*domain.PostbackOutbox
	var err error

	// Use ClaimPendingPostbacks for safe horizontal scaling (default)
	// This uses FOR UPDATE SKIP LOCKED to prevent duplicate processing
	if d.useSkipLocked {
		postbacks, err = d.postbackRepo.ClaimPendingPostbacks(d.batchSize)
	} else {
		// Legacy mode: not safe for multiple replicas
		postbacks, err = d.postbackRepo.GetPendingPostbacks(d.batchSize)
	}

	if err != nil {
		return fmt.Errorf("failed to get pending postbacks: %w", err)
	}

	if len(postbacks) == 0 {
		return nil // No work to do
	}

	d.logger.Info("Processing postback batch", zap.Int("count", len(postbacks)))

	for _, pb := range postbacks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := d.processPostback(ctx, pb); err != nil {
				d.logger.Error("Failed to process postback",
					zap.String("id", pb.ID.String()),
					zap.Error(err),
				)
			}
		}
	}

	return nil
}

// processPostback processes a single postback
func (d *PostbackDispatcher) processPostback(ctx context.Context, pb *domain.PostbackOutbox) error {
	// When using SKIP LOCKED, status is already set to PROCESSING by ClaimPendingPostbacks
	// For legacy mode, update status here
	if !d.useSkipLocked {
		if err := d.postbackRepo.UpdateStatus(pb.ID, domain.PostbackStatusProcessing, nil); err != nil {
			return fmt.Errorf("failed to update status: %w", err)
		}
	}

	// Increment attempt count
	nextRetry := d.calculateNextRetry(pb.AttemptCount + 1)
	if err := d.postbackRepo.IncrementAttempt(pb.ID, &nextRetry); err != nil {
		return fmt.Errorf("failed to increment attempt: %w", err)
	}

	// Build HTTP request
	req, err := d.buildRequest(pb)
	if err != nil {
		d.logger.Error("Failed to build request", zap.Error(err))
		d.markFailed(pb, fmt.Sprintf("build error: %v", err))
		return err
	}

	// Send postback with circuit breaker
	startTime := time.Now()
	var resp *http.Response
	var sendErr error

	_, err = d.circuitBreaker.Execute(func() (interface{}, error) {
		resp, sendErr = d.httpClient.Do(req)
		return nil, sendErr
	})

	duration := time.Since(startTime)
	durationMs := int(duration.Milliseconds())

	// Log attempt
	attempt := &domain.PostbackAttempt{
		ID:            uuid.New(),
		OutboxID:      pb.ID,
		AttemptNumber: pb.AttemptCount + 1,
		DurationMs:    &durationMs,
		CreatedAt:     time.Now(),
	}

	if err != nil {
		errMsg := err.Error()
		attempt.ErrorMessage = &errMsg
		d.logger.Error("Postback failed",
			zap.String("id", pb.ID.String()),
			zap.String("provider", pb.Provider),
			zap.Error(err),
		)
		d.logAttempt(attempt)

		// Check if we should retry
		if pb.AttemptCount+1 >= pb.MaxAttempts {
			d.markFailed(pb, "max attempts reached")
		} else {
			// Will retry later based on next_retry_at
			d.postbackRepo.UpdateStatus(pb.ID, domain.PostbackStatusPending, &nextRetry)
		}
		return err
	}

	defer resp.Body.Close()

	// Read response body
	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)
	status := resp.StatusCode

	attempt.HTTPStatus = &status
	attempt.ResponseBody = &bodyStr

	d.logAttempt(attempt)

	// Check if successful (2xx status codes)
	if status >= 200 && status < 300 {
		d.logger.Info("Postback succeeded",
			zap.String("id", pb.ID.String()),
			zap.String("provider", pb.Provider),
			zap.Int("status", status),
		)
		d.postbackRepo.UpdateStatus(pb.ID, domain.PostbackStatusSuccess, nil)
		return nil
	}

	// Non-2xx response
	err = fmt.Errorf("unexpected status code: %d", status)
	errMsg := err.Error()
	attempt.ErrorMessage = &errMsg
	d.logAttempt(attempt)

	if pb.AttemptCount+1 >= pb.MaxAttempts {
		d.markFailed(pb, fmt.Sprintf("status code: %d", status))
	} else {
		d.postbackRepo.UpdateStatus(pb.ID, domain.PostbackStatusPending, &nextRetry)
	}

	return err
}

// buildRequest builds an HTTP request from a postback
func (d *PostbackDispatcher) buildRequest(pb *domain.PostbackOutbox) (*http.Request, error) {
	var body io.Reader
	if pb.Body != nil && *pb.Body != "" {
		body = strings.NewReader(*pb.Body)
	}

	req, err := http.NewRequest(pb.HTTPMethod, pb.URLTemplateRendered, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Parse and set headers
	if pb.Headers != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(pb.Headers), &headers); err == nil {
			for k, v := range headers {
				req.Header.Set(k, v)
			}
		}
	}

	// Set default content type if POST/PUT and not set
	if (pb.HTTPMethod == "POST" || pb.HTTPMethod == "PUT") && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}

// calculateNextRetry calculates the next retry time using exponential backoff
func (d *PostbackDispatcher) calculateNextRetry(attemptCount int) time.Time {
	// Exponential backoff: 2^attemptCount seconds, max 1 hour
	backoffSeconds := 1 << uint(attemptCount)
	if backoffSeconds > 3600 {
		backoffSeconds = 3600
	}
	return time.Now().Add(time.Duration(backoffSeconds) * time.Second)
}

// markFailed marks a postback as failed and moves to DLQ
func (d *PostbackDispatcher) markFailed(pb *domain.PostbackOutbox, reason string) {
	d.logger.Warn("Postback moved to DLQ",
		zap.String("id", pb.ID.String()),
		zap.String("provider", pb.Provider),
		zap.String("reason", reason),
		zap.Int("attempts", pb.AttemptCount),
	)
	d.postbackRepo.UpdateStatus(pb.ID, domain.PostbackStatusDLQ, nil)
}

// logAttempt logs a postback attempt
func (d *PostbackDispatcher) logAttempt(attempt *domain.PostbackAttempt) {
	if err := d.postbackRepo.CreateAttempt(attempt); err != nil {
		d.logger.Error("Failed to log attempt", zap.Error(err))
	}
}

func main() {
	// Initialize logger
	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()

	// Load configuration
	_ = config.InitConfig(logger, ".", []string{"config.yaml"})

	// Connect to database
	connStr := config.GetDBConnectionString()
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	logger.Info("Database connection established")

	// Initialize repository
	postbackRepo := repository.NewPostbackRepository(db, logger)

	// Create dispatcher
	dispatcher := NewPostbackDispatcher(postbackRepo, logger)

	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start dispatcher
	go func() {
		if err := dispatcher.Run(ctx); err != nil && err != context.Canceled {
			logger.Fatal("Dispatcher error", zap.Error(err))
		}
	}()

	logger.Info("Postback dispatcher started")

	<-quit
	logger.Info("Shutting down postback dispatcher...")
	cancel()

	// Give some time for graceful shutdown
	time.Sleep(2 * time.Second)
	logger.Info("Postback dispatcher stopped")
}
