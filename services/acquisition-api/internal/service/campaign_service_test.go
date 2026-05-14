package service

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"go.uber.org/zap"
)

type fakeCampaignRepo struct {
	getByTenantKeyAndSlugFn   func(string, string) (*domain.Campaign, error)
	getAdminBySlugFn          func(string) (*domain.Campaign, error)
	getAdminByTenantAndSlugFn func(string, string) (*domain.Campaign, error)
	listAllFn                 func(*bool, *string) ([]*domain.Campaign, error)
	listAllForTenantFn        func(string, *bool, *string) ([]*domain.Campaign, error)
	createFn                  func(*domain.Campaign) (*domain.Campaign, error)
	createForTenantFn         func(string, *domain.Campaign) (*domain.Campaign, error)
	updateFn                  func(string, *domain.Campaign) (*domain.Campaign, error)
	updateForTenantFn         func(string, string, *domain.Campaign) (*domain.Campaign, error)
	setEnabledFn              func(string, bool, *string) (*domain.Campaign, error)
	setEnabledForTenantFn     func(string, string, bool, *string) (*domain.Campaign, error)
	validateOfferFn           func(int, *int) error
	validateTenantOfferFn     func(string, int, *int) error
	validateTenantChannelFn   func(string, string, string, *string, domain.FlowType) error
	updatePostbackRulesFn     func(string, json.RawMessage) error
}

func (f *fakeCampaignRepo) GetByTenantKeyAndSlug(tenantKey, slug string) (*domain.Campaign, error) {
	return f.getByTenantKeyAndSlugFn(tenantKey, slug)
}
func (f *fakeCampaignRepo) GetAdminBySlug(slug string) (*domain.Campaign, error) {
	return f.getAdminBySlugFn(slug)
}
func (f *fakeCampaignRepo) GetAdminByTenantAndSlug(tenantID, slug string) (*domain.Campaign, error) {
	return f.getAdminByTenantAndSlugFn(tenantID, slug)
}
func (f *fakeCampaignRepo) ListAll(enabled *bool, country *string) ([]*domain.Campaign, error) {
	return f.listAllFn(enabled, country)
}
func (f *fakeCampaignRepo) ListAllForTenant(tenantID string, enabled *bool, country *string) ([]*domain.Campaign, error) {
	return f.listAllForTenantFn(tenantID, enabled, country)
}
func (f *fakeCampaignRepo) Create(c *domain.Campaign) (*domain.Campaign, error) {
	return f.createFn(c)
}
func (f *fakeCampaignRepo) CreateForTenant(tenantID string, c *domain.Campaign) (*domain.Campaign, error) {
	return f.createForTenantFn(tenantID, c)
}
func (f *fakeCampaignRepo) Update(slug string, c *domain.Campaign) (*domain.Campaign, error) {
	return f.updateFn(slug, c)
}
func (f *fakeCampaignRepo) UpdateForTenant(tenantID, slug string, c *domain.Campaign) (*domain.Campaign, error) {
	return f.updateForTenantFn(tenantID, slug, c)
}
func (f *fakeCampaignRepo) SetEnabled(slug string, enabled bool, updatedBy *string) (*domain.Campaign, error) {
	return f.setEnabledFn(slug, enabled, updatedBy)
}
func (f *fakeCampaignRepo) SetEnabledForTenant(tenantID, slug string, enabled bool, updatedBy *string) (*domain.Campaign, error) {
	return f.setEnabledForTenantFn(tenantID, slug, enabled, updatedBy)
}
func (f *fakeCampaignRepo) UpdatePostbackRules(slug string, rules json.RawMessage) error {
	if f.updatePostbackRulesFn != nil {
		return f.updatePostbackRulesFn(slug, rules)
	}
	return nil
}
func (f *fakeCampaignRepo) ValidateOfferProductMapping(offerProductID int, pricepointID *int) error {
	if f.validateOfferFn == nil {
		return nil
	}
	return f.validateOfferFn(offerProductID, pricepointID)
}
func (f *fakeCampaignRepo) ValidateTenantOfferProductMapping(tenantID string, offerProductID int, pricepointID *int) error {
	if f.validateTenantOfferFn == nil {
		return nil
	}
	return f.validateTenantOfferFn(tenantID, offerProductID, pricepointID)
}
func (f *fakeCampaignRepo) ValidateTenantChannelForCampaign(tenantID, channelID, country string, operator *string, flowType domain.FlowType) error {
	if f.validateTenantChannelFn == nil {
		return nil
	}
	return f.validateTenantChannelFn(tenantID, channelID, country, operator, flowType)
}

