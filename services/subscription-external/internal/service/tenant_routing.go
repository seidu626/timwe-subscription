// slice-harness: allow-new-canonical-path: TMP-007 canonical tenant provider routing module.
package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/lib/pq"
	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/seidu626/subscription-manager/subscription-external/internal/utils"
)

const (
	tenantCredentialPurposeProviderAPI     = "provider_api"
	tenantProviderCredentialDevFallbackEnv = "SUBSCRIPTION_EXTERNAL_DEV_ALLOW_TENANT_PROVIDER_SHARED_CREDENTIAL_FALLBACK"
)

var (
	ErrTenantRoutingRequired       = errors.New("tenant/channel context is required")
	ErrTenantRoutingNotConfigured  = errors.New("tenant routing is not configured")
	ErrTenantChannelNotFound       = errors.New("tenant channel not found")
	ErrUnsupportedChannelOperation = errors.New("unsupported_channel_operation")
	ErrTenantCredentialMissing     = errors.New("tenant channel credential missing")
	ErrTenantCredentialInvalid     = errors.New("tenant channel credential invalid")
)

type ChannelOperation string

const (
	ChannelOperationOptin   ChannelOperation = "optin"
	ChannelOperationMT      ChannelOperation = "mt"
	ChannelOperationCharge  ChannelOperation = "charge"
	ChannelOperationConfirm ChannelOperation = "confirm"
	ChannelOperationStatus  ChannelOperation = "status"
	ChannelOperationOptout  ChannelOperation = "optout"
)

type TenantProviderConfig struct {
	TenantID         string
	ChannelID        string
	Provider         string
	BaseURL          string
	APIKey           string
	Authentication   string
	PartnerServiceID string
	PSK              string
	PartnerRoleID    string
	Realm            string
	SecretRefDisplay string
	// Extended per-tenant account config. Empty string means "not set by tenant";
	// callers use the global config / hardcoded defaults as fallback.
	MTAPIKey             string
	MCC                  string
	MNC                  string
	// LargeAccount overrides the outbound TIMWE largeAccount/shortCode sender field.
	// Precedence: request-level value wins; this tenant value is the fallback; empty = use legacy default.
	LargeAccount         string
	ServiceName          string // store-through only — TODO: wire to request building
	// FreeMTPricepointID is the pricepoint to use for free/zero-rated MT sends.
	// Precedence: product-level PricepointID (non-zero) wins; this value is the fallback.
	FreeMTPricepointID   string
	// MOPricepointIDs is a comma-separated list of valid MO pricepoint IDs for this tenant channel.
	// Precedence: request-level PricepointID (non-zero) wins; first entry here is the fallback.
	MOPricepointIDs      string
	// BillingPricepointIDs is a comma-separated list of billing/DOB pricepoint IDs for this tenant channel.
	// Precedence: request-level PricepointID (non-zero) wins; first entry here is the fallback.
	BillingPricepointIDs string
	HEIVParamSpecKey     string // store-through only — TODO: wire to request building
}

func (c TenantProviderConfig) AuthKey() (string, error) {
	if strings.TrimSpace(c.Authentication) != "" {
		return c.Authentication, nil
	}
	if strings.TrimSpace(c.PartnerServiceID) == "" || strings.TrimSpace(c.PSK) == "" {
		return "", fmt.Errorf("%w: authentication material missing", ErrTenantCredentialInvalid)
	}
	return utils.GetCachedAuthKey(c.PartnerServiceID, c.PSK)
}

func (c TenantProviderConfig) PartnerRoleInt() (int, error) {
	value := strings.TrimSpace(c.PartnerRoleID)
	if value == "" {
		return 0, fmt.Errorf("%w: partner role missing", ErrTenantCredentialInvalid)
	}
	role, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid partner role: %w", ErrTenantCredentialInvalid, err)
	}
	return role, nil
}

type ProviderCredentialSecret struct {
	BaseURL              string `json:"base_url"`
	APIKey               string `json:"api_key"`
	AuthenticationKey    string `json:"authentication_key"`
	PartnerServiceID     string `json:"partner_service_id"`
	PSK                  string `json:"psk"`
	PartnerRoleID        string `json:"partner_role_id"`
	Realm                string `json:"realm"`
	// Extended per-tenant account config (all optional; zero value = absent).
	MTAPIKey              string `json:"mt_api_key"`
	MCC                   string `json:"mcc"`
	MNC                   string `json:"mnc"`
	LargeAccount          string `json:"large_account"`
	ServiceName           string `json:"service_name"`
	FreeMTPricepointID    string `json:"free_mt_pricepoint_id"`
	MOPricepointIDs       string `json:"mo_pricepoint_ids"`
	BillingPricepointIDs  string `json:"billing_pricepoint_ids"`
	HEIVParamSpecKey      string `json:"he_iv_param_spec_key"`
}

