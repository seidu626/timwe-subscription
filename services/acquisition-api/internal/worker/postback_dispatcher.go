package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
	"go.uber.org/zap"
)

const (
	defaultPollInterval = 5 * time.Second
	defaultBatchSize    = 10
	defaultHTTPTimeout  = 10 * time.Second
	defaultRetryBase    = 30 * time.Second
	defaultMaxAttempts  = 5

	// maxResponseBodyLog caps the response body stored per attempt to avoid bloating the DB.
	maxResponseBodyLog = 2048

	// staleCleanupInterval is how often to check for stale PROCESSING records.
	staleCleanupInterval = 60 * time.Second
	// staleThreshold is how long a record can stay in PROCESSING before being reset.
	staleThreshold = 2 * time.Minute
)

// PostbackDispatcherConfig holds tunable parameters for the dispatcher.
type PostbackDispatcherConfig struct {
	PollInterval time.Duration
	BatchSize    int
	HTTPTimeout  time.Duration
	RetryBase    time.Duration
}

// withDefaults returns a copy of the config with zero values replaced by defaults.
func (c PostbackDispatcherConfig) withDefaults() PostbackDispatcherConfig {
	if c.PollInterval <= 0 {
		c.PollInterval = defaultPollInterval
	}
	if c.BatchSize <= 0 {
		c.BatchSize = defaultBatchSize
	}
	if c.HTTPTimeout <= 0 {
		c.HTTPTimeout = defaultHTTPTimeout
	}
	if c.RetryBase <= 0 {
		c.RetryBase = defaultRetryBase
	}
	return c
}

// PostbackDispatcher polls the postback_outbox table and dispatches HTTP requests.
type PostbackDispatcher struct {
	repo   *repository.PostbackRepository
	logger *zap.Logger
	cfg    PostbackDispatcherConfig
	client *http.Client
}

// NewPostbackDispatcher creates a ready-to-start dispatcher.
func NewPostbackDispatcher(repo *repository.PostbackRepository, logger *zap.Logger, cfg PostbackDispatcherConfig) *PostbackDispatcher {
	cfg = cfg.withDefaults()
	return &PostbackDispatcher{
		repo:   repo,
		logger: logger.Named("postback_dispatcher"),
		cfg:    cfg,
		client: &http.Client{Timeout: cfg.HTTPTimeout},
	}
}

// Start runs the poll loop until ctx is cancelled. It is intended to be launched
// in a goroutine: go dispatcher.Start(ctx)
func (d *PostbackDispatcher) Start(ctx context.Context) {
	d.logger.Info("Postback dispatcher started",
		zap.Duration("poll_interval", d.cfg.PollInterval),
		zap.Int("batch_size", d.cfg.BatchSize),
		zap.Duration("http_timeout", d.cfg.HTTPTimeout),
	)

	ticker := time.NewTicker(d.cfg.PollInterval)
	defer ticker.Stop()

	cleanupTicker := time.NewTicker(staleCleanupInterval)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.logger.Info("Postback dispatcher stopping", zap.Error(ctx.Err()))
			return
		case <-ticker.C:
			d.poll(ctx)
		case <-cleanupTicker.C:
			d.cleanupStaleProcessing()
		}
	}
}

// poll claims a batch of pending postbacks and processes each one.
func (d *PostbackDispatcher) poll(ctx context.Context) {
	postbacks, err := d.repo.ClaimPendingPostbacks(d.cfg.BatchSize)
	if err != nil {
		d.logger.Error("Failed to claim pending postbacks", zap.Error(err))
		return
	}
	if len(postbacks) == 0 {
		return
	}

	d.logger.Debug("Processing postback batch", zap.Int("count", len(postbacks)))

	for _, pb := range postbacks {
		if ctx.Err() != nil {
			return
		}
		d.dispatch(ctx, pb)
	}
}

// cleanupStaleProcessing resets PROCESSING records that have been stuck for longer
// than staleThreshold back to PENDING so they can be retried.
func (d *PostbackDispatcher) cleanupStaleProcessing() {
	recovered, err := d.repo.ResetStaleProcessing(staleThreshold)
	if err != nil {
		d.logger.Error("Failed to cleanup stale processing postbacks", zap.Error(err))
		return
	}
	if recovered > 0 {
		d.logger.Warn("Recovered stale PROCESSING postbacks",
			zap.Int64("count", recovered),
			zap.Duration("threshold", staleThreshold),
		)
	}
}

