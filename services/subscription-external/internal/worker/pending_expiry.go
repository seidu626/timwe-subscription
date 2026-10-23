package worker

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/seidu626/subscription-manager/subscription-external/internal/repository"
	"go.uber.org/zap"
)

const (
	defaultPendingExpiryInterval = time.Hour
	// Matches acquisition-api's ACQ_CHARGE_EXPIRY_DAYS default (7 days), the
	// window after which an unpaid acquisition transaction becomes FAILED.
	defaultPendingExpiryWindow    = 7 * 24 * time.Hour
	defaultPendingExpiryBatchSize = 5000
	pendingExpiryBatchPause       = 500 * time.Millisecond
)

// PendingExpirer ends stale pending/preactive subscriptions in batches.
// Implemented by repository.SubscriptionRepository.
type PendingExpirer interface {
	ExpireStalePendingSubscriptions(ctx context.Context, window time.Duration, limit int) (int64, error)
}

var _ PendingExpirer = (*repository.SubscriptionRepository)(nil)

// PendingExpiryConfig holds tunable parameters for the pending expiry worker.
type PendingExpiryConfig struct {
	Enabled   bool
	Interval  time.Duration
	Window    time.Duration
	BatchSize int
}

// PendingExpiryConfigFromEnv reads PENDING_EXPIRY_ENABLED,
// PENDING_EXPIRY_INTERVAL, PENDING_EXPIRY_WINDOW and
// PENDING_EXPIRY_BATCH_SIZE. Invalid values fall back to defaults.
func PendingExpiryConfigFromEnv(logger *zap.Logger) PendingExpiryConfig {
	cfg := PendingExpiryConfig{
		Enabled:   true,
		Interval:  defaultPendingExpiryInterval,
		Window:    defaultPendingExpiryWindow,
		BatchSize: defaultPendingExpiryBatchSize,
	}
	warn := func(name, raw string) {
		logger.Warn("Invalid pending expiry env value; using default",
			zap.String("env", name), zap.String("value", raw))
	}
	if raw := strings.TrimSpace(os.Getenv("PENDING_EXPIRY_ENABLED")); raw != "" {
		if enabled, err := strconv.ParseBool(raw); err == nil {
			cfg.Enabled = enabled
		} else {
			warn("PENDING_EXPIRY_ENABLED", raw)
		}
	}
	if raw := strings.TrimSpace(os.Getenv("PENDING_EXPIRY_INTERVAL")); raw != "" {
		if interval, err := time.ParseDuration(raw); err == nil && interval > 0 {
			cfg.Interval = interval
		} else {
			warn("PENDING_EXPIRY_INTERVAL", raw)
		}
	}
	if raw := strings.TrimSpace(os.Getenv("PENDING_EXPIRY_WINDOW")); raw != "" {
		if window, err := time.ParseDuration(raw); err == nil && window > 0 {
			cfg.Window = window
		} else {
			warn("PENDING_EXPIRY_WINDOW", raw)
		}
	}
	if raw := strings.TrimSpace(os.Getenv("PENDING_EXPIRY_BATCH_SIZE")); raw != "" {
		if size, err := strconv.Atoi(raw); err == nil && size > 0 {
			cfg.BatchSize = size
		} else {
			warn("PENDING_EXPIRY_BATCH_SIZE", raw)
		}
	}
	return cfg
}

// PendingExpiryWorker ends pending/preactive subscriptions that never
// received a DELIVERED charge within the configured window.
type PendingExpiryWorker struct {
	repo   PendingExpirer
	logger *zap.Logger
	cfg    PendingExpiryConfig
	pause  time.Duration
}

// NewPendingExpiryWorker creates a ready-to-start worker.
func NewPendingExpiryWorker(repo PendingExpirer, logger *zap.Logger, cfg PendingExpiryConfig) *PendingExpiryWorker {
	if cfg.Interval <= 0 {
		cfg.Interval = defaultPendingExpiryInterval
	}
	if cfg.Window <= 0 {
		cfg.Window = defaultPendingExpiryWindow
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = defaultPendingExpiryBatchSize
	}
	return &PendingExpiryWorker{
		repo:   repo,
		logger: logger.Named("pending_expiry"),
		cfg:    cfg,
		pause:  pendingExpiryBatchPause,
	}
}

// Start runs one pass at once and then one per interval until ctx is
// cancelled. It returns at once when the worker is disabled. Launch it in a
// goroutine.
func (w *PendingExpiryWorker) Start(ctx context.Context) {
	if !w.cfg.Enabled {
		w.logger.Info("Pending expiry worker disabled")
		return
	}
	w.logger.Info("Pending expiry worker started",
		zap.Duration("interval", w.cfg.Interval),
		zap.Duration("window", w.cfg.Window),
		zap.Int("batch_size", w.cfg.BatchSize),
	)

	ticker := time.NewTicker(w.cfg.Interval)
	defer ticker.Stop()

	w.RunOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Pending expiry worker stopping", zap.Error(ctx.Err()))
			return
		case <-ticker.C:
			w.RunOnce(ctx)
		}
	}
}

// RunOnce expires batches until one comes back short, an error occurs or ctx
// is cancelled, and returns the number of rows ended. Each batch is its own
// statement, so no lock is held across batches.
func (w *PendingExpiryWorker) RunOnce(ctx context.Context) int64 {
	var total int64
	for ctx.Err() == nil {
		expired, err := w.repo.ExpireStalePendingSubscriptions(ctx, w.cfg.Window, w.cfg.BatchSize)
		if err != nil {
			w.logger.Error("Failed to expire stale pending subscriptions", zap.Error(err))
			break
		}
		total += expired
		if expired < int64(w.cfg.BatchSize) {
			break
		}
		select {
		case <-ctx.Done():
		case <-time.After(w.pause):
		}
	}
	if total > 0 {
		w.logger.Info("Expired pending subscriptions without charge evidence",
			zap.Int64("count", total),
			zap.Duration("window", w.cfg.Window),
		)
	}
	return total
}
