package service

import (
	"errors"
	"testing"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/domain"
	"go.uber.org/zap"
)

type fakeCampaignRepo struct {
	getBySlugFn      func(string) (*domain.Campaign, error)
	listEnabledFn    func() ([]*domain.Campaign, error)
	getAdminBySlugFn func(string) (*domain.Campaign, error)
	listAllFn        func(*bool, *string) ([]*domain.Campaign, error)
	createFn         func(*domain.Campaign) (*domain.Campaign, error)
	updateFn         func(string, *domain.Campaign) (*domain.Campaign, error)
	setEnabledFn     func(string, bool, *string) (*domain.Campaign, error)
}

func (f *fakeCampaignRepo) GetBySlug(slug string) (*domain.Campaign, error) {
	return f.getBySlugFn(slug)
}
func (f *fakeCampaignRepo) ListEnabled() ([]*domain.Campaign, error) {
	return f.listEnabledFn()
}
func (f *fakeCampaignRepo) GetAdminBySlug(slug string) (*domain.Campaign, error) {
	return f.getAdminBySlugFn(slug)
}
func (f *fakeCampaignRepo) ListAll(enabled *bool, country *string) ([]*domain.Campaign, error) {
	return f.listAllFn(enabled, country)
}
func (f *fakeCampaignRepo) Create(c *domain.Campaign) (*domain.Campaign, error) {
	return f.createFn(c)
}
func (f *fakeCampaignRepo) Update(slug string, c *domain.Campaign) (*domain.Campaign, error) {
	return f.updateFn(slug, c)
}
func (f *fakeCampaignRepo) SetEnabled(slug string, enabled bool, updatedBy *string) (*domain.Campaign, error) {
	return f.setEnabledFn(slug, enabled, updatedBy)
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
		getBySlugFn:   func(string) (*domain.Campaign, error) { return nil, errors.New("unused") },
		listEnabledFn: func() ([]*domain.Campaign, error) { return nil, errors.New("unused") },
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

