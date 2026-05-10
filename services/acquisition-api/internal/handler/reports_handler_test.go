package handler

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

func TestParseFilters_Defaults(t *testing.T) {
	h := &ReportsHandler{}
	ctx := &fasthttp.RequestCtx{}
	setReportTenant(ctx)

	filters, err := h.parseFilters(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have defaults (30 days ago to tomorrow)
	now := time.Now().UTC()
	expectedStart := now.AddDate(0, 0, -30).Truncate(24 * time.Hour)
	expectedEnd := now.AddDate(0, 0, 1).Truncate(24 * time.Hour)

	if filters.StartDate.Before(expectedStart.Add(-1*time.Hour)) || filters.StartDate.After(expectedStart.Add(1*time.Hour)) {
		t.Errorf("StartDate %v not near expected %v", filters.StartDate, expectedStart)
	}

	if filters.EndDate.Before(expectedEnd.Add(-1*time.Hour)) || filters.EndDate.After(expectedEnd.Add(1*time.Hour)) {
		t.Errorf("EndDate %v not near expected %v", filters.EndDate, expectedEnd)
	}

	if filters.CampaignSlug != nil {
		t.Errorf("CampaignSlug should be nil, got %v", filters.CampaignSlug)
	}
}

func TestParseFilters_WithParams(t *testing.T) {
	h := &ReportsHandler{}
	ctx := &fasthttp.RequestCtx{}
	setReportTenant(ctx)
	ctx.QueryArgs().Set("startDate", "2026-01-01")
	ctx.QueryArgs().Set("endDate", "2026-01-15")
	ctx.QueryArgs().Set("campaignSlug", "gh-test")
	ctx.QueryArgs().Set("country", "GH")

	filters, err := h.parseFilters(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedStart, _ := time.Parse("2006-01-02", "2026-01-01")
	expectedEnd, _ := time.Parse("2006-01-02", "2026-01-16") // +1 day

	if !filters.StartDate.Equal(expectedStart) {
		t.Errorf("StartDate = %v, want %v", filters.StartDate, expectedStart)
	}

	if !filters.EndDate.Equal(expectedEnd) {
		t.Errorf("EndDate = %v, want %v", filters.EndDate, expectedEnd)
	}

	if filters.CampaignSlug == nil || *filters.CampaignSlug != "gh-test" {
		t.Errorf("CampaignSlug = %v, want gh-test", filters.CampaignSlug)
	}

	if filters.Country == nil || *filters.Country != "GH" {
		t.Errorf("Country = %v, want GH", filters.Country)
	}
}

func TestParseFilters_InvalidDates(t *testing.T) {
	h := &ReportsHandler{}

	tests := []struct {
		name       string
		startDate  string
		endDate    string
		shouldFail bool
	}{
		{"invalid start date", "not-a-date", "", true},
		{"invalid end date", "", "not-a-date", true},
		{"end before start", "2026-01-15", "2026-01-01", true},
		{"valid dates", "2026-01-01", "2026-01-15", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &fasthttp.RequestCtx{}
			setReportTenant(ctx)
			if tt.startDate != "" {
				ctx.QueryArgs().Set("startDate", tt.startDate)
			}
			if tt.endDate != "" {
				ctx.QueryArgs().Set("endDate", tt.endDate)
			}

			_, err := h.parseFilters(ctx)

			if tt.shouldFail {
				if err == nil {
					t.Error("expected error, got nil")
				}
				// Check it's a ValidationError
				if _, ok := err.(*domain.ValidationError); !ok {
					t.Errorf("expected ValidationError, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestParseFilters_RejectsUnauthorizedAllTenants(t *testing.T) {
	h := &ReportsHandler{}
	ctx := &fasthttp.RequestCtx{}
	setReportTenant(ctx)
	ctx.QueryArgs().Set("all_tenants", "true")

	_, err := h.parseFilters(ctx)
	if err == nil {
		t.Fatal("expected all_tenants to be rejected for tenant admin")
	}
	filterErr, ok := err.(reportFilterError)
	if !ok {
		t.Fatalf("expected reportFilterError, got %T", err)
	}
	if filterErr.status != fasthttp.StatusForbidden || filterErr.code != "tenant_aggregation_forbidden" {
		t.Fatalf("unexpected error: %#v", filterErr)
	}
}

func TestParseFilters_AllowsPlatformAllTenants(t *testing.T) {
	h := &ReportsHandler{}
	ctx := &fasthttp.RequestCtx{}
	ctx.SetUserValue(tenantctx.FastHTTPUserValueKey, tenantctx.Identity{
		PlatformScoped: true,
		TrustSource:    tenantctx.TrustSourceJWT,
	})
	ctx.QueryArgs().Set("all_tenants", "true")

	filters, err := h.parseFilters(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !filters.AllTenants {
		t.Fatal("expected all_tenants filter")
	}
	if filters.TenantID != nil {
		t.Fatalf("platform all_tenants should not force tenant id, got %v", *filters.TenantID)
	}
}

func TestParseFilters_ResolvesPlatformSelectedTenantKey(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id")).
		WithArgs("nrg").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("11111111-1111-1111-1111-111111111111"))

	h := &ReportsHandler{reportsRepo: repository.NewReportsRepository(db, zap.NewNop())}
	ctx := &fasthttp.RequestCtx{}
	ctx.SetUserValue(tenantctx.FastHTTPUserValueKey, tenantctx.Identity{
		TenantKey:      "nrg",
		PlatformScoped: true,
		TrustSource:    tenantctx.TrustSourceJWT,
	})

	filters, err := h.parseFilters(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filters.TenantID == nil || *filters.TenantID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("tenant id = %v", filters.TenantID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestParseFilters_InvalidChannelSyntax(t *testing.T) {
	h := &ReportsHandler{}
	ctx := &fasthttp.RequestCtx{}
	setReportTenant(ctx)
	ctx.QueryArgs().Set("channel_id", "unknown-channel")

	_, err := h.parseFilters(ctx)
	if err == nil {
		t.Fatal("expected invalid channel error")
	}
	filterErr, ok := err.(reportFilterError)
	if !ok {
		t.Fatalf("expected reportFilterError, got %T", err)
	}
	if filterErr.status != fasthttp.StatusBadRequest || filterErr.code != "invalid_channel" {
		t.Fatalf("unexpected error: %#v", filterErr)
	}
}

func setReportTenant(ctx *fasthttp.RequestCtx) {
	ctx.SetUserValue(tenantctx.FastHTTPUserValueKey, tenantctx.Identity{
		TenantID:    "11111111-1111-1111-1111-111111111111",
		TrustSource: tenantctx.TrustSourceJWT,
	})
}
