package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/seidu626/subscription-manager/subscription/internal/domain"
)

type serviceRepoStub struct {
	fetchFn              func(tenantID, tenantKey string, startDate, endDate time.Time, productID int, shortcode, userIdentifier, entryChannel, sortBy, sortDir string, page, pageSize int) (*domain.ListResponse, error)
	createNotificationFn func(notification *domain.NotificationRequest) error
}

func (s *serviceRepoStub) FetchSubscriptions(tenantID, tenantKey string, startDate, endDate time.Time, productID int, shortcode, userIdentifier, entryChannel, sortBy, sortDir string, page, pageSize int) (*domain.ListResponse, error) {
	if s.fetchFn != nil {
		return s.fetchFn(tenantID, tenantKey, startDate, endDate, productID, shortcode, userIdentifier, entryChannel, sortBy, sortDir, page, pageSize)
	}
	return &domain.ListResponse{}, nil
}

func (s *serviceRepoStub) ConfirmSubscription(request *domain.SubscriptionConfirmationRequest) error {
	return nil
}

func (s *serviceRepoStub) CreateSubscription(request *domain.SubscriptionRequest) error {
	return nil
}

func (s *serviceRepoStub) CreateNotification(notification *domain.NotificationRequest) error {
	if s.createNotificationFn != nil {
		return s.createNotificationFn(notification)
	}
	return nil
}

func (s *serviceRepoStub) OptOutSubscription(request *domain.UnsubscriptionRequest) error {
	return nil
}

func (s *serviceRepoStub) GetSubscriptionStatus(request *domain.GetStatusRequest) (*domain.SubscriptionStatus, error) {
	return nil, nil
}

func TestGetSubscriptions_DefaultPaginationAndErrorContext(t *testing.T) {
	rootErr := errors.New("db offline")
	stub := &serviceRepoStub{
		fetchFn: func(tenantID, tenantKey string, startDate, endDate time.Time, productID int, shortcode, userIdentifier, entryChannel, sortBy, sortDir string, page, pageSize int) (*domain.ListResponse, error) {
			if page != 1 || pageSize != 10 {
				t.Fatalf("expected default pagination page=1 pageSize=10, got page=%d pageSize=%d", page, pageSize)
			}
			if tenantKey != "nrg" {
				t.Fatalf("expected tenant key to be forwarded, got %q", tenantKey)
			}
			return nil, rootErr
		},
	}

	svc := NewSubscriptionService(stub, nil)
	_, err := svc.GetSubscriptions(map[string]string{
		"tenantKey": "nrg",
		"page":      "0",
		"pageSize":  "0",
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

func TestGetSubscriptions_ParsesDateFilters(t *testing.T) {
	stub := &serviceRepoStub{
		fetchFn: func(tenantID, tenantKey string, startDate, endDate time.Time, productID int, shortcode, userIdentifier, entryChannel, sortBy, sortDir string, page, pageSize int) (*domain.ListResponse, error) {
			if startDate.IsZero() {
				t.Fatalf("expected parsed startDate")
			}
			if endDate.IsZero() {
				t.Fatalf("expected parsed endDate")
			}
			if !endDate.After(startDate) {
				t.Fatalf("expected endDate after startDate, got start=%s end=%s", startDate, endDate)
			}
			return &domain.ListResponse{}, nil
		},
	}

	svc := NewSubscriptionService(stub, nil)
	_, err := svc.GetSubscriptions(map[string]string{
		"startDate": "2026-02-20T00:00:00Z",
		"endDate":   "2026-02-20",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestProcessNotification_ValidatesAndPersists(t *testing.T) {
	var captured *domain.NotificationRequest
	svc := NewSubscriptionService(&serviceRepoStub{
		createNotificationFn: func(notification *domain.NotificationRequest) error {
			captured = notification
			return nil
		},
	}, nil)

	if err := svc.ProcessNotification(nil); err == nil {
		t.Fatal("expected error for nil notification")
	}
	if err := svc.ProcessNotification(&domain.NotificationRequest{Type: "CHARGE"}); err == nil {
		t.Fatal("expected error for missing msisdn")
	}

	req := &domain.NotificationRequest{
		Type:   "CHARGE",
		MSISDN: "233241234567",
	}
	if err := svc.ProcessNotification(req); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if captured == nil || captured.Type != "CHARGE" {
		t.Fatalf("expected notification to be persisted, got %+v", captured)
	}
}
