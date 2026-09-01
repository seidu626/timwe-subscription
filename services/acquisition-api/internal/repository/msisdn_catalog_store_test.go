package repository

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	msisdncatalog "github.com/seidu626/subscription-manager/common/msisdn/catalog"
)

func TestSaveVerificationFailsWhenCatalogRecordWasNotUpdated(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE msisdn_catalog")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	store := msisdncatalog.NewStore(db)
	err = store.SaveVerification(context.Background(), "tenant-id", 42, msisdncatalog.VerificationResult{Status: msisdncatalog.StatusVerified, Provider: "MADAPI"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}
