package dispatcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/seidu626/subscription-manager/common/pii"
	"github.com/seidu626/subscription-manager/notification/internal/domain"
	"github.com/seidu626/subscription-manager/notification/internal/observability"
	"github.com/seidu626/subscription-manager/notification/internal/repository"
	"go.uber.org/zap"
)

// PushRepo resolves the Dayline app push-routing inputs (registered device,
// stored channel preference) for a subscriber. Satisfied by
// *repository.PushRepository; a narrow interface so tests can fake it.
type PushRepo interface {
	DeviceToken(ctx context.Context, msisdn, tenantID string) (token string, found bool, err error)
	PreferredChannel(ctx context.Context, msisdn string, partnerRoleID, productID int) (string, error)
}

// PushSender delivers a message via FCM. Satisfied by *push.FCMSender; a
// narrow interface so tests can fake it without real Google credentials.
type PushSender interface {
	Send(ctx context.Context, deviceToken, body string) error
}

// TenantGateway resolves and sends content SMS through a tenant's configured
// HTTP gateway (tenant_channel_credentials purpose sms_api). Satisfied by
// *TenantGatewaySender; a narrow interface so tests can fake it without a
// real DB. Resolve returning (nil, nil) means the tenant has no ACTIVE
// sms_api binding, and the caller should fall back to the TIMWE MT path.
type TenantGateway interface {
	Resolve(ctx context.Context, tenantID string) (*smsGatewayConfig, error)
	Send(ctx context.Context, cfg *smsGatewayConfig, msisdn, text string) error
}

type Config struct {
	BatchSize    int
	PollInterval time.Duration
	MaxAttempts  int
	BackoffBase  time.Duration
	BackoffMax   time.Duration
	MTBaseURL    string
	MTChannel    string
	HTTPTimeout  time.Duration
}

