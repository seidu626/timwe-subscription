package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/seidu626/subscription-manager/subscription-external/internal/monitoring"
	"github.com/seidu626/subscription-manager/subscription-external/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// MockSubscriptionRepositoryForInvalidMSISDN is a mock implementation for testing INVALID_MSISDN functionality
type MockSubscriptionRepositoryForInvalidMSISDN struct {
	mock.Mock
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) CreateInvalidMSISDNLog(log *domain.InvalidMSISDNLog) error {
	args := m.Called(log)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) CheckSubscriptionExists(msisdn string, productId int) (bool, error) {
	args := m.Called(msisdn, productId)
	return args.Bool(0), args.Error(1)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) CheckRenewalNotificationExists(msisdn string, productId int) (bool, error) {
	args := m.Called(msisdn, productId)
	return args.Bool(0), args.Error(1)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) HasAnySubscription(msisdn string) (bool, error) {
	args := m.Called(msisdn)
	return args.Bool(0), args.Error(1)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) DeleteSubscriptionRecord(msisdn string) error {
	args := m.Called(msisdn)
	return args.Error(0)
}

// Mock other methods as needed...
func (m *MockSubscriptionRepositoryForInvalidMSISDN) CreateSubscription(request *domain.SubscriptionRequest) error {
	args := m.Called(request)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) CreateNotification(notification *domain.NotificationRequest) error {
	args := m.Called(notification)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) GenerateCacheKey(startDate, endDate time.Time, productId int, shortcode, userIdentifier, entryChannel string, page, pageSize int) string {
	args := m.Called(startDate, endDate, productId, shortcode, userIdentifier, entryChannel, page, pageSize)
	return args.String(0)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) FetchActiveMsisdnsMissingSomeProducts(productIds []int, offset int, limit int) ([]string, error) {
	args := m.Called(productIds, offset, limit)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) FetchActiveMsisdnsWithProductsWindow(productIds []int, offset int, limit int) ([]string, error) {
	args := m.Called(productIds, offset, limit)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) FetchNotificationsWindow(ntype string, since time.Time, afterId int64, limit int) ([]repository.NotificationRow, error) {
	args := m.Called(ntype, since, afterId, limit)
	return args.Get(0).([]repository.NotificationRow), args.Error(1)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) FetchSubscriptionsNeedingRenewal(cutoff time.Time, afterId int64, limit int) ([]repository.NotificationRow, error) {
	args := m.Called(cutoff, afterId, limit)
	return args.Get(0).([]repository.NotificationRow), args.Error(1)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) UpsertSubscriptionStatus(msisdn string, productId int, status string) error {
	args := m.Called(msisdn, productId, status)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) FetchUnprocessedOptoutNotifications(since time.Time, afterId int64, limit int) ([]repository.NotificationRow, error) {
	args := m.Called(since, afterId, limit)
	return args.Get(0).([]repository.NotificationRow), args.Error(1)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) GetSubscriptionByMSISDNAndProduct(msisdn string, productID int) (*domain.Subscription, error) {
	args := m.Called(msisdn, productID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Subscription), args.Error(1)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) GetLastOptinNotificationTime(msisdn string, productID int) (*time.Time, error) {
	args := m.Called(msisdn, productID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*time.Time), args.Error(1)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) FetchGhostSubscriptions(cutoff time.Time, afterId int64, limit int) ([]repository.NotificationRow, error) {
	args := m.Called(cutoff, afterId, limit)
	return args.Get(0).([]repository.NotificationRow), args.Error(1)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) FetchChargingFailedSubscriptions(filter repository.ChargingFailureFilter) ([]repository.ChargingFailedSubscription, error) {
	args := m.Called(filter)
	return args.Get(0).([]repository.ChargingFailedSubscription), args.Error(1)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) FindAndRemoveSubscription(msisdn string, productId int) error {
	args := m.Called(msisdn, productId)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) GetChargingFailureByMSISDN(msisdn string, productID int) (*repository.ChargingFailedSubscription, error) {
	args := m.Called(msisdn, productID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.ChargingFailedSubscription), args.Error(1)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) GetChargingFailureCount(filter repository.ChargingFailureFilter) (int64, error) {
	args := m.Called(filter)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) GetChargingFailureStats() (map[string]interface{}, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) GetChargingFailureSummary() (map[string]interface{}, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) UpdateChargingHealthStatus(subscriptionID int, status string, reason string) error {
	args := m.Called(subscriptionID, status, reason)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) MarkChargingFailureAsProcessed(subscriptionID int, status string) error {
	args := m.Called(subscriptionID, status)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) GetTotalSubscriptionsCount() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) GetSubscription(msisdn string, productID string) (*domain.SubscriptionWithRenewalInfo, error) {
	args := m.Called(msisdn, productID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SubscriptionWithRenewalInfo), args.Error(1)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) GetLastSuccessfulPayment(msisdn string, productID string) (*time.Time, error) {
	args := m.Called(msisdn, productID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*time.Time), args.Error(1)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) GetRenewalAttemptsCount(msisdn string, productID string, since time.Time) (int, error) {
	args := m.Called(msisdn, productID, since)
	return args.Int(0), args.Error(1)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) GetDailyChurnCount(date time.Time) (int, error) {
	args := m.Called(date)
	return args.Int(0), args.Error(1)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) GetLastRenewalAttempt(msisdn string, productID string) (*time.Time, error) {
	args := m.Called(msisdn, productID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*time.Time), args.Error(1)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) ChurnSubscription(msisdn string, productID string, reason string, churnTime time.Time) error {
	args := m.Called(msisdn, productID, reason, churnTime)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) CreateChurnRecord(record *domain.ChurnRecord) error {
	args := m.Called(record)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) SaveRenewalCycle(cycle *domain.RenewalCycle) error {
	args := m.Called(cycle)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) UpdateSubscriptionStatus(msisdn string, productID string, status string) error {
	args := m.Called(msisdn, productID, status)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) AddToPriorityRetryQueue(item *domain.PriorityRetryQueue) error {
	args := m.Called(item)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) IncrementRenewalAttempt(msisdn string, productID string) error {
	args := m.Called(msisdn, productID)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) GetSubscriptionsNeedingRenewal(hoursThreshold int, limit int) ([]*domain.SubscriptionWithRenewalInfo, error) {
	args := m.Called(hoursThreshold, limit)
	return args.Get(0).([]*domain.SubscriptionWithRenewalInfo), args.Error(1)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) SaveRenewalMetrics(metrics *domain.RenewalMetrics) error {
	args := m.Called(metrics)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) GetDuePriorityRetryItems(limit int) ([]*domain.PriorityRetryQueue, error) {
	args := m.Called(limit)
	return args.Get(0).([]*domain.PriorityRetryQueue), args.Error(1)
}

