package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"go.uber.org/zap"
)

// AdProvider defines the interface for ad provider implementations
type AdProvider interface {
	// Name returns the provider identifier
	Name() string

	// Normalize converts provider-specific attribution params to canonical format
	Normalize(params map[string]string) (*domain.Attribution, error)

	// BuildPostback constructs the HTTP request for a postback event (legacy)
	BuildPostback(event domain.PostbackEvent, attribution *domain.Attribution, outcome map[string]interface{}) (*http.Request, error)
}

// ProviderRegistry manages ad provider implementations
type ProviderRegistry struct {
	providers map[string]AdProvider
	logger    *zap.Logger
}

// NewProviderRegistry creates a new provider registry
func NewProviderRegistry(logger *zap.Logger) *ProviderRegistry {
	registry := &ProviderRegistry{
		providers: make(map[string]AdProvider),
		logger:    logger,
	}

	return registry
}

// Register adds a provider to the registry
func (r *ProviderRegistry) Register(provider AdProvider) {
	r.providers[provider.Name()] = provider
	r.logger.Info("Registered ad provider", zap.String("provider", provider.Name()))
}

// Get retrieves a provider by name
func (r *ProviderRegistry) Get(name string) (AdProvider, error) {
	provider, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", name)
	}
	return provider, nil
}

// List returns all registered provider names
func (r *ProviderRegistry) List() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// PostbackTemplateService handles template-driven postback generation
type PostbackTemplateService struct {
	logger *zap.Logger
}

// NewPostbackTemplateService creates a new postback template service
func NewPostbackTemplateService(logger *zap.Logger) *PostbackTemplateService {
	return &PostbackTemplateService{logger: logger}
}

// ParsePostbackRules parses the campaign's postback_rules JSON into PostbackRules
func (s *PostbackTemplateService) ParsePostbackRules(rulesJSON json.RawMessage) (domain.PostbackRules, error) {
	if len(rulesJSON) == 0 {
		return nil, nil
	}

	var rules domain.PostbackRules
	if err := json.Unmarshal(rulesJSON, &rules); err != nil {
		s.logger.Error("Failed to parse postback rules", zap.Error(err))
		return nil, fmt.Errorf("failed to parse postback rules: %w", err)
	}

	return rules, nil
}

// GetTemplateForEvent returns the postback template for a given event and provider
func (s *PostbackTemplateService) GetTemplateForEvent(rules domain.PostbackRules, event domain.PostbackEvent, provider string) (*domain.PostbackTemplate, bool) {
	if rules == nil {
		return nil, false
	}

	eventRules, exists := rules[string(event)]
	if !exists {
		return nil, false
	}

	template, exists := eventRules[provider]
	if !exists {
		return nil, false
	}

	return &template, true
}

// BuildPostbackFromTemplate builds an HTTP request from a template and context
func (s *PostbackTemplateService) BuildPostbackFromTemplate(template *domain.PostbackTemplate, ctx *domain.PostbackContext) (*http.Request, error) {
	if template == nil {
		return nil, fmt.Errorf("template is nil")
	}

	if ctx.ClickID == "" {
		if strings.Contains(template.URL, "{click_id}") || strings.Contains(template.URL, "{txid}") {
			s.logger.Warn("click_id is empty but URL template requires click identity",
				zap.String("template_url", template.URL),
			)
			return nil, fmt.Errorf("click_id is required for postback template")
		}
	}

	// Render the URL template
	renderedURL := ctx.RenderURL(template.URL)

	// Parse and validate URL
	u, err := url.Parse(renderedURL)
	if err != nil {
		return nil, fmt.Errorf("invalid postback URL after rendering: %w", err)
	}

	// Determine method (default to GET)
	method := strings.ToUpper(template.Method)
	if method == "" {
		method = "GET"
	}

	// Create request
	req, err := http.NewRequest(method, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create postback request: %w", err)
	}

	// Add headers from template
	for key, value := range template.Headers {
		req.Header.Set(key, ctx.RenderURL(value))
	}

	s.logger.Info("Built postback from template",
		zap.String("method", method),
		zap.String("url", u.String()),
		zap.String("click_id", ctx.ClickID),
	)

	return req, nil
}

// MobplusProvider implements the Mobplus ad provider
type MobplusProvider struct {
	logger *zap.Logger
}

// NewMobplusProvider creates a new Mobplus provider
func NewMobplusProvider(logger *zap.Logger) *MobplusProvider {
	return &MobplusProvider{logger: logger}
}

// Name returns the provider identifier
func (p *MobplusProvider) Name() string {
	return "mobplus"
}

// Normalize converts Mobplus-specific params to canonical format
// Supports canonical click_id plus aliases: txid, clickid, cid, subid
func (p *MobplusProvider) Normalize(params map[string]string) (*domain.Attribution, error) {
	attribution := &domain.Attribution{
		Provider: "mobplus",
	}

	// Canonical click_id param with aliases (priority order)
	clickIDParams := []string{"click_id", "txid", "clickid", "cid", "subid"}
	for _, param := range clickIDParams {
		if clickID, ok := params[param]; ok && clickID != "" {
			attribution.ClickID = clickID
			break
		}
	}

	// Map other common params
	if pubID, ok := params["pub_id"]; ok {
		attribution.PubID = pubID
	}
	if sub1, ok := params["sub1"]; ok {
		attribution.Sub1 = sub1
	}
	if sub2, ok := params["sub2"]; ok {
		attribution.Sub2 = sub2
	}
	if sub3, ok := params["sub3"]; ok {
		attribution.Sub3 = sub3
	}

	if campaign, ok := params["campaign"]; ok {
		attribution.CampaignSlug = campaign
	}

	return attribution, nil
}

