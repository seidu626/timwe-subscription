package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"go.uber.org/zap"
)

// CampaignRepo defines the repository contract used by CampaignService.
// This enables unit testing without a real database.
type CampaignRepo interface {
	GetBySlug(slug string) (*domain.Campaign, error)
	GetByTenantKeyAndSlug(tenantKey, slug string) (*domain.Campaign, error)
	ListEnabled() ([]*domain.Campaign, error)
	GetAdminBySlug(slug string) (*domain.Campaign, error)
	GetAdminByTenantAndSlug(tenantID, slug string) (*domain.Campaign, error)
	ListAll(enabled *bool, country *string) ([]*domain.Campaign, error)
	ListAllForTenant(tenantID string, enabled *bool, country *string) ([]*domain.Campaign, error)
	Create(c *domain.Campaign) (*domain.Campaign, error)
	CreateForTenant(tenantID string, c *domain.Campaign) (*domain.Campaign, error)
	Update(slug string, c *domain.Campaign) (*domain.Campaign, error)
	UpdateForTenant(tenantID, slug string, c *domain.Campaign) (*domain.Campaign, error)
	SetEnabled(slug string, enabled bool, updatedBy *string) (*domain.Campaign, error)
	SetEnabledForTenant(tenantID, slug string, enabled bool, updatedBy *string) (*domain.Campaign, error)
	UpdatePostbackRules(slug string, rules json.RawMessage) error
}

// CampaignService handles campaign business logic
type CampaignService struct {
	repo   CampaignRepo
	logger *zap.Logger
}

type campaignOfferMappingValidator interface {
	ValidateOfferProductMapping(offerProductID int, pricepointID *int) error
}

type tenantCampaignValidator interface {
	ValidateTenantOfferProductMapping(tenantID string, offerProductID int, pricepointID *int) error
	ValidateTenantChannelForCampaign(tenantID, channelID, country string, operator *string, flowType domain.FlowType) error
}

var (
	ErrCampaignConflict                  = errors.New("campaign_conflict")
	ErrCampaignChannelCapabilityMismatch = errors.New("channel_capability_mismatch")
	ErrCampaignChannelInactive           = errors.New("channel_inactive")
)

// NewCampaignService creates a new campaign service
func NewCampaignService(repo CampaignRepo, logger *zap.Logger) *CampaignService {
	return &CampaignService{
		repo:   repo,
		logger: logger,
	}
}

// GetBySlug retrieves a campaign by slug and returns public-safe data
func (s *CampaignService) GetBySlug(slug string) (*domain.PublicCampaign, error) {
	campaign, err := s.repo.GetBySlug(slug)
	if err != nil {
		return nil, fmt.Errorf("failed to get campaign: %w", err)
	}

	return campaign.ToPublic(), nil
}

func (s *CampaignService) GetByTenantKeyAndSlug(tenantKey, slug string) (*domain.PublicCampaign, error) {
	campaign, err := s.repo.GetByTenantKeyAndSlug(tenantKey, slug)
	if err != nil {
		return nil, fmt.Errorf("failed to get campaign: %w", err)
	}

	return campaign.ToPublic(), nil
}

// ListEnabled retrieves all enabled campaigns (public-safe)
func (s *CampaignService) ListEnabled() ([]*domain.PublicCampaign, error) {
	campaigns, err := s.repo.ListEnabled()
	if err != nil {
		return nil, fmt.Errorf("failed to list campaigns: %w", err)
	}

	public := make([]*domain.PublicCampaign, len(campaigns))
	for i, c := range campaigns {
		public[i] = c.ToPublic()
	}

	return public, nil
}

// AdminGetBySlug retrieves a campaign by slug (admin/full view).
func (s *CampaignService) AdminGetBySlug(slug string) (*domain.Campaign, error) {
	campaign, err := s.repo.GetAdminBySlug(slug)
	if err != nil {
		return nil, fmt.Errorf("failed to get campaign: %w", err)
	}
	return campaign, nil
}

func (s *CampaignService) AdminGetByTenantAndSlug(tenantID, slug string) (*domain.Campaign, error) {
	campaign, err := s.repo.GetAdminByTenantAndSlug(tenantID, slug)
	if err != nil {
		return nil, fmt.Errorf("failed to get campaign: %w", err)
	}
	return campaign, nil
}

// AdminList retrieves campaigns (enabled + disabled) with optional filters.
func (s *CampaignService) AdminList(enabled *bool, country *string) ([]*domain.Campaign, error) {
	campaigns, err := s.repo.ListAll(enabled, country)
	if err != nil {
		return nil, fmt.Errorf("failed to list campaigns: %w", err)
	}
	return campaigns, nil
}

