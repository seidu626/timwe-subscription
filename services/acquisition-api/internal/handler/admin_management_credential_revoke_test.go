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

const (
	revokeTestTenantID    = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	revokeTestChannelID   = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	revokeTestCredID      = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	revokeTestSecretRowID = "dddddddd-dddd-dddd-dddd-dddddddddddd"
)

func credentialCols() []string {
	return []string{
		"id", "tenant_id", "channel_id", "purpose", "version", "status",
		"secret_ref", "secret_ref_display", "secret_fingerprint",
		"created_by", "created_at", "updated_at", "activated_at", "deactivated_at", "purged_at",
	}
}

func addCredentialRow(rows *sqlmock.Rows, now time.Time, status domain.ChannelCredentialStatus, purgedAt interface{}) *sqlmock.Rows {
	return rows.AddRow(
		revokeTestCredID, revokeTestTenantID, revokeTestChannelID,
		"provider_api", 1, status,
		"secret://"+revokeTestSecretRowID, "secret://****",
		"aaaa1111bbbb2222cccc3333dddd4444eeee5555ffff6666aaaa1111bbbb2222",
		"auth0|operator",
		now, now, now, nil, purgedAt,
	)
}

func expectTenantLookupRevoke(mock sqlmock.Sqlmock, now time.Time) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, tenant_key, name, status, default_country, metadata_json, created_at, updated_at")).
		WithArgs("tenant-a").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "tenant_key", "name", "status", "default_country", "metadata_json", "created_at", "updated_at",
		}).AddRow(revokeTestTenantID, "tenant-a", "Tenant A", domain.TenantStatusActive, "GH", []byte(`{}`), now, now))
}

func newRevokeTestHandler(db *sql.DB) *AdminManagementHandler {
	repo := repository.NewAdminManagementRepository(db, zap.NewNop())
	return NewAdminManagementHandler(service.NewAdminManagementService(repo, zap.NewNop()), zap.NewNop())
}

func revokeIdentity() tenantctx.Identity {
	return tenantctx.Identity{
		TenantKey:   "tenant-a",
		Subject:     "auth0|operator",
		TrustSource: tenantctx.TrustSourceJWT,
	}
}

