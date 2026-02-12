package service

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
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

	columns := []string{
		"id", "slug", "language", "country", "operator", "offer_product_id", "pricepoint_id", "partner_role_id",
		"flow_type", "short_code", "sms_keyword", "price", "billing_cycle", "trial_flags", "terms_url",
		"inline_terms_text", "consent_required", "consent_version", "attribution_mapping", "postback_rules",
		"throttles", "allowed_referrers", "allowed_sources", "landing_page_urls", "tracking_config",
		"enabled", "created_at", "updated_at", "created_by", "updated_by",
	}

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, slug, language")).
		WithArgs(campaignSlug).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(
			campaignID, campaignSlug, "en", "GH", nil, 101, nil, nil,
			"OTP", nil, nil, nil, nil, nil, nil,
			nil, false, nil, nil, nil,
			throttles, pq.StringArray{}, pq.StringArray{}, pq.StringArray{}, nil,
			true, now, now, nil, nil,
		))

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
