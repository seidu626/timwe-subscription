package handler

import (
	"database/sql"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/service"
	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

func TestCreateTenantReturnsCreatedTenantAndAuditReference(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO tenants")).
		WithArgs(sqlmock.AnyArg(), "tenant-a", "Tenant A", domain.TenantStatusActive, "GH", `{"tier":"gold"}`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_key", "name", "status", "default_country", "metadata_json", "created_at", "updated_at"}).
			AddRow("22222222-2222-2222-2222-222222222222", "tenant-a", "Tenant A", domain.TenantStatusActive, "GH", []byte(`{"tier":"gold"}`), now, now))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO admin_activity_logs")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	h := newTenantTestHandler(db)
	var ctx fasthttp.RequestCtx
	ctx.SetUserValue(tenantctx.FastHTTPUserValueKey, tenantctx.Identity{
		Subject:        "auth0|platform",
		PlatformScoped: true,
		TrustSource:    tenantctx.TrustSourceJWT,
	})
	ctx.Request.SetBodyString(`{"tenant_key":"Tenant-A","name":"Tenant A","status":"ACTIVE","default_country":"gh","metadata":{"tier":"gold"}}`)

	h.CreateTenant(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusCreated {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	var body map[string]any
	if err := json.Unmarshal(ctx.Response.Body(), &body); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	if body["tenant_key"] != "tenant-a" || body["audit_log_id"] == "" {
		t.Fatalf("unexpected response body: %#v", body)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCreateTenantRejectsTenantScopedAdmin(t *testing.T) {
	h := &AdminManagementHandler{
		service: service.NewAdminManagementService(nil, zap.NewNop()),
		logger:  zap.NewNop(),
	}
	var ctx fasthttp.RequestCtx
	ctx.SetUserValue(tenantctx.FastHTTPUserValueKey, tenantctx.Identity{
		TenantKey:   "tenant-a",
		Subject:     "auth0|tenant-admin",
		TrustSource: tenantctx.TrustSourceJWT,
	})
	ctx.Request.SetBodyString(`{"tenant_key":"tenant-b","name":"Tenant B","default_country":"GH"}`)

	h.CreateTenant(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusForbidden {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
}

func TestListTenantsReturnsCatalogForPlatformScopedAdmin(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM tenants WHERE")).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, tenant_key, name, status, default_country, metadata_json, created_at, updated_at")).
		WithArgs(20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_key", "name", "status", "default_country", "metadata_json", "created_at", "updated_at"}).
			AddRow("22222222-2222-2222-2222-222222222222", "nrg", "NRG", domain.TenantStatusActive, "GH", []byte(`{"kind":"canonical-default"}`), now, now))

	h := newTenantTestHandler(db)
	var ctx fasthttp.RequestCtx
	ctx.SetUserValue(tenantctx.FastHTTPUserValueKey, tenantctx.Identity{
		Subject:        "auth0|operator",
		PlatformScoped: true,
		TrustSource:    tenantctx.TrustSourceJWT,
	})

	h.ListTenants(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	var body struct {
		Tenants []domain.AdminTenant `json:"tenants"`
	}
	if err := json.Unmarshal(ctx.Response.Body(), &body); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	if len(body.Tenants) != 1 || body.Tenants[0].TenantKey != "nrg" {
		t.Fatalf("unexpected response body: %#v", body)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestListTenantsRejectsTenantScopedAdmin(t *testing.T) {
	h := &AdminManagementHandler{
		service: service.NewAdminManagementService(nil, zap.NewNop()),
		logger:  zap.NewNop(),
	}
	var ctx fasthttp.RequestCtx
	ctx.SetUserValue(tenantctx.FastHTTPUserValueKey, tenantctx.Identity{
		TenantKey:   "nrg",
		Subject:     "auth0|tenant-admin",
		TrustSource: tenantctx.TrustSourceJWT,
	})

	h.ListTenants(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusForbidden {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
}

func TestUpdateTenantPatchesCatalogAndReturnsAuditReference(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	tenantID := "22222222-2222-2222-2222-222222222222"
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, tenant_key, name, status, default_country, metadata_json, created_at, updated_at")).
		WithArgs(tenantID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_key", "name", "status", "default_country", "metadata_json", "created_at", "updated_at"}).
			AddRow(tenantID, "nrg", "NRG", domain.TenantStatusActive, "GH", []byte(`{}`), now, now))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("UPDATE tenants")).
		WithArgs(tenantID, "NRG Prime", domain.TenantStatusActive, "GH", `{"tier":"gold"}`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_key", "name", "status", "default_country", "metadata_json", "created_at", "updated_at"}).
			AddRow(tenantID, "nrg", "NRG Prime", domain.TenantStatusActive, "GH", []byte(`{"tier":"gold"}`), now, now))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO admin_activity_logs")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	h := newTenantTestHandler(db)
	var ctx fasthttp.RequestCtx
	ctx.Request.SetRequestURI("/v1/admin/tenants/" + tenantID)
	ctx.SetUserValue(tenantctx.FastHTTPUserValueKey, tenantctx.Identity{
		Subject:        "auth0|operator",
		PlatformScoped: true,
		TrustSource:    tenantctx.TrustSourceJWT,
	})
	ctx.Request.SetBodyString(`{"name":"NRG Prime","status":"ACTIVE","default_country":"gh","metadata":{"tier":"gold"}}`)

	h.UpdateTenant(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	var body map[string]any
	if err := json.Unmarshal(ctx.Response.Body(), &body); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	if body["name"] != "NRG Prime" || body["audit_log_id"] == "" {
		t.Fatalf("unexpected response body: %#v", body)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestUpdateTenantRejectsTenantScopedAdmin(t *testing.T) {
	h := &AdminManagementHandler{
		service: service.NewAdminManagementService(nil, zap.NewNop()),
		logger:  zap.NewNop(),
	}
	var ctx fasthttp.RequestCtx
	ctx.Request.SetRequestURI("/v1/admin/tenants/22222222-2222-2222-2222-222222222222")
	ctx.SetUserValue(tenantctx.FastHTTPUserValueKey, tenantctx.Identity{
		TenantKey:   "nrg",
		Subject:     "auth0|tenant-admin",
		TrustSource: tenantctx.TrustSourceJWT,
	})
	ctx.Request.SetBodyString(`{"name":"NRG Prime"}`)

	h.UpdateTenant(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusForbidden {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
}

func TestGetCurrentTenantDoesNotTrustRawTenantHeader(t *testing.T) {
	h := &AdminManagementHandler{
		service: service.NewAdminManagementService(nil, zap.NewNop()),
		logger:  zap.NewNop(),
	}
	var ctx fasthttp.RequestCtx
	ctx.Request.Header.Set("X-Tenant-Id", "22222222-2222-2222-2222-222222222222")

	h.GetCurrentTenant(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusForbidden {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
}

func TestGetCurrentTenantHidesInactiveAndUnknownTenants(t *testing.T) {
	inactiveStatus, inactiveBody := currentTenantResponseFor(t, func(mock sqlmock.Sqlmock, now time.Time) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id, tenant_key, name, status, default_country, metadata_json, created_at, updated_at")).
			WithArgs("tenant-a").
			WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_key", "name", "status", "default_country", "metadata_json", "created_at", "updated_at"}).
				AddRow("22222222-2222-2222-2222-222222222222", "tenant-a", "Tenant A", domain.TenantStatusInactive, "GH", []byte(`{}`), now, now))
	})
	unknownStatus, unknownBody := currentTenantResponseFor(t, func(mock sqlmock.Sqlmock, _ time.Time) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id, tenant_key, name, status, default_country, metadata_json, created_at, updated_at")).
			WithArgs("tenant-a").
			WillReturnError(sql.ErrNoRows)
	})

	if inactiveStatus != unknownStatus || string(inactiveBody) != string(unknownBody) {
		t.Fatalf("inactive (%d, %q) and unknown (%d, %q) responses differ", inactiveStatus, inactiveBody, unknownStatus, unknownBody)
	}
	if inactiveStatus != fasthttp.StatusForbidden {
		t.Fatalf("status=%d body=%q", inactiveStatus, inactiveBody)
	}
}

func TestParseJSONImportRowsRejectsTenantInjection(t *testing.T) {
	_, err := parseImportRows("userbase.json", strings.NewReader(`[{"tenant_id":"22222222-2222-2222-2222-222222222222","msisdn":"0201234567","type":"ALLOWLISTED"}]`))
	if err == nil || !strings.Contains(err.Error(), "must not include tenant_id") {
		t.Fatalf("expected tenant_id rejection, got %v", err)
	}
}

func TestParseJSONImportRowsAcceptsTenantNeutralRows(t *testing.T) {
	rows, err := parseImportRows("userbase.json", strings.NewReader(`[{"msisdn":"0201234567","type":"ALLOWLISTED"}]`))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(rows) != 1 || rows[0].MSISDN != "0201234567" || rows[0].Type != "ALLOWLISTED" || rows[0].RowNumber != 1 {
		t.Fatalf("unexpected rows: %#v", rows)
	}
}

func TestCreateChannelRejectsUnsupportedCapability(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, tenant_key, name, status, default_country, metadata_json, created_at, updated_at")).
		WithArgs("tenant-a").
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_key", "name", "status", "default_country", "metadata_json", "created_at", "updated_at"}).
			AddRow("22222222-2222-2222-2222-222222222222", "tenant-a", "Tenant A", domain.TenantStatusActive, "GH", []byte(`{}`), now, now))

	h := newTenantTestHandler(db)
	var ctx fasthttp.RequestCtx
	ctx.SetUserValue(tenantctx.FastHTTPUserValueKey, tenantctx.Identity{
		TenantKey:   "tenant-a",
		Subject:     "auth0|tenant-admin",
		TrustSource: tenantctx.TrustSourceJWT,
	})
	ctx.Request.SetBodyString(`{"provider":"timwe","country":"GH","capabilities":["optin","fax"]}`)

	h.CreateChannel(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusBadRequest || !strings.Contains(string(ctx.Response.Body()), "invalid_capability") {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestBindChannelCredentialRawSecretBackendUnavailableDoesNotEchoSecret(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	tenantID := "22222222-2222-2222-2222-222222222222"
	channelID := "33333333-3333-3333-3333-333333333333"
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, tenant_key, name, status, default_country, metadata_json, created_at, updated_at")).
		WithArgs("tenant-a").
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_key", "name", "status", "default_country", "metadata_json", "created_at", "updated_at"}).
			AddRow(tenantID, "tenant-a", "Tenant A", domain.TenantStatusActive, "GH", []byte(`{}`), now, now))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, tenant_id, channel_key, provider, country, operator, capabilities, status, created_at, updated_at")).
		WithArgs(tenantID, channelID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "channel_key", "provider", "country", "operator", "capabilities", "status", "created_at", "updated_at"}).
			AddRow(channelID, tenantID, "timwe-gh-airteltigo", "timwe", "GH", nil, "{optin,mt}", domain.ChannelStatusActive, now, now))

	h := newTenantTestHandler(db)
	var ctx fasthttp.RequestCtx
	ctx.SetUserValue(tenantctx.FastHTTPUserValueKey, tenantctx.Identity{
		TenantKey:   "tenant-a",
		Subject:     "auth0|tenant-admin",
		TrustSource: tenantctx.TrustSourceJWT,
	})
	ctx.Request.SetRequestURI("/v1/admin/channels/" + channelID + "/credentials")
	ctx.Request.SetBodyString(`{"secret_value":"super-secret"}`)

	h.BindChannelCredential(&ctx)

	body := string(ctx.Response.Body())
	if ctx.Response.StatusCode() != fasthttp.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), body)
	}
	if strings.Contains(body, "super-secret") || strings.Contains(strings.ToLower(body), "secret_value") {
		t.Fatalf("response leaked secret-bearing input: %q", body)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func currentTenantResponseFor(t *testing.T, expect func(sqlmock.Sqlmock, time.Time)) (int, []byte) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	expect(mock, now)
	h := newTenantTestHandler(db)
	var ctx fasthttp.RequestCtx
	ctx.SetUserValue(tenantctx.FastHTTPUserValueKey, tenantctx.Identity{
		TenantKey:   "tenant-a",
		Subject:     "auth0|tenant-admin",
		TrustSource: tenantctx.TrustSourceJWT,
	})

	h.GetCurrentTenant(&ctx)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
	body := append([]byte(nil), ctx.Response.Body()...)
	return ctx.Response.StatusCode(), body
}

func newTenantTestHandler(db *sql.DB) *AdminManagementHandler {
	repo := repository.NewAdminManagementRepository(db, zap.NewNop())
	return NewAdminManagementHandler(service.NewAdminManagementService(repo, zap.NewNop()), zap.NewNop())
}
