package service

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
	"go.uber.org/zap"
)

func newAppSubscriptionTestService(t *testing.T, txService *TransactionService) (*AppSubscriptionService, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	logger := zap.NewNop()
	txRepo := repository.NewTransactionRepository(db, logger)
	campaignRepo := repository.NewCampaignRepository(db, logger)
	svc := NewAppSubscriptionService(txService, txRepo, campaignRepo, nil, logger)
	return svc, mock
}

func TestAppSubscriptionService_Create_RequiresCampaignSlug(t *testing.T) {
	svc, mock := newAppSubscriptionTestService(t, nil)
	_, err := svc.Create("233241234567", "nrg", "")
	requireAppError(t, err, domain.AppErrValidation)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expected no db calls, got: %v", err)
	}
}

func TestMapTransactionServiceError_TranslatesKnownMessages(t *testing.T) {
	cases := []struct {
		in   error
		want domain.AppErrorCode
	}{
		{errors.New("request throttled: try again later"), domain.AppErrRateLimited},
		{errors.New("campaign not found: nrg/daily"), domain.AppErrNotFound},
		{errors.New("transaction not found"), domain.AppErrNotFound},
		{errors.New("transaction is not in confirm_required status"), domain.AppErrConflict},
		{errors.New("consent required but not checked"), domain.AppErrValidation},
		{errors.New("something entirely unexpected"), domain.AppErrValidation},
	}
	for _, tc := range cases {
		got := mapTransactionServiceError(tc.in)
		requireAppError(t, got, tc.want)
	}
}

func TestMapTransactionServiceError_NilIsNil(t *testing.T) {
	if err := mapTransactionServiceError(nil); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// expectAuthorize wires the marketplace ownership lookups: the transaction's
// tenant comes from its own row (GetTenantIDByID + GetByIDForTenant), never
// from the caller's JWT tenant.
func expectAuthorize(mock sqlmock.Sqlmock, txID uuid.UUID, ownerMSISDN string, status domain.TransactionStatus, now time.Time) {
	mock.ExpectQuery(`SELECT tenant_id FROM acquisition_transactions WHERE id = \$1`).
		WithArgs(txID).
		WillReturnRows(sqlmock.NewRows([]string{"tenant_id"}).AddRow(testTenantID))
	mock.ExpectQuery(`FROM acquisition_transactions\s+WHERE id = \$1\s+AND tenant_id = \$2::uuid`).
		WithArgs(txID, testTenantID).
		WillReturnRows(sqlmock.NewRows(acquisitionTransactionColumns()).
			AddRow(txID, uuid.New(), "daily", ownerMSISDN, status, nil,
				nil, nil, nil, nil, nil, nil, false, true, nil, nil, nil, nil, nil, nil,
				nil, nil, nil, nil, nil, nil, nil, nil, false, now, now))
}

func TestAppSubscriptionService_Confirm_RejectsInvalidRef(t *testing.T) {
	svc, mock := newAppSubscriptionTestService(t, nil)
	_, err := svc.Confirm("not-a-uuid", "233241234567", "1234")
	requireAppError(t, err, domain.AppErrNotFound)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expected no db calls, got: %v", err)
	}
}

