package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/seidu626/subscription-manager/common/pii"
	"go.uber.org/zap"
)

// tenantSMSCredentialPurpose is the tenant_channel_credentials.purpose value
// carrying a tenant's outbound SMS gateway configuration. It reuses the same
// table, ACTIVE-status lifecycle, and secret_ref resolution (secret:// via
// tenant_channel_secrets AES-256-GCM, env:// via environment JSON) as the
// provider_api purpose used for TIMWE opt-in credentials, so it is bound and
// rotated through the existing BindChannelCredential admin flow.
const tenantSMSCredentialPurpose = "sms_api"

// defaultOTPMessageTemplate is used when the credential blob does not set
// message_template. {{code}} is the only placeholder.
const defaultOTPMessageTemplate = "Your Dayline login code is {{code}}. It expires in 5 minutes."

// smsGatewayConfig mirrors the JSON blob stored for an sms_api credential.
// The gateway is deliberately generic: any HTTP SMS aggregator (Arkesel,
// Hubtel, mNotify, ...) is described by url + headers + templates with
// {{msisdn}}, {{text}} and {{sender}} placeholders, so onboarding a provider
// is configuration, not code. Two API shapes are supported:
//   - JSON-body APIs (e.g. Arkesel v2): placeholders in body_template,
//     values JSON-escaped.
//   - Query-param APIs (e.g. Arkesel v1 /sms/api?...&to={{msisdn}}&sms={{text}}):
//     placeholders in url, values URL-encoded; body_template omitted.
type smsGatewayConfig struct {
	URL             string            `json:"url"`
	Method          string            `json:"method"`
	Headers         map[string]string `json:"headers"`
	BodyTemplate    string            `json:"body_template"`
	SenderID        string            `json:"sender_id"`
	MessageTemplate string            `json:"message_template"`
	// SuccessBodyContains, when set, requires the gateway's 2xx response body
	// to contain this substring. Needed for gateways that report errors with
	// HTTP 200 and a code field (Arkesel v1 does).
	SuccessBodyContains string `json:"success_body_contains"`
}

func (c *smsGatewayConfig) validate() error {
	if strings.TrimSpace(c.URL) == "" {
		return fmt.Errorf("sms gateway config: url is required")
	}
	if !strings.HasPrefix(c.URL, "https://") && !strings.HasPrefix(c.URL, "http://") {
		return fmt.Errorf("sms gateway config: url must be http(s)")
	}
	if strings.TrimSpace(c.BodyTemplate) == "" && !strings.Contains(c.URL, "{{") {
		return fmt.Errorf("sms gateway config: either body_template or url placeholders are required")
	}
	return nil
}

// TenantSMSSender delivers login OTP codes through the SMS gateway configured
// per tenant (tenant_channel_credentials purpose sms_api). It implements
// OTPSender. A tenant without an ACTIVE sms_api credential fails closed:
// RequestOTP surfaces PROVIDER_ERROR and nothing is sent.
type TenantSMSSender struct {
	db     *sql.DB
	client *http.Client
	logger *zap.Logger

	// resolveConfig is swappable in same-package tests (mirrors SetClock on
	// AppOTPService); production always uses resolveGatewayConfig.
	resolveConfig func(ctx context.Context, tenantKey string) (*smsGatewayConfig, error)
}

// NewTenantSMSSender returns a sender backed by db for credential resolution.
func NewTenantSMSSender(db *sql.DB, logger *zap.Logger) *TenantSMSSender {
	s := &TenantSMSSender{
		db:     db,
		client: &http.Client{Timeout: 10 * time.Second},
		logger: logger,
	}
	s.resolveConfig = s.resolveGatewayConfig
	return s
}

