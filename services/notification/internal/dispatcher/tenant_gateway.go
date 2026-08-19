package dispatcher

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/seidu626/subscription-manager/common/pii"
	"github.com/seidu626/subscription-manager/common/secretcrypto"
	"go.uber.org/zap"
)

// tenantGatewayCredentialPurpose is the tenant_channel_credentials.purpose
// value carrying a tenant's outbound SMS gateway configuration. Mirrors the
// sms_api purpose used by acquisition-api's TenantSMSSender (login OTP); this
// is the content-SMS counterpart for the notification outbox worker.
const tenantGatewayCredentialPurpose = "sms_api"

// tenantGatewayCacheTTL bounds how long a resolved (or absent) tenant gateway
// config is reused. A batch of hundreds of jobs for the same tenant should
// not pay a DB round-trip and a decrypt per job.
const tenantGatewayCacheTTL = 5 * time.Minute

// smsGatewayConfig mirrors the JSON blob stored for an sms_api credential.
// Copied from acquisition-api/internal/service/app_otp_sms_sender.go (a
// separate Go module, so it cannot be imported) to keep the two content vs.
// OTP send paths independently deployable. The gateway is deliberately
// generic: any HTTP SMS aggregator (Arkesel, Hubtel, mNotify, ...) is
// described by url + headers + templates with {{msisdn}}, {{text}} and
// {{sender}} placeholders, so onboarding a provider is configuration, not
// code.
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
	// SuccessField and SuccessValue, when set, require the gateway's 2xx JSON
	// response to carry that top-level field with that value (Arkesel v2:
	// status=success). Preferred over SuccessBodyContains on JSON gateways:
	// a substring match depends on the gateway's key order and whitespace.
	SuccessField string `json:"success_field"`
	SuccessValue string `json:"success_value"`
	// MessageIDPath, when set, is a dot path (array indices as numbers, e.g.
	// "data.0.id") into the send response JSON naming the gateway's message
	// id. The id is stored on the outbox job so the delivery poller can
	// resolve the true handset delivery outcome later; a 2xx "accepted"
	// response alone has been observed on prod to not mean delivered.
	MessageIDPath string `json:"message_id_path"`
	// StatusURL, when set, enables delivery polling: a GET endpoint template
	// with a {{message_id}} placeholder (e.g. Arkesel v2
	// "https://sms.arkesel.com/api/v2/sms/{{message_id}}"), called with the
	// same Headers as the send.
	StatusURL string `json:"status_url"`
	// StatusPath is the dot path into the status response JSON naming the
	// delivery state string (e.g. "data.0.status").
	StatusPath string `json:"status_path"`
	// StatusDeliveredValues / StatusFailedValues classify the StatusPath
	// value (case-insensitive). Anything else counts as still pending.
	StatusDeliveredValues []string `json:"status_delivered_values"`
	StatusFailedValues    []string `json:"status_failed_values"`
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
	if (c.SuccessField == "") != (c.SuccessValue == "") {
		return fmt.Errorf("sms gateway config: success_field and success_value must be set together")
	}
	return nil
}

// checkSuccessBody applies the configured success markers to a 2xx response
// body. Gateways that signal failure inside a 200 response are the reason
// both markers exist; a config with neither trusts the HTTP status alone.
func (c *smsGatewayConfig) checkSuccessBody(body []byte) error {
	if c.SuccessBodyContains != "" && !strings.Contains(string(body), c.SuccessBodyContains) {
		return fmt.Errorf("sms gateway response missing success marker")
	}
	if c.SuccessField == "" {
		return nil
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fmt.Errorf("sms gateway response is not JSON object")
	}
	got, ok := parsed[c.SuccessField]
	if !ok {
		return fmt.Errorf("sms gateway response has no %q field", c.SuccessField)
	}
	// fmt.Sprint so numeric status codes compare as written in config.
	if fmt.Sprint(got) != c.SuccessValue {
		return fmt.Errorf("sms gateway reported %s=%v", c.SuccessField, got)
	}
	return nil
}