func TestCampaignService_AdminCRUD_HappyPath(t *testing.T) {
	logger := zap.NewNop()

	repo := &fakeCampaignRepo{
		getAdminBySlugFn: func(slug string) (*domain.Campaign, error) {
			return &domain.Campaign{Slug: slug, Enabled: true}, nil
		},
		listAllFn: func(enabled *bool, country *string) ([]*domain.Campaign, error) {
			return []*domain.Campaign{{Slug: "a"}, {Slug: "b"}}, nil
		},
		createFn: func(c *domain.Campaign) (*domain.Campaign, error) {
			if c == nil || c.Slug == "" {
				return nil, errors.New("invalid create")
			}
			return &domain.Campaign{Slug: c.Slug, Enabled: c.Enabled}, nil
		},
		updateFn: func(slug string, c *domain.Campaign) (*domain.Campaign, error) {
			if slug == "" || c == nil {
				return nil, errors.New("invalid update")
			}
			return &domain.Campaign{Slug: slug, Enabled: c.Enabled}, nil
		},
		setEnabledFn: func(slug string, enabled bool, updatedBy *string) (*domain.Campaign, error) {
			if slug == "" {
				return nil, errors.New("invalid slug")
			}
			return &domain.Campaign{Slug: slug, Enabled: enabled, UpdatedBy: updatedBy}, nil
		},
		// not used by this test:
	}

	svc := NewCampaignService(repo, logger)

	created, err := svc.AdminCreate(&domain.Campaign{Slug: "test", Enabled: true})
	if err != nil {
		t.Fatalf("AdminCreate error: %v", err)
	}
	if created.Slug != "test" {
		t.Fatalf("expected created slug 'test', got %q", created.Slug)
	}

	got, err := svc.AdminGetBySlug("test")
	if err != nil {
		t.Fatalf("AdminGetBySlug error: %v", err)
	}
	if got.Slug != "test" {
		t.Fatalf("expected slug 'test', got %q", got.Slug)
	}

	list, err := svc.AdminList(nil, nil)
	if err != nil {
		t.Fatalf("AdminList error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 campaigns, got %d", len(list))
	}

	updated, err := svc.AdminUpdate("test", &domain.Campaign{Enabled: false})
	if err != nil {
		t.Fatalf("AdminUpdate error: %v", err)
	}
	if updated.Enabled != false {
		t.Fatalf("expected enabled=false, got %v", updated.Enabled)
	}

	user := "admin"
	toggled, err := svc.AdminSetEnabled("test", true, &user)
	if err != nil {
		t.Fatalf("AdminSetEnabled error: %v", err)
	}
	if toggled.Enabled != true {
		t.Fatalf("expected enabled=true, got %v", toggled.Enabled)
	}
	if toggled.UpdatedBy == nil || *toggled.UpdatedBy != "admin" {
		t.Fatalf("expected UpdatedBy=admin, got %#v", toggled.UpdatedBy)
	}
}