type ProviderCredentialResolver interface {
	// ResolveProviderCredential decrypts/fetches credentials for the given secret ref.
	// tenantID and channelID are used by the DB resolver to scope the lookup to the
	// owning tenant/channel (defence-in-depth against cross-tenant access). Resolvers
	// that do not perform a DB lookup (e.g. env://) may ignore them.
	ResolveProviderCredential(ctx context.Context, secretRef, tenantID, channelID string) (ProviderCredentialSecret, error)
}

type EnvProviderCredentialResolver struct{}

func (EnvProviderCredentialResolver) ResolveProviderCredential(ctx context.Context, secretRef, _, _ string) (ProviderCredentialSecret, error) {
	_ = ctx
	const prefix = "env://"
	if !strings.HasPrefix(secretRef, prefix) {
		return ProviderCredentialSecret{}, fmt.Errorf("%w: unsupported secret reference", ErrTenantCredentialInvalid)
	}
	envName := strings.TrimSpace(strings.TrimPrefix(secretRef, prefix))
	if envName == "" {
		return ProviderCredentialSecret{}, fmt.Errorf("%w: empty env reference", ErrTenantCredentialInvalid)
	}
	raw, ok := os.LookupEnv(envName)
	if !ok || strings.TrimSpace(raw) == "" {
		return ProviderCredentialSecret{}, fmt.Errorf("%w: env reference not found", ErrTenantCredentialMissing)
	}
	var secret ProviderCredentialSecret
	if err := json.Unmarshal([]byte(raw), &secret); err != nil {
		return ProviderCredentialSecret{}, fmt.Errorf("%w: invalid env credential json: %w", ErrTenantCredentialInvalid, err)
	}
	return secret, nil
}

type TenantProviderRouter struct {
	db                  *sql.DB
	cfg                 *config.Config
	credentialResolver  ProviderCredentialResolver
	operationCapability map[ChannelOperation][]string
}

type TenantProviderResolver interface {
	Resolve(ctx context.Context, operation ChannelOperation, route domain.TenantRouteContext) (*TenantProviderConfig, error)
}

func (s *SubscriptionService) SetTenantProviderRouter(router TenantProviderResolver) {
	s.tenantRouter = router
}

func (s *SubscriptionService) providerConfigForRoute(ctx context.Context, operation ChannelOperation, route domain.TenantRouteContext) (*TenantProviderConfig, error) {
	if routeMissingTenant(route) || routeMissingChannel(route) {
		return nil, ErrTenantRoutingRequired
	}
	if s.tenantRouter == nil {
		return nil, ErrTenantRoutingNotConfigured
	}
	return s.tenantRouter.Resolve(ctx, operation, route)
}

func routeMissingTenant(route domain.TenantRouteContext) bool {
	return strings.TrimSpace(route.TenantID) == "" && strings.TrimSpace(route.TenantKey) == ""
}

func routeMissingChannel(route domain.TenantRouteContext) bool {
	return strings.TrimSpace(route.ChannelID) == "" && strings.TrimSpace(route.ChannelKey) == ""
}

func canonicalTenantRoute(route domain.TenantRouteContext, cfg *TenantProviderConfig) domain.TenantRouteContext {
	if cfg == nil {
		return route
	}
	route.TenantID = cfg.TenantID
	route.ChannelID = cfg.ChannelID
	return route
}

func NewTenantProviderRouter(db *sql.DB, cfg *config.Config, resolver ProviderCredentialResolver) *TenantProviderRouter {
	if resolver == nil {
		resolver = EnvProviderCredentialResolver{}
	}
	return &TenantProviderRouter{
		db:                 db,
		cfg:                cfg,
		credentialResolver: resolver,
		operationCapability: map[ChannelOperation][]string{
			ChannelOperationOptin:   []string{"optin"},
			ChannelOperationMT:      []string{"mt", "optin"},
			ChannelOperationCharge:  []string{"charge"},
			ChannelOperationConfirm: []string{"confirm"},
			ChannelOperationStatus:  []string{"optin"},
			ChannelOperationOptout:  []string{"optin"},
		},
	}
}