// cachedGatewayConfig holds a Resolve result (config or "no binding") plus
// its cache expiry. cfg == nil is a valid, cacheable outcome: it means the
// tenant has no ACTIVE sms_api credential and jobs should fall back to MT.
type cachedGatewayConfig struct {
	cfg       *smsGatewayConfig
	expiresAt time.Time
}

// TenantGatewaySender resolves and sends content SMS through a tenant's
// configured HTTP gateway (tenant_channel_credentials purpose sms_api). It
// implements the dispatcher's TenantGateway interface. A tenant without an
// ACTIVE sms_api credential is not an error: Resolve returns (nil, nil) and
// the dispatcher falls back to the TIMWE MT path.
type TenantGatewaySender struct {
	db     *sql.DB
	client *http.Client
	logger *zap.Logger

	mu    sync.Mutex
	cache map[string]cachedGatewayConfig

	// fetchConfig is swappable in tests so Resolve's caching behavior can be
	// exercised without a real DB (mirrors resolveConfig in
	// acquisition-api/internal/service/app_otp_sms_sender.go).
	fetchConfig func(ctx context.Context, tenantID string) (*smsGatewayConfig, error)
}

// NewTenantGatewaySender returns a sender backed by db for credential
// resolution, with a 10s HTTP client timeout matching the acquisition-api
// counterpart.
func NewTenantGatewaySender(db *sql.DB, logger *zap.Logger) *TenantGatewaySender {
	s := &TenantGatewaySender{
		db:     db,
		client: &http.Client{Timeout: 10 * time.Second},
		logger: logger,
		cache:  make(map[string]cachedGatewayConfig),
	}
	s.fetchConfig = s.fetchGatewayConfig
	return s
}

// Resolve returns the tenant's ACTIVE sms_api gateway config, or (nil, nil)
// when the tenant has no such binding. Both outcomes are cached for
// tenantGatewayCacheTTL so a large batch of jobs for one tenant pays at most
// one DB/decrypt round-trip per cache window. A resolution error (decrypt
// failure, malformed blob) is never cached, so a transient failure does not
// pin the tenant to "no gateway" for the cache window.
func (s *TenantGatewaySender) Resolve(ctx context.Context, tenantID string) (*smsGatewayConfig, error) {
	s.mu.Lock()
	if entry, ok := s.cache[tenantID]; ok && time.Now().Before(entry.expiresAt) {
		s.mu.Unlock()
		return entry.cfg, nil
	}
	s.mu.Unlock()

	cfg, err := s.fetchConfig(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.cache[tenantID] = cachedGatewayConfig{cfg: cfg, expiresAt: time.Now().Add(tenantGatewayCacheTTL)}
	s.mu.Unlock()
	return cfg, nil
}

// fetchGatewayConfig loads, decrypts and parses the tenant's ACTIVE sms_api
// credential blob.
func (s *TenantGatewaySender) fetchGatewayConfig(ctx context.Context, tenantID string) (*smsGatewayConfig, error) {
	plaintext, err := resolveTenantGatewayCredentialBlob(ctx, s.db, tenantID)
	if err != nil {
		return nil, err
	}
	if plaintext == nil {
		return nil, nil
	}

	var cfg smsGatewayConfig
	if err := json.Unmarshal(plaintext, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s credential blob: %w", tenantGatewayCredentialPurpose, err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// resolveTenantGatewayCredentialBlob loads and decrypts the plaintext blob of
// a tenant's ACTIVE sms_api credential, keyed by tenant UUID (the outbox job
// carries *TenantID as a UUID, not a tenant_key). Resolution order and
// semantics mirror resolveTenantCredentialBlob in
// acquisition-api/internal/service/app_otp_sms_sender.go: secret:// decrypts
// from tenant_channel_secrets, env:// reads a JSON blob from the named
// environment variable. No ACTIVE credential yields (nil, nil), so callers
// can treat absence as "this tenant does not use a gateway" without
// conflating it with a resolution failure.
func resolveTenantGatewayCredentialBlob(ctx context.Context, db *sql.DB, tenantID string) ([]byte, error) {
	var gotTenantID, channelID, secretRef string
	err := db.QueryRowContext(ctx, `
		SELECT tenant_id, channel_id, secret_ref
		FROM tenant_channel_credentials
		WHERE tenant_id = $1::uuid
		  AND purpose = 'sms_api'
		  AND status = 'ACTIVE'
		ORDER BY version DESC, channel_id
		LIMIT 1
	`, tenantID).Scan(&gotTenantID, &channelID, &secretRef)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fetch %s credential: %w", tenantGatewayCredentialPurpose, err)
	}

	switch {
	case strings.HasPrefix(secretRef, "secret://"):
		plaintext, err := fetchTenantGatewaySecret(ctx, db, strings.TrimPrefix(secretRef, "secret://"), gotTenantID, channelID)
		if err != nil {
			return nil, fmt.Errorf("decrypt %s credential: %w", tenantGatewayCredentialPurpose, err)
		}
		return plaintext, nil
	case strings.HasPrefix(secretRef, "env://"):
		name := strings.TrimPrefix(secretRef, "env://")
		v := os.Getenv(name)
		if v == "" {
			return nil, fmt.Errorf("%s credential env reference %q is unset", tenantGatewayCredentialPurpose, name)
		}
		return []byte(v), nil
	default:
		return nil, fmt.Errorf("unsupported secret ref scheme for %s credential", tenantGatewayCredentialPurpose)
	}
}

// fetchTenantGatewaySecret looks up and decrypts the tenant_channel_secrets
// row for id, scoped to (tenantID, channelID) so a mismatch returns
// sql.ErrNoRows rather than a cross-tenant plaintext read. Mirrors
// GetChannelCredentialSecret in
// acquisition-api/internal/service/channel_credential_secret_store.go.
func fetchTenantGatewaySecret(ctx context.Context, db *sql.DB, id, tenantID, channelID string) ([]byte, error) {
	var ciphertext []byte
	err := db.QueryRowContext(ctx, `
		SELECT s.ciphertext
		FROM tenant_channel_secrets s
		JOIN tenant_channel_credentials c
		  ON c.secret_ref = $4
		 AND c.tenant_id  = s.tenant_id
		 AND c.channel_id = s.channel_id
		WHERE s.id        = $1::uuid
		  AND s.tenant_id = $2::uuid
		  AND s.channel_id = $3::uuid
	`, id, tenantID, channelID, "secret://"+id).Scan(&ciphertext)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("secret not found: %s", id)
		}
		return nil, fmt.Errorf("fetch secret: %w", err)
	}
	return secretcrypto.Decrypt(ciphertext)
}