func TestCampaignService_AdminClone_CopiesConfigurationAndResetsState(t *testing.T) {
	logger := zap.NewNop()

	operator := "AT"
	pricepointID := 77
	partnerRoleID := 12
	shortCode := "601061"
	smsKeyword := "JOIN"
	price := 1.5
	billingCycle := "daily"
	termsURL := "https://example.com/terms"
	inlineTermsText := "terms"
	consentVersion := "v1"
	sourceCreatedBy := "source-admin"
	createdBy := "clone-admin"

	source := &domain.Campaign{
		Slug:               "source-campaign",
		Language:           "en",
		Country:            "GH",
		Operator:           &operator,
		OfferProductID:     27188,
		PricepointID:       &pricepointID,
		PartnerRoleID:      &partnerRoleID,
		FlowType:           domain.FlowTypeOTP,
		ShortCode:          &shortCode,
		SMSKeyword:         &smsKeyword,
		Price:              &price,
		BillingCycle:       &billingCycle,
		TrialFlags:         []byte(`{"trial_days":1}`),
		TermsURL:           &termsURL,
		InlineTermsText:    &inlineTermsText,
		ConsentRequired:    true,
		ConsentVersion:     &consentVersion,
		AttributionMapping: []byte(`{"click_id":"txid"}`),
		PostbackRules:      []byte(`{"subscribed":{"generic":{"url":"https://example.com"}}}`),
		Throttles:          []byte(`{"per_msisdn_per_day":3}`),
		AllowedReferrers:   []string{"https://affiliate.example"},
		AllowedSources:     []string{"facebook"},
		LandingPageURLs:    []string{"https://landing.example/lp/source-campaign"},
		TrackingConfig:     []byte(`{"redirect_url":"https://partner.example/subscribe"}`),
		LPCopy:             []byte(`{"en":{"heroTitle":"A","heDescription":"B","heCta":"C","heModalTitle":"D","heModalConfirm":"E","msisdnDescription":"F","msisdnPlaceholder":"G","msisdnCta":"H","otpDescription":"I","otpPlaceholder":"J","otpCta":"K","successTitle":"L","successBody":"M","consentPrefix":"N","consentTerms":"O","termsHeading":"P","legal":"Q","phoneRequired":"R","phoneInvalid":"S","otpInvalid":"T","consentRequired":"U"}}`),
		Enabled:            true,
		CreatedBy:          &sourceCreatedBy,
	}

	repo := &fakeCampaignRepo{
		getAdminBySlugFn: func(slug string) (*domain.Campaign, error) {
			if slug != "source-campaign" {
				return nil, errors.New("unexpected source slug")
			}
			return source, nil
		},
		createFn: func(c *domain.Campaign) (*domain.Campaign, error) {
			if c.Slug != "copied-campaign" {
				return nil, errors.New("clone slug mismatch")
			}
			if c.Enabled {
				return nil, errors.New("clone must be disabled")
			}
			if c.CreatedBy == nil || *c.CreatedBy != createdBy {
				return nil, errors.New("created_by must be set from clone request")
			}
			if c.UpdatedBy == nil || *c.UpdatedBy != createdBy {
				return nil, errors.New("updated_by must be set from clone request")
			}
			if !reflect.DeepEqual(c.AllowedReferrers, source.AllowedReferrers) {
				return nil, errors.New("allowed_referrers not copied")
			}
			if !reflect.DeepEqual(c.LandingPageURLs, source.LandingPageURLs) {
				return nil, errors.New("landing_page_urls not copied")
			}
			if string(c.TrackingConfig) != string(source.TrackingConfig) {
				return nil, errors.New("tracking_config not copied")
			}
			return c, nil
		},
		// not used by this test
		listAllFn:    func(*bool, *string) ([]*domain.Campaign, error) { return nil, errors.New("unused") },
		updateFn:     func(string, *domain.Campaign) (*domain.Campaign, error) { return nil, errors.New("unused") },
		setEnabledFn: func(string, bool, *string) (*domain.Campaign, error) { return nil, errors.New("unused") },
	}

	svc := NewCampaignService(repo, logger)
	cloned, err := svc.AdminClone("source-campaign", "copied-campaign", &createdBy)
	if err != nil {
		t.Fatalf("AdminClone error: %v", err)
	}
	if cloned.Slug != "copied-campaign" {
		t.Fatalf("expected cloned slug copied-campaign, got %q", cloned.Slug)
	}
	if cloned.Enabled {
		t.Fatalf("expected cloned campaign disabled, got enabled=%v", cloned.Enabled)
	}
}

