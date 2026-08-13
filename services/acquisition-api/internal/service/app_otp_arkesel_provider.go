package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/common/pii"
	"go.uber.org/zap"
)

// tenantOTPCredentialPurpose is the tenant_channel_credentials.purpose value
// carrying a tenant's delegated-OTP configuration. Its presence is what puts a
// tenant in delegated mode; tenants without it keep the local OTP lifecycle.
const tenantOTPCredentialPurpose = "otp_api"

// arkeselOTPCodeSlot is the placeholder Arkesel substitutes with the generated
// code. A message without it is rejected by the provider (code 1002), so we
// reject it at config-validation time instead.
const arkeselOTPCodeSlot = "%otp_code%"

const defaultArkeselOTPMessage = "Your Dayline login code is %otp_code%. It expires in %expiry% minutes."

// Arkesel OTP response codes (see the One Time Password section of
// https://developers.arkesel.com). The provider returns these inside HTTP 200,
// so the body, not the status, carries the outcome.
const (
	arkeselOTPGenerateOK   = "1000"
	arkeselOTPVerifyOK     = "1100"
	arkeselOTPInvalidCode  = "1104"
	arkeselOTPCodeExpired  = "1105"
	arkeselOTPBadNumberGen = "1005"
)

// Sentinel outcomes a delegated OTP provider distinguishes for the caller.
// Every other failure is an opaque provider error: the login path must not
// leak a provider's internal taxonomy to the app.
var (
	// ErrDelegatedOTPCodeInvalid means the provider rejected the code. The
	// caller still owns the attempt ceiling; see AppOTPService.VerifyOTP.
	ErrDelegatedOTPCodeInvalid = errors.New("delegated otp code invalid")
	// ErrDelegatedOTPExpired means the provider's own code lifetime elapsed.
	ErrDelegatedOTPExpired = errors.New("delegated otp expired")
)

// arkeselOTPConfig mirrors the JSON blob stored for an otp_api credential.
// Unlike smsGatewayConfig this is deliberately provider-shaped rather than
// generic: delegated OTP is a two-call protocol whose outcomes are numeric
// body codes, and pretending that is portable across aggregators would hide
// the coupling rather than remove it.
type arkeselOTPConfig struct {
	GenerateURL     string            `json:"generate_url"`
	VerifyURL       string            `json:"verify_url"`
	Headers         map[string]string `json:"headers"`
	SenderID        string            `json:"sender_id"`
	MessageTemplate string            `json:"message_template"`
	Length          int               `json:"length"`
	ExpiryMinutes   int               `json:"expiry_minutes"`
	Medium          string            `json:"medium"`
	Type            string            `json:"type"`
}

// withDefaults fills the optional fields so an operator blob only has to carry
// the URLs, headers and sender id.
func (c *arkeselOTPConfig) withDefaults() {
	if strings.TrimSpace(c.MessageTemplate) == "" {
		c.MessageTemplate = defaultArkeselOTPMessage
	}
	if c.Length == 0 {
		c.Length = domain.AppLoginOTPCodeLength
	}
	if c.ExpiryMinutes == 0 {
		// Match the local TTL so switching modes keeps one code lifetime.
		c.ExpiryMinutes = int(domain.AppLoginOTPTTL / time.Minute)
	}
	if strings.TrimSpace(c.Medium) == "" {
		c.Medium = "sms"
	}
	if strings.TrimSpace(c.Type) == "" {
		c.Type = "numeric"
	}
}

// validate rejects blobs the provider would reject at call time, so a
// misconfigured tenant fails at resolution with a clear message rather than as
// an opaque login failure. Bounds are Arkesel's documented limits.
func (c *arkeselOTPConfig) validate() error {
	for _, u := range []struct {
		name  string
		value string
	}{{"generate_url", c.GenerateURL}, {"verify_url", c.VerifyURL}} {
		if strings.TrimSpace(u.value) == "" {
			return fmt.Errorf("otp gateway config: %s is required", u.name)
		}
		if !strings.HasPrefix(u.value, "https://") && !strings.HasPrefix(u.value, "http://") {
			return fmt.Errorf("otp gateway config: %s must be http(s)", u.name)
		}
	}
	if !strings.Contains(c.MessageTemplate, arkeselOTPCodeSlot) {
		return fmt.Errorf("otp gateway config: message_template must contain %s", arkeselOTPCodeSlot)
	}
	if c.Length < 6 || c.Length > 15 {
		return fmt.Errorf("otp gateway config: length must be 6..15")
	}
	if c.ExpiryMinutes < 1 || c.ExpiryMinutes > 10 {
		return fmt.Errorf("otp gateway config: expiry_minutes must be 1..10")
	}
	// Arkesel drops sends whose sender id exceeds 11 characters.
	if len(c.SenderID) > 11 {
		return fmt.Errorf("otp gateway config: sender_id must be at most 11 characters")
	}
	if c.Medium != "sms" && c.Medium != "voice" {
		return fmt.Errorf("otp gateway config: medium must be sms or voice")
	}
	if c.Type != "numeric" && c.Type != "alphanumeric" {
		return fmt.Errorf("otp gateway config: type must be numeric or alphanumeric")
	}
	return nil
}

// ArkeselOTPProvider delegates code generation, delivery and verification to
// Arkesel's OTP endpoints for tenants holding an ACTIVE otp_api credential.
//
// It deliberately owns only code custody. Arkesel's verify endpoint enforces
// no attempt ceiling of its own (measured 2026-08-13: 46 consecutive wrong
// codes against one live OTP all returned the same invalid-code response, with
// no lockout and no per-attempt cost), so the caller keeps the rate limit and
// the attempt counter. See AppOTPService.VerifyOTP.
type ArkeselOTPProvider struct {
	db     *sql.DB
	client *http.Client
	logger *zap.Logger

	// resolveConfig is swappable in same-package tests (mirrors the pattern on
	// TenantSMSSender); production always uses resolveOTPConfig.
	resolveConfig func(ctx context.Context, tenantKey string) (*arkeselOTPConfig, error)
}

