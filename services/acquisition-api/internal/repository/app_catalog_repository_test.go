package repository

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/zap"
)

var appCatalogColumns = []string{
	"tenant_key", "name", "slug", "price", "billing_cycle", "flow_type", "country",
	"app_name", "app_tagline", "app_description", "app_category",
	"app_artwork_url", "app_sample_content", "lp_copy",
	"app_featured_rank", "subscriber_count",
}

func TestListAppCatalog_PrefersAppColumnsOverLPCopyFallback(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	lpCopy := `{"en":{"heroTitle":"LP Title","heDescription":"LP Tagline","successBody":"LP Description"}}`
	mock.ExpectQuery(`FROM campaigns c`).
		WithArgs("nrg").
		WillReturnRows(sqlmock.NewRows(appCatalogColumns).
			AddRow("nrg", "NRG", "daily-tips", 2.5, "DAILY", "OTP", "GH",
				"App Name", "App Tagline", "App Description", "wellness",
				"https://cdn/art.png", "sample text", lpCopy, 1, 42))

	repo := NewCampaignRepository(db, zap.NewNop())
	products, err := repo.ListAppCatalog("nrg", "")
	if err != nil {
		t.Fatalf("ListAppCatalog: %v", err)
	}
	if len(products) != 1 {
		t.Fatalf("expected 1 product, got %d", len(products))
	}
	p := products[0]
	if p.Name != "App Name" || p.Tagline != "App Tagline" || p.Description != "App Description" {
		t.Fatalf("expected app_* columns to win over lp_copy, got %+v", p)
	}
	if p.Category != "wellness" || p.ArtworkURL != "https://cdn/art.png" || p.SampleContent != "sample text" {
		t.Fatalf("unexpected category/artwork/sample: %+v", p)
	}
	if p.Currency != "GHS" {
		t.Fatalf("expected currency GHS for country GH, got %q", p.Currency)
	}
	if p.Price == nil || *p.Price != 2.5 {
		t.Fatalf("unexpected price: %+v", p.Price)
	}
	if !p.Featured {
		t.Fatalf("expected non-null app_featured_rank to mark the product featured, got %+v", p)
	}
	if p.SubscriberCount != 42 {
		t.Fatalf("expected subscriber_count 42 from the active-subscriptions subquery, got %d", p.SubscriberCount)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestListAppCatalog_FallsBackToLPCopyWhenAppColumnsNull(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	lpCopy := `{"en":{"heroTitle":"LP Title","heDescription":"LP Tagline","successBody":"LP Description"}}`
	mock.ExpectQuery(`FROM campaigns c`).
		WithArgs("nrg").
		WillReturnRows(sqlmock.NewRows(appCatalogColumns).
			AddRow("nrg", "NRG", "daily-tips", nil, nil, "OTP", "NG",
				nil, nil, nil, nil, nil, nil, lpCopy, nil, 0))

	repo := NewCampaignRepository(db, zap.NewNop())
	products, err := repo.ListAppCatalog("nrg", "")
	if err != nil {
		t.Fatalf("ListAppCatalog: %v", err)
	}
	if len(products) != 1 {
		t.Fatalf("expected 1 product, got %d", len(products))
	}
	p := products[0]
	if p.Name != "LP Title" || p.Tagline != "LP Tagline" || p.Description != "LP Description" {
		t.Fatalf("expected lp_copy fallback fields, got %+v", p)
	}
	if p.Category != "" || p.ArtworkURL != "" || p.SampleContent != "" {
		t.Fatalf("expected no lp_copy source for category/artwork/sample, got %+v", p)
	}
	if p.Currency != "NGN" {
		t.Fatalf("expected currency NGN for country NG, got %q", p.Currency)
	}
	if p.Featured {
		t.Fatalf("expected NULL app_featured_rank to leave the product unfeatured, got %+v", p)
	}
	if p.SubscriberCount != 0 {
		t.Fatalf("expected subscriber_count 0, got %d", p.SubscriberCount)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestListAppCatalog_FallsBackToSlugWhenNoNameSource(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`FROM campaigns c`).
		WithArgs("nrg").
		WillReturnRows(sqlmock.NewRows(appCatalogColumns).
			AddRow("nrg", "NRG", "daily-tips", nil, nil, "OTP", "ZZ",
				nil, nil, nil, nil, nil, nil, nil, nil, 0))

	repo := NewCampaignRepository(db, zap.NewNop())
	products, err := repo.ListAppCatalog("nrg", "")
	if err != nil {
		t.Fatalf("ListAppCatalog: %v", err)
	}
	if len(products) != 1 || products[0].Name != "daily-tips" {
		t.Fatalf("expected slug fallback, got %+v", products)
	}
	if products[0].Currency != "" {
		t.Fatalf("expected empty currency for unknown country, got %q", products[0].Currency)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestListAppCatalog_FiltersByCountryWhenProvided(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`FROM campaigns c`).
		WithArgs("nrg", "GH").
		WillReturnRows(sqlmock.NewRows(appCatalogColumns))

	repo := NewCampaignRepository(db, zap.NewNop())
	if _, err := repo.ListAppCatalog("nrg", "GH"); err != nil {
		t.Fatalf("ListAppCatalog: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expected country filter to be applied as a second query arg: %v", err)
	}
}

func TestListAppCatalog_RequiresTenant(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewCampaignRepository(db, zap.NewNop())
	if _, err := repo.ListAppCatalog("  ", ""); err == nil {
		t.Fatalf("expected error for empty tenant")
	}
}

func TestCurrencyForCountry(t *testing.T) {
	cases := map[string]string{
		"GH": "GHS",
		"gh": "GHS",
		"NG": "NGN",
		"KE": "KES",
		"US": "",
		"":   "",
	}
	for country, want := range cases {
		if got := currencyForCountry(country); got != want {
			t.Fatalf("currencyForCountry(%q) = %q, want %q", country, got, want)
		}
	}
}