func (m *MockSubscriptionRepositoryForInvalidMSISDN) UpdatePriorityRetryItem(item *domain.PriorityRetryQueue) error {
	args := m.Called(item)
	return args.Error(0)
}

func TestDetectAndLogInvalidMSISDNEnhanced(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := &MockSubscriptionRepositoryForInvalidMSISDN{}

	service := &SubscriptionService{
		logger: logger,
		repo:   mockRepo,
	}

	// Test case 1: INVALID_MSISDN in response code
	response := &domain.MTResponse{
		Code:    "INVALID_MSISDN",
		Message: "Invalid MSISDN",
		ResponseData: map[string]interface{}{
			"subscriptionResult": "null",
			"subscriptionError":  "null",
		},
		RequestID: "test-request-id",
	}

	mtReq := domain.MTRequest{
		ProductID:          123,
		PricepointID:       456,
		UserIdentifier:     "233123456789",
		UserIdentifierType: "MSISDN",
		EntryChannel:       "WEB",
		MoTransactionUUID:  "test-tx-id",
	}

	partnerId := 789

	// Set up mock expectations
	mockRepo.On("CreateInvalidMSISDNLog", mock.AnythingOfType("*domain.InvalidMSISDNLog")).Return(nil)
	mockRepo.On("HasAnySubscription", mock.Anything).Return(false, nil)

	// This should trigger the logging and async cleanup
	service.detectAndLogInvalidMSISDN(response, mtReq, partnerId)

	// Wait a bit for the async cleanup to complete
	time.Sleep(100 * time.Millisecond)

	// Verify that the mock was called correctly
	mockRepo.AssertExpectations(t)
}

