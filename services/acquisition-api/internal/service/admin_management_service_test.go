package service

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"go.uber.org/zap"
)

func TestCreateTenantRequiresPlatformScope(t *testing.T) {
	svc := NewAdminManagementService(nil, zap.NewNop())
	_, _, err := svc.CreateTenant(&domain.TenantCreateInput{
		TenantKey:      "tenant-a",
		Name:           "Tenant A",
		Status:         domain.TenantStatusActive,
		DefaultCountry: "GH",
		Metadata:       []byte(`{}`),
	}, tenantctx.Identity{TenantKey: "tenant-a"}, nil, nil)
	if !errors.Is(err, ErrAdminForbidden) {
		t.Fatalf("expected ErrAdminForbidden, got %v", err)
	}
}

func TestValidateTenantCreateInputNormalizesAndRejectsInvalidMetadata(t *testing.T) {
	input := &domain.TenantCreateInput{
		TenantKey:      "  Tenant-A  ",
		Name:           "  Tenant A  ",
		DefaultCountry: "gh",
		Metadata:       []byte(`{"tier":"gold"}`),
	}
	if err := validateTenantCreateInput(input); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}
	if input.TenantKey != "tenant-a" || input.Name != "Tenant A" || input.DefaultCountry != "GH" || input.Status != domain.TenantStatusActive {
		t.Fatalf("input not normalized: %#v", input)
	}

	input.Metadata = []byte(`null`)
	if err := validateTenantCreateInput(input); err == nil || !strings.Contains(err.Error(), "metadata must be a JSON object") {
		t.Fatalf("expected metadata object error, got %v", err)
	}
}

func TestResolveCurrentTenantRejectsMissingAcceptedContext(t *testing.T) {
	svc := NewAdminManagementService(nil, zap.NewNop())
	_, err := svc.ResolveCurrentTenant(tenantctx.Identity{})
	if !errors.Is(err, ErrTenantContextMissing) {
		t.Fatalf("expected ErrTenantContextMissing, got %v", err)
	}
}

func TestResolveCurrentTenantHidesInactiveTenant(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, tenant_key, name, status, default_country, metadata_json, created_at, updated_at")).
		WithArgs("tenant-a").
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_key", "name", "status", "default_country", "metadata_json", "created_at", "updated_at"}).
			AddRow("22222222-2222-2222-2222-222222222222", "tenant-a", "Tenant A", domain.TenantStatusInactive, "GH", []byte(`{}`), now, now))

	svc := NewAdminManagementService(repository.NewAdminManagementRepository(db, zap.NewNop()), zap.NewNop())
	_, err = svc.ResolveCurrentTenant(tenantctx.Identity{TenantKey: "Tenant-A"})
	if !errors.Is(err, ErrTenantUnavailable) {
		t.Fatalf("expected ErrTenantUnavailable, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestNormalizeChannelCreateInputCanonicalizesCapabilitiesAndKey(t *testing.T) {
	enabled := true
	operator := " AirtelTigo "
	input := &domain.ChannelCreateInput{
		Provider:     " TIMWE ",
		Country:      "gh",
		Operator:     &operator,
		Capabilities: []string{"mt", "optin", "MT", "confirm"},
		Enabled:      &enabled,
	}

	out, err := normalizeChannelCreateInput(input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out.ChannelKey != "timwe-gh-airteltigo" || out.Provider != "timwe" || out.Country != "GH" {
		t.Fatalf("unexpected normalization: %#v", out)
	}
	if got := strings.Join(out.Capabilities, ","); got != "confirm,mt,optin" {
		t.Fatalf("unexpected capabilities: %s", got)
	}
}

func TestNormalizeChannelCreateInputRejectsInvalidCapability(t *testing.T) {
	_, err := normalizeChannelCreateInput(&domain.ChannelCreateInput{
		Provider:     "timwe",
		Country:      "GH",
		Capabilities: []string{"optin", "fax"},
	})
	if !errors.Is(err, ErrInvalidInput) || !strings.Contains(err.Error(), "invalid_capability") {
		t.Fatalf("expected invalid_capability error, got %v", err)
	}
}

func TestNormalizeChannelCreateInputRejectsChargeWithoutMT(t *testing.T) {
	_, err := normalizeChannelCreateInput(&domain.ChannelCreateInput{
		Provider:     "timwe",
		Country:      "GH",
		Capabilities: []string{"charge"},
	})
	if !errors.Is(err, ErrInvalidInput) || !strings.Contains(err.Error(), "charge requires mt") {
		t.Fatalf("expected charge dependency error, got %v", err)
	}
}
