package handler

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

func TestRetryPostbackRequiresTenantContext(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	h := NewPostbackAdminHandler(repository.NewPostbackRepository(db, zap.NewNop()), zap.NewNop())
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/v1/admin/postbacks/" + uuid.NewString() + "/retry")
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)

	h.RetryPostback(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusForbidden {
		t.Fatalf("expected 403 without tenant context, got %d", ctx.Response.StatusCode())
	}
}

func TestRetryPostbackCrossTenantReturnsNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	tenantID := "11111111-1111-1111-1111-111111111111"
	postbackID := uuid.New()
	mock.ExpectExec("UPDATE postback_outbox").
		WithArgs(tenantID, postbackID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	h := NewPostbackAdminHandler(repository.NewPostbackRepository(db, zap.NewNop()), zap.NewNop())
	ctx := &fasthttp.RequestCtx{}
	ctx.SetUserValue(tenantctx.FastHTTPUserValueKey, tenantctx.Identity{
		TenantID:    tenantID,
		TrustSource: tenantctx.TrustSourceJWT,
	})
	ctx.Request.SetRequestURI("/v1/admin/postbacks/" + postbackID.String() + "/retry")
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)

	h.RetryPostback(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusNotFound {
		t.Fatalf("expected 404 for cross-tenant retry, got %d body=%s", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestPostbackTenantIDFromRequestResolvesPlatformTenantKey(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("FROM tenants").
		WithArgs("nrg").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("66d39a9a-f1ef-4721-a31c-5bb966d25c3d"))

	h := NewPostbackAdminHandler(repository.NewPostbackRepository(db, zap.NewNop()), zap.NewNop())
	ctx := &fasthttp.RequestCtx{}
	ctx.SetUserValue(tenantctx.FastHTTPUserValueKey, tenantctx.Identity{
		PlatformScoped: true,
		TenantKey:      "nrg",
		TrustSource:    tenantctx.TrustSourceJWT,
	})

	tenantID, ok := h.postbackTenantIDFromRequest(ctx)
	if !ok {
		t.Fatal("expected tenant key to resolve")
	}
	if tenantID != "66d39a9a-f1ef-4721-a31c-5bb966d25c3d" {
		t.Fatalf("tenantID = %q", tenantID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