// Send posts text to msisdn through the gateway described by cfg. The full
// request URL and headers are never logged: query strings and headers can
// carry the gateway API key. Only a masked msisdn is logged. On success it
// returns the gateway's message id when cfg.MessageIDPath resolves one;
// otherwise the id is empty and the job stays delivery-untracked.
func (s *TenantGatewaySender) Send(ctx context.Context, cfg *smsGatewayConfig, msisdn, text string) (string, error) {
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
		return "", fmt.Errorf("build sms gateway request: %s", redactURLInError(err, reqURL))
	}
	if hasBody {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("sms gateway request failed: %s", redactURLInError(err, reqURL))
	}
	defer resp.Body.Close()

	// Read enough of the body for the JSON success check; the truncated
	// snippet is for failure diagnostics only.
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	safeSnippet := string(body)
	if len(safeSnippet) > 512 {
		safeSnippet = safeSnippet[:512]
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		s.logger.Warn("tenant sms gateway rejected content send",
			zap.String("msisdn", pii.MaskMSISDN(msisdn)),
			zap.Int("status", resp.StatusCode),
			zap.String("response", safeSnippet),
		)
		return "", fmt.Errorf("sms gateway status %d", resp.StatusCode)
	}

	if err := cfg.checkSuccessBody(body); err != nil {
		s.logger.Warn("tenant sms gateway response did not indicate success",
			zap.String("msisdn", pii.MaskMSISDN(msisdn)),
			zap.String("response", safeSnippet),
		)
		return "", err
	}

	messageID := ""
	if cfg.MessageIDPath != "" {
		messageID, err = extractJSONPath(body, cfg.MessageIDPath)
		if err != nil {
			// The send itself succeeded; a missing id only loses delivery
			// tracking for this one message, so log rather than fail the job
			// (failing would resend an SMS the subscriber already got).
			s.logger.Warn("tenant sms gateway response missing message id",
				zap.String("msisdn", pii.MaskMSISDN(msisdn)),
				zap.String("message_id_path", cfg.MessageIDPath),
				zap.String("response", safeSnippet),
			)
			messageID = ""
		}
	}
	return messageID, nil
}

