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

	now := time.Now()

	jobs, err := a.repo.ClaimSentOutboxTx(ctx, tx, a.cfg.BatchSize)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if err := a.advanceForJob(ctx, tx, job.JobID, job.SubscriptionID, job.SeriesID, job.SentAt, now); err != nil {
			return err
		}
	}

	// FAILED jobs are terminal for the job itself, but the state must still move
	// to its next slot. Without this, next_send_at stays in the past and the
	// planner keeps re-claiming the state, hitting the idempotency_key conflict
	// on every tick (the dead-loop this pass fixes).
	failedJobs, err := a.repo.ClaimFailedOutboxTx(ctx, tx, a.cfg.BatchSize)
	if err != nil {
		return err
	}
	for _, job := range failedJobs {
		if err := a.rescheduleForJob(ctx, tx, job.JobID, job.SubscriptionID, job.SeriesID, now); err != nil {
			return err
		}
	}

	if len(jobs) == 0 && len(failedJobs) == 0 {
		return nil
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

// rescheduleForJob moves a FAILED job's state to its next scheduled slot
// without advancing the content cursor: the content was never delivered, so
// the same item is due again rather than skipped.
func (a *Advancer) rescheduleForJob(ctx context.Context, tx *sql.Tx, jobID string, subscriptionID int64, seriesID int64, now time.Time) error {
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

	// lastSentAt=now pushes the computed slot past now regardless of catchup
	// mode, since the FAILED job was never actually sent.
	nextSendAt, err := scheduler.NextSendAt(*rule, now, now, subscription.StartDate)
	if err != nil {
		a.logger.Warn("advancer schedule failed", zap.String("job_id", jobID), zap.Error(err))
		return a.repo.MarkOutboxProcessedTx(ctx, tx, jobID)
	}

	if err := a.repo.RescheduleStateTx(ctx, tx, subscriptionID, seriesID, nextSendAt); err != nil {
		return err
	}

	a.logger.Info("failed job rescheduled",
		zap.String("job_id", jobID),
		zap.Int64("series_id", seriesID),
		zap.Int64("subscription_id", subscriptionID),
		zap.Time("next_send_at", nextSendAt),
	)

	return a.repo.MarkOutboxProcessedTx(ctx, tx, jobID)
}