func TestCampaignService_AdminClone_SourceNotFound(t *testing.T) {
	logger := zap.NewNop()

	repo := &fakeCampaignRepo{
		getAdminBySlugFn: func(slug string) (*domain.Campaign, error) {
			return nil, errors.New("campaign not found: " + slug)
		},
		// not used by this test
		listAllFn:    func(*bool, *string) ([]*domain.Campaign, error) { return nil, errors.New("unused") },
		createFn:     func(*domain.Campaign) (*domain.Campaign, error) { return nil, errors.New("unused") },
		updateFn:     func(string, *domain.Campaign) (*domain.Campaign, error) { return nil, errors.New("unused") },
		setEnabledFn: func(string, bool, *string) (*domain.Campaign, error) { return nil, errors.New("unused") },
	}

	svc := NewCampaignService(repo, logger)
	_, err := svc.AdminClone("missing-campaign", "copy-campaign", nil)
	if err == nil || !strings.Contains(err.Error(), "failed to get source campaign") {
		t.Fatalf("expected source lookup error, got %v", err)
	}
}

func TestCampaignService_AdminCreate_RejectsInvalidOfferMapping(t *testing.T) {
	logger := zap.NewNop()
	createCalled := false

	repo := &fakeCampaignRepo{
		validateOfferFn: func(offerProductID int, pricepointID *int) error {
			return errors.New("offer_product_id 9999 is not present in products mapping")
		},
		createFn: func(*domain.Campaign) (*domain.Campaign, error) {
			createCalled = true
			return nil, errors.New("should not be called")
		},
		// not used by this test
		getAdminBySlugFn: func(string) (*domain.Campaign, error) { return nil, errors.New("unused") },
		listAllFn:        func(*bool, *string) ([]*domain.Campaign, error) { return nil, errors.New("unused") },
		updateFn:         func(string, *domain.Campaign) (*domain.Campaign, error) { return nil, errors.New("unused") },
		setEnabledFn:     func(string, bool, *string) (*domain.Campaign, error) { return nil, errors.New("unused") },
	}

	svc := NewCampaignService(repo, logger)
	_, err := svc.AdminCreate(&domain.Campaign{
		Slug:           "test-campaign",
		OfferProductID: 9999,
	})
	if err == nil || !strings.Contains(err.Error(), "invalid campaign offer mapping") {
		t.Fatalf("expected mapping validation error, got %v", err)
	}
	if createCalled {
		t.Fatal("expected create not to be called when mapping validation fails")
	}
}

func TestCampaignService_AdminCreateForTenantPersistsTenantAndChannel(t *testing.T) {
	logger := zap.NewNop()
	channelID := "33333333-3333-3333-3333-333333333333"
	tenantID := "22222222-2222-2222-2222-222222222222"

	repo := &fakeCampaignRepo{
		validateTenantOfferFn: func(gotTenantID string, offerProductID int, pricepointID *int) error {
			if gotTenantID != tenantID || offerProductID != 27188 {
				t.Fatalf("unexpected tenant offer validation: tenant=%q offer=%d", gotTenantID, offerProductID)
			}
			return nil
		},
		validateTenantChannelFn: func(gotTenantID, gotChannelID, country string, operator *string, flowType domain.FlowType) error {
			if gotTenantID != tenantID || gotChannelID != channelID || country != "GH" || flowType != domain.FlowTypeOTP {
				t.Fatalf("unexpected channel validation: tenant=%q channel=%q country=%q flow=%q", gotTenantID, gotChannelID, country, flowType)
			}
			return nil
		},
		createForTenantFn: func(gotTenantID string, c *domain.Campaign) (*domain.Campaign, error) {
			if gotTenantID != tenantID {
				t.Fatalf("expected tenant %q, got %q", tenantID, gotTenantID)
			}
			if c.TenantID == nil || *c.TenantID != tenantID {
				t.Fatalf("expected campaign tenant_id to be set, got %#v", c.TenantID)
			}
			if c.ChannelID == nil || *c.ChannelID != channelID {
				t.Fatalf("expected campaign channel_id to be set, got %#v", c.ChannelID)
			}
			return c, nil
		},
	}

	svc := NewCampaignService(repo, logger)
	created, err := svc.AdminCreateForTenant(tenantID, &domain.Campaign{
		Slug:           "daily",
		ChannelID:      &channelID,
		Country:        "GH",
		OfferProductID: 27188,
		FlowType:       domain.FlowTypeOTP,
	})
	if err != nil {
		t.Fatalf("expected tenant campaign create to pass, got %v", err)
	}
	if created.Slug != "daily" {
		t.Fatalf("expected created campaign daily, got %q", created.Slug)
	}
}