// renderURLTemplate substitutes {{key}} placeholders with URL-encoded values
// for query-param style gateways.
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

// deliveryStatusMaxBody bounds how much of a gateway status response is read.
const deliveryStatusMaxBody = 8192

// DeliveryStatus queries the gateway for the handset delivery state of a
// previously sent message. It returns one of "DELIVERED", "FAILED" or
// "PENDING" plus the gateway's raw status string for diagnostics. Requires
// cfg.StatusURL and cfg.StatusPath; callers must not poll configs without
// them.
func (s *TenantGatewaySender) DeliveryStatus(ctx context.Context, cfg *smsGatewayConfig, messageID string) (string, string, error) {
	if strings.TrimSpace(cfg.StatusURL) == "" || strings.TrimSpace(cfg.StatusPath) == "" {
		return "", "", fmt.Errorf("sms gateway config has no delivery status endpoint")
	}
	reqURL := renderURLTemplate(cfg.StatusURL, map[string]string{"message_id": messageID})

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("build sms gateway status request: %s", redactURLInError(err, reqURL))
	}
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("sms gateway status request failed: %s", redactURLInError(err, reqURL))
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, deliveryStatusMaxBody))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(body)
		if len(snippet) > 256 {
			snippet = snippet[:256]
		}
		return "", "", fmt.Errorf("sms gateway status endpoint returned %d: %s", resp.StatusCode, snippet)
	}

	raw, err := extractJSONPath(body, cfg.StatusPath)
	if err != nil {
		return "", "", fmt.Errorf("sms gateway status response: %w", err)
	}
	return classifyDeliveryStatus(cfg, raw), raw, nil
}

// classifyDeliveryStatus maps a gateway status string onto our tri-state via
// the configured value lists (case-insensitive). Unrecognized values stay
// PENDING so a new gateway wording never falsely finalizes a job; the poller
// stops rechecking on its age cutoff regardless.
func classifyDeliveryStatus(cfg *smsGatewayConfig, raw string) string {
	for _, v := range cfg.StatusDeliveredValues {
		if strings.EqualFold(strings.TrimSpace(raw), strings.TrimSpace(v)) {
			return deliveryStatusDelivered
		}
	}
	for _, v := range cfg.StatusFailedValues {
		if strings.EqualFold(strings.TrimSpace(raw), strings.TrimSpace(v)) {
			return deliveryStatusFailed
		}
	}
	return deliveryStatusPending
}

// extractJSONPath resolves a dot path ("data.0.id") through nested JSON
// objects and arrays and returns the value at the leaf rendered as a string.
func extractJSONPath(body []byte, path string) (string, error) {
	var current any
	if err := json.Unmarshal(body, &current); err != nil {
		return "", fmt.Errorf("response is not JSON")
	}
	for _, segment := range strings.Split(path, ".") {
		switch node := current.(type) {
		case map[string]any:
			val, ok := node[segment]
			if !ok {
				return "", fmt.Errorf("path %q not found in response", path)
			}
			current = val
		case []any:
			idx, err := strconv.Atoi(segment)
			if err != nil || idx < 0 || idx >= len(node) {
				return "", fmt.Errorf("path %q not found in response", path)
			}
			current = node[idx]
		default:
			return "", fmt.Errorf("path %q not found in response", path)
		}
	}
	switch leaf := current.(type) {
	case string:
		if strings.TrimSpace(leaf) == "" {
			return "", fmt.Errorf("path %q resolved to empty value", path)
		}
		return leaf, nil
	case float64, bool:
		return fmt.Sprint(leaf), nil
	default:
		return "", fmt.Errorf("path %q resolved to a non-scalar value", path)
	}
}