func (r *TenantProviderRouter) Resolve(ctx context.Context, operation ChannelOperation, route domain.TenantRouteContext) (*TenantProviderConfig, error) {
	if r == nil || r.db == nil {
		return nil, ErrTenantRoutingNotConfigured
	}
	if route.TenantID == "" && route.TenantKey == "" {
		return nil, ErrTenantRoutingRequired
	}
	if route.ChannelID == "" && route.ChannelKey == "" {
		return nil, ErrTenantRoutingRequired
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT
			c.id::text,
			c.tenant_id::text,
			c.provider,
			c.capabilities,
			cred.secret_ref,
			cred.secret_ref_display
		FROM tenant_channels c
		JOIN tenants t ON t.id = c.tenant_id
		LEFT JOIN tenant_channel_credentials cred
			ON cred.tenant_id = c.tenant_id
			AND cred.channel_id = c.id
			AND cred.purpose = $5
			AND cred.status = 'ACTIVE'
		WHERE ($1 = '' OR c.tenant_id::text = $1)
			AND ($2 = '' OR t.tenant_key = $2)
			AND ($3 = '' OR c.id::text = $3)
			AND ($4 = '' OR c.channel_key = $4)
			AND c.status = 'ACTIVE'
		LIMIT 1
	`, route.TenantID, route.TenantKey, route.ChannelID, route.ChannelKey, tenantCredentialPurposeProviderAPI)

	var (
		channelID        string
		tenantID         string
		provider         string
		capabilities     pq.StringArray
		secretRef        sql.NullString
		secretRefDisplay sql.NullString
	)
	if err := row.Scan(&channelID, &tenantID, &provider, &capabilities, &secretRef, &secretRefDisplay); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTenantChannelNotFound
		}
		return nil, fmt.Errorf("resolve tenant channel: %w", err)
	}
	if !operationAllowed(operation, []string(capabilities), r.operationCapability) {
		return nil, ErrUnsupportedChannelOperation
	}
	if !secretRef.Valid || strings.TrimSpace(secretRef.String) == "" {
		return nil, ErrTenantCredentialMissing
	}

	secret, err := r.credentialResolver.ResolveProviderCredential(ctx, secretRef.String, tenantID, channelID)
	if err != nil {
		return nil, err
	}
	if strings.ToLower(strings.TrimSpace(provider)) != "timwe" {
		return nil, fmt.Errorf("%w: provider %s", ErrUnsupportedChannelOperation, provider)
	}
	allowSharedCredentialFallback := tenantProviderSharedCredentialFallbackEnabled(r.cfg)
	rawBaseURL := firstNonEmpty(secret.BaseURL, r.cfg.Application.TIMWE.BaseURL)
	if err := validateProviderBaseURL(rawBaseURL, r.cfg); err != nil {
		return nil, fmt.Errorf("%w: base_url validation: %s", ErrTenantCredentialInvalid, err.Error())
	}
	cfg := &TenantProviderConfig{
		TenantID:         tenantID,
		ChannelID:        channelID,
		Provider:         provider,
		BaseURL:          strings.TrimRight(rawBaseURL, "/"),
		APIKey:           tenantProviderCredentialValue(secret.APIKey, r.cfg.Application.TIMWE.APIKey, allowSharedCredentialFallback),
		Authentication:   tenantProviderCredentialValue(secret.AuthenticationKey, r.cfg.Application.TIMWE.AuthenticationKey, allowSharedCredentialFallback),
		PartnerServiceID: tenantProviderCredentialValue(secret.PartnerServiceID, r.cfg.Application.TIMWE.PartnerServiceID, allowSharedCredentialFallback),
		PSK:              tenantProviderCredentialValue(secret.PSK, r.cfg.Application.TIMWE.Psk, allowSharedCredentialFallback),
		PartnerRoleID:    firstNonEmpty(secret.PartnerRoleID, r.cfg.Application.TIMWE.PartnerRoleID),
		Realm:            firstNonEmpty(secret.Realm, r.cfg.Application.TIMWE.Realm),
		// Extended fields: mcc/mnc/mt_api_key fall back to global config; the rest are
		// tenant-specific only (empty when absent — callers apply their own defaults).
		MTAPIKey:             firstNonEmpty(secret.MTAPIKey, r.cfg.Application.TIMWE.MTAPIKey),
		MCC:                  firstNonEmpty(secret.MCC, r.cfg.Application.TIMWE.MCC),
		MNC:                  firstNonEmpty(secret.MNC, r.cfg.Application.TIMWE.MNC),
		LargeAccount:         secret.LargeAccount,
		ServiceName:          secret.ServiceName,
		FreeMTPricepointID:   secret.FreeMTPricepointID,
		MOPricepointIDs:      secret.MOPricepointIDs,
		BillingPricepointIDs: secret.BillingPricepointIDs,
		HEIVParamSpecKey:     secret.HEIVParamSpecKey,
	}
	if secretRefDisplay.Valid {
		cfg.SecretRefDisplay = secretRefDisplay.String
	}
	if cfg.BaseURL == "" || cfg.APIKey == "" || cfg.PartnerRoleID == "" {
		return nil, fmt.Errorf("%w: provider config incomplete", ErrTenantCredentialInvalid)
	}
	if !tenantProviderAuthMaterialPresent(cfg) {
		return nil, fmt.Errorf("%w: provider authentication material incomplete", ErrTenantCredentialInvalid)
	}
	return cfg, nil
}

func tenantProviderCredentialValue(secretValue, sharedValue string, allowSharedFallback bool) string {
	if strings.TrimSpace(secretValue) != "" {
		return strings.TrimSpace(secretValue)
	}
	if allowSharedFallback {
		return strings.TrimSpace(sharedValue)
	}
	return ""
}

func tenantProviderSharedCredentialFallbackEnabled(cfg *config.Config) bool {
	if cfg == nil || cfg.Application.Environment != config.DEVELOPMENT {
		return false
	}
	raw, ok := os.LookupEnv(tenantProviderCredentialDevFallbackEnv)
	if !ok {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "t", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func tenantProviderAuthMaterialPresent(cfg *TenantProviderConfig) bool {
	if cfg == nil {
		return false
	}
	if strings.TrimSpace(cfg.Authentication) != "" {
		return true
	}
	return strings.TrimSpace(cfg.PartnerServiceID) != "" && strings.TrimSpace(cfg.PSK) != ""
}

func operationAllowed(operation ChannelOperation, capabilities []string, policy map[ChannelOperation][]string) bool {
	allowed := policy[operation]
	if len(allowed) == 0 {
		return false
	}
	have := make(map[string]struct{}, len(capabilities))
	for _, capability := range capabilities {
		have[strings.ToLower(strings.TrimSpace(capability))] = struct{}{}
	}
	for _, capability := range allowed {
		if _, ok := have[capability]; ok {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
// validateProviderBaseURL performs SSRF defence on a BaseURL loaded from tenant
// secrets before it is used to build HTTP requests. It enforces HTTPS (relaxed
// to HTTP in DEVELOPMENT), rejects userinfo, and rejects literal IP addresses in
// private/loopback/link-local/metadata ranges.
func validateProviderBaseURL(rawURL string, cfg *config.Config) error {
	if strings.TrimSpace(rawURL) == "" {
		return fmt.Errorf("base_url is empty")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("base_url is not a valid URL: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	isDevelopment := cfg != nil && cfg.Application.Environment == config.DEVELOPMENT
	switch scheme {
	case "https":
		// always allowed
	case "http":
		if !isDevelopment {
			return fmt.Errorf("base_url must use HTTPS (got http:// in non-development environment)")
		}
	default:
		return fmt.Errorf("base_url has unsupported scheme %q; only https is allowed", scheme)
	}
	if u.User != nil && u.User.String() != "" {
		return fmt.Errorf("base_url must not contain userinfo credentials")
	}
	host := u.Hostname()
	if strings.TrimSpace(host) == "" {
		return fmt.Errorf("base_url has no host")
	}
	ip := net.ParseIP(host)
	if ip != nil {
		if err := rejectPrivateIP(ip); err != nil {
			return err
		}
	}
	return nil
}

// rejectPrivateIP returns an error if ip falls within a private/loopback/
// link-local/metadata range that must not be reachable via tenant-supplied URLs.
func rejectPrivateIP(ip net.IP) error {
	for _, cidr := range blockedCIDRs {
		if cidr.Contains(ip) {
			return fmt.Errorf("base_url resolves to a disallowed IP range (%s)", cidr.String())
		}
	}
	return nil
}

// blockedCIDRs is the set of IP ranges that are never allowed in tenant BaseURLs.
var blockedCIDRs = func() []*net.IPNet {
	ranges := []string{
		"10.0.0.0/8",       // RFC1918
		"172.16.0.0/12",    // RFC1918
		"192.168.0.0/16",   // RFC1918
		"127.0.0.0/8",      // loopback v4
		"169.254.0.0/16",   // link-local / AWS metadata v4
		"::1/128",          // loopback v6
		"fc00::/7",         // unique-local v6
		"fe80::/10",        // link-local v6
	}
	out := make([]*net.IPNet, 0, len(ranges))
	for _, r := range ranges {
		_, ipnet, err := net.ParseCIDR(r)
		if err == nil {
			out = append(out, ipnet)
		}
	}
	return out
}()
