package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
	"go.uber.org/zap"
)

const reconcilerTenantID = "11111111-1111-1111-1111-111111111111"

type fakeChargePromoter struct {
	calls []uuid.UUID
	fail  map[uuid.UUID]error
}

func (f *fakeChargePromoter) PromoteOnChargeEvidence(tx *domain.AcquisitionTransaction) error {
	f.calls = append(f.calls, tx.ID)
	if tx.TenantID == nil || *tx.TenantID != reconcilerTenantID {
		return errors.New("tenant not loaded")
	}
	return f.fail[tx.ID]
}

func newTestChargeReconciler(t *testing.T, promoter ChargePromoter, cfg ChargeReconcilerConfig) (*ChargeReconciler, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	repo := repository.NewTransactionRepository(db, zap.NewNop())
	return NewChargeReconciler(repo, promoter, zap.NewNop(), cfg), mock
}

func enabledReconcilerConfig() ChargeReconcilerConfig {
	return ChargeReconcilerConfig{Enabled: true, Interval: time.Minute, ExpiryDays: 7, PromoteLookbackDays: 30}
}

func expectCandidateLoad(mock sqlmock.Sqlmock, id uuid.UUID, status domain.TransactionStatus) {
	now := time.Now()
	cols := []string{
		"id", "correlation_id", "campaign_slug", "msisdn", "status", "next_action",
		"next_action_payload", "ad_provider", "click_id", "attribution_data",
		"ip_address", "user_agent", "consent_required", "consent_checked",
		"consent_version", "consent_timestamp", "landing_version_hash",
		"offer_product_id", "pricepoint_id", "partner_role_id",
		"timwe_transaction_id", "transaction_auth_code", "timwe_status",
		"he_source", "he_msisdn", "he_operator",
		"charged_at", "charge_payout", "conversion_postback_sent",
		"created_at", "updated_at",
	}
	mock.ExpectQuery(`FROM acquisition_transactions\s+WHERE id = \$1\s+AND tenant_id = \$2::uuid`).
		WithArgs(id, reconcilerTenantID).
		WillReturnRows(sqlmock.NewRows(cols).AddRow(
			id, uuid.New(), "daily", "233241234567", string(status), nil,
			nil, nil, nil, nil, nil, nil, false, true, nil, nil, nil, 33261, nil, nil,
			nil, nil, nil, nil, nil, nil, nil, nil, false, now, now))
}

const expireSQL = `UPDATE acquisition_transactions t\s+SET status = 'FAILED',\s+next_action = NULL,\s+` +
	`next_action_payload = jsonb_build_object\('failure_reason', 'NO_CHARGE_EVIDENCE', 'expired_at', now\(\)\)`

