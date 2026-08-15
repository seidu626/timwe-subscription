package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/zap"
)

func newAppFeedRepoForTest(t *testing.T) (*AppFeedRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewAppFeedRepository(db, zap.NewNop()), mock
}

func TestDeriveTitle_UsesStoredTitleWhenPresent(t *testing.T) {
	got := deriveTitle(sql.NullString{String: "Weekly Tip", Valid: true}, "irrelevant body text")
	if got != "Weekly Tip" {
		t.Errorf("got %q, want %q", got, "Weekly Tip")
	}
}

func TestDeriveTitle_FallsBackToFirst60CharsWhenTitleMissing(t *testing.T) {
	body := "This is a very long message body that certainly exceeds sixty characters in total length, definitely."
	got := deriveTitle(sql.NullString{}, body)
	want := string([]rune(body)[:60])
	if got != want {
		t.Errorf("got %q (len %d), want %q (len %d)", got, len([]rune(got)), want, len([]rune(want)))
	}
	if len([]rune(got)) != 60 {
		t.Errorf("expected fallback title to be exactly 60 runes, got %d", len([]rune(got)))
	}
}

func TestDeriveTitle_FallsBackToWholeBodyWhenShort(t *testing.T) {
	got := deriveTitle(sql.NullString{}, "short body")
	if got != "short body" {
		t.Errorf("got %q, want %q", got, "short body")
	}
}

func TestDeriveTitle_BlankStoredTitleFallsBackToBody(t *testing.T) {
	got := deriveTitle(sql.NullString{String: "   ", Valid: true}, "fallback text")
	if got != "fallback text" {
		t.Errorf("got %q, want %q", got, "fallback text")
	}
}

func TestListFeed_OrdersByPublishedAtDescAndAppliesReadState(t *testing.T) {
	repo, mock := newAppFeedRepoForTest(t)

	now := time.Now()
	older := now.Add(-time.Hour)
	rows := sqlmock.NewRows([]string{
		"content_item_id", "product_slug", "product_name", "title", "message_text", "content_kind", "link_url", "cta_label", "published_at", "is_read",
	}).
		AddRow(int64(2), "careerify-tips", "Careerify Tips", nil, "Newer item body", "LINK", "https://careerify.example/app", "Open", now, false).
		AddRow(int64(1), "careerify-tips", "Careerify Tips", "Custom Title", "Older item body", "TEXT", nil, nil, older, true)

	mock.ExpectQuery(`SELECT \* FROM \(`).
		WithArgs("233241234567", 50).
		WillReturnRows(rows)

	items, err := repo.ListFeed(context.Background(), "233241234567", 50)
	if err != nil {
		t.Fatalf("ListFeed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].ID != 2 || items[0].Read {
		t.Errorf("expected first item id=2 unread, got id=%d read=%v", items[0].ID, items[0].Read)
	}
	if items[1].ID != 1 || !items[1].Read || items[1].Title != "Custom Title" {
		t.Errorf("unexpected second item: %+v", items[1])
	}
	if items[0].Title != "Newer item body" {
		t.Errorf("expected fallback title from body, got %q", items[0].Title)
	}
	if items[0].ContentKind != "LINK" || items[0].LinkURL == nil || *items[0].LinkURL != "https://careerify.example/app" || items[0].CTALabel == nil || *items[0].CTALabel != "Open" {
		t.Errorf("expected link fields on first item, got %+v", items[0])
	}
	if items[1].ContentKind != "TEXT" || items[1].LinkURL != nil || items[1].CTALabel != nil {
		t.Errorf("expected null link fields on text item, got %+v", items[1])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestListFeed_EmptyWhenNoDeliveredContent(t *testing.T) {
	repo, mock := newAppFeedRepoForTest(t)

	rows := sqlmock.NewRows([]string{
		"content_item_id", "product_slug", "product_name", "title", "message_text", "content_kind", "link_url", "cta_label", "published_at", "is_read",
	})
	mock.ExpectQuery(`SELECT \* FROM \(`).
		WithArgs("233241234567", 50).
		WillReturnRows(rows)

	items, err := repo.ListFeed(context.Background(), "233241234567", 50)
	if err != nil {
		t.Fatalf("ListFeed: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestGetFeedItem_NotFoundReturnsErrNoRows(t *testing.T) {
	repo, mock := newAppFeedRepoForTest(t)

	mock.ExpectQuery(`SELECT`).
		WithArgs("233241234567", int64(99)).
		WillReturnError(sql.ErrNoRows)

	_, err := repo.GetFeedItem(context.Background(), "233241234567", 99)
	if err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestMarkRead_NotDeliveredReturnsErrNoRows(t *testing.T) {
	repo, mock := newAppFeedRepoForTest(t)

	mock.ExpectQuery(`SELECT 1 FROM message_outbox`).
		WithArgs("233241234567", int64(5)).
		WillReturnError(sql.ErrNoRows)

	err := repo.MarkRead(context.Background(), "233241234567", 5)
	if err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestMarkRead_UpsertsWhenDelivered(t *testing.T) {
	repo, mock := newAppFeedRepoForTest(t)

	mock.ExpectQuery(`SELECT 1 FROM message_outbox`).
		WithArgs("233241234567", int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(1))
	mock.ExpectExec(`INSERT INTO app_feed_read_state`).
		WithArgs("233241234567", int64(5)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.MarkRead(context.Background(), "233241234567", 5); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestUpsertDevice_IdempotentOnConflictToken(t *testing.T) {
	repo, mock := newAppFeedRepoForTest(t)

	mock.ExpectExec(`INSERT INTO app_devices`).
		WithArgs("233241234567", "careerify", "fcm-token-abc", "android").
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := repo.UpsertDevice(context.Background(), "233241234567", "careerify", "fcm-token-abc", "android"); err != nil {
		t.Fatalf("UpsertDevice: %v", err)
	}

	// Re-registering the same token (e.g. app reinstall) must upsert, not
	// fail on the UNIQUE constraint.
	mock.ExpectExec(`INSERT INTO app_devices`).
		WithArgs("233241234567", "careerify", "fcm-token-abc", "ios").
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := repo.UpsertDevice(context.Background(), "233241234567", "careerify", "fcm-token-abc", "ios"); err != nil {
		t.Fatalf("UpsertDevice (re-register): %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestUpsertNotificationPref_Upserts(t *testing.T) {
	repo, mock := newAppFeedRepoForTest(t)

	mock.ExpectExec(`INSERT INTO app_notification_prefs`).
		WithArgs("233241234567", "careerify-tips", "PUSH").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.UpsertNotificationPref(context.Background(), "233241234567", "careerify-tips", "PUSH"); err != nil {
		t.Fatalf("UpsertNotificationPref: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
