package handler

import (
	"testing"
	"time"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/valyala/fasthttp"
)

func TestParseFilters_Defaults(t *testing.T) {
	h := &ReportsHandler{}
	ctx := &fasthttp.RequestCtx{}

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