func TestAppSubscriptionService_Confirm_RejectsMismatchedMSISDN(t *testing.T) {
	svc, mock := newAppSubscriptionTestService(t, nil)
	txID := uuid.New()

	expectAuthorize(mock, txID, "233299999999", domain.StatusConfirmRequired, time.Now())

	// A different caller (msisdn 233241234567) tries to confirm a
	// transaction that belongs to 233299999999: must not reach
	// TransactionService.ConfirmTransaction (txService is nil here).
	_, err := svc.Confirm(txID.String(), "233241234567", "1234")
	requireAppError(t, err, domain.AppErrNotFound)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAppSubscriptionService_Confirm_RejectsUnownedTransaction(t *testing.T) {
	svc, mock := newAppSubscriptionTestService(t, nil)
	txID := uuid.New()

	mock.ExpectQuery(`SELECT tenant_id FROM acquisition_transactions WHERE id = \$1`).
		WithArgs(txID).
		WillReturnError(sql.ErrNoRows)

	_, err := svc.Confirm(txID.String(), "233241234567", "1234")
	requireAppError(t, err, domain.AppErrNotFound)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAppSubscriptionService_List_SpansTenants(t *testing.T) {
	svc, mock := newAppSubscriptionTestService(t, nil)
	now := time.Now()

	mock.ExpectQuery(`FROM acquisition_transactions t\s+JOIN campaigns c ON c.slug = t.campaign_slug\s+JOIN tenants tn`).
		WithArgs("233241234567").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "campaign_slug", "status", "created_at", "charged_at",
			"price", "billing_cycle", "app_name", "lp_copy", "country",
			"tenant_key", "name",
		}).AddRow(
			uuid.New().String(), "daily", "SUBSCRIBED", now, nil,
			2.0, "DAILY", "Daily Tips", nil, "GH",
			"nrg", "NRG",
		).AddRow(
			uuid.New().String(), "careers", "SUBSCRIBED", now, nil,
			1.0, "DAILY", "Career Boost", nil, "GH",
			"careerify", "Careerify",
		))

	subs, err := svc.List("233241234567")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("expected 2 subscriptions, got %+v", subs)
	}
	if subs[0].Tenant != "nrg" || subs[0].TenantName != "NRG" || subs[0].Status != "ACTIVE" {
		t.Fatalf("unexpected first subscription: %+v", subs[0])
	}
	if !strings.HasPrefix(subs[0].NextChargeHint, "Renews ") {
		t.Fatalf("expected ACTIVE daily subscription to carry a next-charge hint, got %+v", subs[0])
	}
	if subs[1].Tenant != "careerify" || subs[1].ProductName != "Career Boost" {
		t.Fatalf("unexpected second subscription: %+v", subs[1])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAppSubscriptionService_Cancel_RejectsInvalidRef(t *testing.T) {
	svc, mock := newAppSubscriptionTestService(t, nil)
	err := svc.Cancel("not-a-uuid", "233241234567")
	requireAppError(t, err, domain.AppErrNotFound)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expected no db calls, got: %v", err)
	}
}

func TestAppSubscriptionService_Cancel_RejectsMismatchedMSISDN(t *testing.T) {
	svc, mock := newAppSubscriptionTestService(t, nil)
	txID := uuid.New()

	expectAuthorize(mock, txID, "233299999999", domain.StatusSubscribed, time.Now())

	err := svc.Cancel(txID.String(), "233241234567")
	requireAppError(t, err, domain.AppErrNotFound)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAppSubscriptionService_Cancel_PendingCancelsLocallyWithoutProvider(t *testing.T) {
	svc, mock := newAppSubscriptionTestService(t, nil)
	txID := uuid.New()

	// A PENDING opt-in never activated at the provider: cancel must mark
	// the row CANCELLED locally and never reach the tenant/campaign/optout
	// path (optoutClient is nil here, so any provider attempt would error).
	expectAuthorize(mock, txID, "233241234567", domain.StatusPending, time.Now())
	mock.ExpectExec(`UPDATE acquisition_transactions\s+SET status = \$1`).
		WithArgs(string(domain.StatusCancelled), nil, nil, txID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := svc.Cancel(txID.String(), "233241234567"); err != nil {
		t.Fatalf("expected pending cancel to succeed locally, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAppSubscriptionService_Cancel_AlreadyCancelledIsIdempotent(t *testing.T) {
	svc, mock := newAppSubscriptionTestService(t, nil)
	txID := uuid.New()

	expectAuthorize(mock, txID, "233241234567", domain.StatusCancelled, time.Now())

	if err := svc.Cancel(txID.String(), "233241234567"); err != nil {
		t.Fatalf("expected repeat cancel to be a no-op, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expected no writes for an already-cancelled subscription: %v", err)
	}
}

func TestAppSubscriptionService_Cancel_FailsWhenCampaignHasNoChannel(t *testing.T) {
	svc, mock := newAppSubscriptionTestService(t, nil)
	txID := uuid.New()
	now := time.Now()

	expectAuthorize(mock, txID, "233241234567", domain.StatusSubscribed, now)
	mock.ExpectQuery(`SELECT tenant_key\s+FROM tenants\s+WHERE id = \$1::uuid`).
		WithArgs(testTenantID).
		WillReturnRows(sqlmock.NewRows([]string{"tenant_key"}).AddRow(testTenantKey))

	campaignID := 1
	expectTenantCampaign(mock, testTenantKey, "daily",
		campaignID, "daily", "en", "GH", nil, 101, nil, nil,
		"OTP", nil, nil, nil, nil, nil, nil,
		nil, false, nil, nil, nil,
		`{}`, pq.StringArray{}, pq.StringArray{}, pq.StringArray{}, nil, nil,
		true, now, now, nil, nil,
	)

	err := svc.Cancel(txID.String(), "233241234567")
	requireAppError(t, err, domain.AppErrProviderError)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
