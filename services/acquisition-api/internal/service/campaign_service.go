package service

import (
	"fmt"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"go.uber.org/zap"
)

// CampaignRepo defines the repository contract used by CampaignService.
// This enables unit testing without a real database.
type CampaignRepo interface {
	GetBySlug(slug string) (*domain.Campaign, error)
	ListEnabled() ([]*domain.Campaign, error)
	GetAdminBySlug(slug string) (*domain.Campaign, error)
	ListAll(enabled *bool, country *string) ([]*domain.Campaign, error)
	Create(c *domain.Campaign) (*domain.Campaign, error)
	Update(slug string, c *domain.Campaign) (*domain.Campaign, error)
	SetEnabled(slug string, enabled bool, updatedBy *string) (*domain.Campaign, error)
}

// CampaignService handles campaign business logic
type CampaignService struct {
	repo   CampaignRepo
	logger *zap.Logger
}

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

// AdminList retrieves campaigns (enabled + disabled) with optional filters.
func (s *CampaignService) AdminList(enabled *bool, country *string) ([]*domain.Campaign, error) {
	campaigns, err := s.repo.ListAll(enabled, country)
	if err != nil {
		return nil, fmt.Errorf("failed to list campaigns: %w", err)
	}
	return campaigns, nil
}

// AdminCreate creates a new campaign.
func (s *CampaignService) AdminCreate(c *domain.Campaign) (*domain.Campaign, error) {
	created, err := s.repo.Create(c)
	if err != nil {
		return nil, fmt.Errorf("failed to create campaign: %w", err)
	}
	return created, nil
}

// AdminUpdate updates an existing campaign by slug (slug is immutable).
func (s *CampaignService) AdminUpdate(slug string, c *domain.Campaign) (*domain.Campaign, error) {
	updated, err := s.repo.Update(slug, c)
	if err != nil {
		return nil, fmt.Errorf("failed to update campaign: %w", err)
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
