package advancer

import (
	"context"
	"database/sql"
	"time"

	"github.com/seidu626/subscription-manager/cadence-engine/internal/repository"
	"github.com/seidu626/subscription-manager/cadence-engine/internal/scheduler"
	"go.uber.org/zap"
)

type AdvancerConfig struct {
	BatchSize    int
	PollInterval time.Duration
}

type Advancer struct {
	repo   *repository.CadenceRepository
	logger *zap.Logger
	cfg    AdvancerConfig
}

func NewAdvancer(repo *repository.CadenceRepository, logger *zap.Logger, cfg AdvancerConfig) *Advancer {
	return &Advancer{repo: repo, logger: logger, cfg: cfg}
}

func (a *Advancer) Run(ctx context.Context) error {
	ticker := time.NewTicker(a.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := a.processBatch(ctx); err != nil {
				a.logger.Error("advancer batch failed", zap.Error(err))
			}
		}
	}
}

func (a *Advancer) processBatch(ctx context.Context) error {
	tx, err := a.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	jobs, err := a.repo.ClaimSentOutboxTx(ctx, tx, a.cfg.BatchSize)
	if err != nil {
		return err
	}
	if len(jobs) == 0 {
		return nil
	}

	now := time.Now()
	for _, job := range jobs {
		if err := a.advanceForJob(ctx, tx, job.JobID, job.SubscriptionID, job.SeriesID, job.SentAt, now); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (a *Advancer) advanceForJob(ctx context.Context, tx *sql.Tx, jobID string, subscriptionID int64, seriesID int64, sentAt *time.Time, now time.Time) error {
	subscription, err := a.repo.GetSubscriptionTx(ctx, tx, subscriptionID)
	if err != nil {
		a.logger.Warn("advancer subscription missing", zap.Int64("subscription_id", subscriptionID), zap.Error(err))
		return a.repo.MarkOutboxProcessedTx(ctx, tx, jobID)
	}

	rule, err := a.repo.GetScheduleRuleTx(ctx, tx, seriesID)
	if err != nil {
		a.logger.Warn("advancer rule missing", zap.Int64("series_id", seriesID), zap.Error(err))
		return a.repo.MarkOutboxProcessedTx(ctx, tx, jobID)
	}

	actualSentAt := now
	if sentAt != nil && !sentAt.IsZero() {
		actualSentAt = *sentAt
	}

	nextSendAt, err := scheduler.NextSendAt(*rule, now, actualSentAt, subscription.StartDate)
	if err != nil {
		a.logger.Warn("advancer schedule failed", zap.String("job_id", jobID), zap.Error(err))
		return a.repo.MarkOutboxProcessedTx(ctx, tx, jobID)
	}

	if err := a.repo.AdvanceStateTx(ctx, tx, subscriptionID, seriesID, nextSendAt, actualSentAt); err != nil {
		return err
	}

	return a.repo.MarkOutboxProcessedTx(ctx, tx, jobID)
}