func TestHandleInvalidMSISDNCleanup(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	msisdn := "233123456789"
	productId := 123
	requestID := "test-request-id"

	// Test case 1: Subscription exists and deletion succeeds
	t.Run("subscription exists and deletion succeeds", func(t *testing.T) {
		mockRepo := &MockSubscriptionRepositoryForInvalidMSISDN{}
		service := &SubscriptionService{
			logger: logger,
			repo:   mockRepo,
		}

		mockRepo.On("HasAnySubscription", msisdn).Return(true, nil)
		mockRepo.On("DeleteSubscriptionRecord", msisdn).Return(nil)

		service.handleInvalidMSISDNCleanup(msisdn, productId, requestID)

		// Wait for completion
		time.Sleep(100 * time.Millisecond)

		mockRepo.AssertExpectations(t)
	})

	// Test case 2: Subscription doesn't exist
	t.Run("subscription does not exist", func(t *testing.T) {
		mockRepo := &MockSubscriptionRepositoryForInvalidMSISDN{}
		service := &SubscriptionService{
			logger: logger,
			repo:   mockRepo,
		}

		mockRepo.On("HasAnySubscription", msisdn).Return(false, nil)

		service.handleInvalidMSISDNCleanup(msisdn, productId, requestID)

		// Wait for completion
		time.Sleep(100 * time.Millisecond)

		mockRepo.AssertExpectations(t)
	})

	// Test case 3: Deletion fails multiple times
	t.Run("deletion fails multiple times", func(t *testing.T) {
		mockRepo := &MockSubscriptionRepositoryForInvalidMSISDN{}
		service := &SubscriptionService{
			logger: logger,
			repo:   mockRepo,
		}

		mockRepo.On("HasAnySubscription", msisdn).Return(true, nil)
		mockRepo.On("DeleteSubscriptionRecord", msisdn).Return(fmt.Errorf("database error")).Times(3)

		service.handleInvalidMSISDNCleanup(msisdn, productId, requestID)

		// Wait for completion (including retries)
		time.Sleep(500 * time.Millisecond)

		mockRepo.AssertExpectations(t)
	})
}

func TestBatchHandleInvalidMSISDNs(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := &MockSubscriptionRepositoryForInvalidMSISDN{}

	service := &SubscriptionService{
		logger: logger,
		repo:   mockRepo,
	}

	// Create test data
	responses := []*domain.MTResponse{
		{
			Code:    "INVALID_MSISDN",
			Message: "Invalid MSISDN",
			ResponseData: map[string]interface{}{
				"subscriptionResult": "INVALID_MSISDN",
			},
			RequestID: "test-request-id-1",
		},
		{
			Code:    "SUCCESS",
			Message: "Success",
			ResponseData: map[string]interface{}{
				"subscriptionResult": "OPTIN_ALREADY_ACTIVE",
			},
			RequestID: "test-request-id-2",
		},
		{
			Code:    "INVALID_MSISDN",
			Message: "Invalid MSISDN",
			ResponseData: map[string]interface{}{
				"subscriptionError": "Invalid MSISDN",
			},
			RequestID: "test-request-id-3",
		},
	}

	requests := []domain.MTRequest{
		{
			ProductID:          123,
			PricepointID:       456,
			UserIdentifier:     "233123456789",
			UserIdentifierType: "MSISDN",
			EntryChannel:       "WEB",
			MoTransactionUUID:  "test-tx-id-1",
		},
		{
			ProductID:          124,
			PricepointID:       457,
			UserIdentifier:     "233123456790",
			UserIdentifierType: "MSISDN",
			EntryChannel:       "SMS",
			MoTransactionUUID:  "test-tx-id-2",
		},
		{
			ProductID:          125,
			PricepointID:       458,
			UserIdentifier:     "233123456791",
			UserIdentifierType: "MSISDN",
			EntryChannel:       "WEB",
			MoTransactionUUID:  "test-tx-id-3",
		},
	}

	partnerId := 789

	// Set up mock expectations for the two invalid MSISDNs
	mockRepo.On("CreateInvalidMSISDNLog", mock.AnythingOfType("*domain.InvalidMSISDNLog")).Return(nil).Times(2)
	mockRepo.On("HasAnySubscription", "233123456789").Return(true, nil)
	mockRepo.On("HasAnySubscription", "233123456791").Return(true, nil)
	mockRepo.On("DeleteSubscriptionRecord", "233123456789").Return(nil)
	mockRepo.On("DeleteSubscriptionRecord", "233123456791").Return(nil)

	// Process batch
	service.BatchHandleInvalidMSISDNs(responses, requests, partnerId)

	// Wait for async operations to complete
	time.Sleep(200 * time.Millisecond)

	// Verify expectations
	mockRepo.AssertExpectations(t)
}

