package repository

import (
	"os"
	"strings"
	"testing"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
)

func TestMissingCapabilitiesForFlowRequiresOTPOptInAndConfirm(t *testing.T) {
	missing := missingCapabilitiesForFlow(domain.FlowTypeOTP, []string{"optin"})
	if len(missing) != 1 || missing[0] != "confirm" {
		t.Fatalf("expected missing confirm, got %#v", missing)
	}

	if missing := missingCapabilitiesForFlow(domain.FlowTypeOTP, []string{"optin", "confirm"}); len(missing) != 0 {
		t.Fatalf("expected no missing capabilities, got %#v", missing)
	}
}

func TestTenantCampaignMigrationUsesScopedSlugUniqueness(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/add_tenant_z_campaign_binding.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(raw)
	for _, expected := range []string{
		"idx_campaigns_tenant_slug",
		"ON campaigns (tenant_id, slug)",
		"idx_campaigns_legacy_slug",
		"campaigns_tenant_channel_fk",
	} {
		if !strings.Contains(sql, expected) {
			t.Fatalf("expected migration to contain %q", expected)
		}
	}
}
