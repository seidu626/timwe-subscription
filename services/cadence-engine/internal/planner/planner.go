package planner

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/cadence-engine/internal/domain"
	"github.com/seidu626/subscription-manager/cadence-engine/internal/repository"
	"go.uber.org/zap"
)

type PlannerConfig struct {
	BatchSize        int
	PollInterval     time.Duration
	InflightDuration time.Duration
}

type Planner struct {
	repo   *repository.CadenceRepository
	logger *zap.Logger
	cfg    PlannerConfig
}

func NewPlanner(repo *repository.CadenceRepository, logger *zap.Logger, cfg PlannerConfig) *Planner {
	return &Planner{repo: repo, logger: logger, cfg: cfg}
}

func (p *Planner) Run(ctx context.Context) error {
	ticker := time.NewTicker(p.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := p.processBatch(ctx); err != nil {
				p.logger.Error("planner batch failed", zap.Error(err))
			}
		}
	}
}

func (p *Planner) processBatch(ctx context.Context) error {
	tx, err := p.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	dueStates, err := p.repo.ClaimDueStatesTx(ctx, tx, p.cfg.BatchSize)
	if err != nil {
		return err
	}
	if len(dueStates) == 0 {
		return nil
	}

	for _, state := range dueStates {
		if err := p.planForState(ctx, tx, state); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (p *Planner) planForState(ctx context.Context, tx *sql.Tx, state domain.DueState) error {
	series, err := p.repo.GetSeriesTx(ctx, tx, state.SeriesID)
	if err != nil {
		return err
	}
	if !series.IsActive {
		return p.repo.StopStateTx(ctx, tx, state.SubscriptionID, state.SeriesID, "series_inactive")
	}

	subscription, err := p.repo.GetSubscriptionTx(ctx, tx, state.SubscriptionID)
	if err != nil {
		return err
	}

	var item *domain.ContentItem
	switch series.Mode {
	case "POOL":
		item, err = p.repo.GetPoolContentItemTx(ctx, tx, series.ID, series.ContentVersion)
	default:
		item, err = p.repo.GetSequentialContentItemTx(ctx, tx, series.ID, series.ContentVersion, state.CursorSeq)
	}
	if err != nil {
		if err == sql.ErrNoRows {
			return p.repo.StopStateTx(ctx, tx, state.SubscriptionID, state.SeriesID, "no_content")
		}
		return err
	}

	jobID := uuid.New().String()
	idempotencyKey := fmt.Sprintf("%d:%d:%d:%d:%d",
		subscription.PartnerRoleID,
		subscription.ID,
		series.ID,
		series.ContentVersion,
		state.CursorSeq,
	)

	job := domain.OutboxJob{
		JobID:          jobID,
		IdempotencyKey: idempotencyKey,
		SubscriptionID: subscription.ID,
		SeriesID:       series.ID,
		ContentItemID:  item.ID,
		PlannedSendAt:  state.NextSendAt,
		Status:         "PENDING",
		Attempt:        0,
	}

	inserted, err := p.repo.InsertOutboxTx(ctx, tx, job)
	if err != nil {
		return err
	}

	inflightUntil := time.Now().Add(p.cfg.InflightDuration)
	if inserted {
		return p.repo.UpdateInflightTx(ctx, tx, subscription.ID, series.ID, &jobID, inflightUntil)
	}

	return p.repo.UpdateInflightTx(ctx, tx, subscription.ID, series.ID, nil, inflightUntil)
}
