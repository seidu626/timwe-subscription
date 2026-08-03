package service

import (
	"database/sql"
	"database/sql/driver"
	"regexp"
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

// --- pure-function coverage: status mapping, ranking, slug generation ---

func TestPartnerAcquisitionStatus(t *testing.T) {
	tests := []struct {
		name               string
		action             PartnerSubscriptionAction
		subscriptionResult string
		wantStatus         domain.TransactionStatus
		wantMapped         bool
	}{
		{"optin already active maps to subscribed", PartnerSubscriptionActionOptin, "OPTIN_ALREADY_ACTIVE", domain.StatusSubscribed, true},
		{"optin active wait charging maps to subscribed", PartnerSubscriptionActionOptin, "OPTIN_ACTIVE_WAIT_CHARGING", domain.StatusSubscribed, true},
		{"optin preactive wait conf maps to confirm required", PartnerSubscriptionActionOptin, "OPTIN_PREACTIVE_WAIT_CONF", domain.StatusConfirmRequired, true},
		{"optin unrecognized result is unmapped", PartnerSubscriptionActionOptin, "OPTIN_CONFIG_NOT_FOUND", "", false},
		{"optin empty result is unmapped", PartnerSubscriptionActionOptin, "", "", false},
		{"confirm always subscribes regardless of result", PartnerSubscriptionActionConfirm, "", domain.StatusSubscribed, true},
		{"confirm ignores result content", PartnerSubscriptionActionConfirm, "anything", domain.StatusSubscribed, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, mapped := partnerAcquisitionStatus(tt.action, tt.subscriptionResult)
			if mapped != tt.wantMapped || status != tt.wantStatus {
				t.Fatalf("partnerAcquisitionStatus(%q, %q) = (%q, %v), want (%q, %v)",
					tt.action, tt.subscriptionResult, status, mapped, tt.wantStatus, tt.wantMapped)
			}
		})
	}
}

func TestPartnerStatusRank_IsMonotonicAlongLifecycle(t *testing.T) {
	order := []domain.TransactionStatus{
		domain.StatusPending,
		domain.StatusActionRequired,
		domain.StatusConfirmRequired,
		domain.StatusSubscribed,
		domain.StatusCharged,
	}
	for i := 1; i < len(order); i++ {
		if partnerStatusRank(order[i]) <= partnerStatusRank(order[i-1]) {
			t.Fatalf("expected rank(%s) > rank(%s) so later lifecycle steps are never regressed by a replay",
				order[i], order[i-1])
		}
	}
}

func TestPartnerCampaignSlug_IsDeterministicSanitizedAndTenantScoped(t *testing.T) {
	slug1 := partnerCampaignSlug(testTenantID, "Web GH! AirtelTigo")
	slug2 := partnerCampaignSlug(testTenantID, "Web GH! AirtelTigo")
	if slug1 != slug2 {
		t.Fatalf("expected partnerCampaignSlug to be deterministic for repeated calls, got %q and %q", slug1, slug2)
	}
	if !strings.HasPrefix(slug1, "direct-web-gh--airteltigo-") {
		t.Fatalf("expected sanitized channel-key prefix, got %q", slug1)
	}
	if len(slug1) > 100 {
		t.Fatalf("expected slug within the campaigns.slug length budget, got %d chars: %q", len(slug1), slug1)
	}
	otherTenantSlug := partnerCampaignSlug("22222222-2222-2222-2222-222222222222", "Web GH! AirtelTigo")
	if otherTenantSlug == slug1 {
		t.Fatalf("expected different tenants sharing a channel_key to get different slugs (campaigns.slug is unique globally), got %q for both", slug1)
	}
}

func TestSanitizeSlugSegment(t *testing.T) {
	got := sanitizeSlugSegment(" Web GH! AirtelTigo_1 ")
	want := "web-gh--airteltigo-1"
	if got != want {
		t.Fatalf("sanitizeSlugSegment() = %q, want %q", got, want)
	}
}

// --- service-level coverage: validation, campaign auto-create, idempotency ---

func newPartnerTestService(db *sql.DB) *TransactionService {
	logger := zap.NewNop()
	return NewTransactionService(
		repository.NewTransactionRepository(db, logger),
		repository.NewCampaignRepository(db, logger),
		repository.NewPostbackRepository(db, logger),
		NewProviderRegistry(logger),
		fakeTIMWEClient{},
		logger,
	)
}

