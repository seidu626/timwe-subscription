package service

import (
	"database/sql"
	"fmt"
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

type fakeTIMWEClient struct{}

func (fakeTIMWEClient) OptIn(msisdn string, productID int, entryChannel string, trackingFields map[string]string, partnerRoleID string) (*TIMWEResponse, error) {
	return &TIMWEResponse{Success: true}, nil
}

func (fakeTIMWEClient) Confirm(msisdn string, productID int, entryChannel string, partnerRoleID string, authCode string) (*TIMWEResponse, error) {
	return &TIMWEResponse{Success: true}, nil
}

type capturingTIMWEClient struct {
	lastOptInMSISDN string
	optInCalled     bool
}

func (c *capturingTIMWEClient) OptIn(msisdn string, productID int, entryChannel string, trackingFields map[string]string, partnerRoleID string) (*TIMWEResponse, error) {
	c.optInCalled = true
	c.lastOptInMSISDN = msisdn
	return &TIMWEResponse{Success: true}, nil
}

func (c *capturingTIMWEClient) Confirm(msisdn string, productID int, entryChannel string, partnerRoleID string, authCode string) (*TIMWEResponse, error) {
	return &TIMWEResponse{Success: true}, nil
}

type timweOptInSuccessNoConfirmClient struct{}

func (timweOptInSuccessNoConfirmClient) OptIn(msisdn string, productID int, entryChannel string, trackingFields map[string]string, partnerRoleID string) (*TIMWEResponse, error) {
	return &TIMWEResponse{
		Success:         true,
		Status:          "SUCCESS",
		RequiresConfirm: false,
	}, nil
}

func (timweOptInSuccessNoConfirmClient) Confirm(msisdn string, productID int, entryChannel string, partnerRoleID string, authCode string) (*TIMWEResponse, error) {
	return &TIMWEResponse{Success: true}, nil
}

type timweOptInRequiresConfirmClient struct{}

func (timweOptInRequiresConfirmClient) OptIn(msisdn string, productID int, entryChannel string, trackingFields map[string]string, partnerRoleID string) (*TIMWEResponse, error) {
	return &TIMWEResponse{
		Success:         true,
		Status:          "OPTIN_PIN_WAITING",
		RequiresConfirm: true,
	}, nil
}

func (timweOptInRequiresConfirmClient) Confirm(msisdn string, productID int, entryChannel string, partnerRoleID string, authCode string) (*TIMWEResponse, error) {
	return &TIMWEResponse{Success: true}, nil
}

type timweConfirmAmbiguousSuccessClient struct{}

func (timweConfirmAmbiguousSuccessClient) OptIn(msisdn string, productID int, entryChannel string, trackingFields map[string]string, partnerRoleID string) (*TIMWEResponse, error) {
	return &TIMWEResponse{Success: true}, nil
}

func (timweConfirmAmbiguousSuccessClient) Confirm(msisdn string, productID int, entryChannel string, partnerRoleID string, authCode string) (*TIMWEResponse, error) {
	return &TIMWEResponse{
		Success: false,
		Status:  "SUCCESS",
		Message: "Confirmation pending",
	}, nil
}

type capturingConfirmTIMWEClient struct {
	lastConfirmMSISDN      string
	lastConfirmProductID   int
	lastConfirmPartnerRole string
}

func (c *capturingConfirmTIMWEClient) OptIn(msisdn string, productID int, entryChannel string, trackingFields map[string]string, partnerRoleID string) (*TIMWEResponse, error) {
	return &TIMWEResponse{Success: true}, nil
}

func (c *capturingConfirmTIMWEClient) Confirm(msisdn string, productID int, entryChannel string, partnerRoleID string, authCode string) (*TIMWEResponse, error) {
	c.lastConfirmMSISDN = msisdn
	c.lastConfirmProductID = productID
	c.lastConfirmPartnerRole = partnerRoleID
	return &TIMWEResponse{Success: true}, nil
}

type fallbackPricepointTIMWEClient struct {
	confirmProductIDs []int
}

func (c *fallbackPricepointTIMWEClient) OptIn(msisdn string, productID int, entryChannel string, trackingFields map[string]string, partnerRoleID string) (*TIMWEResponse, error) {
	return &TIMWEResponse{Success: true}, nil
}

func (c *fallbackPricepointTIMWEClient) Confirm(msisdn string, productID int, entryChannel string, partnerRoleID string, authCode string) (*TIMWEResponse, error) {
	c.confirmProductIDs = append(c.confirmProductIDs, productID)
	return nil, fmt.Errorf("request failed with status code: 400 (code=INTERNAL_ERROR message=MT response error [INVALID_PRICEPOINT_ID]: Invalid PricepointId)")
}

func TestNormalizeProviderMessage(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: ""},
		{name: "whitespace", input: "   ", want: ""},
		{name: "null literal", input: "null", want: ""},
		{name: "nil literal", input: "nil", want: ""},
		{name: "normal text", input: "Confirmation pending", want: "Confirmation pending"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeProviderMessage(tc.input); got != tc.want {
				t.Fatalf("unexpected normalized message: got=%q want=%q", got, tc.want)
			}
		})
	}
}

