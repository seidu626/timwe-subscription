package dispatcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/seidu626/subscription-manager/notification/internal/domain"
	"github.com/seidu626/subscription-manager/notification/internal/repository"
	"go.uber.org/zap"
)

type Config struct {
	BatchSize     int
	PollInterval  time.Duration
	MaxAttempts   int
	BackoffBase   time.Duration
	BackoffMax    time.Duration
	MTBaseURL     string
	MTChannel     string
	HTTPTimeout   time.Duration
}

type Dispatcher struct {
	repo       *repository.OutboxRepository
	logger     *zap.Logger
	httpClient *http.Client
	cfg        Config
}

func NewDispatcher(repo *repository.OutboxRepository, logger *zap.Logger, cfg Config) *Dispatcher {
	return &Dispatcher{
		repo:   repo,
		logger: logger,
		httpClient: &http.Client{
			Timeout: cfg.HTTPTimeout,
		},
		cfg: cfg,
	}
}

func (d *Dispatcher) Run(ctx context.Context) error {
	ticker := time.NewTicker(d.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := d.processBatch(ctx); err != nil {
				d.logger.Error("dispatcher batch failed", zap.Error(err))
			}
		}
	}
}

func (d *Dispatcher) processBatch(ctx context.Context) error {
	jobs, err := d.repo.ClaimPendingJobs(ctx, d.cfg.BatchSize)
	if err != nil {
		return err
	}
	if len(jobs) == 0 {
		return nil
	}

	for _, job := range jobs {
		if err := d.processJob(ctx, job); err != nil {
			d.logger.Warn("dispatcher job failed", zap.String("job_id", job.JobID), zap.Error(err))
		}
	}

	return nil
}

func (d *Dispatcher) processJob(ctx context.Context, job domain.OutboxJob) error {
	err := d.sendMT(ctx, job)
	if err == nil {
		return d.repo.MarkSent(ctx, job.JobID)
	}

	if job.Attempt >= d.cfg.MaxAttempts {
		return d.repo.MarkFailed(ctx, job.JobID, err.Error())
	}

	nextRetry := d.calculateNextRetry(job.Attempt)
	return d.repo.ScheduleRetry(ctx, job.JobID, nextRetry, err.Error())
}

func (d *Dispatcher) sendMT(ctx context.Context, job domain.OutboxJob) error {
	if d.cfg.MTBaseURL == "" {
		return fmt.Errorf("missing MT base URL")
	}
	channel := d.cfg.MTChannel
	if channel == "" {
		channel = "SMS"
	}

	payload := map[string]interface{}{
		"productId": job.ProductID,
		"msisdn":    job.MSISDN,
		"text":      job.MessageText,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal MT payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/external/v1/%s/mt", d.cfg.MTBaseURL, channel), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create MT request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("MT request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("MT status %d", resp.StatusCode)
	}

	var parsed struct {
		InError bool   `json:"inError"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return fmt.Errorf("decode MT response: %w", err)
	}
	if parsed.InError {
		return fmt.Errorf("MT error code %s: %s", parsed.Code, parsed.Message)
	}

	return nil
}

func (d *Dispatcher) calculateNextRetry(attempt int) time.Time {
	backoff := time.Duration(math.Pow(2, float64(attempt-1))) * d.cfg.BackoffBase
	if backoff > d.cfg.BackoffMax {
		backoff = d.cfg.BackoffMax
	}
	return time.Now().Add(backoff)
}