func (s *CampaignService) AdminListForTenant(tenantID string, enabled *bool, country *string) ([]*domain.Campaign, error) {
	campaigns, err := s.repo.ListAllForTenant(tenantID, enabled, country)
	if err != nil {
		return nil, fmt.Errorf("failed to list campaigns: %w", err)
	}
	return campaigns, nil
}

// AdminCreate creates a new campaign.
func (s *CampaignService) AdminCreate(c *domain.Campaign) (*domain.Campaign, error) {
	if err := s.validateOfferMapping(c); err != nil {
		return nil, err
	}

	created, err := s.repo.Create(c)
	if err != nil {
		return nil, fmt.Errorf("failed to create campaign: %w", err)
	}
	return created, nil
}

func (s *CampaignService) AdminCreateForTenant(tenantID string, c *domain.Campaign) (*domain.Campaign, error) {
	if err := s.validateTenantCampaign(tenantID, c); err != nil {
		return nil, err
	}

	created, err := s.repo.CreateForTenant(tenantID, c)
	if err != nil {
		return nil, mapCampaignWriteError("failed to create campaign", err)
	}
	return created, nil
}

// AdminUpdate updates an existing campaign by slug (slug is immutable).
func (s *CampaignService) AdminUpdate(slug string, c *domain.Campaign) (*domain.Campaign, error) {
	if err := s.validateOfferMapping(c); err != nil {
		return nil, err
	}

	updated, err := s.repo.Update(slug, c)
	if err != nil {
		return nil, fmt.Errorf("failed to update campaign: %w", err)
	}
	return updated, nil
}

func (s *CampaignService) AdminUpdateForTenant(tenantID, slug string, c *domain.Campaign) (*domain.Campaign, error) {
	if err := s.validateTenantCampaign(tenantID, c); err != nil {
		return nil, err
	}

	updated, err := s.repo.UpdateForTenant(tenantID, slug, c)
	if err != nil {
		return nil, mapCampaignWriteError("failed to update campaign", err)
	}
	return updated, nil
}

// AdminSetEnabled enables/disables a campaign.
func (s *CampaignService) AdminSetEnabled(slug string, enabled bool, updatedBy *string) (*domain.Campaign, error) {
	updated, err := s.repo.SetEnabled(slug, enabled, updatedBy)
	if err != nil {
		return nil, fmt.Errorf("failed to set enabled: %w", err)
	}
	return updated, nil
}

func (s *CampaignService) AdminSetEnabledForTenant(tenantID, slug string, enabled bool, updatedBy *string) (*domain.Campaign, error) {
	updated, err := s.repo.SetEnabledForTenant(tenantID, slug, enabled, updatedBy)
	if err != nil {
		return nil, fmt.Errorf("failed to set enabled: %w", err)
	}
	return updated, nil
}

// AdminGetPostbackRules returns the current postback_rules for a campaign.
func (s *CampaignService) AdminGetPostbackRules(slug string) (json.RawMessage, error) {
	campaign, err := s.repo.GetAdminBySlug(slug)
	if err != nil {
		return nil, fmt.Errorf("failed to get campaign: %w", err)
	}
	if len(campaign.PostbackRules) == 0 {
		return json.RawMessage("{}"), nil
	}
	return campaign.PostbackRules, nil
}

// AdminUpdatePostbackRules validates and updates the postback_rules for a campaign.
func (s *CampaignService) AdminUpdatePostbackRules(slug string, rules json.RawMessage) error {
	// Validate that the JSON parses as domain.PostbackRules
	var parsed domain.PostbackRules
	if err := json.Unmarshal(rules, &parsed); err != nil {
		return fmt.Errorf("invalid postback_rules: %w", err)
	}

	if err := s.repo.UpdatePostbackRules(slug, rules); err != nil {
		return fmt.Errorf("failed to update postback_rules: %w", err)
	}
	return nil
}