// dispatch sends a single postback HTTP request and records the outcome.
func (d *PostbackDispatcher) dispatch(ctx context.Context, pb *domain.PostbackOutbox) {
	log := d.logger.With(
		zap.String("outbox_id", pb.ID.String()),
		zap.String("event", string(pb.Event)),
		zap.String("provider", pb.Provider),
		zap.Int("attempt", pb.AttemptCount+1),
	)

	// Build the HTTP request.
	req, err := d.buildRequest(ctx, pb)
	if err != nil {
		log.Error("Failed to build HTTP request", zap.Error(err))
		d.recordAttempt(log, pb, nil, 0, err)
		d.handleFailure(log, pb)
		return
	}

	// Execute and time the request.
	start := time.Now()
	resp, err := d.client.Do(req)
	durationMs := int(time.Since(start).Milliseconds())

	if err != nil {
		log.Warn("Postback HTTP request failed",
			zap.String("url", pb.URLTemplateRendered),
			zap.Error(err),
			zap.Int("duration_ms", durationMs),
		)
		d.recordAttempt(log, pb, nil, durationMs, err)
		d.handleFailure(log, pb)
		return
	}
	defer resp.Body.Close()

	// Read response body (truncated).
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, int64(maxResponseBodyLog)))
	respBody := string(bodyBytes)
	statusCode := resp.StatusCode

	if statusCode >= 200 && statusCode < 300 {
		log.Info("Postback delivered successfully",
			zap.Int("http_status", statusCode),
			zap.Int("duration_ms", durationMs),
		)
		d.recordAttempt(log, pb, &statusCode, durationMs, nil)
		d.handleSuccess(log, pb)
	} else {
		log.Warn("Postback received non-2xx response",
			zap.Int("http_status", statusCode),
			zap.String("response_body", truncate(respBody, 256)),
			zap.Int("duration_ms", durationMs),
		)
		respErr := fmt.Errorf("HTTP %d: %s", statusCode, truncate(respBody, 256))
		d.recordAttempt(log, pb, &statusCode, durationMs, respErr)
		d.handleFailure(log, pb)
	}
}

// buildRequest constructs an *http.Request from the outbox record.
func (d *PostbackDispatcher) buildRequest(ctx context.Context, pb *domain.PostbackOutbox) (*http.Request, error) {
	method := strings.ToUpper(pb.HTTPMethod)
	if method == "" {
		method = http.MethodGet
	}

	var bodyReader io.Reader
	if pb.Body != nil && *pb.Body != "" {
		bodyReader = bytes.NewBufferString(*pb.Body)
	}

	req, err := http.NewRequestWithContext(ctx, method, pb.URLTemplateRendered, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	// Parse and apply headers from JSON string.
	if pb.Headers != "" && pb.Headers != "{}" {
		headers := make(map[string]string)
		if err := json.Unmarshal([]byte(pb.Headers), &headers); err != nil {
			d.logger.Warn("Failed to parse postback headers, sending without custom headers",
				zap.String("outbox_id", pb.ID.String()),
				zap.Error(err),
			)
		} else {
			for k, v := range headers {
				req.Header.Set(k, v)
			}
		}
	}

	return req, nil
}

// recordAttempt persists an attempt record in the database.
func (d *PostbackDispatcher) recordAttempt(log *zap.Logger, pb *domain.PostbackOutbox, httpStatus *int, durationMs int, attemptErr error) {
	attempt := &domain.PostbackAttempt{
		ID:            uuid.New(),
		OutboxID:      pb.ID,
		AttemptNumber: pb.AttemptCount + 1,
		HTTPStatus:    httpStatus,
		DurationMs:    &durationMs,
		CreatedAt:     time.Now(),
	}

	if attemptErr != nil {
		msg := attemptErr.Error()
		attempt.ErrorMessage = &msg
	}

	if err := d.repo.CreateAttempt(attempt); err != nil {
		log.Error("Failed to persist postback attempt", zap.Error(err))
	}
}

// handleSuccess marks the outbox record as successful.
func (d *PostbackDispatcher) handleSuccess(log *zap.Logger, pb *domain.PostbackOutbox) {
	if err := d.repo.IncrementAttempt(pb.ID, nil); err != nil {
		log.Error("Failed to increment attempt count", zap.Error(err))
	}
	if err := d.repo.UpdateStatus(pb.ID, domain.PostbackStatusSuccess, nil); err != nil {
		log.Error("Failed to mark postback as success", zap.Error(err))
	}
}

// handleFailure increments the attempt counter and either schedules a retry or
// moves the record to DLQ when max attempts are exhausted.
func (d *PostbackDispatcher) handleFailure(log *zap.Logger, pb *domain.PostbackOutbox) {
	nextAttempt := pb.AttemptCount + 1

	if nextAttempt >= pb.MaxAttempts || nextAttempt >= defaultMaxAttempts {
		log.Warn("Postback exhausted all retries, moving to DLQ",
			zap.Int("attempts", nextAttempt),
			zap.Int("max_attempts", pb.MaxAttempts),
		)
		if err := d.repo.IncrementAttempt(pb.ID, nil); err != nil {
			log.Error("Failed to increment attempt count", zap.Error(err))
		}
		if err := d.repo.UpdateStatus(pb.ID, domain.PostbackStatusDLQ, nil); err != nil {
			log.Error("Failed to mark postback as DLQ", zap.Error(err))
		}
		return
	}

	// Exponential backoff: base * attempt number
	retryAt := time.Now().Add(d.cfg.RetryBase * time.Duration(nextAttempt))

	log.Info("Scheduling postback retry",
		zap.Int("next_attempt", nextAttempt+1),
		zap.Time("retry_at", retryAt),
	)

	if err := d.repo.IncrementAttempt(pb.ID, &retryAt); err != nil {
		log.Error("Failed to increment attempt count", zap.Error(err))
	}
	if err := d.repo.UpdateStatus(pb.ID, domain.PostbackStatusPending, &retryAt); err != nil {
		log.Error("Failed to schedule postback retry", zap.Error(err))
	}
}

// truncate shortens s to at most n bytes.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
