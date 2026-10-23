package worker

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
	"go.uber.org/zap"
)

const (
	defaultChargeReconcilerInterval = 60 * time.Second
	defaultChargeExpiryDays         = 7
	defaultChargePromoteLookback    = 30
	chargePromoteBatchSize          = 200
)

// ChargePromoter applies charge success to a transaction matched to payment
// evidence. Implemented by service.TransactionService.
type ChargePromoter interface {
	PromoteOnChargeEvidence(tx *domain.AcquisitionTransaction, alreadyPaying bool) error
}

// ChargeReconcilerConfig holds tunable parameters for the charge reconciler.
type ChargeReconcilerConfig struct {
	Enabled             bool
	Interval            time.Duration
	ExpiryDays          int
	PromoteLookbackDays int
}

// ChargeReconcilerConfigFromEnv reads ACQ_CHARGE_RECONCILER_ENABLED,
// ACQ_CHARGE_RECONCILER_INTERVAL, ACQ_CHARGE_EXPIRY_DAYS and
// ACQ_CHARGE_PROMOTE_LOOKBACK_DAYS. Invalid values fall back to defaults.
func ChargeReconcilerConfigFromEnv(logger *zap.Logger) ChargeReconcilerConfig {
	cfg := ChargeReconcilerConfig{
		Enabled:             true,
		Interval:            defaultChargeReconcilerInterval,
		ExpiryDays:          defaultChargeExpiryDays,
		PromoteLookbackDays: defaultChargePromoteLookback,
	}
	warn := func(name, raw string) {
		logger.Warn("Invalid charge reconciler env value; using default",
			zap.String("env", name), zap.String("value", raw))
	}
	if raw := strings.TrimSpace(os.Getenv("ACQ_CHARGE_RECONCILER_ENABLED")); raw != "" {
		if enabled, err := strconv.ParseBool(raw); err == nil {
			cfg.Enabled = enabled
		} else {
			warn("ACQ_CHARGE_RECONCILER_ENABLED", raw)
		}
	}
	if raw := strings.TrimSpace(os.Getenv("ACQ_CHARGE_RECONCILER_INTERVAL")); raw != "" {
		if interval, err := time.ParseDuration(raw); err == nil && interval > 0 {
			cfg.Interval = interval
		} else {
			warn("ACQ_CHARGE_RECONCILER_INTERVAL", raw)
		}
	}
	if raw := strings.TrimSpace(os.Getenv("ACQ_CHARGE_EXPIRY_DAYS")); raw != "" {
		if days, err := strconv.Atoi(raw); err == nil && days > 0 {
			cfg.ExpiryDays = days
		} else {
			warn("ACQ_CHARGE_EXPIRY_DAYS", raw)
		}
	}
	if raw := strings.TrimSpace(os.Getenv("ACQ_CHARGE_PROMOTE_LOOKBACK_DAYS")); raw != "" {
		if days, err := strconv.Atoi(raw); err == nil && days > 0 {
			cfg.PromoteLookbackDays = days
		} else {
			warn("ACQ_CHARGE_PROMOTE_LOOKBACK_DAYS", raw)
		}
	}
	return cfg
}

// ChargeReconciler promotes transactions with DELIVERED CHARGE/USER_RENEWED
// notifications to CHARGED and expires pre-charge transactions that never
// received payment evidence.
type ChargeReconciler struct {
	repo     *repository.TransactionRepository
	promoter ChargePromoter
	logger   *zap.Logger
	cfg      ChargeReconcilerConfig
}

// NewChargeReconciler creates a ready-to-start reconciler.
func NewChargeReconciler(repo *repository.TransactionRepository, promoter ChargePromoter, logger *zap.Logger, cfg ChargeReconcilerConfig) *ChargeReconciler {
	if cfg.Interval <= 0 {
		cfg.Interval = defaultChargeReconcilerInterval
	}
	if cfg.ExpiryDays <= 0 {
		cfg.ExpiryDays = defaultChargeExpiryDays
	}
	if cfg.PromoteLookbackDays <= 0 {
		cfg.PromoteLookbackDays = defaultChargePromoteLookback
	}
	return &ChargeReconciler{
		repo:     repo,
		promoter: promoter,
		logger:   logger.Named("charge_reconciler"),
		cfg:      cfg,
	}
}

// Start runs the reconcile loop until ctx is cancelled. It returns at once
// when the reconciler is disabled. Launch it in a goroutine.
func (r *ChargeReconciler) Start(ctx context.Context) {
	if !r.cfg.Enabled {
		r.logger.Info("Charge reconciler disabled")
		return
	}
	r.logger.Info("Charge reconciler started",
		zap.Duration("interval", r.cfg.Interval),
		zap.Int("expiry_days", r.cfg.ExpiryDays),
		zap.Int("promote_lookback_days", r.cfg.PromoteLookbackDays),
	)

	ticker := time.NewTicker(r.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("Charge reconciler stopping", zap.Error(ctx.Err()))
			return
		case <-ticker.C:
			r.Tick(ctx)
		}
	}
}

// Tick runs one promote pass followed by one expire pass. Promote runs first
// so a transaction whose evidence arrived is never expired in the same tick.
func (r *ChargeReconciler) Tick(ctx context.Context) {
	if !r.cfg.Enabled {
		return
	}
	r.promote(ctx)
	if ctx.Err() != nil {
		return
	}
	r.expire()
}

func (r *ChargeReconciler) promote(ctx context.Context) {
	candidates, err := r.repo.ListChargeEvidenceCandidates(r.cfg.PromoteLookbackDays, chargePromoteBatchSize)
	if err != nil {
		r.logger.Error("Failed to list charge evidence candidates", zap.Error(err))
		return
	}
	promoted, failed := 0, 0
	for _, c := range candidates {
		if ctx.Err() != nil {
			return
		}
		log := r.logger.With(zap.String("transaction_id", c.ID.String()), zap.String("tenant_id", c.TenantID))
		tx, err := r.repo.GetByIDForTenant(c.ID, c.TenantID)
		if err != nil {
			failed++
			log.Error("Failed to load charge evidence candidate", zap.Error(err))
			continue
		}
		previousStatus := tx.Status
		if err := r.promoter.PromoteOnChargeEvidence(tx, c.AlreadyPaying); err != nil {
			failed++
			log.Error("Failed to promote transaction on charge evidence", zap.Error(err))
			continue
		}
		promoted++
		log.Info("Transaction promoted to CHARGED on charge evidence",
			zap.String("previous_status", string(previousStatus)),
			zap.Bool("already_paying", c.AlreadyPaying),
			zap.String("campaign_slug", tx.CampaignSlug),
		)
	}
	if len(candidates) > 0 {
		r.logger.Info("Charge evidence promote pass complete",
			zap.Int("candidates", len(candidates)),
			zap.Int("promoted", promoted),
			zap.Int("failed", failed),
		)
	}
}

func (r *ChargeReconciler) expire() {
	expired, err := r.repo.ExpireUnchargedTransactions(r.cfg.ExpiryDays)
	if err != nil {
		r.logger.Error("Failed to expire uncharged transactions", zap.Error(err))
		return
	}
	if expired > 0 {
		r.logger.Info("Expired transactions without charge evidence",
			zap.Int64("count", expired),
			zap.Int("expiry_days", r.cfg.ExpiryDays),
		)
	}
}