func TestIsInvalidMSISDNResponse(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	service := &SubscriptionService{
		logger: logger,
	}

	// Test case 1: INVALID_MSISDN in main response code
	response1 := &domain.MTResponse{
		Code: "INVALID_MSISDN",
		ResponseData: map[string]interface{}{
			"subscriptionResult": "null",
			"subscriptionError":  "null",
		},
	}
	assert.True(t, service.isInvalidMSISDNResponse(response1))

	// Test case 2: INVALID_MSISDN in subscription result
	response2 := &domain.MTResponse{
		Code: "SUCCESS",
		ResponseData: map[string]interface{}{
			"subscriptionResult": "INVALID_MSISDN",
			"subscriptionError":  "null",
		},
	}
	assert.True(t, service.isInvalidMSISDNResponse(response2))

	// Test case 3: INVALID_MSISDN in subscription error
	response3 := &domain.MTResponse{
		Code: "SUCCESS",
		ResponseData: map[string]interface{}{
			"subscriptionResult": "null",
			"subscriptionError":  "Invalid MSISDN",
		},
	}
	assert.True(t, service.isInvalidMSISDNResponse(response3))

	// Test case 4: Valid response
	response4 := &domain.MTResponse{
		Code: "SUCCESS",
		ResponseData: map[string]interface{}{
			"subscriptionResult": "OPTIN_ALREADY_ACTIVE",
			"subscriptionError":  "null",
		},
	}
	assert.False(t, service.isInvalidMSISDNResponse(response4))
}

func TestExtractSubscriptionResult(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	service := &SubscriptionService{
		logger: logger,
	}

	// Test case 1: Valid subscription result
	response1 := &domain.MTResponse{
		ResponseData: map[string]interface{}{
			"subscriptionResult": "OPTIN_ALREADY_ACTIVE",
		},
	}
	assert.Equal(t, "OPTIN_ALREADY_ACTIVE", service.extractSubscriptionResult(response1))

	// Test case 2: Missing subscription result
	response2 := &domain.MTResponse{
		ResponseData: map[string]interface{}{},
	}
	assert.Equal(t, "", service.extractSubscriptionResult(response2))

	// Test case 3: Nil subscription result
	response3 := &domain.MTResponse{
		ResponseData: map[string]interface{}{
			"subscriptionResult": nil,
		},
	}
	assert.Equal(t, "", service.extractSubscriptionResult(response3))
}

func TestExtractSubscriptionError(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	service := &SubscriptionService{
		logger: logger,
	}

	// Test case 1: Valid subscription error
	response1 := &domain.MTResponse{
		ResponseData: map[string]interface{}{
			"subscriptionError": "Invalid MSISDN",
		},
	}
	assert.Equal(t, "Invalid MSISDN", service.extractSubscriptionError(response1))

	// Test case 2: Missing subscription error
	response2 := &domain.MTResponse{
		ResponseData: map[string]interface{}{},
	}
	assert.Equal(t, "", service.extractSubscriptionError(response2))

	// Test case 3: Nil subscription error
	response3 := &domain.MTResponse{
		ResponseData: map[string]interface{}{
			"subscriptionError": nil,
		},
	}
	assert.Equal(t, "", service.extractSubscriptionError(response3))
}