// NewArkeselOTPProvider returns a provider backed by db for credential
// resolution.
func NewArkeselOTPProvider(db *sql.DB, logger *zap.Logger) *ArkeselOTPProvider {
	p := &ArkeselOTPProvider{
		db:     db,
		client: &http.Client{Timeout: 10 * time.Second},
		logger: logger,
	}
	p.resolveConfig = p.resolveOTPConfig
	return p
}

// Configured reports whether tenantKey delegates its OTP lifecycle. A
// resolution failure is returned rather than reported as "not configured", so
// a broken credential never silently downgrades the tenant to a different
// authentication path.
func (p *ArkeselOTPProvider) Configured(ctx context.Context, tenantKey string) (bool, error) {
	cfg, err := p.resolveConfig(ctx, tenantKey)
	if err != nil {
		return false, err
	}
	return cfg != nil, nil
}

// Generate asks the provider to mint and deliver a code. The code itself never
// reaches this process, which is the point of delegating.
func (p *ArkeselOTPProvider) Generate(ctx context.Context, msisdn, tenantKey string) error {
	cfg, err := p.resolveConfig(ctx, tenantKey)
	if err != nil {
		return err
	}
	if cfg == nil {
		return fmt.Errorf("no ACTIVE %s credential for tenant %q", tenantOTPCredentialPurpose, tenantKey)
	}

	payload := map[string]any{
		"number":    msisdn,
		"message":   cfg.MessageTemplate,
		"sender_id": cfg.SenderID,
		"medium":    cfg.Medium,
		"type":      cfg.Type,
		"length":    cfg.Length,
		"expiry":    cfg.ExpiryMinutes,
	}

	code, message, err := p.call(ctx, cfg, cfg.GenerateURL, payload, msisdn, tenantKey)
	if err != nil {
		return err
	}
	switch code {
	case arkeselOTPGenerateOK:
		p.logger.Info("delegated otp generated",
			zap.String("tenant_key", tenantKey),
			zap.String("msisdn", pii.MaskMSISDN(msisdn)),
		)
		return nil
	case arkeselOTPBadNumberGen:
		return fmt.Errorf("otp provider rejected the number")
	default:
		return fmt.Errorf("otp provider generate failed: %s (%s)", code, message)
	}
}

// Verify submits a code the user supplied. Callers must translate
// ErrDelegatedOTPCodeInvalid and ErrDelegatedOTPExpired into their own
// taxonomy, and must apply their own attempt ceiling before calling.
func (p *ArkeselOTPProvider) Verify(ctx context.Context, msisdn, tenantKey, code string) error {
	cfg, err := p.resolveConfig(ctx, tenantKey)
	if err != nil {
		return err
	}
	if cfg == nil {
		return fmt.Errorf("no ACTIVE %s credential for tenant %q", tenantOTPCredentialPurpose, tenantKey)
	}

	payload := map[string]any{"number": msisdn, "code": code}
	respCode, message, err := p.call(ctx, cfg, cfg.VerifyURL, payload, msisdn, tenantKey)
	if err != nil {
		return err
	}
	switch respCode {
	case arkeselOTPVerifyOK:
		return nil
	case arkeselOTPInvalidCode:
		return ErrDelegatedOTPCodeInvalid
	case arkeselOTPCodeExpired:
		return ErrDelegatedOTPExpired
	default:
		return fmt.Errorf("otp provider verify failed: %s (%s)", respCode, message)
	}
}

// call posts payload and returns the provider's body code and message. The
// submitted OTP code is never logged, and neither is the response body: on the
// verify call it can echo the code back.
func (p *ArkeselOTPProvider) call(
	ctx context.Context,
	cfg *arkeselOTPConfig,
	endpoint string,
	payload map[string]any,
	msisdn, tenantKey string,
) (string, string, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", "", fmt.Errorf("encode otp provider request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return "", "", fmt.Errorf("build otp provider request: %s", redactURLInError(err, endpoint))
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("otp provider request failed: %s", redactURLInError(err, endpoint))
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		p.logger.Warn("otp provider rejected request",
			zap.String("tenant_key", tenantKey),
			zap.String("msisdn", pii.MaskMSISDN(msisdn)),
			zap.Int("status", resp.StatusCode),
		)
		return "", "", fmt.Errorf("otp provider status %d", resp.StatusCode)
	}

	var parsed struct {
		Code    any    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", "", fmt.Errorf("otp provider response is not JSON object")
	}
	if parsed.Code == nil {
		return "", "", fmt.Errorf("otp provider response has no code field")
	}
	// fmt.Sprint because the provider documents string codes but is not
	// consistent about quoting them.
	return fmt.Sprint(parsed.Code), parsed.Message, nil
}

// resolveOTPConfig loads the tenant's ACTIVE otp_api credential blob, or
// (nil, nil) when the tenant has none and therefore keeps the local lifecycle.
func (p *ArkeselOTPProvider) resolveOTPConfig(ctx context.Context, tenantKey string) (*arkeselOTPConfig, error) {
	plaintext, err := resolveTenantCredentialBlob(ctx, p.db, tenantKey, tenantOTPCredentialPurpose)
	if err != nil || plaintext == nil {
		return nil, err
	}

	var cfg arkeselOTPConfig
	if err := json.Unmarshal(plaintext, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s credential blob: %w", tenantOTPCredentialPurpose, err)
	}
	cfg.withDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}