func campaignColumns() []string {
	return []string{
		"id", "slug", "language", "country", "operator", "offer_product_id", "pricepoint_id", "partner_role_id",
		"flow_type", "short_code", "sms_keyword", "price", "billing_cycle", "trial_flags", "terms_url",
		"inline_terms_text", "consent_required", "consent_version", "attribution_mapping", "postback_rules",
		"throttles", "allowed_referrers", "allowed_sources", "landing_page_urls", "tracking_config", "lp_copy",
		"enabled", "created_at", "updated_at", "created_by", "updated_by",
	}
}

func acquisitionTransactionColumns() []string {
	return []string{
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
}

func expectNoExistingCampaignMSISDNTransaction(mock sqlmock.Sqlmock, campaignSlug, msisdn string) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, correlation_id, campaign_slug, msisdn, status, next_action")).
		WithArgs(campaignSlug, msisdn, "CONFIRM_REQUIRED", "ACTION_REQUIRED", sqlmock.AnyArg()).
		WillReturnError(sql.ErrNoRows)
}

func TestCreateTransaction_UsesHEMSISDNForThrottle(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	logger := zap.NewNop()
	campaignRepo := repository.NewCampaignRepository(db, logger)
	txRepo := repository.NewTransactionRepository(db, logger)
	postbackRepo := repository.NewPostbackRepository(db, logger)
	providerReg := NewProviderRegistry(logger)
	providerReg.Register(NewGenericProvider(logger))

	service := NewTransactionService(txRepo, campaignRepo, postbackRepo, providerReg, fakeTIMWEClient{}, logger)

	campaignSlug := "test-campaign"
	formMSISDN := "233201234567"
	heMSISDN := "233241234567"
	throttles := `{"per_msisdn_per_day":1}`
	now := time.Now()
	campaignID := 1

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, slug, language")).
		WithArgs(campaignSlug).
		WillReturnRows(sqlmock.NewRows(campaignColumns()).AddRow(
			campaignID, campaignSlug, "en", "GH", nil, 101, nil, nil,
			"OTP", nil, nil, nil, nil, nil, nil,
			nil, false, nil, nil, nil,
			throttles, pq.StringArray{}, pq.StringArray{}, pq.StringArray{}, nil, nil,
			true, now, now, nil, nil,
		))

	expectNoExistingCampaignMSISDNTransaction(mock, campaignSlug, heMSISDN)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
		WithArgs(campaignSlug, heMSISDN).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	source := domain.HESourceReal
	req := &domain.CreateTransactionRequest{
		CampaignSlug:   campaignSlug,
		MSISDN:         formMSISDN,
		ConsentChecked: true,
		HESource:       &source,
		HEMSISDN:       &heMSISDN,
	}

	_, err = service.CreateTransaction(req)
	if err == nil || !strings.Contains(err.Error(), "throttled") {
		t.Fatalf("expected throttled error, got: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestCreateTransaction_NormalizesGhanaMSISDNBeforeOptIn(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	logger := zap.NewNop()
	campaignRepo := repository.NewCampaignRepository(db, logger)
	txRepo := repository.NewTransactionRepository(db, logger)
	postbackRepo := repository.NewPostbackRepository(db, logger)
	providerReg := NewProviderRegistry(logger)
	providerReg.Register(NewGenericProvider(logger))

	timweSpy := &capturingTIMWEClient{}
	service := NewTransactionService(txRepo, campaignRepo, postbackRepo, providerReg, timweSpy, logger)

	campaignSlug := "test-campaign"
	now := time.Now()
	campaignID := 1

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, slug, language")).
		WithArgs(campaignSlug).
		WillReturnRows(sqlmock.NewRows(campaignColumns()).AddRow(
			campaignID, campaignSlug, "en", "GH", nil, 101, nil, nil,
			"OTP", nil, nil, nil, nil, nil, nil,
			nil, false, nil, nil, nil,
			nil, pq.StringArray{}, pq.StringArray{}, pq.StringArray{}, nil, nil,
			true, now, now, nil, nil,
		))

	expectNoExistingCampaignMSISDNTransaction(mock, campaignSlug, "233561914461")

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO acquisition_transactions")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	req := &domain.CreateTransactionRequest{
		CampaignSlug:   campaignSlug,
		MSISDN:         "0561914461",
		ConsentChecked: true,
	}

	_, err = service.CreateTransaction(req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !timweSpy.optInCalled {
		t.Fatal("expected TIMWE OptIn to be called")
	}
	if timweSpy.lastOptInMSISDN != "233561914461" {
		t.Fatalf("expected normalized msisdn 233561914461, got %s", timweSpy.lastOptInMSISDN)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestCreateTransaction_InvalidGhanaMSISDNSkipsOptIn(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	logger := zap.NewNop()
	campaignRepo := repository.NewCampaignRepository(db, logger)
	txRepo := repository.NewTransactionRepository(db, logger)
	postbackRepo := repository.NewPostbackRepository(db, logger)
	providerReg := NewProviderRegistry(logger)
	providerReg.Register(NewGenericProvider(logger))

	timweSpy := &capturingTIMWEClient{}
	service := NewTransactionService(txRepo, campaignRepo, postbackRepo, providerReg, timweSpy, logger)

	campaignSlug := "test-campaign"
	now := time.Now()
	campaignID := 1

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, slug, language")).
		WithArgs(campaignSlug).
		WillReturnRows(sqlmock.NewRows(campaignColumns()).AddRow(
			campaignID, campaignSlug, "en", "GH", nil, 101, nil, nil,
			"OTP", nil, nil, nil, nil, nil, nil,
			nil, false, nil, nil, nil,
			nil, pq.StringArray{}, pq.StringArray{}, pq.StringArray{}, nil, nil,
			true, now, now, nil, nil,
		))

	req := &domain.CreateTransactionRequest{
		CampaignSlug:   campaignSlug,
		MSISDN:         "123456",
		ConsentChecked: true,
	}

	_, err = service.CreateTransaction(req)
	if err == nil || !strings.Contains(err.Error(), "invalid msisdn format") {
		t.Fatalf("expected invalid msisdn format error, got: %v", err)
	}

	if timweSpy.optInCalled {
		t.Fatal("expected TIMWE OptIn not to be called for invalid msisdn")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestCreateTransaction_ReturnsExistingCampaignMSISDNTransactionBeforeThrottle(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	logger := zap.NewNop()
	campaignRepo := repository.NewCampaignRepository(db, logger)
	txRepo := repository.NewTransactionRepository(db, logger)
	postbackRepo := repository.NewPostbackRepository(db, logger)
	providerReg := NewProviderRegistry(logger)
	providerReg.Register(NewGenericProvider(logger))

	timweSpy := &capturingTIMWEClient{}
	service := NewTransactionService(txRepo, campaignRepo, postbackRepo, providerReg, timweSpy, logger)

	campaignSlug := "test-campaign-existing"
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, slug, language")).
		WithArgs(campaignSlug).
		WillReturnRows(sqlmock.NewRows(campaignColumns()).AddRow(
			1, campaignSlug, "en", "GH", nil, 101, nil, nil,
			"OTP", nil, nil, nil, nil, nil, nil,
			nil, false, nil, nil, nil,
			nil, pq.StringArray{}, pq.StringArray{}, pq.StringArray{}, nil, nil,
			true, now, now, nil, nil,
		))

	existingID := uuid.New()
	correlationID := uuid.New()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, correlation_id, campaign_slug, msisdn, status, next_action")).
		WithArgs(campaignSlug, "233561914461", "CONFIRM_REQUIRED", "ACTION_REQUIRED", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows(acquisitionTransactionColumns()).AddRow(
			existingID, correlationID, campaignSlug, "233561914461", "CONFIRM_REQUIRED", "OTP",
			`{"transaction_id":"`+existingID.String()+`","prompt":"Please enter the confirmation code sent to your phone"}`, nil, nil, nil,
			nil, nil, false, false,
			nil, nil, nil,
			nil, nil, nil,
			nil, nil, nil,
			nil, nil, nil,
			nil, nil, false,
			now, now,
		))

	resp, err := service.CreateTransaction(&domain.CreateTransactionRequest{
		CampaignSlug:   campaignSlug,
		MSISDN:         "0561914461",
		ConsentChecked: true,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if resp.TransactionID != existingID {
		t.Fatalf("expected existing transaction ID %s, got %s", existingID, resp.TransactionID)
	}
	if resp.Status != domain.StatusConfirmRequired {
		t.Fatalf("expected status %s, got %s", domain.StatusConfirmRequired, resp.Status)
	}
	if resp.NextAction != domain.NextActionOTP {
		t.Fatalf("expected next_action %s, got %s", domain.NextActionOTP, resp.NextAction)
	}
	if timweSpy.optInCalled {
		t.Fatal("expected TIMWE OptIn not to be called when existing transaction is returned")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestCreateTransaction_StalePendingTransactionTriggersFreshOptIn(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	logger := zap.NewNop()
	campaignRepo := repository.NewCampaignRepository(db, logger)
	txRepo := repository.NewTransactionRepository(db, logger)
	postbackRepo := repository.NewPostbackRepository(db, logger)
	providerReg := NewProviderRegistry(logger)
	providerReg.Register(NewGenericProvider(logger))

	timweSpy := &capturingTIMWEClient{}
	service := NewTransactionService(txRepo, campaignRepo, postbackRepo, providerReg, timweSpy, logger)
	service.SetPendingTransactionTTL(10 * time.Minute)

	campaignSlug := "test-campaign-stale"
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, slug, language")).
		WithArgs(campaignSlug).
		WillReturnRows(sqlmock.NewRows(campaignColumns()).AddRow(
			1, campaignSlug, "en", "GH", nil, 101, nil, nil,
			"OTP", nil, nil, nil, nil, nil, nil,
			nil, false, nil, nil, nil,
			nil, pq.StringArray{}, pq.StringArray{}, pq.StringArray{}, nil, nil,
			true, now, now, nil, nil,
		))

	expectNoExistingCampaignMSISDNTransaction(mock, campaignSlug, "233561914461")

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO acquisition_transactions")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	resp, err := service.CreateTransaction(&domain.CreateTransactionRequest{
		CampaignSlug:   campaignSlug,
		MSISDN:         "0561914461",
		ConsentChecked: true,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !timweSpy.optInCalled {
		t.Fatal("expected fresh opt-in call when no reusable pending transaction is found")
	}
	if resp.Status != domain.StatusConfirmRequired {
		t.Fatalf("expected status %s, got %s", domain.StatusConfirmRequired, resp.Status)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestCreateTransaction_OTPFlowRequiresConfirmOnOptInSuccessWithoutHE(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	logger := zap.NewNop()
	campaignRepo := repository.NewCampaignRepository(db, logger)
	txRepo := repository.NewTransactionRepository(db, logger)
	postbackRepo := repository.NewPostbackRepository(db, logger)
	providerReg := NewProviderRegistry(logger)
	providerReg.Register(NewGenericProvider(logger))

	service := NewTransactionService(txRepo, campaignRepo, postbackRepo, providerReg, timweOptInSuccessNoConfirmClient{}, logger)

	campaignSlug := "test-campaign-otp"
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, slug, language")).
		WithArgs(campaignSlug).
		WillReturnRows(sqlmock.NewRows(campaignColumns()).AddRow(
			1, campaignSlug, "en", "GH", nil, 101, nil, nil,
			"OTP", nil, nil, nil, nil, nil, nil,
			nil, false, nil, nil, nil,
			nil, pq.StringArray{}, pq.StringArray{}, pq.StringArray{}, nil, nil,
			true, now, now, nil, nil,
		))

	expectNoExistingCampaignMSISDNTransaction(mock, campaignSlug, "233561914461")

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO acquisition_transactions")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	resp, err := service.CreateTransaction(&domain.CreateTransactionRequest{
		CampaignSlug:   campaignSlug,
		MSISDN:         "0561914461",
		ConsentChecked: true,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if resp.Status != domain.StatusConfirmRequired {
		t.Fatalf("expected status %s, got %s", domain.StatusConfirmRequired, resp.Status)
	}
	if resp.NextAction != domain.NextActionOTP {
		t.Fatalf("expected next_action %s, got %s", domain.NextActionOTP, resp.NextAction)
	}
	if _, ok := resp.Payload["transaction_id"]; !ok {
		t.Fatal("expected transaction_id in payload")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestCreateTransaction_OTPFlowBypassesConfirmWhenHEIsDetected(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	logger := zap.NewNop()
	campaignRepo := repository.NewCampaignRepository(db, logger)
	txRepo := repository.NewTransactionRepository(db, logger)
	postbackRepo := repository.NewPostbackRepository(db, logger)
	providerReg := NewProviderRegistry(logger)
	providerReg.Register(NewGenericProvider(logger))

	service := NewTransactionService(txRepo, campaignRepo, postbackRepo, providerReg, timweOptInSuccessNoConfirmClient{}, logger)

	campaignSlug := "test-campaign-otp-he"
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, slug, language")).
		WithArgs(campaignSlug).
		WillReturnRows(sqlmock.NewRows(campaignColumns()).AddRow(
			1, campaignSlug, "en", "GH", nil, 101, nil, nil,
			"OTP", nil, nil, nil, nil, nil, nil,
			nil, false, nil, nil, nil,
			nil, pq.StringArray{}, pq.StringArray{}, pq.StringArray{}, nil, nil,
			true, now, now, nil, nil,
		))

	heMSISDN := "233561914461"
	expectNoExistingCampaignMSISDNTransaction(mock, campaignSlug, heMSISDN)

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO acquisition_transactions")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	heSource := domain.HESourceSimulated
	resp, err := service.CreateTransaction(&domain.CreateTransactionRequest{
		CampaignSlug:   campaignSlug,
		MSISDN:         "0561914461",
		ConsentChecked: true,
		HESource:       &heSource,
		HEMSISDN:       &heMSISDN,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if resp.Status != domain.StatusSubscribed {
		t.Fatalf("expected status %s, got %s", domain.StatusSubscribed, resp.Status)
	}
	if resp.NextAction != domain.NextActionShowInstructions {
		t.Fatalf("expected next_action %s, got %s", domain.NextActionShowInstructions, resp.NextAction)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestCreateTransaction_RedirectFlowDoesNotEnforceOTP(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	logger := zap.NewNop()
	campaignRepo := repository.NewCampaignRepository(db, logger)
	txRepo := repository.NewTransactionRepository(db, logger)
	postbackRepo := repository.NewPostbackRepository(db, logger)
	providerReg := NewProviderRegistry(logger)
	providerReg.Register(NewGenericProvider(logger))

	service := NewTransactionService(txRepo, campaignRepo, postbackRepo, providerReg, timweOptInRequiresConfirmClient{}, logger)

	campaignSlug := "test-campaign-redirect"
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, slug, language")).
		WithArgs(campaignSlug).
		WillReturnRows(sqlmock.NewRows(campaignColumns()).AddRow(
			1, campaignSlug, "en", "GH", nil, 101, nil, nil,
			"REDIRECT", nil, nil, nil, nil, nil, nil,
			nil, false, nil, nil, nil,
			nil, pq.StringArray{}, pq.StringArray{}, pq.StringArray{}, nil, nil,
			true, now, now, nil, nil,
		))

	expectNoExistingCampaignMSISDNTransaction(mock, campaignSlug, "233561914461")

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO acquisition_transactions")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	resp, err := service.CreateTransaction(&domain.CreateTransactionRequest{
		CampaignSlug:   campaignSlug,
		MSISDN:         "0561914461",
		ConsentChecked: true,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if resp.NextAction == domain.NextActionOTP || resp.Status == domain.StatusConfirmRequired {
		t.Fatalf("expected redirect flow not to enforce OTP, got status=%s next_action=%s", resp.Status, resp.NextAction)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestCreateTransaction_RedirectFlowReturnsRedirectActionWhenConfigured(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	logger := zap.NewNop()
	campaignRepo := repository.NewCampaignRepository(db, logger)
	txRepo := repository.NewTransactionRepository(db, logger)
	postbackRepo := repository.NewPostbackRepository(db, logger)
	providerReg := NewProviderRegistry(logger)
	providerReg.Register(NewGenericProvider(logger))

	service := NewTransactionService(txRepo, campaignRepo, postbackRepo, providerReg, timweOptInSuccessNoConfirmClient{}, logger)

	campaignSlug := "test-campaign-redirect-configured"
	now := time.Now()
	redirectCfg := `{"redirect_url":"https://partner.example.com/subscribe"}`

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, slug, language")).
		WithArgs(campaignSlug).
		WillReturnRows(sqlmock.NewRows(campaignColumns()).AddRow(
			1, campaignSlug, "en", "GH", nil, 101, nil, nil,
			"REDIRECT", nil, nil, nil, nil, nil, nil,
			nil, false, nil, nil, nil,
			nil, pq.StringArray{}, pq.StringArray{}, pq.StringArray{}, redirectCfg, nil,
			true, now, now, nil, nil,
		))

	expectNoExistingCampaignMSISDNTransaction(mock, campaignSlug, "233561914461")

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO acquisition_transactions")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	resp, err := service.CreateTransaction(&domain.CreateTransactionRequest{
		CampaignSlug:   campaignSlug,
		MSISDN:         "0561914461",
		ConsentChecked: true,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if resp.Status != domain.StatusActionRequired {
		t.Fatalf("expected status %s, got %s", domain.StatusActionRequired, resp.Status)
	}
	if resp.NextAction != domain.NextActionRedirect {
		t.Fatalf("expected next_action %s, got %s", domain.NextActionRedirect, resp.NextAction)
	}
	if got, ok := resp.Payload["redirect_url"].(string); !ok || got == "" {
		t.Fatalf("expected redirect_url in payload, got %+v", resp.Payload)
	}
	if got, ok := resp.Payload["url"].(string); !ok || got == "" {
		t.Fatalf("expected url in payload, got %+v", resp.Payload)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestCreateTransaction_MixedFlowStillSupportsExplicitProviderConfirm(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	logger := zap.NewNop()
	campaignRepo := repository.NewCampaignRepository(db, logger)
	txRepo := repository.NewTransactionRepository(db, logger)
	postbackRepo := repository.NewPostbackRepository(db, logger)
	providerReg := NewProviderRegistry(logger)
	providerReg.Register(NewGenericProvider(logger))

	service := NewTransactionService(txRepo, campaignRepo, postbackRepo, providerReg, timweOptInRequiresConfirmClient{}, logger)

	campaignSlug := "test-campaign-mixed"
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, slug, language")).
		WithArgs(campaignSlug).
		WillReturnRows(sqlmock.NewRows(campaignColumns()).AddRow(
			1, campaignSlug, "en", "GH", nil, 101, nil, nil,
			"MIXED", nil, nil, nil, nil, nil, nil,
			nil, false, nil, nil, nil,
			nil, pq.StringArray{}, pq.StringArray{}, pq.StringArray{}, nil, nil,
			true, now, now, nil, nil,
		))

	expectNoExistingCampaignMSISDNTransaction(mock, campaignSlug, "233561914461")

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO acquisition_transactions")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	resp, err := service.CreateTransaction(&domain.CreateTransactionRequest{
		CampaignSlug:   campaignSlug,
		MSISDN:         "0561914461",
		ConsentChecked: true,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if resp.Status != domain.StatusConfirmRequired {
		t.Fatalf("expected status %s, got %s", domain.StatusConfirmRequired, resp.Status)
	}
	if resp.NextAction != domain.NextActionOTP {
		t.Fatalf("expected next_action %s, got %s", domain.NextActionOTP, resp.NextAction)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestCreateTransaction_ClickToSMSFlowRemainsOpenSMS(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	logger := zap.NewNop()
	campaignRepo := repository.NewCampaignRepository(db, logger)
	txRepo := repository.NewTransactionRepository(db, logger)
	postbackRepo := repository.NewPostbackRepository(db, logger)
	providerReg := NewProviderRegistry(logger)
	providerReg.Register(NewGenericProvider(logger))

	service := NewTransactionService(txRepo, campaignRepo, postbackRepo, providerReg, timweOptInSuccessNoConfirmClient{}, logger)

	campaignSlug := "test-click-to-sms"
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, slug, language")).
		WithArgs(campaignSlug).
		WillReturnRows(sqlmock.NewRows(campaignColumns()).AddRow(
			1, campaignSlug, "en", "GH", nil, 101, nil, nil,
			"CLICK_TO_SMS", "601061", "JOIN", nil, nil, nil, nil,
			nil, false, nil, nil, nil,
			nil, pq.StringArray{}, pq.StringArray{}, pq.StringArray{}, nil, nil,
			true, now, now, nil, nil,
		))

	expectNoExistingCampaignMSISDNTransaction(mock, campaignSlug, "233561914461")

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO acquisition_transactions")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	resp, err := service.CreateTransaction(&domain.CreateTransactionRequest{
		CampaignSlug:   campaignSlug,
		MSISDN:         "0561914461",
		ConsentChecked: true,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if resp.Status != domain.StatusActionRequired {
		t.Fatalf("expected status %s, got %s", domain.StatusActionRequired, resp.Status)
	}
	if resp.NextAction != domain.NextActionOpenSMS {
		t.Fatalf("expected next_action %s, got %s", domain.NextActionOpenSMS, resp.NextAction)
	}
	if _, ok := resp.Payload["sms_link"]; !ok {
		t.Fatal("expected sms_link in payload")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestConfirmTransaction_AmbiguousSuccessRemainsConfirmRequired(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	logger := zap.NewNop()
	campaignRepo := repository.NewCampaignRepository(db, logger)
	txRepo := repository.NewTransactionRepository(db, logger)
	postbackRepo := repository.NewPostbackRepository(db, logger)
	providerReg := NewProviderRegistry(logger)
	providerReg.Register(NewGenericProvider(logger))

	service := NewTransactionService(txRepo, campaignRepo, postbackRepo, providerReg, timweConfirmAmbiguousSuccessClient{}, logger)

	transactionID := uuid.New()
	correlationID := uuid.New()
	campaignSlug := "confirm-campaign"
	timweTransactionID := "timwe-tx-123"
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, correlation_id, campaign_slug, msisdn, status, next_action")).
		WithArgs(transactionID).
		WillReturnRows(sqlmock.NewRows(acquisitionTransactionColumns()).AddRow(
			transactionID, correlationID, campaignSlug, "233561914461", "CONFIRM_REQUIRED", "OTP",
			`{"transaction_id":"`+transactionID.String()+`"}`, nil, nil, nil,
			nil, nil, false, false,
			nil, nil, nil,
			101, nil, 2117,
			timweTransactionID, "0000", "OPTIN_WAITING",
			nil, nil, nil,
			nil, nil, false,
			now, now,
		))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, slug, language")).
		WithArgs(campaignSlug).
		WillReturnRows(sqlmock.NewRows(campaignColumns()).AddRow(
			1, campaignSlug, "en", "GH", nil, 101, nil, 2117,
			"OTP", nil, nil, nil, nil, nil, nil,
			nil, false, nil, nil, nil,
			nil, pq.StringArray{}, pq.StringArray{}, pq.StringArray{}, nil, nil,
			true, now, now, nil, nil,
		))

	mock.ExpectExec(regexp.QuoteMeta("UPDATE acquisition_transactions")).
		WithArgs(timweTransactionID, "1234", "SUCCESS", transactionID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	resp, err := service.ConfirmTransaction(transactionID, "1234")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp.Status != domain.StatusConfirmRequired {
		t.Fatalf("expected status %s, got %s", domain.StatusConfirmRequired, resp.Status)
	}
	if msg, ok := resp.Payload["message"].(string); !ok || msg == "" {
		t.Fatalf("expected pending confirmation message, got %+v", resp.Payload)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestConfirmTransaction_UsesTransactionScopedProductWhenCampaignChanges(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	logger := zap.NewNop()
	campaignRepo := repository.NewCampaignRepository(db, logger)
	txRepo := repository.NewTransactionRepository(db, logger)
	postbackRepo := repository.NewPostbackRepository(db, logger)
	providerReg := NewProviderRegistry(logger)
	providerReg.Register(NewGenericProvider(logger))

	timweSpy := &capturingConfirmTIMWEClient{}
	service := NewTransactionService(txRepo, campaignRepo, postbackRepo, providerReg, timweSpy, logger)

	transactionID := uuid.New()
	correlationID := uuid.New()
	campaignSlug := "confirm-campaign-drift"
	timweTransactionID := "timwe-tx-555"
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, correlation_id, campaign_slug, msisdn, status, next_action")).
		WithArgs(transactionID).
		WillReturnRows(sqlmock.NewRows(acquisitionTransactionColumns()).AddRow(
			transactionID, correlationID, campaignSlug, "233561914461", "CONFIRM_REQUIRED", "OTP",
			`{"transaction_id":"`+transactionID.String()+`"}`, nil, nil, nil,
			nil, nil, false, false,
			nil, nil, nil,
			8509, nil, 2117,
			timweTransactionID, "0000", "OPTIN_WAITING",
			nil, nil, nil,
			nil, nil, false,
			now, now,
		))

	// Simulate campaign product/role drift after opt-in.
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, slug, language")).
		WithArgs(campaignSlug).
		WillReturnRows(sqlmock.NewRows(campaignColumns()).AddRow(
			1, campaignSlug, "en", "GH", nil, 9999, nil, 3333,
			"OTP", nil, nil, nil, nil, nil, nil,
			nil, false, nil, nil, nil,
			nil, pq.StringArray{}, pq.StringArray{}, pq.StringArray{}, nil, nil,
			true, now, now, nil, nil,
		))

	mock.ExpectExec(regexp.QuoteMeta("UPDATE acquisition_transactions")).
		WithArgs(domain.StatusSubscribed, sqlmock.AnyArg(), sqlmock.AnyArg(), transactionID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	resp, err := service.ConfirmTransaction(transactionID, "1234")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp.Status != domain.StatusSubscribed {
		t.Fatalf("expected status %s, got %s", domain.StatusSubscribed, resp.Status)
	}

	if timweSpy.lastConfirmProductID != 8509 {
		t.Fatalf("expected confirm productID=8509, got %d", timweSpy.lastConfirmProductID)
	}
	if timweSpy.lastConfirmPartnerRole != "2117" {
		t.Fatalf("expected confirm partnerRoleID=2117, got %s", timweSpy.lastConfirmPartnerRole)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestConfirmTransaction_DoesNotRetryWithPricepointOnInvalidPricepointID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	logger := zap.NewNop()
	campaignRepo := repository.NewCampaignRepository(db, logger)
	txRepo := repository.NewTransactionRepository(db, logger)
	postbackRepo := repository.NewPostbackRepository(db, logger)
	providerReg := NewProviderRegistry(logger)
	providerReg.Register(NewGenericProvider(logger))

	timweSpy := &fallbackPricepointTIMWEClient{}
	service := NewTransactionService(txRepo, campaignRepo, postbackRepo, providerReg, timweSpy, logger)

	transactionID := uuid.New()
	correlationID := uuid.New()
	campaignSlug := "confirm-campaign-pricepoint"
	timweTransactionID := "timwe-tx-777"
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, correlation_id, campaign_slug, msisdn, status, next_action")).
		WithArgs(transactionID).
		WillReturnRows(sqlmock.NewRows(acquisitionTransactionColumns()).AddRow(
			transactionID, correlationID, campaignSlug, "233561914461", "CONFIRM_REQUIRED", "OTP",
			`{"transaction_id":"`+transactionID.String()+`"}`, nil, nil, nil,
			nil, nil, false, false,
			nil, nil, nil,
			8509, 14397, 2117,
			timweTransactionID, "0000", "OPTIN_WAITING",
			nil, nil, nil,
			nil, nil, false,
			now, now,
		))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, slug, language")).
		WithArgs(campaignSlug).
		WillReturnRows(sqlmock.NewRows(campaignColumns()).AddRow(
			1, campaignSlug, "en", "GH", nil, 8509, 14397, 2117,
			"OTP", nil, nil, nil, nil, nil, nil,
			nil, false, nil, nil, nil,
			nil, pq.StringArray{}, pq.StringArray{}, pq.StringArray{}, nil, nil,
			true, now, now, nil, nil,
		))

	resp, err := service.ConfirmTransaction(transactionID, "1234")
	if err == nil {
		t.Fatal("expected confirm to fail for INVALID_PRICEPOINT_ID, got nil error")
	}
	if resp != nil {
		t.Fatalf("expected nil response, got %+v", resp)
	}
	if !strings.Contains(err.Error(), "INVALID_PRICEPOINT_ID") {
		t.Fatalf("expected INVALID_PRICEPOINT_ID error, got %v", err)
	}
	if len(timweSpy.confirmProductIDs) != 1 {
		t.Fatalf("expected 1 confirm attempt, got %d", len(timweSpy.confirmProductIDs))
	}
	if timweSpy.confirmProductIDs[0] != 8509 {
		t.Fatalf("expected confirm attempt with offer product id, got %d", timweSpy.confirmProductIDs[0])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