func TestRevokeChannelCredentialReturnsOKAndPurgesCiphertext(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	expectTenantLookupRevoke(mock, now)

	// pre-fetch (service.GetChannelCredentialByID)
	mock.ExpectQuery(regexp.QuoteMeta("FROM tenant_channel_credentials\n\t\tWHERE tenant_id = $1 AND channel_id = $2 AND id = $3")).
		WithArgs(revokeTestTenantID, revokeTestChannelID, revokeTestCredID).
		WillReturnRows(addCredentialRow(sqlmock.NewRows(credentialCols()), now, domain.ChannelCredentialStatusActive, nil))

	mock.ExpectBegin()

	// FOR UPDATE lock
	mock.ExpectQuery(regexp.QuoteMeta("FOR UPDATE")).
		WithArgs(revokeTestTenantID, revokeTestChannelID, revokeTestCredID).
		WillReturnRows(addCredentialRow(sqlmock.NewRows(credentialCols()), now, domain.ChannelCredentialStatusActive, nil))

	// count active
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
		WithArgs(revokeTestTenantID, revokeTestChannelID, "provider_api").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	// UPDATE to REVOKED
	mock.ExpectQuery(regexp.QuoteMeta("UPDATE tenant_channel_credentials")).
		WillReturnRows(addCredentialRow(sqlmock.NewRows(credentialCols()), now, domain.ChannelCredentialStatusRevoked, now))

	// DELETE ciphertext
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM tenant_channel_secrets WHERE id = $1::uuid")).
		WithArgs(revokeTestSecretRowID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// activity log
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO admin_activity_logs")).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectCommit()

	h := newRevokeTestHandler(db)
	var ctx fasthttp.RequestCtx
	ctx.Request.SetRequestURI("/v1/admin/channels/" + revokeTestChannelID + "/credentials/" + revokeTestCredID)
	ctx.SetUserValue(tenantctx.FastHTTPUserValueKey, revokeIdentity())

	h.RevokeChannelCredential(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	var body map[string]any
	if err := json.Unmarshal(ctx.Response.Body(), &body); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	cred, _ := body["credential"].(map[string]any)
	if cred["status"] != "REVOKED" {
		t.Fatalf("expected REVOKED, got %v", cred["status"])
	}
	if body["was_only_active"] != true {
		t.Fatalf("expected was_only_active=true, got %v", body["was_only_active"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRevokeChannelCredentialUnknownCredentialReturns404(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	expectTenantLookupRevoke(mock, now)

	mock.ExpectQuery(regexp.QuoteMeta("FROM tenant_channel_credentials")).
		WithArgs(revokeTestTenantID, revokeTestChannelID, revokeTestCredID).
		WillReturnError(sql.ErrNoRows)

	h := newRevokeTestHandler(db)
	var ctx fasthttp.RequestCtx
	ctx.Request.SetRequestURI("/v1/admin/channels/" + revokeTestChannelID + "/credentials/" + revokeTestCredID)
	ctx.SetUserValue(tenantctx.FastHTTPUserValueKey, revokeIdentity())

	h.RevokeChannelCredential(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusNotFound {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
}

func TestRevokeChannelCredentialWrongChannelOwnershipReturns404(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	expectTenantLookupRevoke(mock, now)

	differentChannel := "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	// DB returns no rows: credential does not belong to differentChannel
	mock.ExpectQuery(regexp.QuoteMeta("FROM tenant_channel_credentials")).
		WithArgs(revokeTestTenantID, differentChannel, revokeTestCredID).
		WillReturnError(sql.ErrNoRows)

	h := newRevokeTestHandler(db)
	var ctx fasthttp.RequestCtx
	ctx.Request.SetRequestURI("/v1/admin/channels/" + differentChannel + "/credentials/" + revokeTestCredID)
	ctx.SetUserValue(tenantctx.FastHTTPUserValueKey, revokeIdentity())

	h.RevokeChannelCredential(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusNotFound {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
}

func TestRevokeChannelCredentialDoubleRevokeReturns409(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	expectTenantLookupRevoke(mock, now)

	// pre-fetch: already REVOKED
	mock.ExpectQuery(regexp.QuoteMeta("FROM tenant_channel_credentials")).
		WithArgs(revokeTestTenantID, revokeTestChannelID, revokeTestCredID).
		WillReturnRows(addCredentialRow(sqlmock.NewRows(credentialCols()), now, domain.ChannelCredentialStatusRevoked, now))

	// tx lock fetch: still REVOKED → idempotent path
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("FOR UPDATE")).
		WithArgs(revokeTestTenantID, revokeTestChannelID, revokeTestCredID).
		WillReturnRows(addCredentialRow(sqlmock.NewRows(credentialCols()), now, domain.ChannelCredentialStatusRevoked, now))
	mock.ExpectCommit()

	h := newRevokeTestHandler(db)
	var ctx fasthttp.RequestCtx
	ctx.Request.SetRequestURI("/v1/admin/channels/" + revokeTestChannelID + "/credentials/" + revokeTestCredID)
	ctx.SetUserValue(tenantctx.FastHTTPUserValueKey, revokeIdentity())

	h.RevokeChannelCredential(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusConflict {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	var body map[string]any
	if err := json.Unmarshal(ctx.Response.Body(), &body); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	cred, _ := body["credential"].(map[string]any)
	if cred["status"] != "REVOKED" {
		t.Fatalf("expected REVOKED in conflict body, got %v", cred["status"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRevokeChannelCredentialNoTenantContextForbidden(t *testing.T) {
	h := &AdminManagementHandler{
		service: service.NewAdminManagementService(nil, zap.NewNop()),
		logger:  zap.NewNop(),
	}
	var ctx fasthttp.RequestCtx
	ctx.Request.SetRequestURI("/v1/admin/channels/" + revokeTestChannelID + "/credentials/" + revokeTestCredID)

	h.RevokeChannelCredential(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusForbidden {
		t.Fatalf("status=%d body=%q", ctx.Response.StatusCode(), ctx.Response.Body())
	}
}
