package service

import (
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
	"go.uber.org/zap"
)

func TestPostbackTemplateRequiresClickIDWhenTemplateUsesClickPlaceholder(t *testing.T) {
	svc := NewPostbackTemplateService(zap.NewNop())
	_, err := svc.BuildPostbackFromTemplate(&domain.PostbackTemplate{
		Method: "GET",
		URL:    "https://partner.example/postback?txid={click_id}",
	}, &domain.PostbackContext{TransactionID: uuid.NewString()})
	if err == nil || !strings.Contains(err.Error(), "click_id is required") {
		t.Fatalf("expected click_id requirement error, got %v", err)
	}
}

func TestPostbackTemplateUsesMSISDNHashWithoutRawMSISDN(t *testing.T) {
	tx := &domain.AcquisitionTransaction{
		ID:           uuid.New(),
		CampaignSlug: "daily",
		MSISDN:       "233241234567",
		Status:       domain.StatusCharged,
	}
	ctx := domain.NewPostbackContext(tx, &domain.Attribution{Provider: "generic", ClickID: "click-1"})

	req, err := NewPostbackTemplateService(zap.NewNop()).BuildPostbackFromTemplate(&domain.PostbackTemplate{
		Method: "GET",
		URL:    "https://partner.example/postback?msisdn_hash={msisdn_hash}&click_id={click_id}",
	}, ctx)
	if err != nil {
		t.Fatalf("expected postback request: %v", err)
	}
	raw := req.URL.String()
	if strings.Contains(raw, tx.MSISDN) {
		t.Fatalf("raw MSISDN leaked in rendered URL: %s", raw)
	}
	if !strings.Contains(raw, ctx.MSISDNHash) {
		t.Fatalf("expected MSISDN hash in rendered URL: %s", raw)
	}
}

func TestEnqueuePostbackPersistsTenantChannelAndRenderedURL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	repo := repository.NewPostbackRepository(db, zap.NewNop())
	svc := &TransactionService{
		postbackRepo:     repo,
		providerReg:      NewProviderRegistry(zap.NewNop()),
		postbackTemplate: NewPostbackTemplateService(zap.NewNop()),
		logger:           zap.NewNop(),
	}
	tenantID := "11111111-1111-1111-1111-111111111111"
	channelID := "22222222-2222-2222-2222-222222222222"
	tx := &domain.AcquisitionTransaction{
		ID:           uuid.New(),
		TenantID:     &tenantID,
		CampaignSlug: "daily",
		MSISDN:       "233241234567",
		Status:       domain.StatusCharged,
	}
	campaign := &domain.Campaign{
		TenantID:  &tenantID,
		ChannelID: &channelID,
		Slug:      "daily",
		PostbackRules: []byte(`{
			"conversion": {
				"mobplus": {"method": "GET", "url": "https://partner.example/cb?txid={click_id}&pub_id={pub_id}"}
			}
		}`),
	}

	mock.ExpectExec("INSERT INTO postback_outbox").
		WithArgs(
			sqlmock.AnyArg(),
			sql.NullString{String: tenantID, Valid: true},
			sql.NullString{String: channelID, Valid: true},
			tx.ID,
			domain.PostbackEventConversion,
			"mobplus",
			"https://partner.example/cb?txid=click-1&pub_id=pub-1",
			"GET",
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

	err = svc.enqueuePostback(tx, domain.PostbackEventConversion, &domain.Attribution{
		Provider: "mobplus",
		ClickID:  "click-1",
		PubID:    "pub-1",
	}, campaign)
	if err != nil {
		t.Fatalf("enqueue postback: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestEnqueuePostbackRecordsMissingClickAsFailedOutbox(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	repo := repository.NewPostbackRepository(db, zap.NewNop())
	svc := &TransactionService{
		postbackRepo:     repo,
		providerReg:      NewProviderRegistry(zap.NewNop()),
		postbackTemplate: NewPostbackTemplateService(zap.NewNop()),
		logger:           zap.NewNop(),
	}
	tenantID := "11111111-1111-1111-1111-111111111111"
	tx := &domain.AcquisitionTransaction{
		ID:           uuid.New(),
		TenantID:     &tenantID,
		CampaignSlug: "daily",
		MSISDN:       "233241234567",
		Status:       domain.StatusCharged,
	}
	campaign := &domain.Campaign{
		TenantID: &tenantID,
		Slug:     "daily",
		PostbackRules: []byte(`{
			"conversion": {
				"mobplus": {"method": "GET", "url": "https://partner.example/cb?txid={click_id}"}
			}
		}`),
	}

	mock.ExpectExec("INSERT INTO postback_outbox").
		WithArgs(
			sqlmock.AnyArg(),
			sql.NullString{String: tenantID, Valid: true},
			sql.NullString{},
			tx.ID,
			domain.PostbackEventConversion,
			"mobplus",
			"skipped://postback",
			"GET",
			"{}",
			sql.NullString{},
			sqlmock.AnyArg(),
			0,
			0,
			sql.NullTime{},
			domain.PostbackStatusFailed,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = svc.enqueuePostback(tx, domain.PostbackEventConversion, &domain.Attribution{Provider: "mobplus"}, campaign)
	if !errors.Is(err, ErrPostbackFailureRecorded) {
		t.Fatalf("expected recorded failure error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRecordFailedPostbackRequiresTransaction(t *testing.T) {
	svc := &TransactionService{logger: zap.NewNop()}
	if err := svc.recordFailedPostback(nil, domain.PostbackEventConversion, "mobplus", nil, "missing"); err == nil {
		t.Fatal("expected transaction required error")
	}
}

func TestCampaignChannelIDTrimsBlankValues(t *testing.T) {
	blank := " "
	if got := campaignChannelID(&domain.Campaign{ChannelID: &blank}); got != nil {
		t.Fatalf("expected nil channel id for blank value, got %q", *got)
	}
	channel := " 22222222-2222-2222-2222-222222222222 "
	got := campaignChannelID(&domain.Campaign{ChannelID: &channel})
	if got == nil || *got != strings.TrimSpace(channel) {
		t.Fatalf("expected trimmed channel id, got %#v", got)
	}
}
