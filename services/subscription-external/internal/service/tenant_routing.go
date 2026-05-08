// slice-harness: allow-new-canonical-path: TMP-007 canonical tenant provider routing module.
package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/lib/pq"
	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/seidu626/subscription-manager/subscription-external/internal/utils"
)

const (
	tenantCredentialPurposeProviderAPI = "provider_api"
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
	BaseURL           string `json:"base_url"`
	APIKey            string `json:"api_key"`
	AuthenticationKey string `json:"authentication_key"`
	PartnerServiceID  string `json:"partner_service_id"`
	PSK               string `json:"psk"`
	PartnerRoleID     string `json:"partner_role_id"`
	Realm             string `json:"realm"`
}

type ProviderCredentialResolver interface {
	ResolveProviderCredential(ctx context.Context, secretRef string) (ProviderCredentialSecret, error)
}

type EnvProviderCredentialResolver struct{}

func (EnvProviderCredentialResolver) ResolveProviderCredential(ctx context.Context, secretRef string) (ProviderCredentialSecret, error) {
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

func (s *SubscriptionService) providerConfigOrLegacy(ctx context.Context, operation ChannelOperation, route domain.TenantRouteContext) (*TenantProviderConfig, error) {
	if isEmptyTenantRoute(route) {
		return s.legacyProviderConfig(), nil
	}
	if s.tenantRouter == nil {
		return nil, ErrTenantRoutingNotConfigured
	}
	return s.tenantRouter.Resolve(ctx, operation, route)
}

func (s *SubscriptionService) legacyProviderConfig() *TenantProviderConfig {
	return &TenantProviderConfig{
		BaseURL:          strings.TrimRight(s.config.Application.TIMWE.BaseURL, "/"),
		APIKey:           s.config.Application.TIMWE.APIKey,
		Authentication:   s.config.Application.TIMWE.AuthenticationKey,
		PartnerServiceID: s.config.Application.TIMWE.PartnerServiceID,
		PSK:              s.config.Application.TIMWE.Psk,
		PartnerRoleID:    s.config.Application.TIMWE.PartnerRoleID,
		Realm:            s.config.Application.TIMWE.Realm,
		Provider:         "timwe",
	}
}

func isEmptyTenantRoute(route domain.TenantRouteContext) bool {
	return strings.TrimSpace(route.TenantID) == "" &&
		strings.TrimSpace(route.TenantKey) == "" &&
		strings.TrimSpace(route.ChannelID) == "" &&
		strings.TrimSpace(route.ChannelKey) == ""
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

	secret, err := r.credentialResolver.ResolveProviderCredential(ctx, secretRef.String)
	if err != nil {
		return nil, err
	}
	if strings.ToLower(strings.TrimSpace(provider)) != "timwe" {
		return nil, fmt.Errorf("%w: provider %s", ErrUnsupportedChannelOperation, provider)
	}
	cfg := &TenantProviderConfig{
		TenantID:         tenantID,
		ChannelID:        channelID,
		Provider:         provider,
		BaseURL:          strings.TrimRight(firstNonEmpty(secret.BaseURL, r.cfg.Application.TIMWE.BaseURL), "/"),
		APIKey:           firstNonEmpty(secret.APIKey, r.cfg.Application.TIMWE.APIKey),
		Authentication:   firstNonEmpty(secret.AuthenticationKey, r.cfg.Application.TIMWE.AuthenticationKey),
		PartnerServiceID: firstNonEmpty(secret.PartnerServiceID, r.cfg.Application.TIMWE.PartnerServiceID),
		PSK:              firstNonEmpty(secret.PSK, r.cfg.Application.TIMWE.Psk),
		PartnerRoleID:    firstNonEmpty(secret.PartnerRoleID, r.cfg.Application.TIMWE.PartnerRoleID),
		Realm:            firstNonEmpty(secret.Realm, r.cfg.Application.TIMWE.Realm),
	}
	if secretRefDisplay.Valid {
		cfg.SecretRefDisplay = secretRefDisplay.String
	}
	if cfg.BaseURL == "" || cfg.APIKey == "" || cfg.PartnerRoleID == "" {
		return nil, fmt.Errorf("%w: provider config incomplete", ErrTenantCredentialInvalid)
	}
	return cfg, nil
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
