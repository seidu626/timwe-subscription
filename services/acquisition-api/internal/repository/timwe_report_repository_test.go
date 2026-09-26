package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/zap"
)

func TestGetSubscriptionHealthWithoutProviderImport(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tenantID := "11111111-1111-1111-1111-111111111111"
	mock.ExpectQuery("FROM subscriptions s").WithArgs(tenantID, 32535, "6555").
		WillReturnRows(sqlmock.NewRows([]string{"active", "inactive", "other", "total"}).AddRow(3, 1, 0, 4))
	mock.ExpectQuery("FROM timwe_report_syncs").WithArgs(tenantID, 32535, "6555").
		WillReturnRows(sqlmock.NewRows([]string{"last_success_at", "last_error_code", "window_start", "window_end"}))
	repo := NewReportsRepository(db, zap.NewNop())
	day := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	got, err := repo.GetSubscriptionHealth(context.Background(), tenantID, day, day, 32535, "6555", 30*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if got.Subscriptions.Total != 4 || got.Provider.State != "unavailable" || got.Provider.SuccessBillings != nil || got.Provider.Revenue != nil || got.Provider.Daily == nil {
		t.Fatalf("unexpected health response: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReplaceTIMWEReportRollsBackFailedWindow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	source := TIMWEReportSource{Key: "careerify", TenantID: "11111111-1111-1111-1111-111111111111", Shortcode: "6555", ProductID: 32535, PartnerID: 1919}
	day := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO timwe_report_syncs").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT shortcode, product_id, partner_id FROM timwe_report_syncs").
		WillReturnRows(sqlmock.NewRows([]string{"shortcode", "product_id", "partner_id"}).AddRow("6555", 32535, 1919))
	mock.ExpectExec("DELETE FROM timwe_report_daily").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO timwe_report_daily").WillReturnError(errors.New("insert failed"))
	mock.ExpectRollback()
	repo := NewReportsRepository(db, zap.NewNop())
	rows := []TIMWEReportRow{{Day: day, PartnerID: 1919, Shortcode: "6555", ProductID: 32535, ProductName: "Careerify", PricepointID: 70946, PricepointValue: "0.40", SuccessBillings: 2, Revenue: "0.80"}}
	if err := repo.ReplaceTIMWEReport(context.Background(), source, day, day, rows, day); err == nil {
		t.Fatal("expected transaction failure")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