type Dispatcher struct {
	repo       *repository.OutboxRepository
	logger     *zap.Logger
	httpClient *http.Client
	cfg        Config

	pushRepo     PushRepo
	pushSender   PushSender
	pushWarnOnce sync.Once

	tenantGateway TenantGateway
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

// WithPush wires PUSH-channel routing into the dispatcher. It is a separate,
// chainable step (rather than a NewDispatcher parameter) so existing callers
// that construct a Dispatcher without push support, including
// metrics_test.go, keep compiling unchanged. pushSender may be nil when FCM
// credentials are not configured (see docs/dayline-app-api-contract.md):
// PUSH-eligible jobs then fall back to SMS with a once-per-process warning,
// never dropped.
func (d *Dispatcher) WithPush(pushRepo PushRepo, pushSender PushSender) *Dispatcher {
	d.pushRepo = pushRepo
	d.pushSender = pushSender
	return d
}

// WithTenantGateway wires per-tenant SMS gateway routing into the
// dispatcher. It is chainable like WithPush so existing callers that
// construct a Dispatcher without it keep compiling unchanged. When gateway
// is nil, or a job's TenantID is unset, SMS keeps going through the TIMWE MT
// path exactly as before.
func (d *Dispatcher) WithTenantGateway(gateway TenantGateway) *Dispatcher {
	d.tenantGateway = gateway
	return d
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

// resolveChannel decides how job should be delivered: PUSH (with the
// device token to send to) when the subscriber prefers PUSH, has a
// registered device, and FCM is configured; SMS otherwise. Lookup failures
// fail safe to SMS rather than blocking delivery.
func (d *Dispatcher) resolveChannel(ctx context.Context, job domain.OutboxJob) (channel string, deviceToken string) {
	if job.DeliveryChannel == channelSMS {
		return channelSMS, ""
	}
	if d.pushRepo == nil {
		return channelSMS, ""
	}

	var tenantID string
	if job.TenantID != nil {
		tenantID = *job.TenantID
	}
	token, hasDevice, err := d.pushRepo.DeviceToken(ctx, job.MSISDN, tenantID)
	if err != nil {
		d.logger.Warn("push device lookup failed, falling back to SMS", zap.String("job_id", job.JobID), zap.Error(err))
		return channelSMS, ""
	}

	preferred := ""
	if job.DeliveryChannel != channelPush {
		var err error
		preferred, err = d.pushRepo.PreferredChannel(ctx, job.MSISDN, job.PartnerRoleID, job.ProductID)
		if err != nil {
			d.logger.Warn("push preference lookup failed, falling back to SMS", zap.String("job_id", job.JobID), zap.Error(err))
			return channelSMS, ""
		}
	}

	pushConfigured := d.pushSender != nil
	decision := decideChannel(channelDecisionInput{
		preferredChannel: preferred,
		hasDevice:        hasDevice,
		pushConfigured:   pushConfigured,
		deliveryChannel:  job.DeliveryChannel,
	})
	if decision == channelPush {
		return channelPush, token
	}

	if hasDevice && (job.DeliveryChannel == channelPush || preferred == channelPush) && !pushConfigured {
		d.pushWarnOnce.Do(func() {
			d.logger.Warn("FCM_CREDENTIALS_JSON_PATH not set: PUSH-preferred jobs are falling back to SMS")
		})
	}
	return channelSMS, ""
}

func (d *Dispatcher) processJob(ctx context.Context, job domain.OutboxJob) error {
	channel, deviceToken := d.resolveChannel(ctx, job)

	var err error
	if channel == channelPush {
		err = d.pushSender.Send(ctx, deviceToken, job.MessageText)
	} else {
		err = d.sendSMS(ctx, job)
	}

	if err == nil {
		recordDispatch(job, "sent")
		d.logger.Info("dispatcher job sent", append(d.jobFields(job), zap.String("delivery_channel", channel))...)
		return d.repo.MarkSent(ctx, job.JobID, channel)
	}

	if job.Attempt >= d.cfg.MaxAttempts {
		recordDispatch(job, "failed")
		d.logger.Warn("dispatcher job failed permanently", append(d.jobFields(job), zap.Error(err))...)
		return d.repo.MarkFailed(ctx, job.JobID, err.Error())
	}

	nextRetry := d.calculateNextRetry(job.Attempt)
	recordDispatch(job, "retry")
	d.logger.Warn("dispatcher job scheduled for retry", append(d.jobFields(job), zap.Error(err), zap.Time("next_retry_at", nextRetry))...)
	return d.repo.ScheduleRetry(ctx, job.JobID, nextRetry, err.Error())
}

func (d *Dispatcher) jobFields(job domain.OutboxJob) []zap.Field {
	labels := observability.WorkerLabels(job.TenantID, job.ChannelID, "notification_worker")
	return []zap.Field{
		zap.String("job_id", job.JobID),
		zap.String("tenant_id", labels.TenantID),
		zap.String("channel_id", labels.ChannelID),
		zap.String("worker", labels.Worker),
		zap.Int("attempt", job.Attempt),
	}
}

// sendSMS routes a content SMS job. A tenant with an ACTIVE sms_api gateway
// binding sends through that HTTP gateway (e.g. Arkesel v2); every other
// tenant keeps going through the TIMWE MT path, which has been failing on
// prod since May (106k FAILED jobs, "MT status 400/403").
func (d *Dispatcher) sendSMS(ctx context.Context, job domain.OutboxJob) error {
	if d.tenantGateway != nil && job.TenantID != nil && *job.TenantID != "" {
		cfg, err := d.tenantGateway.Resolve(ctx, *job.TenantID)
		if err != nil {
			return err
		}
		if cfg != nil {
			if err := d.tenantGateway.Send(ctx, cfg, job.MSISDN, job.MessageText); err != nil {
				return err
			}
			d.logger.Info("content sms dispatched via tenant gateway",
				zap.String("tenant_id", *job.TenantID),
				zap.String("msisdn", pii.MaskMSISDN(job.MSISDN)),
			)
			return nil
		}
	}
	return d.sendMT(ctx, job)
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
	if job.ChannelID != nil && *job.ChannelID != "" {
		payload["channelId"] = *job.ChannelID
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
	if job.TenantID != nil && *job.TenantID != "" {
		req.Header.Set("X-Tenant-Id", *job.TenantID)
	}
	if job.ChannelID != nil && *job.ChannelID != "" {
		req.Header.Set("X-Tenant-Channel-Id", *job.ChannelID)
	}

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