func TestBatchCreateInvalidMSISDNLogs(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := &MockSubscriptionRepositoryForInvalidMSISDN{}

	service := &SubscriptionService{
		logger: logger,
		repo:   mockRepo,
	}

	// Create test logs
	logs := []*domain.InvalidMSISDNLog{
		{
			MSISDN:       "233123456789",
			ProductID:    &[]int{123}[0],
			ResponseCode: "INVALID_MSISDN",
			CreatedAt:    time.Now(),
		},
		{
			MSISDN:       "233123456790",
			ProductID:    &[]int{124}[0],
			ResponseCode: "INVALID_MSISDN",
			CreatedAt:    time.Now(),
		},
	}

	// Set up mock expectations
	mockRepo.On("CreateInvalidMSISDNLog", mock.AnythingOfType("*domain.InvalidMSISDNLog")).Return(nil).Times(2)

	// Process batch
	service.batchCreateInvalidMSISDNLogs(logs)

	// Wait for completion
	time.Sleep(100 * time.Millisecond)

	// Verify expectations
	mockRepo.AssertExpectations(t)
}

func TestBatchCleanupInvalidMSISDNSubscriptions(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := &MockSubscriptionRepositoryForInvalidMSISDN{}

	service := &SubscriptionService{
		logger: logger,
		repo:   mockRepo,
	}

	// Create cleanup tasks
	cleanupTasks := []struct {
		msisdn    string
		productId int
		requestID string
	}{
		{
			msisdn:    "233123456789",
			productId: 123,
			requestID: "test-request-id-1",
		},
		{
			msisdn:    "233123456790",
			productId: 124,
			requestID: "test-request-id-2",
		},
	}

	// Set up mock expectations
	mockRepo.On("HasAnySubscription", "233123456789").Return(true, nil)
	mockRepo.On("HasAnySubscription", "233123456790").Return(true, nil)
	mockRepo.On("DeleteSubscriptionRecord", "233123456789").Return(nil)
	mockRepo.On("DeleteSubscriptionRecord", "233123456790").Return(nil)

	// Process cleanup
	service.batchCleanupInvalidMSISDNSubscriptions(cleanupTasks)

	// Wait for completion
	time.Sleep(200 * time.Millisecond)

	// Verify expectations
	mockRepo.AssertExpectations(t)
}

func TestInvalidMSISDNMetrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	metrics := monitoring.NewInvalidMSISDNMetrics(logger)

	// Test initial state
	assert.Equal(t, int64(0), metrics.TotalInvalidMSISDNsDetected)
	assert.Equal(t, int64(0), metrics.TotalSubscriptionsCleaned)

	// Test recording operations
	metrics.RecordInvalidMSISDNDetected()
	metrics.RecordSubscriptionCleaned(100 * time.Millisecond)
	metrics.RecordCleanupFailure("database_error", fmt.Errorf("connection failed"))

	// Verify metrics
	assert.Equal(t, int64(1), metrics.TotalInvalidMSISDNsDetected)
	assert.Equal(t, int64(1), metrics.TotalSubscriptionsCleaned)
	assert.Equal(t, int64(1), metrics.TotalCleanupFailures)

	// Test success rate calculation
	successRate := metrics.GetSuccessRate()
	assert.Equal(t, 50.0, successRate) // 1 success, 1 failure = 50%

	// Test metrics snapshot
	snapshot := metrics.GetMetricsSnapshot()
	assert.NotNil(t, snapshot)
	assert.Equal(t, int64(1), snapshot["total_invalid_msisdns_detected"])
	assert.Equal(t, int64(1), snapshot["total_subscriptions_cleaned"])

	// Test reset
	metrics.Reset()
	assert.Equal(t, int64(0), metrics.TotalInvalidMSISDNsDetected)
	assert.Equal(t, int64(0), metrics.TotalSubscriptionsCleaned)
}