// BuildPostback constructs a Mobplus postback request (legacy fallback)
// Production usage should use template-driven postbacks via PostbackTemplateService
func (p *MobplusProvider) BuildPostback(event domain.PostbackEvent, attribution *domain.Attribution, outcome map[string]interface{}) (*http.Request, error) {
	if attribution.ClickID == "" {
		return nil, fmt.Errorf("click_id is required for Mobplus postback")
	}

	// Check for template URL in outcome (preferred)
	if templateURL, ok := outcome["postback_url"].(string); ok && templateURL != "" {
		ctx := &domain.PostbackContext{
			ClickID:      attribution.ClickID,
			CampaignSlug: attribution.CampaignSlug,
			Sub1:         attribution.Sub1,
			Sub2:         attribution.Sub2,
			Sub3:         attribution.Sub3,
		}

		if txID, ok := outcome["transaction_id"].(string); ok {
			ctx.TransactionID = txID
		}
		if payout, ok := outcome["payout"].(string); ok {
			ctx.Payout = payout
		}

		renderedURL := ctx.RenderURL(templateURL)
		u, err := url.Parse(renderedURL)
		if err != nil {
			return nil, fmt.Errorf("invalid postback URL: %w", err)
		}

		method := "GET"
		if m, ok := outcome["method"].(string); ok && m != "" {
			method = strings.ToUpper(m)
		}

		return http.NewRequest(method, u.String(), nil)
	}

	// Fallback: use default Mobplus format (legacy, should be removed in future)
	p.logger.Warn("Using legacy hard-coded Mobplus postback URL - migrate to template-driven config")

	campaignKey := "93a7719300a247d18ca8db99b8c91461"
	if key, ok := outcome["campaign_key"].(string); ok && key != "" {
		campaignKey = key
	}

	baseURL := "http://m.mobplus.net"
	path := fmt.Sprintf("/c/p/%s", campaignKey)
	postbackURL := baseURL + path

	u, err := url.Parse(postbackURL)
	if err != nil {
		return nil, fmt.Errorf("invalid postback URL: %w", err)
	}

	q := u.Query()
	q.Set("txid", attribution.ClickID)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("POST", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create postback request: %w", err)
	}

	return req, nil
}

// GenericProvider implements a generic ad provider for testing/fallback
type GenericProvider struct {
	logger *zap.Logger
}

// NewGenericProvider creates a new generic provider
func NewGenericProvider(logger *zap.Logger) *GenericProvider {
	return &GenericProvider{logger: logger}
}

// Name returns the provider identifier
func (p *GenericProvider) Name() string {
	return "generic"
}

// Normalize converts generic params to canonical format
// Supports canonical click_id plus aliases: txid, clickid, cid, subid
func (p *GenericProvider) Normalize(params map[string]string) (*domain.Attribution, error) {
	attribution := &domain.Attribution{
		Provider: "generic",
	}

	// Canonical click_id param with aliases (priority order)
	clickIDParams := []string{"click_id", "txid", "clickid", "cid", "subid"}
	for _, param := range clickIDParams {
		if clickID, ok := params[param]; ok && clickID != "" {
			attribution.ClickID = clickID
			break
		}
	}

	if pubID, ok := params["pub_id"]; ok {
		attribution.PubID = pubID
	}
	if sub1, ok := params["sub1"]; ok {
		attribution.Sub1 = sub1
	}
	if sub2, ok := params["sub2"]; ok {
		attribution.Sub2 = sub2
	}
	if sub3, ok := params["sub3"]; ok {
		attribution.Sub3 = sub3
	}
	if campaign, ok := params["campaign"]; ok {
		attribution.CampaignSlug = campaign
	}

	return attribution, nil
}

// BuildPostback constructs a generic postback request
func (p *GenericProvider) BuildPostback(event domain.PostbackEvent, attribution *domain.Attribution, outcome map[string]interface{}) (*http.Request, error) {
	// Generic provider expects postback_url in outcome
	postbackURL, ok := outcome["postback_url"].(string)
	if !ok || postbackURL == "" {
		return nil, fmt.Errorf("postback_url is required for generic provider")
	}

	ctx := &domain.PostbackContext{
		ClickID:      attribution.ClickID,
		CampaignSlug: attribution.CampaignSlug,
		Sub1:         attribution.Sub1,
		Sub2:         attribution.Sub2,
		Sub3:         attribution.Sub3,
	}

	if txID, ok := outcome["transaction_id"].(string); ok {
		ctx.TransactionID = txID
	}

	renderedURL := ctx.RenderURL(postbackURL)
	u, err := url.Parse(renderedURL)
	if err != nil {
		return nil, fmt.Errorf("invalid postback URL: %w", err)
	}

	// Add event to query params
	q := u.Query()
	q.Set("event", string(event))
	u.RawQuery = q.Encode()

	// Default to GET for generic
	method := "GET"
	if m, ok := outcome["method"].(string); ok && m != "" {
		method = strings.ToUpper(m)
	}

	req, err := http.NewRequest(method, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create postback request: %w", err)
	}

	return req, nil
}