func TestHandlePartnerSubscription_ValidatesRequiredFields(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()
	svc := newPartnerTestService(db)

	tests := []struct {
		name          string
		req           *PartnerSubscriptionRequest
		wantErrSubstr string
	}{
		{"nil request", nil, "request is required"},
		{"missing tenant_id", &PartnerSubscriptionRequest{MSISDN: "233241234567", Action: PartnerSubscriptionActionOptin}, "tenant_id is required"},
		{"missing msisdn", &PartnerSubscriptionRequest{TenantID: testTenantID, Action: PartnerSubscriptionActionOptin}, "msisdn is required"},
		{"missing action", &PartnerSubscriptionRequest{TenantID: testTenantID, MSISDN: "233241234567"}, "action is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.HandlePartnerSubscription(tt.req)
			if err == nil || !strings.Contains(err.Error(), tt.wantErrSubstr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErrSubstr, err)
			}
		})
	}

	// None of the above should ever touch the database.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestHandlePartnerSubscription_UnmappedResultIsNoopWithoutError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()
	svc := newPartnerTestService(db)

	req := &PartnerSubscriptionRequest{
		Action:             PartnerSubscriptionActionOptin,
		TenantID:           testTenantID,
		MSISDN:             "233241234567",
		SubscriptionResult: "OPTIN_CONFIG_NOT_FOUND",
	}
	if err := svc.HandlePartnerSubscription(req); err != nil {
		t.Fatalf("expected unmapped result to be a silent no-op, got error: %v", err)
	}

	// A status that carries no reporting signal must never touch the
	// database (no campaign lookup, no transaction write).
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

// adminCampaignRowValues builds a driver.Value row matching adminCampaignColumns()
// for the plain (non-aliased) SELECT used by GetAdminBySlug/GetAdminByTenantAndSlug,
// as opposed to expectTenantCampaign's aliased "c.id, c.tenant_id, ..." query.
func adminCampaignRowValues(tenantID, slug, country string, productID int) []driver.Value {
	now := time.Now()
	return []driver.Value{
		1, tenantID, nil, slug, "en", country, nil,
		productID, nil, nil, string(domain.FlowTypeMixed),
		nil, nil, nil, nil, nil, nil,
		nil, false, nil,
		nil, nil, nil,
		pq.StringArray{}, pq.StringArray{}, pq.StringArray{},
		nil, nil,
		true, now, now,
		nil, nil,
	}
}

func expectAdminCampaignLookup(mock sqlmock.Sqlmock, args []driver.Value, rows *sqlmock.Rows, err error) {
	q := mock.ExpectQuery(regexp.QuoteMeta("SELECT id, tenant_id, channel_id, slug, language, country, operator, offer_product_id, pricepoint_id")).
		WithArgs(args...)
	if err != nil {
		q.WillReturnError(err)
		return
	}
	q.WillReturnRows(rows)
}

