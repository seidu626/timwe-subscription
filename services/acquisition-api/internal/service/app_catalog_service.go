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

// Marketplace returns every active tenant's enabled catalog grouped per
// tenant, optionally filtered by country. Tenants with no listable products
// are omitted (the repository only returns rows that have a campaign).
func (s *AppCatalogService) Marketplace(country string) ([]*domain.AppMarketplaceTenant, error) {
	products, err := s.campaignRepo.ListAppCatalog("", country)
	if err != nil {
		return nil, err
	}
	tenants := make([]*domain.AppMarketplaceTenant, 0)
	byKey := make(map[string]*domain.AppMarketplaceTenant)
	for _, product := range products {
		section, ok := byKey[product.Tenant]
		if !ok {
			section = &domain.AppMarketplaceTenant{
				TenantKey:  product.Tenant,
				TenantName: product.TenantName,
				Products:   make([]*domain.AppCatalogProduct, 0, 4),
			}
			byKey[product.Tenant] = section
			tenants = append(tenants, section)
		}
		section.Products = append(section.Products, product)
	}
	return tenants, nil
}
