package repository

import (
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"go.uber.org/zap"
)

func TestReportQueryBuildersIncludeTenantAndChannelPredicates(t *testing.T) {
	tenantID := "11111111-1111-1111-1111-111111111111"
	channelID := "22222222-2222-2222-2222-222222222222"
	campaignSlug := "gh-campaign"
	filters := domain.ReportFilters{
		StartDate:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:      time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		TenantID:     &tenantID,
		ChannelID:    &channelID,
		CampaignSlug: &campaignSlug,
	}

	landingQuery, landingArgs := buildLandingAggregateQuery(filters)
	assertContains(t, landingQuery, "EXISTS (SELECT 1 FROM campaigns c")
	assertContains(t, landingQuery, "c.tenant_id = $4::uuid")
	assertContains(t, landingQuery, "c.channel_id = $5::uuid")
	if len(landingArgs) != 6 {
		t.Fatalf("landing args = %d, want 6", len(landingArgs))
	}

	txQuery, txArgs := buildTransactionAggregateQuery(filters)
	assertContains(t, txQuery, "at.tenant_id = $3::uuid")
	assertContains(t, txQuery, "at.campaign_slug = $4")
	assertContains(t, txQuery, "c.channel_id = $6::uuid")
	if len(txArgs) != 7 {
		t.Fatalf("transaction args = %d, want 7", len(txArgs))
	}
}

func TestTransactionCampaignPredicateRequiresMatchingTenant(t *testing.T) {
	got := transactionCampaignPredicate("c", "at")
	assertContains(t, got, "c.slug = at.campaign_slug")
	assertContains(t, got, "c.tenant_id = at.tenant_id")
	if stringsContains(got, "tenant_id IS NULL") {
		t.Fatalf("transaction campaign predicate must not accept tenantless campaign rows: %s", got)
	}
}

func TestGetKPIsReturnsZeroedTenantReportWithFiltersEchoed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	tenantID := "11111111-1111-1111-1111-111111111111"
	filters := domain.ReportFilters{
		StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		TenantID:  &tenantID,
	}

	mock.ExpectQuery("(?s)FROM landing_events le.*c.tenant_id = \\$3::uuid").
		WithArgs(filters.StartDate, filters.EndDate, tenantID).
		WillReturnRows(sqlmock.NewRows([]string{"views", "clicks"}).AddRow(0, 0))
	mock.ExpectQuery("(?s)FROM acquisition_transactions at.*at.tenant_id = \\$3::uuid").
		WithArgs(filters.StartDate, filters.EndDate, tenantID).
		WillReturnRows(sqlmock.NewRows([]string{"total", "subscribed", "charged"}).AddRow(0, 0, 0))
	mock.ExpectQuery("(?s)LEFT JOIN campaigns c.*at.tenant_id = \\$3::uuid").
		WithArgs(filters.StartDate, filters.EndDate, tenantID).
		WillReturnRows(sqlmock.NewRows([]string{"revenue"}).AddRow(0.0))

	repo := NewReportsRepository(db, zap.NewNop())
	got, err := repo.GetKPIs(filters)
	if err != nil {
		t.Fatalf("GetKPIs: %v", err)
	}
	if got.LandingViews != 0 || got.Transactions != 0 || got.Charged != 0 || got.EstimatedRevenue != 0 {
		t.Fatalf("expected zeroed KPI report, got %#v", got)
	}
	if got.Filters.TenantID == nil || *got.Filters.TenantID != tenantID {
		t.Fatalf("tenant filter not echoed: %#v", got.Filters)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestGetKPIsExposesRevenueDatasourceFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	tenantID := "11111111-1111-1111-1111-111111111111"
	filters := domain.ReportFilters{
		StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		TenantID:  &tenantID,
	}

	mock.ExpectQuery("(?s)FROM landing_events le").
		WillReturnRows(sqlmock.NewRows([]string{"views", "clicks"}).AddRow(1, 1))
	mock.ExpectQuery("(?s)FROM acquisition_transactions at").
		WillReturnRows(sqlmock.NewRows([]string{"total", "subscribed", "charged"}).AddRow(1, 1, 1))
	mock.ExpectQuery("(?s)LEFT JOIN campaigns c").
		WillReturnError(errors.New("revenue source offline"))

	repo := NewReportsRepository(db, zap.NewNop())
	_, err = repo.GetKPIs(filters)
	if err == nil {
		t.Fatal("expected revenue datasource failure")
	}
	assertContains(t, err.Error(), "failed to calculate revenue")
}

func TestGetCampaignPerformanceReturnsEmptyArrayWhenNoRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	filters := domain.ReportFilters{
		StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	}

	mock.ExpectQuery("(?s)FROM campaigns c").
		WithArgs(filters.StartDate, filters.EndDate).
		WillReturnRows(sqlmock.NewRows([]string{
			"slug",
			"country",
			"views",
			"transactions",
			"subscribed",
			"charged",
			"revenue",
		}))

	repo := NewReportsRepository(db, zap.NewNop())
	got, err := repo.GetCampaignPerformance(filters)
	if err != nil {
		t.Fatalf("GetCampaignPerformance: %v", err)
	}
	if got.Campaigns == nil {
		t.Fatal("expected campaigns to be an empty slice, got nil")
	}
	if len(got.Campaigns) != 0 {
		t.Fatalf("campaigns len = %d, want 0", len(got.Campaigns))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestGetTimeSeriesReturnsEmptyArrayWhenNoRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	filters := domain.ReportFilters{
		StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	}

	mock.ExpectQuery("(?s)FROM landing_events le").
		WithArgs(filters.StartDate, filters.EndDate).
		WillReturnRows(sqlmock.NewRows([]string{"timestamp", "views"}))
	mock.ExpectQuery("(?s)FROM acquisition_transactions at").
		WithArgs(filters.StartDate, filters.EndDate).
		WillReturnRows(sqlmock.NewRows([]string{"timestamp", "transactions", "subscribed", "charged", "revenue"}))

	repo := NewReportsRepository(db, zap.NewNop())
	got, err := repo.GetTimeSeries(filters, "daily")
	if err != nil {
		t.Fatalf("GetTimeSeries: %v", err)
	}
	if got.DataPoints == nil {
		t.Fatal("expected data points to be an empty slice, got nil")
	}
	if len(got.DataPoints) != 0 {
		t.Fatalf("data points len = %d, want 0", len(got.DataPoints))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !stringsContains(got, want) {
		t.Fatalf("expected %q to contain %q", got, want)
	}
}

func stringsContains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return substr == ""
}
