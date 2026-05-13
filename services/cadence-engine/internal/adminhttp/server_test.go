package adminhttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/seidu626/subscription-manager/cadence-engine/internal/repository"
	"github.com/seidu626/subscription-manager/common/auth/tenantctx"
	"go.uber.org/zap"
)

func TestParseCSVImport_MissingHeader(t *testing.T) {
	_, errs := parseCSVImport(strings.NewReader(""))
	if len(errs) == 0 {
		t.Fatalf("expected errors")
	}
}

func TestParseCSVImport_ValidSequential(t *testing.T) {
	csv := strings.TrimSpace(`
partner_role_id,product_id,series_name,mode,content_version,seq_no,message_text,is_active
1,10,News,SEQUENTIAL,1,1,Hello,true
1,10,News,SEQUENTIAL,1,2,World,true
`)
	req, errs := parseCSVImport(strings.NewReader(csv))
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %#v", errs)
	}
	if req.RowCount != 2 {
		t.Fatalf("expected row_count 2, got %d", req.RowCount)
	}
	if len(req.Series) != 1 {
		t.Fatalf("expected 1 series group, got %d", len(req.Series))
	}
	g := req.Series[0]
	if g.PartnerRoleID != 1 || g.ProductID != 10 || g.SeriesName != "News" || g.Mode != "SEQUENTIAL" {
		t.Fatalf("unexpected group: %#v", g)
	}
	items := g.ItemsByVersion[1]
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].SeqNo != 1 || items[1].SeqNo != 2 {
		t.Fatalf("unexpected seq numbers: %#v", items)
	}
}

func TestParseCSVImport_PoolAllowsBlankSeqNo(t *testing.T) {
	csv := strings.TrimSpace(`
partner_role_id,product_id,series_name,mode,content_version,seq_no,message_text,is_active
1,10,Pool,POOL,2,,A,true
1,10,Pool,POOL,2,,B,true
`)
	req, errs := parseCSVImport(strings.NewReader(csv))
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %#v", errs)
	}
	g := req.Series[0]
	items := g.ItemsByVersion[2]
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].SeqNo != 1 || items[1].SeqNo != 2 {
		t.Fatalf("expected generated seq_no 1,2 got %#v", items)
	}
}

func TestParseCSVImport_ConflictingModePerSeries(t *testing.T) {
	csv := strings.TrimSpace(`
partner_role_id,product_id,series_name,mode,content_version,seq_no,message_text,is_active
1,10,Mixed,SEQUENTIAL,1,1,A,true
1,10,Mixed,POOL,1,2,B,true
`)
	_, errs := parseCSVImport(strings.NewReader(csv))
	if len(errs) == 0 {
		t.Fatalf("expected errors")
	}
}

func TestHandleSeriesReturnsErrWhenTenantMissing(t *testing.T) {
	const ErrTenantScope = "tenant context required"

	s := &Server{
		logger: zap.NewNop(),
		access: &access{staticToken: "secret-token"},
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/cadence/series", nil)
	req.Header.Set("X-Admin-Token", "secret-token")
	rr := httptest.NewRecorder()

	s.handleSeries(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected error 403 without tenant scope, got %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), ErrTenantScope) {
		t.Fatalf("unexpected body: %s", rr.Body.String())
	}
}

func TestTenantScopeResolvesPlatformTenantKey(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	s := &Server{
		logger: zap.NewNop(),
		repo:   repository.NewCadenceRepository(db, zap.NewNop()),
	}
	mock.ExpectQuery("FROM tenants").
		WithArgs("nrg").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("66d39a9a-f1ef-4721-a31c-5bb966d25c3d"))

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/cadence/series", nil)
	req = req.WithContext(tenantctx.WithIdentity(req.Context(), tenantctx.Identity{
		PlatformScoped: true,
		TenantKey:      "nrg",
	}))
	rr := httptest.NewRecorder()

	tenantID, _, ok := s.tenantScope(rr, req)
	if !ok {
		t.Fatalf("expected tenant scope, status=%d body=%s", rr.Code, rr.Body.String())
	}
	if tenantID != "66d39a9a-f1ef-4721-a31c-5bb966d25c3d" {
		t.Fatalf("tenantID = %q", tenantID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestHealthReportsObservabilityStatus(t *testing.T) {
	s := &Server{logger: zap.NewNop()}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	s.handleHealth(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	observability, ok := body["observability"].(map[string]any)
	if !ok {
		t.Fatalf("expected observability status, got %#v", body)
	}
	if observability["tenant_labels"] != "enabled" || observability["pii_labels"] != "rejected" {
		t.Fatalf("unexpected observability status: %#v", observability)
	}
}
