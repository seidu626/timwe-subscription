package adminhttp

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/seidu626/subscription-manager/cadence-engine/internal/domain"
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

func TestValidateContentFields_ContentKindContract(t *testing.T) {
	link := "https://careerify.example/app"
	cta := "Open app"
	longCTA := strings.Repeat("x", 41)
	cases := []struct {
		name      string
		kind      string
		linkURL   *string
		ctaLabel  *string
		wantKind  string
		wantError bool
	}{
		{name: "default text", wantKind: "TEXT"},
		{name: "text rejects link", kind: "TEXT", linkURL: &link, wantError: true},
		{name: "text rejects cta", kind: "TEXT", ctaLabel: &cta, wantError: true},
		{name: "link accepts http URL", kind: "LINK", linkURL: &link, ctaLabel: &cta, wantKind: "LINK"},
		{name: "link requires URL", kind: "LINK", wantError: true},
		{name: "link rejects non-http URL", kind: "LINK", linkURL: ptrString("ftp://example.com"), wantError: true},
		{name: "link rejects long cta", kind: "LINK", linkURL: &link, ctaLabel: &longCTA, wantError: true},
		{name: "rejects unknown kind", kind: "VIDEO", wantError: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotKind, _, _, err := validateContentFields(tc.kind, tc.linkURL, tc.ctaLabel)
			if tc.wantError {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotKind != tc.wantKind {
				t.Fatalf("kind = %q, want %q", gotKind, tc.wantKind)
			}
		})
	}
}

func TestNormalizeDeliveryChannel(t *testing.T) {
	for _, value := range []string{"", "USER_PREF", "sms", " push "} {
		if _, err := normalizeDeliveryChannel(value); err != nil {
			t.Fatalf("normalizeDeliveryChannel(%q): %v", value, err)
		}
	}
	if _, err := normalizeDeliveryChannel("EMAIL"); err == nil {
		t.Fatal("expected invalid delivery_channel error")
	}
}

