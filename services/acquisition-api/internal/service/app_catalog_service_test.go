package service

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
	"go.uber.org/zap"
)

func newAppCatalogTestService(t *testing.T) (*AppCatalogService, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	campaignRepo := repository.NewCampaignRepository(db, zap.NewNop())
	return NewAppCatalogService(campaignRepo), mock
}

func appCatalogColumns() []string {
	return []string{
		"tenant_key", "name", "slug", "price", "billing_cycle", "flow_type", "country",
		"app_name", "app_tagline", "app_description", "app_category",
		"app_artwork_url", "app_sample_content", "lp_copy",
		"app_featured_rank", "subscriber_count",
	}
}

func TestAppCatalogService_List_RequiresTenant(t *testing.T) {
	svc, mock := newAppCatalogTestService(t)
	_, err := svc.List("  ", "GH")
	requireAppError(t, err, domain.AppErrValidation)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expected no db calls, got: %v", err)
	}
}

func TestAppCatalogService_Marketplace_GroupsProductsPerTenant(t *testing.T) {
	svc, mock := newAppCatalogTestService(t)

	// No tenant filter: the only bind arg is the country.
	mock.ExpectQuery(`FROM campaigns c\s+JOIN tenants t ON t.id = c.tenant_id\s+WHERE c.enabled = true AND t.status = 'ACTIVE' AND c.price IS NOT NULL\s+AND c.country = \$1\s+ORDER BY t.tenant_key, c.app_featured_rank NULLS LAST, c.slug`).
		WithArgs("GH").
		WillReturnRows(sqlmock.NewRows(appCatalogColumns()).
			AddRow("careerify", "Careerify", "career-daily", 1.0, "DAILY", "OTP", "GH",
				"Career Daily", "Grow every day", nil, nil, nil, nil, nil, nil, 0).
			AddRow("nrg", "NRG", "nrg-fitness", 2.0, "DAILY", "OTP", "GH",
				"NRG Fitness", nil, nil, nil, nil, nil, nil, 1, 7).
			AddRow("nrg", "NRG", "nrg-wellness", 2.0, "DAILY", "OTP", "GH",
				"NRG Wellness", nil, nil, nil, nil, nil, nil, nil, 0))

	tenants, err := svc.Marketplace("GH")
	if err != nil {
		t.Fatalf("Marketplace: %v", err)
	}
	if len(tenants) != 2 {
		t.Fatalf("expected 2 tenant sections, got %+v", tenants)
	}
	if tenants[0].TenantKey != "careerify" || tenants[0].TenantName != "Careerify" || len(tenants[0].Products) != 1 {
		t.Fatalf("unexpected first section: %+v", tenants[0])
	}
	if tenants[1].TenantKey != "nrg" || len(tenants[1].Products) != 2 {
		t.Fatalf("unexpected second section: %+v", tenants[1])
	}
	if p := tenants[1].Products[0]; p.Tenant != "nrg" || p.TenantName != "NRG" || p.Name != "NRG Fitness" {
		t.Fatalf("unexpected product tenant attribution: %+v", p)
	}
	if p := tenants[1].Products[0]; !p.Featured || p.SubscriberCount != 7 {
		t.Fatalf("expected featured rank and subscriber count to survive the service layer: %+v", p)
	}
	if p := tenants[0].Products[0]; p.Featured || p.SubscriberCount != 0 {
		t.Fatalf("expected unranked product to stay unfeatured: %+v", p)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAppCatalogService_List_KeepsTenantFilter(t *testing.T) {
	svc, mock := newAppCatalogTestService(t)

	mock.ExpectQuery(`WHERE c.enabled = true AND t.status = 'ACTIVE' AND c.price IS NOT NULL\s+AND t.tenant_key = \$1\s+AND c.country = \$2`).
		WithArgs("careerify", "GH").
		WillReturnRows(sqlmock.NewRows(appCatalogColumns()).
			AddRow("careerify", "Careerify", "career-daily", 1.0, "DAILY", "OTP", "GH",
				"Career Daily", nil, nil, nil, nil, nil, nil, nil, 3))

	products, err := svc.List("careerify", "GH")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(products) != 1 || products[0].Tenant != "careerify" || products[0].Name != "Career Daily" {
		t.Fatalf("unexpected products: %+v", products)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