func TestChargeReconcilerTick_PromotesCandidatesAndContinuesPastFailures(t *testing.T) {
	failing, missing, ok := uuid.New(), uuid.New(), uuid.New()
	promoter := &fakeChargePromoter{fail: map[uuid.UUID]error{failing: errors.New("mark charged failed")}}
	r, mock := newTestChargeReconciler(t, promoter, enabledReconcilerConfig())

	mock.ExpectQuery(`(?s)SELECT t.id, t.tenant_id::text\s+FROM acquisition_transactions t.*`+
		`t.status = 'FAILED' AND t.next_action_payload->>'failure_reason' = 'NO_CHARGE_EVIDENCE'.*`+
		`newer.offer_product_id = t.offer_product_id.*`+
		`n.type IN \('CHARGE', 'USER_RENEWED'\).*upper\(trim\(n.mno_delivery_code\)\) = 'DELIVERED'.*`+
		`n.created_at >= \(t.created_at AT TIME ZONE 'UTC'\) - interval '5 minutes'.*LIMIT \$2`).
		WithArgs(30, 200).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id"}).
			AddRow(failing, reconcilerTenantID).
			AddRow(missing, reconcilerTenantID).
			AddRow(ok, reconcilerTenantID))
	expectCandidateLoad(mock, failing, domain.StatusSubscribed)
	mock.ExpectQuery(`FROM acquisition_transactions\s+WHERE id = \$1\s+AND tenant_id = \$2::uuid`).
		WithArgs(missing, reconcilerTenantID).
		WillReturnError(errors.New("connection reset"))
	expectCandidateLoad(mock, ok, domain.StatusConfirmRequired)
	mock.ExpectExec(expireSQL).WithArgs(7).WillReturnResult(sqlmock.NewResult(0, 0))

	r.Tick(context.Background())

	if len(promoter.calls) != 2 || promoter.calls[0] != failing || promoter.calls[1] != ok {
		t.Fatalf("expected promote attempts for both loadable candidates, got %v", promoter.calls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestChargeReconcilerTick_ExpiresUnchargedWithFailurePayload(t *testing.T) {
	promoter := &fakeChargePromoter{}
	r, mock := newTestChargeReconciler(t, promoter, enabledReconcilerConfig())

	mock.ExpectQuery(`SELECT t.id, t.tenant_id::text`).
		WithArgs(30, 200).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id"}))
	mock.ExpectExec(`(?s)` + expireSQL + `.*t.status IN \('SUBSCRIBED', 'CONFIRM_REQUIRED', 'ACTION_REQUIRED'\)` +
		`.*t.charged_at IS NULL.*t.created_at < \(now\(\) AT TIME ZONE 'UTC'\) - \(\$1::int \* interval '1 day'\)` +
		`.*AND NOT EXISTS \(.*upper\(trim\(n.mno_delivery_code\)\) = 'DELIVERED'`).
		WithArgs(7).
		WillReturnResult(sqlmock.NewResult(0, 3))

	r.Tick(context.Background())

	if len(promoter.calls) != 0 {
		t.Fatalf("expected no promotions, got %v", promoter.calls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestChargeReconcilerTick_ExpireRunsWhenPromoteListFails(t *testing.T) {
	r, mock := newTestChargeReconciler(t, &fakeChargePromoter{}, enabledReconcilerConfig())
	mock.ExpectQuery(`SELECT t.id, t.tenant_id::text`).WillReturnError(errors.New("db down"))
	mock.ExpectExec(expireSQL).WithArgs(7).WillReturnResult(sqlmock.NewResult(0, 0))

	r.Tick(context.Background())

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestChargeReconcilerDisabledViaEnvIsNoop(t *testing.T) {
	t.Setenv("ACQ_CHARGE_RECONCILER_ENABLED", "false")
	cfg := ChargeReconcilerConfigFromEnv(zap.NewNop())
	if cfg.Enabled {
		t.Fatal("expected reconciler disabled")
	}
	promoter := &fakeChargePromoter{}
	r, mock := newTestChargeReconciler(t, promoter, cfg)

	r.Tick(context.Background())
	done := make(chan struct{})
	go func() {
		r.Start(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected Start to return immediately when disabled")
	}

	if len(promoter.calls) != 0 {
		t.Fatalf("expected no promotions, got %v", promoter.calls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expected no SQL when disabled: %v", err)
	}
}

func TestChargeReconcilerConfigFromEnv(t *testing.T) {
	for _, name := range []string{"ACQ_CHARGE_RECONCILER_ENABLED", "ACQ_CHARGE_RECONCILER_INTERVAL", "ACQ_CHARGE_EXPIRY_DAYS", "ACQ_CHARGE_PROMOTE_LOOKBACK_DAYS"} {
		t.Setenv(name, "")
	}
	cfg := ChargeReconcilerConfigFromEnv(zap.NewNop())
	if !cfg.Enabled || cfg.Interval != 60*time.Second || cfg.ExpiryDays != 7 || cfg.PromoteLookbackDays != 30 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}

	t.Setenv("ACQ_CHARGE_RECONCILER_INTERVAL", "15s")
	t.Setenv("ACQ_CHARGE_EXPIRY_DAYS", "3")
	t.Setenv("ACQ_CHARGE_PROMOTE_LOOKBACK_DAYS", "10")
	cfg = ChargeReconcilerConfigFromEnv(zap.NewNop())
	if cfg.Interval != 15*time.Second || cfg.ExpiryDays != 3 || cfg.PromoteLookbackDays != 10 {
		t.Fatalf("unexpected overrides: %+v", cfg)
	}

	t.Setenv("ACQ_CHARGE_RECONCILER_ENABLED", "maybe")
	t.Setenv("ACQ_CHARGE_RECONCILER_INTERVAL", "-1s")
	t.Setenv("ACQ_CHARGE_EXPIRY_DAYS", "0")
	t.Setenv("ACQ_CHARGE_PROMOTE_LOOKBACK_DAYS", "abc")
	cfg = ChargeReconcilerConfigFromEnv(zap.NewNop())
	if !cfg.Enabled || cfg.Interval != 60*time.Second || cfg.ExpiryDays != 7 || cfg.PromoteLookbackDays != 30 {
		t.Fatalf("invalid values should fall back to defaults: %+v", cfg)
	}
}
