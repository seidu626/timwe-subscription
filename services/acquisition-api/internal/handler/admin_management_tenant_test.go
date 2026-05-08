package handler

import (
	"database/sql"
	"encoding/json"
	"regexp"
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
