package service

import (
	"context"
	"encoding/json"
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

type countingCredentialStore struct {
	calls int
}

func (s *countingCredentialStore) PutChannelCredential(ctx context.Context, input ChannelCredentialSecretInput) (ChannelCredentialSecretRef, error) {
	s.calls++
	return ChannelCredentialSecretRef{SecretRef: "vault://tenant/channel/provider", SecretRefDisplay: "vault://[REDACTED]"}, nil
}

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

func TestListTenantsRequiresPlatformScope(t *testing.T) {
	svc := NewAdminManagementService(nil, zap.NewNop())
	_, _, err := svc.ListTenants(tenantctx.Identity{TenantKey: "nrg"}, &domain.TenantListFilter{})
	if !errors.Is(err, ErrAdminForbidden) {
		t.Fatalf("expected ErrAdminForbidden, got %v", err)
	}
}

func TestValidateTenantUpdateInputNormalizesPartialPatch(t *testing.T) {
	name := "  NRG Prime  "
	status := domain.TenantStatus("inactive")
	country := "gh"
	metadata := json.RawMessage(`{"tier":"gold"}`)
	input := &domain.TenantUpdateInput{
		Name:           &name,
		Status:         &status,
		DefaultCountry: &country,
		Metadata:       &metadata,
	}
	if err := validateTenantUpdateInput(input); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}
	if *input.Name != "NRG Prime" || *input.Status != domain.TenantStatusInactive || *input.DefaultCountry != "GH" {
		t.Fatalf("input not normalized: %#v", input)
	}

	invalid := json.RawMessage(`[]`)
	input = &domain.TenantUpdateInput{Metadata: &invalid}
	if err := validateTenantUpdateInput(input); err == nil || !strings.Contains(err.Error(), "metadata must be a JSON object") {
		t.Fatalf("expected metadata object error, got %v", err)
	}
}

func TestUpdateTenantRequiresPlatformScope(t *testing.T) {
	svc := NewAdminManagementService(nil, zap.NewNop())
	name := "NRG Prime"
	_, _, err := svc.UpdateTenant("22222222-2222-2222-2222-222222222222", &domain.TenantUpdateInput{Name: &name}, tenantctx.Identity{TenantKey: "nrg"}, nil, nil)
	if !errors.Is(err, ErrAdminForbidden) {
		t.Fatalf("expected ErrAdminForbidden, got %v", err)
	}
}

func TestUpdateTenantAuditsCatalogChange(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	tenantID := "22222222-2222-2222-2222-222222222222"
	name := "NRG Prime"
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, tenant_key, name, status, default_country, metadata_json, created_at, updated_at")).
		WithArgs(tenantID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_key", "name", "status", "default_country", "metadata_json", "created_at", "updated_at"}).
			AddRow(tenantID, "nrg", "NRG", domain.TenantStatusActive, "GH", []byte(`{}`), now, now))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("UPDATE tenants")).
		WithArgs(tenantID, name, nil, nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_key", "name", "status", "default_country", "metadata_json", "created_at", "updated_at"}).
			AddRow(tenantID, "nrg", "NRG Prime", domain.TenantStatusActive, "GH", []byte(`{}`), now, now))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO admin_activity_logs")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	svc := NewAdminManagementService(repository.NewAdminManagementRepository(db, zap.NewNop()), zap.NewNop())
	tenant, auditID, err := svc.UpdateTenant(tenantID, &domain.TenantUpdateInput{Name: &name}, tenantctx.Identity{PlatformScoped: true, Subject: "auth0|operator"}, nil, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if tenant.Name != "NRG Prime" || auditID == "" {
		t.Fatalf("unexpected result: tenant=%#v auditID=%q", tenant, auditID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
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

func TestBindChannelCredentialRejectsInactiveBeforeSecretStore(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	tenantID := "22222222-2222-2222-2222-222222222222"
	channelID := "33333333-3333-3333-3333-333333333333"
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, tenant_id, channel_key, provider, country, operator, capabilities, status, created_at, updated_at")).
		WithArgs(tenantID, channelID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "channel_key", "provider", "country", "operator", "capabilities", "status", "created_at", "updated_at"}).
			AddRow(channelID, tenantID, "timwe-gh-airteltigo", "timwe", "GH", nil, "{optin,mt}", domain.ChannelStatusInactive, now, now))

	store := &countingCredentialStore{}
	svc := NewAdminManagementService(repository.NewAdminManagementRepository(db, zap.NewNop()), zap.NewNop())
	svc.SetChannelCredentialSecretStore(store)

	_, err = svc.BindChannelCredential(context.Background(), tenantID, channelID, &domain.ChannelCredentialBindInput{
		SecretValue: "super-secret",
	}, nil, nil)
	if !errors.Is(err, ErrAdminInvalidState) || !strings.Contains(err.Error(), "channel_inactive") {
		t.Fatalf("expected channel_inactive error, got %v", err)
	}
	if store.calls != 0 {
		t.Fatalf("secret store was called before inactive channel rejection")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestBindChannelCredentialRequiresBackendForRawSecret(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	tenantID := "22222222-2222-2222-2222-222222222222"
	channelID := "33333333-3333-3333-3333-333333333333"
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, tenant_id, channel_key, provider, country, operator, capabilities, status, created_at, updated_at")).
		WithArgs(tenantID, channelID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "channel_key", "provider", "country", "operator", "capabilities", "status", "created_at", "updated_at"}).
			AddRow(channelID, tenantID, "timwe-gh-airteltigo", "timwe", "GH", nil, "{optin,mt}", domain.ChannelStatusActive, now, now))

	svc := NewAdminManagementService(repository.NewAdminManagementRepository(db, zap.NewNop()), zap.NewNop())
	_, err = svc.BindChannelCredential(context.Background(), tenantID, channelID, &domain.ChannelCredentialBindInput{
		SecretValue: "super-secret",
	}, nil, nil)
	if !errors.Is(err, ErrAdminDependencyUnavailable) {
		t.Fatalf("expected dependency error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
