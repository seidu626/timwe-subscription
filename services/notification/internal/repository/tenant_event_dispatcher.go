package repository

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/seidu626/subscription-manager/common/secretcrypto"
	"github.com/seidu626/subscription-manager/common/tenantevents"
)

const tenantEventContractVersion = "tenant-channel-v1.0.0"

// tenantEventHTTPClient calls the platform's own subscription-external for opt-outs, an internal address.
var tenantEventHTTPClient = &http.Client{Timeout: 10 * time.Second}

// tenantWebhookHTTPClient posts to tenant-configured URLs. A tenant admin chooses that URL, so the
// client dials only public addresses (checked after DNS resolution, so rebinding cannot reach an
// internal service), ignores proxy settings and never follows redirects.
var tenantWebhookHTTPClient = newTenantWebhookHTTPClient(tenantevents.AllowPrivateTargets())

var errTenantWebhookTargetBlocked = errors.New("tenant webhook target is not a public address")

func newTenantWebhookHTTPClient(allowPrivate bool) *http.Client {
	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			if allowPrivate {
				return nil
			}
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return errTenantWebhookTargetBlocked
			}
			addr, err := netip.ParseAddr(host)
			if err != nil || !tenantevents.PublicAddr(addr) {
				return fmt.Errorf("%w: %s", errTenantWebhookTargetBlocked, host)
			}
			return nil
		},
	}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		MaxIdleConns:          20,
		IdleConnTimeout:       90 * time.Second,
	}
	return &http.Client{
		Timeout:       10 * time.Second,
		Transport:     transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// activeTenantEventBindingSQL picks the credential events are delivered with: an ACTIVE tenant_events
// credential on an ACTIVE channel of an ACTIVE tenant, newest version first. Enqueue and dispatch share
// it, so an event is only queued for a binding the dispatcher would also accept.
const activeTenantEventBindingSQL = `SELECT c.channel_id::text, c.secret_ref FROM tenant_channel_credentials c
  JOIN tenants t ON t.id = c.tenant_id AND t.status = 'ACTIVE'
  JOIN tenant_channels ch ON ch.id = c.channel_id AND ch.tenant_id = c.tenant_id AND ch.status = 'ACTIVE'
  WHERE c.tenant_id = $1::uuid AND c.purpose = 'tenant_events' AND c.status = 'ACTIVE'
  ORDER BY c.version DESC, c.channel_id LIMIT 1`

// tenantOptoutDisabled closes an opt-out request whose tenant binding does not set honor_optout_action.
const tenantOptoutDisabled = "disabled: tenant opt-out requests are off for this binding"

// batchSubscriberEventSQL reports whether the event's subscriber has a newest subscription row the
// platform created itself (campaign_url INTERNAL: batch or admin opt-in) rather than the tenant or a
// provider callback.
const batchSubscriberEventSQL = `SELECT COALESCE((SELECT s.campaign_url = 'INTERNAL' FROM subscriptions s
    WHERE s.tenant_id = n.tenant_id AND s.user_identifier = n.msisdn AND s.product_id = n.product_id
    ORDER BY s.created_at DESC, s.id DESC LIMIT 1), false)
  FROM tenant_event_outbox o JOIN notifications n ON n.id = o.source_notification_id AND n.tenant_id = o.tenant_id
  WHERE o.tenant_id = $1::uuid AND o.event_id = $2`

// tenantEventBatch caps deliveries per dispatcher tick so the opt-out queue still gets a turn.
const tenantEventBatch = 200

type dueTenantEvent struct {
	ID       string
	TenantID string
	Body     []byte
	Attempts int
}

// RunTenantEventDispatcher sends due events and opt-out requests until ctx ends.
func (r *NotificationRepository) RunTenantEventDispatcher(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 3 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		// Drain due rows each tick: a renewal run can bring thousands of charges, and each one is a vote waiting on us.
		for i := 0; i < tenantEventBatch; i++ {
			claimed, err := r.dispatchOneTenantEvent(ctx)
			if err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("tenant event dispatch: %v", err)
			}
			if !claimed || err != nil {
				break
			}
		}
		if err := r.dispatchOneTenantOptout(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("tenant optout dispatch: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *NotificationRepository) dispatchOneTenantEvent(ctx context.Context) (bool, error) {
	var job dueTenantEvent
	err := r.db.QueryRowContext(ctx, `WITH due AS (
		SELECT candidate.tenant_id, candidate.event_id FROM tenant_event_outbox candidate
		WHERE NOT EXISTS (SELECT 1 FROM tenant_event_outbox earlier
          WHERE earlier.tenant_id = candidate.tenant_id
          AND earlier.payload->>'msisdn' = candidate.payload->>'msisdn'
          AND earlier.status IN ('pending','sending')
          AND (earlier.created_at, earlier.source_notification_id) < (candidate.created_at, candidate.source_notification_id))
        AND status IN ('pending','sending') AND next_attempt_at <= now()
		ORDER BY next_attempt_at, created_at FOR UPDATE SKIP LOCKED LIMIT 1
	) UPDATE tenant_event_outbox o
	SET status = 'sending', attempts = attempts + 1,
	    next_attempt_at = now() + interval '1 minute', updated_at = now()
	FROM due WHERE o.tenant_id = due.tenant_id AND o.event_id = due.event_id
	RETURNING o.event_id, o.tenant_id::text, o.payload, o.attempts`).Scan(&job.ID, &job.TenantID, &job.Body, &job.Attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim tenant event: %w", err)
	}
	cfg, err := r.resolveTenantEventConfig(ctx, job.TenantID)
	if err != nil || cfg == nil {
		reason := "tenant_events credential missing or revoked"
		if err != nil {
			reason = "tenant_events credential unavailable: " + err.Error()
		}
		_, err = r.db.ExecContext(ctx, `UPDATE tenant_event_outbox SET status = 'pending',
    attempts = GREATEST(attempts - 1, 0), next_attempt_at = now() + interval '1 hour',
    last_error = $3, updated_at = now() WHERE tenant_id = $1::uuid AND event_id = $2`, job.TenantID, job.ID, reason)
		return true, err
	}
	// Checked at send time, so events queued before the flag changed follow the current binding.
	if !cfg.SendBatchSubscriberEvents {
		var batch bool
		err = r.db.QueryRowContext(ctx, batchSubscriberEventSQL, job.TenantID, job.ID).Scan(&batch)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return true, fmt.Errorf("lookup batch subscriber: %w", err)
		}
		if batch {
			_, err = r.db.ExecContext(ctx, `UPDATE tenant_event_outbox SET status = 'skipped', last_error = $3, updated_at = now()
    WHERE tenant_id = $1::uuid AND event_id = $2`, job.TenantID, job.ID, "BATCH_SUBSCRIBER")
			return true, err
		}
	}
	var payload tenantEventPayload
	if err := json.Unmarshal(job.Body, &payload); err != nil {
		return true, r.finishTenantEvent(ctx, job, 0, nil, "invalid stored payload", false, false)
	}
	if payload.Type == "CHARGE_SUCCESS" {
		if payload.Amount == nil {
			payload.Amount = &cfg.DefaultAmount
		}
		if payload.Currency == "" {
			payload.Currency = cfg.DefaultCurrency
		}
	}
	job.Body, err = json.Marshal(payload)
	if err != nil {
		return true, err
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(cfg.SigningSecret))
	mac.Write([]byte(ts + "." + job.ID + "."))
	mac.Write(job.Body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.URL, bytes.NewReader(job.Body))
	if err != nil {
		return true, r.finishTenantEvent(ctx, job, 0, nil, err.Error(), false, false)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Timwe-Timestamp", ts)
	req.Header.Set("X-Timwe-Event-Id", job.ID)
	req.Header.Set("X-Timwe-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	req.Header.Set("X-Timwe-Contract-Version", tenantEventContractVersion)
	resp, err := tenantWebhookHTTPClient.Do(req)
	if err != nil {
		return true, r.finishTenantEvent(ctx, job, 0, nil, err.Error(), true, false)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if err != nil {
		return true, r.finishTenantEvent(ctx, job, resp.StatusCode, nil, err.Error(), true, false)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var reply struct {
			Action string `json:"action"`
		}
		_ = json.Unmarshal(body, &reply)
		return true, r.finishTenantEvent(ctx, job, resp.StatusCode, body, "", false, reply.Action == "optout" && cfg.HonorOptoutAction)
	}
	return true, r.finishTenantEvent(ctx, job, resp.StatusCode, body,
		fmt.Sprintf("HTTP %d", resp.StatusCode), resp.StatusCode >= 500, false)
}

type cachedTenantEventConfig struct {
	Ref       string
	Config    *tenantevents.Config
	ExpiresAt time.Time
}

// Resolve the active binding on every delivery so revocation bypasses the decrypt cache.
func (r *NotificationRepository) resolveTenantEventConfig(ctx context.Context, tenantID string) (*tenantevents.Config, error) {
	var channelID, ref string
	err := r.db.QueryRowContext(ctx, activeTenantEventBindingSQL, tenantID).Scan(&channelID, &ref)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lookup active binding: %w", err)
	}
	r.tenantEventMu.Lock()
	cached, ok := r.tenantEventCache[tenantID]
	r.tenantEventMu.Unlock()
	if ok && cached.Ref == ref && time.Now().Before(cached.ExpiresAt) {
		return cached.Config, nil
	}
	var plaintext []byte
	switch {
	case strings.HasPrefix(ref, "secret://"):
		var ciphertext []byte
		err = r.db.QueryRowContext(ctx, `SELECT s.ciphertext FROM tenant_channel_secrets s
   JOIN tenant_channel_credentials c ON c.secret_ref = $4 AND c.tenant_id = s.tenant_id AND c.channel_id = s.channel_id
   WHERE s.id = $1::uuid AND s.tenant_id = $2::uuid AND s.channel_id = $3::uuid
    AND c.purpose = 'tenant_events' AND c.status = 'ACTIVE'`, strings.TrimPrefix(ref, "secret://"), tenantID, channelID, ref).Scan(&ciphertext)
		if err != nil {
			return nil, fmt.Errorf("fetch encrypted event credential: %w", err)
		}
		plaintext, err = secretcrypto.Decrypt(ciphertext)
		if err != nil {
			return nil, fmt.Errorf("decrypt event credential: %w", err)
		}
	case strings.HasPrefix(ref, "env://"):
		plaintext = []byte(os.Getenv(strings.TrimPrefix(ref, "env://")))
		if len(plaintext) == 0 {
			return nil, fmt.Errorf("event credential environment value is unset")
		}
	default:
		return nil, fmt.Errorf("unsupported event credential secret reference")
	}
	cfg, err := tenantevents.Parse(plaintext)
	if err != nil {
		return nil, err
	}
	r.tenantEventMu.Lock()
	if r.tenantEventCache == nil {
		r.tenantEventCache = make(map[string]cachedTenantEventConfig)
	}
	r.tenantEventCache[tenantID] = cachedTenantEventConfig{Ref: ref, Config: cfg, ExpiresAt: time.Now().Add(15 * time.Second)}
	r.tenantEventMu.Unlock()
	return cfg, nil
}

func retryDelay(attempt int) time.Duration {
	switch attempt {
	case 1:
		return 30 * time.Second
	case 2:
		return 2 * time.Minute
	case 3:
		return 10 * time.Minute
	case 4:
		return 30 * time.Minute
	default:
		return time.Hour
	}
}

func (r *NotificationRepository) finishTenantEvent(ctx context.Context, job dueTenantEvent, statusCode int, body []byte, lastError string, retry, optout bool) error {
	state := "failed"
	if retry {
		state = "dead"
	}
	next := time.Now()
	if statusCode >= 200 && statusCode < 300 {
		state = "delivered"
	}
	if retry && job.Attempts < 12 {
		state, next = "pending", time.Now().Add(retryDelay(job.Attempts))
	}
	_, err := r.db.ExecContext(ctx, `UPDATE tenant_event_outbox
		SET status = $3, next_attempt_at = $4, last_http_status = NULLIF($5,0),
		    last_error = NULLIF($6,''), response_body = $7, updated_at = now(),
		    optout_status = CASE WHEN $8 THEN 'pending' ELSE optout_status END,
		    optout_next_attempt_at = CASE WHEN $8 THEN now() ELSE optout_next_attempt_at END
		WHERE tenant_id = $1::uuid AND event_id = $2`, job.TenantID, job.ID, state, next, statusCode, lastError, string(body), optout)
	return err
}

func (r *NotificationRepository) dispatchOneTenantOptout(ctx context.Context) error {
	var job dueTenantEvent
	var tenantKey, channelKey, msisdn string
	var productID int
	var mcc, mnc sql.NullString
	err := r.db.QueryRowContext(ctx, `WITH due AS (
		SELECT tenant_id, event_id FROM tenant_event_outbox
		WHERE optout_status IN ('pending','sending') AND optout_next_attempt_at <= now()
		ORDER BY optout_next_attempt_at FOR UPDATE SKIP LOCKED LIMIT 1
	) UPDATE tenant_event_outbox o
	SET optout_status = 'sending', optout_attempts = optout_attempts + 1,
	    optout_next_attempt_at = now() + interval '1 minute', updated_at = now()
	FROM due WHERE o.tenant_id = due.tenant_id AND o.event_id = due.event_id
	RETURNING o.event_id, o.tenant_id::text, o.optout_attempts`).Scan(&job.ID, &job.TenantID, &job.Attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("claim tenant optout: %w", err)
	}
	// Check again at send time: the binding may have been rebound without the flag since the reply came in.
	cfg, err := r.resolveTenantEventConfig(ctx, job.TenantID)
	if err != nil {
		return r.finishTenantOptout(ctx, job, "tenant_events credential unavailable: "+err.Error(), true)
	}
	if cfg == nil || !cfg.HonorOptoutAction {
		return r.finishTenantOptout(ctx, job, tenantOptoutDisabled, false)
	}
	err = r.db.QueryRowContext(ctx, `SELECT t.tenant_key, c.channel_key, n.msisdn, n.product_id, n.mcc, n.mnc
		FROM tenant_event_outbox o
		JOIN tenants t ON t.id = o.tenant_id
		JOIN tenant_channels c ON c.id = o.channel_id AND c.tenant_id = o.tenant_id
		JOIN notifications n ON n.id = o.source_notification_id AND n.tenant_id = o.tenant_id
		WHERE o.tenant_id = $1::uuid AND o.event_id = $2`, job.TenantID, job.ID).
		Scan(&tenantKey, &channelKey, &msisdn, &productID, &mcc, &mnc)
	if err != nil {
		return r.finishTenantOptout(ctx, job, "route unavailable: "+err.Error(), false)
	}
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("SUBSCRIPTION_EXTERNAL_BASE_URL")), "/")
	trustSecret := os.Getenv("GATEWAY_TRUST_SECRET")
	if base == "" || trustSecret == "" {
		return r.finishTenantOptout(ctx, job, "optout transport unavailable", true)
	}
	optout := map[string]any{"userIdentifier": msisdn, "userIdentifierType": "MSISDN", "productId": productID}
	if mcc.Valid && mcc.String != "" {
		optout["mcc"] = mcc.String
	}
	if mnc.Valid && mnc.String != "" {
		optout["mnc"] = mnc.String
	}
	payload, _ := json.Marshal(optout)
	endpoint := base + "/api/v1/subscription-external/partners/optout?tenant_key=" + url.QueryEscape(tenantKey) + "&channel_key=" + url.QueryEscape(channelKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return r.finishTenantOptout(ctx, job, err.Error(), true)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(tenantctx.HeaderGatewayTrust, tenantctx.GatewayTrustToken(trustSecret))
	resp, err := tenantEventHTTPClient.Do(req)
	if err != nil {
		return r.finishTenantOptout(ctx, job, err.Error(), true)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return r.finishTenantOptout(ctx, job, fmt.Sprintf("HTTP %d", resp.StatusCode), resp.StatusCode >= 500)
	}
	return r.finishTenantOptout(ctx, job, "HTTP "+strconv.Itoa(resp.StatusCode), false)
}

func (r *NotificationRepository) finishTenantOptout(ctx context.Context, job dueTenantEvent, outcome string, retry bool) error {
	state := "done"
	next := time.Now()
	if retry && job.Attempts < 12 {
		state, next = "pending", time.Now().Add(retryDelay(job.Attempts))
	}
	if (retry && job.Attempts >= 12) || (outcome != "" && !strings.HasPrefix(outcome, "HTTP 2") && !retry) {
		state = "failed"
	}
	_, err := r.db.ExecContext(ctx, `UPDATE tenant_event_outbox
		SET optout_status = $3, optout_next_attempt_at = $4, optout_outcome = $5, updated_at = now()
		WHERE tenant_id = $1::uuid AND event_id = $2`, job.TenantID, job.ID, state, next, outcome)
	return err
}