func ptrString(value string) *string {
	return &value
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

func TestHandleSeriesPreviewSequential(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	s := &Server{
		logger: zap.NewNop(),
		repo:   repository.NewCadenceRepository(db, zap.NewNop()),
	}

	prefTime := time.Date(2000, 1, 1, 8, 0, 0, 0, time.UTC)
	mock.ExpectQuery("FROM message_schedule_rules").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"series_id", "rule_kind", "preferred_time", "days_of_week", "n_days",
			"send_start_time", "send_end_time", "timezone", "max_per_day", "catchup_mode",
		}).AddRow(int64(7), "DAILY", prefTime, 0, 0, prefTime, prefTime.Add(14*time.Hour), "Africa/Accra", 1, "SKIP"))

	created := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery("FROM message_content_items").
		WithArgs(int64(7), 2, true).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "tenant_id", "channel_id", "series_id", "content_version", "seq_no", "message_text",
			"content_kind", "link_url", "cta_label", "is_active", "created_at",
		}).
			AddRow(int64(1), nil, nil, int64(7), 2, 1, "Day one tip", "TEXT", nil, nil, true, created).
			AddRow(int64(2), nil, nil, int64(7), 2, 2, "Day two tip", "TEXT", nil, nil, true, created))

	series := &domain.MessageSeries{ID: 7, Mode: "SEQUENTIAL", ContentVersion: 2}
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/cadence/series/7/preview?count=3", nil)
	rr := httptest.NewRecorder()

	s.handleSeriesPreview(rr, req, series)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Mode        string `json:"mode"`
		PoolSize    int    `json:"pool_size"`
		Occurrences []struct {
			N           int    `json:"n"`
			SendAt      string `json:"send_at"`
			SeqNo       int    `json:"seq_no"`
			MessageText string `json:"message_text"`
			EndsSeries  bool   `json:"ends_series"`
		} `json:"occurrences"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v body=%s", err, rr.Body.String())
	}
	if len(body.Occurrences) != 3 {
		t.Fatalf("occurrences = %d, want 3 (two content + end marker)", len(body.Occurrences))
	}
	if body.Occurrences[0].MessageText != "Day one tip" || body.Occurrences[1].MessageText != "Day two tip" {
		t.Errorf("content pairing wrong: %+v", body.Occurrences)
	}
	if !body.Occurrences[2].EndsSeries {
		t.Errorf("third occurrence should mark series end: %+v", body.Occurrences[2])
	}
	if body.Occurrences[0].SendAt == "" || body.Occurrences[0].SendAt == body.Occurrences[1].SendAt {
		t.Errorf("send times must be distinct ascending: %+v", body.Occurrences)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestHandleSeriesPreviewRequiresRule(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	s := &Server{
		logger: zap.NewNop(),
		repo:   repository.NewCadenceRepository(db, zap.NewNop()),
	}
	mock.ExpectQuery("FROM message_schedule_rules").
		WithArgs(int64(7)).
		WillReturnError(sql.ErrNoRows)

	series := &domain.MessageSeries{ID: 7, Mode: "SEQUENTIAL", ContentVersion: 1}
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/cadence/series/7/preview", nil)
	rr := httptest.NewRecorder()

	s.handleSeriesPreview(rr, req, series)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandleSeriesReactivateDryRun(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	s := &Server{logger: zap.NewNop(), repo: repository.NewCadenceRepository(db, zap.NewNop())}

	mock.ExpectQuery("FROM subscription_message_state sms").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(42)))

	series := &domain.MessageSeries{ID: 7, IsActive: false}
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/cadence/series/7/reactivate", strings.NewReader(`{"dry_run":true}`))
	rr := httptest.NewRecorder()

	s.handleSeriesReactivate(rr, req, series)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		DryRun          bool  `json:"dry_run"`
		ResumableStates int64 `json:"resumable_states"`
		IsActive        bool  `json:"is_active"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v body=%s", err, rr.Body.String())
	}
	if !body.DryRun || body.ResumableStates != 42 || body.IsActive {
		t.Errorf("unexpected body: %s", rr.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestHandleSeriesReactivateResumesStoppedStates(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	s := &Server{logger: zap.NewNop(), repo: repository.NewCadenceRepository(db, zap.NewNop())}

	mock.ExpectQuery("FROM subscription_message_state sms").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(2)))

	prefTime := time.Date(2000, 1, 1, 8, 0, 0, 0, time.UTC)
	mock.ExpectQuery("FROM message_schedule_rules").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"series_id", "rule_kind", "preferred_time", "days_of_week", "n_days",
			"send_start_time", "send_end_time", "timezone", "max_per_day", "catchup_mode",
		}).AddRow(int64(7), "DAILY", prefTime, 0, 0, prefTime, prefTime.Add(14*time.Hour), "Africa/Accra", 1, "SKIP"))

	mock.ExpectExec("UPDATE product_message_series").
		WithArgs(true, int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery("FROM subscription_message_state sms").
		WithArgs(int64(7), int64(0), 500).
		WillReturnRows(sqlmock.NewRows([]string{"subscription_id", "start_date"}).
			AddRow(int64(11), start).
			AddRow(int64(12), start))

	mock.ExpectExec("UPDATE subscription_message_state sms").
		WillReturnResult(sqlmock.NewResult(0, 2))

	series := &domain.MessageSeries{ID: 7, IsActive: false}
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/cadence/series/7/reactivate", nil)
	rr := httptest.NewRecorder()

	s.handleSeriesReactivate(rr, req, series)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Status        string `json:"status"`
		ResumedStates int64  `json:"resumed_states"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v body=%s", err, rr.Body.String())
	}
	if body.Status != "reactivated" || body.ResumedStates != 2 {
		t.Errorf("unexpected body: %s", rr.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestHandleSeriesReactivateRequiresRuleWhenResuming(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	s := &Server{logger: zap.NewNop(), repo: repository.NewCadenceRepository(db, zap.NewNop())}

	mock.ExpectQuery("FROM subscription_message_state sms").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(5)))
	mock.ExpectQuery("FROM message_schedule_rules").
		WithArgs(int64(7)).
		WillReturnError(sql.ErrNoRows)

	series := &domain.MessageSeries{ID: 7, IsActive: false}
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/cadence/series/7/reactivate", nil)
	rr := httptest.NewRecorder()

	s.handleSeriesReactivate(rr, req, series)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status=%d, want 409; body=%s", rr.Code, rr.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
