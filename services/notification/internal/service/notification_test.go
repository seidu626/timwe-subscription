package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/seidu626/subscription-manager/notification/internal/domain"
)

type serviceRepoStub struct {
	fetchFn func(startDate, endDate time.Time, tenantID, channelID, partnerRole, msisdn, entryChannel, notificationType, sortBy, sortDir string, page, pageSize int) (*domain.ListResponse, error)
}

func (s *serviceRepoStub) FetchNotifications(startDate, endDate time.Time, tenantID, channelID, partnerRole, msisdn, entryChannel, notificationType, sortBy, sortDir string, page, pageSize int) (*domain.ListResponse, error) {
	if s.fetchFn != nil {
		return s.fetchFn(startDate, endDate, tenantID, channelID, partnerRole, msisdn, entryChannel, notificationType, sortBy, sortDir, page, pageSize)
	}
	return &domain.ListResponse{}, nil
}

func (s *serviceRepoStub) Save(notification *domain.NotificationRequest) error {
	return nil
}

func (s *serviceRepoStub) TenantIDByKey(_ context.Context, tenantKey string) (string, error) {
	return strings.TrimSpace(tenantKey), nil
}

func (s *serviceRepoStub) ChannelIDByKeys(_ context.Context, tenantID, channelKey string) (string, error) {
	return strings.TrimSpace(channelKey) + "-uuid", nil
}

func TestGetNotifications_DefaultPaginationAndErrorContext(t *testing.T) {
	rootErr := errors.New("db offline")
	stub := &serviceRepoStub{
		fetchFn: func(startDate, endDate time.Time, tenantID, channelID, partnerRole, msisdn, entryChannel, notificationType, sortBy, sortDir string, page, pageSize int) (*domain.ListResponse, error) {
			if page != 1 || pageSize != 10 {
				t.Fatalf("expected default page=1 pageSize=10, got page=%d pageSize=%d", page, pageSize)
			}
			return nil, rootErr
		},
	}

	svc := NewNotificationService(stub)
	_, err := svc.GetNotifications(map[string]string{
		"page":     "0",
		"pageSize": "0",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "page=1 pageSize=10") {
		t.Fatalf("expected pagination context in error, got: %v", err)
	}
	if !errors.Is(err, rootErr) {
		t.Fatalf("expected wrapped root error, got: %v", err)
	}
}

func TestGetNotifications_ParsesDateAndTypeFilters(t *testing.T) {
	stub := &serviceRepoStub{
		fetchFn: func(startDate, endDate time.Time, tenantID, channelID, partnerRole, msisdn, entryChannel, notificationType, sortBy, sortDir string, page, pageSize int) (*domain.ListResponse, error) {
			if startDate.IsZero() {
				t.Fatalf("expected parsed startDate")
			}
			if endDate.IsZero() {
				t.Fatalf("expected parsed endDate")
			}
			if notificationType != "USER_OPTIN" {
				t.Fatalf("expected type USER_OPTIN, got %q", notificationType)
			}
			if entryChannel != "WEB" {
				t.Fatalf("expected entryChannel WEB, got %q", entryChannel)
			}
			if tenantID != "tenant-1" || channelID != "channel-1" {
				t.Fatalf("expected tenant/channel filters, got tenant=%q channel=%q", tenantID, channelID)
			}
			return &domain.ListResponse{}, nil
		},
	}

	svc := NewNotificationService(stub)
	_, err := svc.GetNotifications(map[string]string{
		"startDate":    "2026-02-20T00:00:00Z",
		"endDate":      "2026-02-20",
		"tenantId":     "tenant-1",
		"channelId":    "channel-1",
		"type":         "USER_OPTIN",
		"entryChannel": "WEB",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
