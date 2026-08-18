package adminhttp

import (
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

func newContentTestServer(t *testing.T) (*Server, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	s := &Server{
		logger: zap.NewNop(),
		repo:   repository.NewCadenceRepository(db, zap.NewNop()),
	}
	return s, mock, func() { db.Close() }
}

func contentItemRows(text string, version int, active bool) *sqlmock.Rows {
	created := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	return sqlmock.NewRows([]string{
		"id", "tenant_id", "channel_id", "series_id", "content_version", "seq_no", "message_text",
		"content_kind", "link_url", "cta_label", "is_active", "created_at", "updated_at",
	}).AddRow(int64(12), nil, nil, int64(7), version, 3, text, "TEXT", nil, nil, active, created, created)
}

func TestHandleContentItemPatch_EditsTextAndRecordsActor(t *testing.T) {
	s, mock, closeDB := newContentTestServer(t)
	defer closeDB()

	mock.ExpectQuery("FROM message_content_items").
		WithArgs(int64(12), int64(7)).
		WillReturnRows(contentItemRows("Old text", 2, true))
	mock.ExpectBegin()
	mock.ExpectExec("set_config").
		WithArgs("admin@example.com").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE message_content_items").
		WithArgs(int64(12), "New text", "TEXT", nil, nil, true).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("FROM message_content_items").
		WithArgs(int64(12), int64(7)).
		WillReturnRows(contentItemRows("New text", 2, true))

	series := &domain.MessageSeries{ID: 7, ContentVersion: 1}
	req := httptest.NewRequest(http.MethodPatch, "/v1/admin/cadence/series/7/content/12",
		strings.NewReader(`{"message_text":"New text"}`))
	req = req.WithContext(tenantctx.WithIdentity(req.Context(), tenantctx.Identity{Email: "admin@example.com"}))
	rr := httptest.NewRecorder()

	s.handleContentSubroute(rr, req, series, "12")

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "New text") {
		t.Fatalf("expected updated item in response, got %s", rr.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestHandleContentItemPatch_RefusesLastActiveDeactivationOnLiveVersion(t *testing.T) {
	s, mock, closeDB := newContentTestServer(t)
	defer closeDB()

	mock.ExpectQuery("FROM message_content_items").
		WithArgs(int64(12), int64(7)).
		WillReturnRows(contentItemRows("Only item", 2, true))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT COUNT").
		WithArgs(int64(7), 2, int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))
	mock.ExpectRollback()

	series := &domain.MessageSeries{ID: 7, ContentVersion: 2} // live version
	req := httptest.NewRequest(http.MethodPatch, "/v1/admin/cadence/series/7/content/12",
		strings.NewReader(`{"is_active":false}`))
	rr := httptest.NewRecorder()

	s.handleContentSubroute(rr, req, series, "12")

	if rr.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestHandleContentItemPatch_RejectsLinkKindWithoutURL(t *testing.T) {
	s, mock, closeDB := newContentTestServer(t)
	defer closeDB()

	mock.ExpectQuery("FROM message_content_items").
		WithArgs(int64(12), int64(7)).
		WillReturnRows(contentItemRows("Plain tip", 2, true))

	series := &domain.MessageSeries{ID: 7, ContentVersion: 1}
	req := httptest.NewRequest(http.MethodPatch, "/v1/admin/cadence/series/7/content/12",
		strings.NewReader(`{"content_kind":"LINK"}`))
	rr := httptest.NewRecorder()

	s.handleContentSubroute(rr, req, series, "12")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestHandleContentImpact_ReportsLiveBlastRadius(t *testing.T) {
	s, mock, closeDB := newContentTestServer(t)
	defer closeDB()

	mock.ExpectQuery("FROM subscription_message_state").
		WithArgs(int64(7), 2).
		WillReturnRows(sqlmock.NewRows([]string{"active_states", "pending_jobs"}).AddRow(int64(120), int64(4)))

	series := &domain.MessageSeries{ID: 7, ContentVersion: 2}
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/cadence/series/7/content/impact?contentVersion=2", nil)
	rr := httptest.NewRecorder()

	s.handleContentSubroute(rr, req, series, "impact")

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, want := range []string{`"is_live":true`, `"active_states":120`, `"pending_jobs":4`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s in %s", want, body)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestHandleContentClone_DefaultsToNextVersion(t *testing.T) {
	s, mock, closeDB := newContentTestServer(t)
	defer closeDB()

	mock.ExpectQuery("SELECT COUNT").
		WithArgs(int64(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(30)))
	mock.ExpectQuery("MAX\\(content_version\\)").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(2))
	mock.ExpectQuery("SELECT COUNT").
		WithArgs(int64(7), 3).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO message_content_items").
		WithArgs(int64(7), 1, 3).
		WillReturnResult(sqlmock.NewResult(0, 30))
	mock.ExpectCommit()

	series := &domain.MessageSeries{ID: 7, ContentVersion: 1}
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/cadence/series/7/content/clone",
		strings.NewReader(`{"from_version":1}`))
	rr := httptest.NewRecorder()

	s.handleContentSubroute(rr, req, series, "clone")

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, want := range []string{`"to_version":3`, `"items_copied":30`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s in %s", want, body)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}

func TestHandleContentClone_RefusesNonEmptyTarget(t *testing.T) {
	s, mock, closeDB := newContentTestServer(t)
	defer closeDB()

	mock.ExpectQuery("SELECT COUNT").
		WithArgs(int64(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(5)))
	mock.ExpectQuery("SELECT COUNT").
		WithArgs(int64(7), 2).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(3)))

	series := &domain.MessageSeries{ID: 7, ContentVersion: 1}
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/cadence/series/7/content/clone",
		strings.NewReader(`{"from_version":1,"to_version":2}`))
	rr := httptest.NewRecorder()

	s.handleContentSubroute(rr, req, series, "clone")

	if rr.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet: %v", err)
	}
}