func TestCampaignService_AdminCreateForTenantRejectsTenantProductMismatch(t *testing.T) {
	logger := zap.NewNop()
	createCalled := false
	channelID := "33333333-3333-3333-3333-333333333333"

	repo := &fakeCampaignRepo{
		validateTenantOfferFn: func(string, int, *int) error {
			return errors.New("offer_product_id 9999 is not present in tenant products")
		},
		validateTenantChannelFn: func(string, string, string, *string, domain.FlowType) error {
			return nil
		},
		createForTenantFn: func(string, *domain.Campaign) (*domain.Campaign, error) {
			createCalled = true
			return nil, errors.New("should not be called")
		},
	}

	svc := NewCampaignService(repo, logger)
	_, err := svc.AdminCreateForTenant("tenant-a", &domain.Campaign{
		Slug:           "daily",
		ChannelID:      &channelID,
		Country:        "GH",
		OfferProductID: 9999,
		FlowType:       domain.FlowTypeOTP,
	})
	if err == nil || !strings.Contains(err.Error(), "invalid campaign offer mapping") {
		t.Fatalf("expected tenant product validation error, got %v", err)
	}
	if createCalled {
		t.Fatal("expected create not to be called")
	}
}

func TestCampaignService_AdminCreateForTenantMapsChannelCapabilityMismatch(t *testing.T) {
	logger := zap.NewNop()
	channelID := "33333333-3333-3333-3333-333333333333"
	repo := &fakeCampaignRepo{
		validateTenantOfferFn: func(string, int, *int) error {
			return nil
		},
		validateTenantChannelFn: func(string, string, string, *string, domain.FlowType) error {
			return errors.New("channel_capability_mismatch: missing confirm")
		},
		createForTenantFn: func(string, *domain.Campaign) (*domain.Campaign, error) {
			return nil, errors.New("should not be called")
		},
	}

	svc := NewCampaignService(repo, logger)
	_, err := svc.AdminCreateForTenant("tenant-a", &domain.Campaign{
		Slug:           "daily",
		ChannelID:      &channelID,
		Country:        "GH",
		OfferProductID: 27188,
		FlowType:       domain.FlowTypeOTP,
	})
	if !errors.Is(err, ErrCampaignChannelCapabilityMismatch) {
		t.Fatalf("expected channel capability mismatch, got %v", err)
	}
}

func TestCampaignService_AdminCreateForTenantMapsDuplicateSlugConflict(t *testing.T) {
	logger := zap.NewNop()
	channelID := "33333333-3333-3333-3333-333333333333"
	repo := &fakeCampaignRepo{
		validateTenantOfferFn: func(string, int, *int) error {
			return nil
		},
		validateTenantChannelFn: func(string, string, string, *string, domain.FlowType) error {
			return nil
		},
		createForTenantFn: func(string, *domain.Campaign) (*domain.Campaign, error) {
			return nil, errors.New(`pq: duplicate key value violates unique constraint "idx_campaigns_tenant_slug"`)
		},
	}

	svc := NewCampaignService(repo, logger)
	_, err := svc.AdminCreateForTenant("tenant-a", &domain.Campaign{
		Slug:           "daily",
		ChannelID:      &channelID,
		Country:        "GH",
		OfferProductID: 27188,
		FlowType:       domain.FlowTypeOTP,
	})
	if !errors.Is(err, ErrCampaignConflict) {
		t.Fatalf("expected campaign conflict, got %v", err)
	}
}
