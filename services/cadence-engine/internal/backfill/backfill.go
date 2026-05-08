package backfill

import (
	"context"
	"time"

	"github.com/seidu626/subscription-manager/cadence-engine/internal/repository"
	"github.com/seidu626/subscription-manager/cadence-engine/internal/scheduler"
	"go.uber.org/zap"
)

type BackfillConfig struct {
	BatchSize    int
	PollInterval time.Duration
}

type Backfill struct {
	repo   *repository.CadenceRepository
	logger *zap.Logger
	cfg    BackfillConfig
}

func NewBackfill(repo *repository.CadenceRepository, logger *zap.Logger, cfg BackfillConfig) *Backfill {
	return &Backfill{
		repo:   repo,
		logger: logger,
		cfg:    cfg,
	}
}

func (b *Backfill) Run(ctx context.Context) error {
	ticker := time.NewTicker(b.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := b.processBatch(ctx); err != nil {
				b.logger.Error("backfill batch failed", zap.Error(err))
			}
		}
	}
}

func (b *Backfill) processBatch(ctx context.Context) error {
	missing, err := b.repo.ListMissingStates(ctx, b.cfg.BatchSize)
	if err != nil {
		return err
	}
	if len(missing) == 0 {
		return nil
	}

	for _, item := range missing {
		nextSendAt, err := scheduler.FirstSendAt(item.Rule, time.Now(), item.StartDate)
		if err != nil {
			b.logger.Warn("backfill compute failed",
				zap.Int64("subscription_id", item.SubscriptionID),
				zap.Int64("series_id", item.SeriesID),
				zap.Error(err),
			)
			continue
		}

		if err := b.repo.InsertState(ctx, item.TenantID, item.ChannelID, item.SubscriptionID, item.SeriesID, nextSendAt); err != nil {
			b.logger.Error("backfill insert failed",
				zap.Int64("subscription_id", item.SubscriptionID),
				zap.Int64("series_id", item.SeriesID),
				zap.Error(err),
			)
		}
	}

	return nil
}
