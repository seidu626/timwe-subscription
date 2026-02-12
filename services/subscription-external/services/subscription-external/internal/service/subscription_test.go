package service

import (
	"context"
	"time"

	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
)

// MockUserBaseRepository is a simple mock implementation for testing
type MockUserBaseRepository struct {
	isStaffResult bool
	isStaffError  error
}

func (m *MockUserBaseRepository) IsPremierOrStaff(msisdn string) (bool, error) {
	return m.isStaffResult, m.isStaffError
}

func (m *MockUserBaseRepository) FilterMSISDNS(msisdns []string) ([]string, error) {
	// Simple implementation for testing
	if m.isStaffError != nil {
		return nil, m.isStaffError
	}

	var filtered []string
	for _, msisdn := range msisdns {
		if !m.isStaffResult {
			filtered = append(filtered, msisdn)
		}
	}
	return filtered, nil
}

func (m *MockUserBaseRepository) LoadExclusionList() (map[string]bool, error) {
	return make(map[string]bool), nil
}

func (m *MockUserBaseRepository) GetExistingMSISDNS(ctx context.Context, msisdns []string) ([]string, error) {
	return []string{}, nil
}

func (m *MockUserBaseRepository) InsertUserRecords(ctx context.Context, records []*domain.UserBase) error {
	return nil
}

func (m *MockUserBaseRepository) GetInvalidMSISDNS(ctx context.Context, msisdns []string) ([]string, error) {
	// Simple implementation for testing - return empty list
	return []string{}, nil
}

// MockSubscriptionRepository is a simple mock implementation for testing
type MockSubscriptionRepository struct {
	subscriptionExists bool
	renewalExists      bool
	subscriptionError  error
	renewalError       error
	createError        error
	notificationError  error
	invalidMSISDNError error
}

func (m *MockSubscriptionRepository) FetchActiveMsisdnsWithoutProductsWindow(productIds []int, offset int, limit int) ([]string, error) {
	return []string{"233200000001", "233200000002"}, nil
}

// Add new method to satisfy interface
func (m *MockSubscriptionRepository) FetchActiveMsisdnsWithProductsWindow(productIds []int, offset int, limit int) ([]string, error) {
	return []string{"233200000001", "233200000002"}, nil
}

func (m *MockSubscriptionRepository) CreateSubscription(request *domain.SubscriptionRequest) error {
	return m.createError
}

func (m *MockSubscriptionRepository) CreateNotification(notification *domain.NotificationRequest) error {
	return m.notificationError
}

func (m *MockSubscriptionRepository) CreateInvalidMSISDNLog(log *domain.InvalidMSISDNLog) error {
	return m.invalidMSISDNError
}

func (m *MockSubscriptionRepository) CheckSubscriptionExists(msisdn string, productId int) (bool, error) {
	return m.subscriptionExists, m.subscriptionError
}

func (m *MockSubscriptionRepository) CheckRenewalNotificationExists(msisdn string, productId int) (bool, error) {
	return m.renewalExists, m.renewalError
}

func (m *MockSubscriptionRepository) GenerateCacheKey(startDate, endDate time.Time, productId int, shortcode, userIdentifier, entryChannel string, page, pageSize int) string {
	return ""
}

// ... existing tests remain unchanged ...