// SendLoginOTP renders and posts the OTP SMS via the tenant's gateway. The
// OTP code is never logged.
func (s *TenantSMSSender) SendLoginOTP(msisdn, tenantKey, code string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cfg, err := s.resolveConfig(ctx, tenantKey)
	if err != nil {
		return err
	}

	messageTemplate := cfg.MessageTemplate
	if strings.TrimSpace(messageTemplate) == "" {
		messageTemplate = defaultOTPMessageTemplate
	}
	text := strings.ReplaceAll(messageTemplate, "{{code}}", code)

	values := map[string]string{
		"msisdn": msisdn,
		"text":   text,
		"sender": cfg.SenderID,
	}
	reqURL := renderURLTemplate(cfg.URL, values)

	var bodyReader io.Reader
	hasBody := strings.TrimSpace(cfg.BodyTemplate) != ""
	if hasBody {
		bodyReader = strings.NewReader(renderJSONTemplate(cfg.BodyTemplate, values))
	}

	method := strings.ToUpper(strings.TrimSpace(cfg.Method))
	if method == "" {
		if hasBody {
			method = http.MethodPost
		} else {
			method = http.MethodGet
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		// Never wrap the raw error: it can embed the full URL, whose query
		// string may carry the gateway api key.
		return fmt.Errorf("build sms gateway request: %s", redactURLInError(err, reqURL))
	}
	if hasBody {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("sms gateway request failed: %s", redactURLInError(err, reqURL))
	}
	defer resp.Body.Close()

	// The response snippet is for failure diagnostics only; scrub the OTP
	// code in case the gateway echoes the message back.
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	safeSnippet := strings.ReplaceAll(string(snippet), code, "******")

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		s.logger.Warn("sms gateway rejected otp send",
			zap.String("tenant_key", tenantKey),
			zap.String("msisdn", pii.MaskMSISDN(msisdn)),
			zap.Int("status", resp.StatusCode),
			zap.String("response", safeSnippet),
		)
		return fmt.Errorf("sms gateway status %d", resp.StatusCode)
	}

	if cfg.SuccessBodyContains != "" && !strings.Contains(string(snippet), cfg.SuccessBodyContains) {
		s.logger.Warn("sms gateway response did not indicate success",
			zap.String("tenant_key", tenantKey),
			zap.String("msisdn", pii.MaskMSISDN(msisdn)),
			zap.String("response", safeSnippet),
		)
		return fmt.Errorf("sms gateway response missing success marker")
	}

	s.logger.Info("app login otp sms dispatched",
		zap.String("tenant_key", tenantKey),
		zap.String("msisdn", pii.MaskMSISDN(msisdn)),
	)
	return nil
}

// resolveGatewayConfig loads and decrypts the tenant's ACTIVE sms_api
// credential blob. Resolution order and semantics match the provider_api
// pattern in GetChannelAccountConfig / subscription-external's composite
// resolver: secret:// decrypts from tenant_channel_secrets, env:// reads a
// JSON blob from the named environment variable.
func (s *TenantSMSSender) resolveGatewayConfig(ctx context.Context, tenantKey string) (*smsGatewayConfig, error) {
	var tenantID, channelID, secretRef string
	err := s.db.QueryRowContext(ctx, `
		SELECT cred.tenant_id, cred.channel_id, cred.secret_ref
		FROM tenant_channel_credentials cred
		JOIN tenants t ON t.id = cred.tenant_id
		WHERE t.tenant_key = $1
		  AND cred.purpose = $2
		  AND cred.status = 'ACTIVE'
		ORDER BY cred.version DESC, cred.channel_id
		LIMIT 1
	`, tenantKey, tenantSMSCredentialPurpose).Scan(&tenantID, &channelID, &secretRef)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no ACTIVE %s credential for tenant %q", tenantSMSCredentialPurpose, tenantKey)
	}
	if err != nil {
		return nil, fmt.Errorf("fetch %s credential: %w", tenantSMSCredentialPurpose, err)
	}

	var plaintext []byte
	switch {
	case strings.HasPrefix(secretRef, "secret://"):
		plaintext, err = GetChannelCredentialSecret(ctx, s.db, strings.TrimPrefix(secretRef, "secret://"), tenantID, channelID)
		if err != nil {
			return nil, fmt.Errorf("decrypt %s credential: %w", tenantSMSCredentialPurpose, err)
		}
	case strings.HasPrefix(secretRef, "env://"):
		name := strings.TrimPrefix(secretRef, "env://")
		v := os.Getenv(name)
		if v == "" {
			return nil, fmt.Errorf("%s credential env reference %q is unset", tenantSMSCredentialPurpose, name)
		}
		plaintext = []byte(v)
	default:
		return nil, fmt.Errorf("unsupported secret ref scheme for %s credential", tenantSMSCredentialPurpose)
	}

	var cfg smsGatewayConfig
	if err := json.Unmarshal(plaintext, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s credential blob: %w", tenantSMSCredentialPurpose, err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// renderURLTemplate substitutes {{key}} placeholders with URL-encoded values
// for query-param style gateways (Arkesel v1 and similar).
func renderURLTemplate(template string, values map[string]string) string {
	out := template
	for k, v := range values {
		out = strings.ReplaceAll(out, "{{"+k+"}}", url.QueryEscape(v))
	}
	return out
}

// redactURLInError strips the query string (where gateway api keys live) from
// any occurrence of reqURL inside err's message.
func redactURLInError(err error, reqURL string) string {
	msg := err.Error()
	if u, parseErr := url.Parse(reqURL); parseErr == nil && u.RawQuery != "" {
		u.RawQuery = "REDACTED"
		msg = strings.ReplaceAll(msg, reqURL, u.String())
	}
	return msg
}

// renderJSONTemplate substitutes {{key}} placeholders with JSON-escaped
// values so operator templates stay valid JSON regardless of message content.
// The escaped value is the json.Marshal string form minus its surrounding
// quotes, because placeholders sit inside quoted template fields.
func renderJSONTemplate(template string, values map[string]string) string {
	out := template
	for k, v := range values {
		escaped, _ := json.Marshal(v)
		out = strings.ReplaceAll(out, "{{"+k+"}}", string(escaped[1:len(escaped)-1]))
	}
	return out
}
