package dispatcher

import (
	"context"
	"time"

	"github.com/seidu626/subscription-manager/common/pii"
	"github.com/seidu626/subscription-manager/notification/internal/domain"
	"github.com/seidu626/subscription-manager/notification/internal/repository"
	"go.uber.org/zap"
)

// Delivery status values stored in message_outbox.delivery_status. PENDING
// jobs keep being polled; the others are terminal.
const (
	deliveryStatusPending   = "PENDING"
	deliveryStatusDelivered = "DELIVERED"
	deliveryStatusFailed    = "FAILED"
	// deliveryStatusUnconfirmed marks jobs whose gateway never reported a
	// terminal state within the poll window: accepted, but delivery to the
	// handset was never confirmed. Observed 2026-08-19: Arkesel accepted a
	// welcome SMS that never arrived.
	deliveryStatusUnconfirmed = "UNCONFIRMED"
	// deliveryStatusUntracked marks jobs whose gateway config has no status
	// endpoint, so no verdict is obtainable.
	deliveryStatusUntracked = "UNTRACKED"
)

// DeliveryPollerConfig bounds the poller's work. RecheckAfter throttles how
// often one job is re-queried; GiveUpAfter converts stale PENDING jobs to
// UNCONFIRMED so the poll set cannot grow without bound.
type DeliveryPollerConfig struct {
	PollInterval time.Duration
	BatchSize    int
	RecheckAfter time.Duration
	GiveUpAfter  time.Duration
}

// DeliveryPoller resolves the true handset delivery outcome of gateway-sent
// SMS jobs. The dispatcher marks a job SENT when the gateway accepts it; this
// poller upgrades that to DELIVERED/FAILED by querying the gateway's status
// endpoint with the provider message id captured at send time.
type DeliveryPoller struct {
	repo    *repository.OutboxRepository
	gateway TenantGateway
	logger  *zap.Logger
	cfg     DeliveryPollerConfig
}

func NewDeliveryPoller(repo *repository.OutboxRepository, gateway TenantGateway, logger *zap.Logger, cfg DeliveryPollerConfig) *DeliveryPoller {
	return &DeliveryPoller{repo: repo, gateway: gateway, logger: logger, cfg: cfg}
}

func (p *DeliveryPoller) Run(ctx context.Context) error {
	ticker := time.NewTicker(p.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := p.pollBatch(ctx); err != nil {
				p.logger.Error("delivery poll batch failed", zap.Error(err))
			}
		}
	}
}

func (p *DeliveryPoller) pollBatch(ctx context.Context) error {
	checks, err := p.repo.ClaimDeliveryChecks(ctx, p.cfg.BatchSize, p.cfg.RecheckAfter, p.cfg.GiveUpAfter)
	if err != nil {
		return err
	}
	for _, check := range checks {
		p.pollJob(ctx, check)
	}
	return nil
}

func (p *DeliveryPoller) pollJob(ctx context.Context, check domain.DeliveryCheck) {
	cfg, err := p.gateway.Resolve(ctx, check.TenantID)
	if err != nil {
		p.logger.Warn("delivery poll: gateway resolve failed",
			zap.String("job_id", check.JobID), zap.Error(err))
		return
	}
	if cfg == nil || cfg.StatusURL == "" || cfg.StatusPath == "" {
		// The tenant's gateway offers (or is configured with) no status
		// endpoint: terminal, stop polling this job.
		if err := p.repo.UpdateDeliveryStatus(ctx, check.JobID, deliveryStatusUntracked, "", nil); err != nil {
			p.logger.Warn("delivery poll: mark untracked failed", zap.String("job_id", check.JobID), zap.Error(err))
		}
		return
	}

	status, raw, err := p.gateway.DeliveryStatus(ctx, cfg, check.ProviderMessageID)
	if err != nil {
		// Transient by assumption: delivery_checked_at was already bumped by
		// the claim, so this job waits RecheckAfter before the next try and
		// ages out via GiveUpAfter if the endpoint keeps failing.
		p.logger.Warn("delivery poll: status query failed",
			zap.String("job_id", check.JobID),
			zap.String("msisdn", pii.MaskMSISDN(check.MSISDN)),
			zap.Error(err))
		return
	}

	var deliveredAt *time.Time
	if status == deliveryStatusDelivered {
		now := time.Now()
		deliveredAt = &now
	}
	if err := p.repo.UpdateDeliveryStatus(ctx, check.JobID, status, raw, deliveredAt); err != nil {
		p.logger.Warn("delivery poll: update failed", zap.String("job_id", check.JobID), zap.Error(err))
		return
	}
	if status != deliveryStatusPending {
		p.logger.Info("delivery status resolved",
			zap.String("job_id", check.JobID),
			zap.String("msisdn", pii.MaskMSISDN(check.MSISDN)),
			zap.String("delivery_status", status),
			zap.String("gateway_status", raw),
		)
	}
}