func TestHandlePartnerSubscription_AutoCreatesCampaignAndTransactionOnFirstUse(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()
	svc := newPartnerTestService(db)

	channelID := "22222222-2222-2222-2222-222222222222"
	channelKey := "Web GH AirtelTigo"
	slug := partnerCampaignSlug(testTenantID, channelKey)
	now := time.Now()

	// 1. ensurePartnerCampaign: no existing synthetic campaign yet.
	expectAdminCampaignLookup(mock, []driver.Value{testTenantID, slug}, nil, sql.ErrNoRows)

	// 2. country/operator resolved from the tenant channel referenced by channel_id.
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, tenant_id, channel_key, provider, country, operator, capabilities, status, created_at, updated_at")).
		WithArgs(testTenantID, channelID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "channel_key", "provider", "country", "operator", "capabilities", "status", "created_at", "updated_at"}).
			AddRow(channelID, testTenantID, channelKey, "TIMWE", "GH", nil, pq.StringArray{"optin", "confirm"}, "ACTIVE", now, now))

	// 3. campaign insert (Create), returning the new slug.
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO campaigns")).
		WillReturnRows(sqlmock.NewRows([]string{"slug"}).AddRow(slug))

	// 4. Create()'s own GetAdminBySlug confirmation read.
	expectAdminCampaignLookup(mock, []driver.Value{slug},
		sqlmock.NewRows(adminCampaignColumns()).AddRow(adminCampaignRowValues(testTenantID, slug, "GH", 14397)...), nil)

	// 5. CreateForTenant()'s GetAdminByTenantAndSlug confirmation read.
	expectAdminCampaignLookup(mock, []driver.Value{testTenantID, slug},
		sqlmock.NewRows(adminCampaignColumns()).AddRow(adminCampaignRowValues(testTenantID, slug, "GH", 14397)...), nil)

	// 6. no existing transaction for this timwe_transaction_id yet.
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, correlation_id, campaign_slug, msisdn, status, next_action")).
		WithArgs(testTenantID, "timwe-tx-1").
		WillReturnError(sql.ErrNoRows)

	// 7. new acquisition transaction is inserted.
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO acquisition_transactions")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	req := &PartnerSubscriptionRequest{
		Action:             PartnerSubscriptionActionOptin,
		TenantID:           testTenantID,
		ChannelID:          channelID,
		ChannelKey:         channelKey,
		MSISDN:             "233241234567",
		ProductID:          14397,
		TimweTransactionID: "timwe-tx-1",
		SubscriptionResult: "OPTIN_ACTIVE_WAIT_CHARGING",
	}
	if err := svc.HandlePartnerSubscription(req); err != nil {
		t.Fatalf("HandlePartnerSubscription: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestHandlePartnerSubscription_ReusesExistingCampaignWithoutChannelLookup(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()
	svc := newPartnerTestService(db)

	channelKey := "web-gh-airteltigo"
	slug := partnerCampaignSlug(testTenantID, channelKey)

	// campaign already provisioned: ensurePartnerCampaign must short-circuit
	// before ever touching tenant_channels or issuing a Create.
	expectAdminCampaignLookup(mock, []driver.Value{testTenantID, slug},
		sqlmock.NewRows(adminCampaignColumns()).AddRow(adminCampaignRowValues(testTenantID, slug, "GH", 14397)...), nil)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, correlation_id, campaign_slug, msisdn, status, next_action")).
		WithArgs(testTenantID, "timwe-tx-2").
		WillReturnError(sql.ErrNoRows)

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO acquisition_transactions")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	req := &PartnerSubscriptionRequest{
		Action:             PartnerSubscriptionActionOptin,
		TenantID:           testTenantID,
		ChannelKey:         channelKey,
		MSISDN:             "233241234568",
		ProductID:          14397,
		TimweTransactionID: "timwe-tx-2",
		SubscriptionResult: "OPTIN_ACTIVE_WAIT_CHARGING",
	}
	if err := svc.HandlePartnerSubscription(req); err != nil {
		t.Fatalf("HandlePartnerSubscription: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

// acquisitionTransactionRowValues builds a driver.Value row matching
// acquisitionTransactionColumns() for the tenant-scoped transaction finders.
func acquisitionTransactionRowValues(id, correlationID uuid.UUID, campaignSlug, msisdn string, status domain.TransactionStatus, timweTxID, subscriptionResult string, productID int) []driver.Value {
	now := time.Now()
	return []driver.Value{
		id.String(), correlationID.String(), campaignSlug, msisdn, string(status), nil,
		nil, nil, nil, nil,
		nil, nil, false, true,
		nil, nil, nil,
		productID, nil, nil,
		timweTxID, nil, subscriptionResult,
		nil, nil, nil,
		nil, nil, false,
		now, now,
	}
}

func TestHandlePartnerSubscription_ReplayAtSameOrLowerRankIsIdempotentNoop(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()
	svc := newPartnerTestService(db)

	channelKey := "web-gh-airteltigo"
	slug := partnerCampaignSlug(testTenantID, channelKey)
	existingID := uuid.New()

	expectAdminCampaignLookup(mock, []driver.Value{testTenantID, slug},
		sqlmock.NewRows(adminCampaignColumns()).AddRow(adminCampaignRowValues(testTenantID, slug, "GH", 14397)...), nil)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, correlation_id, campaign_slug, msisdn, status, next_action")).
		WithArgs(testTenantID, "timwe-tx-3").
		WillReturnRows(sqlmock.NewRows(acquisitionTransactionColumns()).
			AddRow(acquisitionTransactionRowValues(existingID, uuid.New(), slug, "233241234569", domain.StatusSubscribed, "timwe-tx-3", "OPTIN_ACTIVE_WAIT_CHARGING", 14397)...))

	// A replayed optin carrying the same already-applied result must not
	// issue any further query or exec against the transaction.
	req := &PartnerSubscriptionRequest{
		Action:             PartnerSubscriptionActionOptin,
		TenantID:           testTenantID,
		ChannelKey:         channelKey,
		MSISDN:             "233241234569",
		ProductID:          14397,
		TimweTransactionID: "timwe-tx-3",
		SubscriptionResult: "OPTIN_ACTIVE_WAIT_CHARGING",
	}
	if err := svc.HandlePartnerSubscription(req); err != nil {
		t.Fatalf("HandlePartnerSubscription: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestHandlePartnerSubscription_ConfirmAdvancesExistingTransactionStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()
	svc := newPartnerTestService(db)

	channelKey := "web-gh-airteltigo"
	slug := partnerCampaignSlug(testTenantID, channelKey)
	existingID := uuid.New()

	expectAdminCampaignLookup(mock, []driver.Value{testTenantID, slug},
		sqlmock.NewRows(adminCampaignColumns()).AddRow(adminCampaignRowValues(testTenantID, slug, "GH", 14397)...), nil)

	// existing transaction is still at CONFIRM_REQUIRED (the pre-subscribed
	// status left behind by a prior optin), so a confirm notification must
	// advance it to SUBSCRIBED rather than creating a duplicate row.
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, correlation_id, campaign_slug, msisdn, status, next_action")).
		WithArgs(testTenantID, "timwe-tx-4").
		WillReturnRows(sqlmock.NewRows(acquisitionTransactionColumns()).
			AddRow(acquisitionTransactionRowValues(existingID, uuid.New(), slug, "233241234570", domain.StatusConfirmRequired, "timwe-tx-4", "OPTIN_PREACTIVE_WAIT_CONF", 14397)...))

	mock.ExpectExec(regexp.QuoteMeta("UPDATE acquisition_transactions")).
		WithArgs(domain.StatusSubscribed, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	req := &PartnerSubscriptionRequest{
		Action:             PartnerSubscriptionActionConfirm,
		TenantID:           testTenantID,
		ChannelKey:         channelKey,
		MSISDN:             "233241234570",
		ProductID:          14397,
		TimweTransactionID: "timwe-tx-4",
	}
	if err := svc.HandlePartnerSubscription(req); err != nil {
		t.Fatalf("HandlePartnerSubscription: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