// AdminClone creates a new campaign by cloning an existing one with a new slug.
// The cloned campaign is always disabled to avoid accidental activation.
func (s *CampaignService) AdminClone(sourceSlug, newSlug string, createdBy *string) (*domain.Campaign, error) {
	source, err := s.repo.GetAdminBySlug(sourceSlug)
	if err != nil {
		return nil, fmt.Errorf("failed to get source campaign: %w", err)
	}

	newSlug = strings.TrimSpace(newSlug)
	if newSlug == "" {
		return nil, fmt.Errorf("new_slug is required")
	}

	var normalizedCreatedBy *string
	if createdBy != nil {
		v := strings.TrimSpace(*createdBy)
		if v != "" {
			normalizedCreatedBy = &v
		}
	}

	clone := &domain.Campaign{
		Slug:               newSlug,
		Language:           source.Language,
		Country:            source.Country,
		Operator:           cloneStringPtr(source.Operator),
		OfferProductID:     source.OfferProductID,
		PricepointID:       cloneIntPtr(source.PricepointID),
		PartnerRoleID:      cloneIntPtr(source.PartnerRoleID),
		FlowType:           source.FlowType,
		ShortCode:          cloneStringPtr(source.ShortCode),
		SMSKeyword:         cloneStringPtr(source.SMSKeyword),
		Price:              cloneFloat64Ptr(source.Price),
		BillingCycle:       cloneStringPtr(source.BillingCycle),
		TrialFlags:         cloneRawMessage(source.TrialFlags),
		TermsURL:           cloneStringPtr(source.TermsURL),
		InlineTermsText:    cloneStringPtr(source.InlineTermsText),
		ConsentRequired:    source.ConsentRequired,
		ConsentVersion:     cloneStringPtr(source.ConsentVersion),
		AttributionMapping: cloneRawMessage(source.AttributionMapping),
		PostbackRules:      cloneRawMessage(source.PostbackRules),
		Throttles:          cloneRawMessage(source.Throttles),
		AllowedReferrers:   cloneStringSlice(source.AllowedReferrers),
		AllowedSources:     cloneStringSlice(source.AllowedSources),
		LandingPageURLs:    cloneStringSlice(source.LandingPageURLs),
		TrackingConfig:     cloneRawMessage(source.TrackingConfig),
		LPCopy:             cloneRawMessage(source.LPCopy),
		Enabled:            false,
		CreatedBy:          normalizedCreatedBy,
		UpdatedBy:          normalizedCreatedBy,
	}

	if err := s.validateOfferMapping(clone); err != nil {
		return nil, err
	}

	created, err := s.repo.Create(clone)
	if err != nil {
		return nil, fmt.Errorf("failed to clone campaign: %w", err)
	}
	return created, nil
}

func cloneStringPtr(v *string) *string {
	if v == nil {
		return nil
	}
	out := *v
	return &out
}

func cloneIntPtr(v *int) *int {
	if v == nil {
		return nil
	}
	out := *v
	return &out
}

func cloneFloat64Ptr(v *float64) *float64 {
	if v == nil {
		return nil
	}
	out := *v
	return &out
}

func cloneRawMessage(v []byte) []byte {
	if len(v) == 0 {
		return nil
	}
	return append([]byte(nil), v...)
}

func cloneStringSlice(v []string) []string {
	if len(v) == 0 {
		return nil
	}
	return append([]string(nil), v...)
}

func (s *CampaignService) validateOfferMapping(c *domain.Campaign) error {
	if c == nil {
		return fmt.Errorf("campaign payload is required")
	}

	validator, ok := s.repo.(campaignOfferMappingValidator)
	if !ok {
		return nil
	}

	if err := validator.ValidateOfferProductMapping(c.OfferProductID, c.PricepointID); err != nil {
		return fmt.Errorf("invalid campaign offer mapping: %w", err)
	}

	return nil
}

func (s *CampaignService) validateTenantCampaign(tenantID string, c *domain.Campaign) error {
	if c == nil {
		return fmt.Errorf("campaign payload is required")
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}

	validator, ok := s.repo.(tenantCampaignValidator)
	if !ok {
		return fmt.Errorf("tenant campaign validator is not configured")
	}

	if err := validator.ValidateTenantOfferProductMapping(tenantID, c.OfferProductID, c.PricepointID); err != nil {
		return fmt.Errorf("invalid campaign offer mapping: %w", err)
	}

	channelID := ""
	if c.ChannelID != nil {
		channelID = strings.TrimSpace(*c.ChannelID)
	}
	if channelID == "" {
		return fmt.Errorf("channel_id is required")
	}
	if err := validator.ValidateTenantChannelForCampaign(tenantID, channelID, c.Country, c.Operator, c.FlowType); err != nil {
		switch {
		case strings.Contains(err.Error(), "channel_capability_mismatch"):
			return fmt.Errorf("%w: %v", ErrCampaignChannelCapabilityMismatch, err)
		case strings.Contains(err.Error(), "channel_inactive"):
			return fmt.Errorf("%w: %v", ErrCampaignChannelInactive, err)
		default:
			return fmt.Errorf("invalid campaign channel binding: %w", err)
		}
	}

	c.TenantID = &tenantID
	c.ChannelID = &channelID
	return nil
}

func mapCampaignWriteError(prefix string, err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "duplicate key value") ||
		strings.Contains(strings.ToLower(err.Error()), "idx_campaigns_tenant_slug") {
		return fmt.Errorf("%s: %w: %v", prefix, ErrCampaignConflict, err)
	}
	return fmt.Errorf("%s: %w", prefix, err)
}
