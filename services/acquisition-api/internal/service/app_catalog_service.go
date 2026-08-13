package service

import (
	"strings"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
)

// AppCatalogService serves the Dayline app's public product catalog.
type AppCatalogService struct {
	campaignRepo *repository.CampaignRepository
}

// NewAppCatalogService creates a new AppCatalogService.
func NewAppCatalogService(campaignRepo *repository.CampaignRepository) *AppCatalogService {
	return &AppCatalogService{campaignRepo: campaignRepo}
}

// List returns the enabled catalog for tenant, optionally filtered by country.
func (s *AppCatalogService) List(tenantKey, country string) ([]*domain.AppCatalogProduct, error) {
	tenantKey = strings.TrimSpace(tenantKey)
	if tenantKey == "" {
		return nil, domain.NewAppError(domain.AppErrValidation, "tenant is required")
	}
	products, err := s.campaignRepo.ListAppCatalog(tenantKey, country)
	if err != nil {
		return nil, err
	}
	return products, nil
}
