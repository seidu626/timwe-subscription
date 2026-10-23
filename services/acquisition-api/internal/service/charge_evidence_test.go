package service

import (
	"database/sql"
	"database/sql/driver"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"go.uber.org/zap"
)

func newChargeEvidenceTestTx(status domain.TransactionStatus, postbackSent bool) *domain.AcquisitionTransaction {
	tenantID := testTenantID
	timweTxID := "timwe-tx-1"
	provider := "mobplus"
	clickID := "click-1"
	productID := 33261
	return &domain.AcquisitionTransaction{
		ID:                     uuid.New(),
		TenantID:               &tenantID,
		CampaignSlug:           "daily",
		MSISDN:                 "233241234567",
		Status:                 status,
		OfferProductID:         &productID,
		TimweTransactionID:     &timweTxID,
		AdProvider:             &provider,
		ClickID:                &clickID,
		ConversionPostbackSent: postbackSent,
	}
}

func TestPromoteOnChargeEvidence_MarksChargedAndEnqueuesConversion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	svc := newPartnerTestService(db)
	svc.providerReg.Register(NewMobplusProvider(zap.NewNop()))
	tx := newChargeEvidenceTestTx(domain.StatusSubscribed, false)

	mock.ExpectExec(`UPDATE acquisition_transactions\s+SET status = \$1, charged_at = \$2`).
		WithArgs(domain.StatusCharged, sqlmock.AnyArg(), sql.NullString{}, tx.ID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// No campaign postback_rules: fall back to the registered mobplus provider.
	expectAdminCampaignLookup(mock, []driver.Value{testTenantID, "daily"}, nil, sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO postback_outbox").
		WithArgs(
			sqlmock.AnyArg(),
			sql.NullString{String: testTenantID, Valid: true},
			sql.NullString{},
			tx.ID,
			domain.PostbackEventConversion,
			"mobplus",
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sql.NullString{},
			sql.NullString{},
			0,
			5,
			sql.NullTime{},
			domain.PostbackStatusPending,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`SET conversion_postback_sent = true`).
		WithArgs(tx.ID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := svc.PromoteOnChargeEvidence(tx, false); err != nil {
		t.Fatalf("promote: %v", err)
	}
	if tx.Status != domain.StatusCharged || tx.ChargedAt == nil {
		t.Fatalf("expected CHARGED with charged_at, got status=%s charged_at=%v", tx.Status, tx.ChargedAt)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestPromoteOnChargeEvidence_PostbackAlreadySentMarksChargedWithoutSecondPostback(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	svc := newPartnerTestService(db)
	tx := newChargeEvidenceTestTx(domain.StatusSubscribed, true)

	// Only the CHARGED mark: no campaign lookup, no outbox insert, no
	// conversion_postback_sent write (sqlmock fails on any unexpected call).
	mock.ExpectExec(`UPDATE acquisition_transactions\s+SET status = \$1, charged_at = \$2`).
		WithArgs(domain.StatusCharged, sqlmock.AnyArg(), sql.NullString{}, tx.ID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := svc.PromoteOnChargeEvidence(tx, false); err != nil {
		t.Fatalf("promote: %v", err)
	}
	if tx.Status != domain.StatusCharged || tx.ChargedAt == nil {
		t.Fatalf("expected CHARGED with charged_at, got status=%s charged_at=%v", tx.Status, tx.ChargedAt)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestPromoteOnChargeEvidence_AdvancesConfirmRequiredBeforeCharging(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	svc := newPartnerTestService(db)
	tx := newChargeEvidenceTestTx(domain.StatusConfirmRequired, true)

	mock.ExpectExec(`UPDATE acquisition_transactions\s+SET status = \$1, next_action = \$2`).
		WithArgs(string(domain.StatusSubscribed), nil, nil, tx.ID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE acquisition_transactions\s+SET status = \$1, charged_at = \$2`).
		WithArgs(domain.StatusCharged, sqlmock.AnyArg(), sql.NullString{}, tx.ID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := svc.PromoteOnChargeEvidence(tx, false); err != nil {
		t.Fatalf("promote: %v", err)
	}
	if tx.Status != domain.StatusCharged {
		t.Fatalf("expected CHARGED, got %s", tx.Status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestPromoteOnChargeEvidence_RequiresTransaction(t *testing.T) {
	svc := &TransactionService{logger: zap.NewNop()}
	if err := svc.PromoteOnChargeEvidence(nil, false); err == nil {
		t.Fatal("expected transaction required error")
	}
}

func TestPromoteOnChargeEvidence_AlreadyPayingMarksChargedWithoutPostback(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	svc := newPartnerTestService(db)
	svc.providerReg.Register(NewMobplusProvider(zap.NewNop()))
	// The MSISDN paid before this transaction existed: a re-optin by an
	// existing subscriber must not fire a conversion postback, even with
	// a registered provider and an unsent postback flag.
	tx := newChargeEvidenceTestTx(domain.StatusConfirmRequired, false)

	mock.ExpectExec(`UPDATE acquisition_transactions\s+SET status = \$1, charged_at = \$2`).
		WithArgs(domain.StatusCharged, sqlmock.AnyArg(), sql.NullString{}, tx.ID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := svc.PromoteOnChargeEvidence(tx, true); err != nil {
		t.Fatalf("promote: %v", err)
	}
	if tx.Status != domain.StatusCharged || tx.ChargedAt == nil {
		t.Fatalf("expected CHARGED with charged_at, got status=%s charged_at=%v", tx.Status, tx.ChargedAt)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
