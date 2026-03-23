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

// MockSubscriptionRepositoryComplete is a complete mock implementation for testing
type MockSubscriptionRepositoryComplete struct {
	mock.Mock
}

// Basic subscription methods
func (m *MockSubscriptionRepositoryComplete) CreateSubscription(request *domain.SubscriptionRequest) error {
	args := m.Called(request)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryComplete) CreateNotification(notification *domain.NotificationRequest) error {
	args := m.Called(notification)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryComplete) CreateInvalidMSISDNLog(log *domain.InvalidMSISDNLog) error {
	args := m.Called(log)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryComplete) CheckSubscriptionExists(msisdn string, productId int) (bool, error) {
	args := m.Called(msisdn, productId)
	return args.Bool(0), args.Error(1)
}

func (m *MockSubscriptionRepositoryComplete) CheckRenewalNotificationExists(msisdn string, productId int) (bool, error) {
	args := m.Called(msisdn, productId)
	return args.Bool(0), args.Error(1)
}

func (m *MockSubscriptionRepositoryComplete) HasAnySubscription(msisdn string) (bool, error) {
	args := m.Called(msisdn)
	return args.Bool(0), args.Error(1)
}

func (m *MockSubscriptionRepositoryComplete) DeleteSubscriptionRecord(msisdn string) error {
	args := m.Called(msisdn)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryComplete) FindAndRemoveSubscription(msisdn string, productId int) error {
	args := m.Called(msisdn, productId)
	return args.Error(0)
}

// Utility methods
func (m *MockSubscriptionRepositoryComplete) GenerateCacheKey(startDate, endDate time.Time, productId int, shortcode, userIdentifier, entryChannel string, page, pageSize int) string {
	args := m.Called(startDate, endDate, productId, shortcode, userIdentifier, entryChannel, page, pageSize)
	return args.String(0)
}

func (m *MockSubscriptionRepositoryComplete) FetchActiveMsisdnsMissingSomeProducts(productIds []int, offset int, limit int) ([]string, error) {
	args := m.Called(productIds, offset, limit)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockSubscriptionRepositoryComplete) FetchActiveMsisdnsWithProductsWindow(productIds []int, offset int, limit int) ([]string, error) {
	args := m.Called(productIds, offset, limit)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockSubscriptionRepositoryComplete) FetchNotificationsWindow(ntype string, since time.Time, afterId int64, limit int) ([]repository.NotificationRow, error) {
	args := m.Called(ntype, since, afterId, limit)
	return args.Get(0).([]repository.NotificationRow), args.Error(1)
}

func (m *MockSubscriptionRepositoryComplete) FetchSubscriptionsNeedingRenewal(cutoff time.Time, afterId int64, limit int) ([]repository.NotificationRow, error) {
	args := m.Called(cutoff, afterId, limit)
	return args.Get(0).([]repository.NotificationRow), args.Error(1)
}

func (m *MockSubscriptionRepositoryComplete) UpsertSubscriptionStatus(msisdn string, productId int, status string) error {
	args := m.Called(msisdn, productId, status)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryComplete) FetchUnprocessedOptoutNotifications(since time.Time, afterId int64, limit int) ([]repository.NotificationRow, error) {
	args := m.Called(since, afterId, limit)
	return args.Get(0).([]repository.NotificationRow), args.Error(1)
}

func (m *MockSubscriptionRepositoryComplete) FetchChargeSuccessNotifications(since time.Time, afterID int64, limit int) ([]repository.ChargeSuccessNotificationRow, error) {
	args := m.Called(since, afterID, limit)
	return args.Get(0).([]repository.ChargeSuccessNotificationRow), args.Error(1)
}

func (m *MockSubscriptionRepositoryComplete) GetSubscriptionByMSISDNAndProduct(msisdn string, productID int) (*domain.Subscription, error) {
	args := m.Called(msisdn, productID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Subscription), args.Error(1)
}

func (m *MockSubscriptionRepositoryComplete) GetLastOptinNotificationTime(msisdn string, productID int) (*time.Time, error) {
	args := m.Called(msisdn, productID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*time.Time), args.Error(1)
}

func (m *MockSubscriptionRepositoryComplete) FetchGhostSubscriptions(cutoff time.Time, afterId int64, limit int) ([]repository.NotificationRow, error) {
	args := m.Called(cutoff, afterId, limit)
	return args.Get(0).([]repository.NotificationRow), args.Error(1)
}

func (m *MockSubscriptionRepositoryComplete) FetchChargingFailedSubscriptions(filter repository.ChargingFailureFilter) ([]repository.ChargingFailedSubscription, error) {
	args := m.Called(filter)
	return args.Get(0).([]repository.ChargingFailedSubscription), args.Error(1)
}

func (m *MockSubscriptionRepositoryComplete) GetChargingFailureByMSISDN(msisdn string, productID int) (*repository.ChargingFailedSubscription, error) {
	args := m.Called(msisdn, productID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.ChargingFailedSubscription), args.Error(1)
}

func (m *MockSubscriptionRepositoryComplete) GetChargingFailureCount(filter repository.ChargingFailureFilter) (int64, error) {
	args := m.Called(filter)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSubscriptionRepositoryComplete) GetChargingFailureStats() (map[string]interface{}, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockSubscriptionRepositoryComplete) GetChargingFailureSummary() (map[string]interface{}, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockSubscriptionRepositoryComplete) MarkChargingFailureAsProcessed(subscriptionID int, status string) error {
	args := m.Called(subscriptionID, status)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryComplete) UpdateChargingHealthStatus(subscriptionID int, status string, reason string) error {
	args := m.Called(subscriptionID, status, reason)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryComplete) GetTotalSubscriptionsCount() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSubscriptionRepositoryComplete) GetSubscription(msisdn string, productID string) (*domain.SubscriptionWithRenewalInfo, error) {
	args := m.Called(msisdn, productID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SubscriptionWithRenewalInfo), args.Error(1)
}

func (m *MockSubscriptionRepositoryComplete) GetLastSuccessfulPayment(msisdn string, productID string) (*time.Time, error) {
	args := m.Called(msisdn, productID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*time.Time), args.Error(1)
}

func (m *MockSubscriptionRepositoryComplete) GetRenewalAttemptsCount(msisdn string, productID string, since time.Time) (int, error) {
	args := m.Called(msisdn, productID, since)
	return args.Int(0), args.Error(1)
}

func (m *MockSubscriptionRepositoryComplete) GetDailyChurnCount(date time.Time) (int, error) {
	args := m.Called(date)
	return args.Int(0), args.Error(1)
}

func (m *MockSubscriptionRepositoryComplete) GetLastRenewalAttempt(msisdn string, productID string) (*time.Time, error) {
	args := m.Called(msisdn, productID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*time.Time), args.Error(1)
}

func (m *MockSubscriptionRepositoryComplete) ChurnSubscription(msisdn string, productID string, reason string, churnTime time.Time) error {
	args := m.Called(msisdn, productID, reason, churnTime)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryComplete) CreateChurnRecord(record *domain.ChurnRecord) error {
	args := m.Called(record)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryComplete) SaveRenewalCycle(cycle *domain.RenewalCycle) error {
	args := m.Called(cycle)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryComplete) UpdateSubscriptionStatus(msisdn string, productID string, status string) error {
	args := m.Called(msisdn, productID, status)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryComplete) AddToPriorityRetryQueue(item *domain.PriorityRetryQueue) error {
	args := m.Called(item)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryComplete) IncrementRenewalAttempt(msisdn string, productID string) error {
	args := m.Called(msisdn, productID)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryComplete) GetSubscriptionsNeedingRenewal(hoursThreshold int, limit int) ([]*domain.SubscriptionWithRenewalInfo, error) {
	args := m.Called(hoursThreshold, limit)
	return args.Get(0).([]*domain.SubscriptionWithRenewalInfo), args.Error(1)
}

func (m *MockSubscriptionRepositoryComplete) SaveRenewalMetrics(metrics *domain.RenewalMetrics) error {
	args := m.Called(metrics)
	return args.Error(0)
}

func (m *MockSubscriptionRepositoryComplete) GetDuePriorityRetryItems(limit int) ([]*domain.PriorityRetryQueue, error) {
	args := m.Called(limit)
	return args.Get(0).([]*domain.PriorityRetryQueue), args.Error(1)
}

func (m *MockSubscriptionRepositoryComplete) UpdatePriorityRetryItem(item *domain.PriorityRetryQueue) error {
	args := m.Called(item)
	return args.Error(0)
}

// TestIntegrationInvalidMSISDNHandling tests the complete INVALID_MSISDN handling flow
func TestIntegrationInvalidMSISDNHandling(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := &MockSubscriptionRepositoryComplete{}

	service := &SubscriptionService{
		logger: logger,
		repo:   mockRepo,
	}

	// Test scenario: INVALID_MSISDN detected during opt-in
	msisdn := "233123456789"
	productId := 123
	requestID := "test-request-id-123"

	// Set up mock expectations for the complete flow
	mockRepo.On("CreateInvalidMSISDNLog", mock.AnythingOfType("*domain.InvalidMSISDNLog")).Return(nil)
	mockRepo.On("HasAnySubscription", msisdn).Return(true, nil) // User has subscriptions
	mockRepo.On("DeleteSubscriptionRecord", msisdn).Return(nil) // Cleanup succeeds

	// Create test response and request
	response := &domain.MTResponse{
		Code:    "INVALID_MSISDN",
		Message: "Invalid MSISDN",
		ResponseData: map[string]interface{}{
			"subscriptionResult": "INVALID_MSISDN",
			"subscriptionError":  "null",
		},
		RequestID: requestID,
	}

	mtReq := domain.MTRequest{
		ProductID:          productId,
		PricepointID:       456,
		UserIdentifier:     msisdn,
		UserIdentifierType: "MSISDN",
		EntryChannel:       "WEB",
		MoTransactionUUID:  "test-tx-id-123",
	}

	partnerId := 789

	// Execute the complete flow
	service.detectAndLogInvalidMSISDN(response, mtReq, partnerId)

	// Wait for async cleanup to complete
	time.Sleep(200 * time.Millisecond)

	// Verify all mock expectations were met
	mockRepo.AssertExpectations(t)
}

// TestIntegrationProductIndependentCleanup tests that cleanup removes ALL subscriptions regardless of product
func TestIntegrationProductIndependentCleanup(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := &MockSubscriptionRepositoryComplete{}

	service := &SubscriptionService{
		logger: logger,
		repo:   mockRepo,
	}

	// Test scenario: User has multiple product subscriptions, INVALID_MSISDN occurs
	msisdn := "233123456790"
	productId := 456 // This is the product that triggered the error
	requestID := "test-request-id-456"

	// Set up mock expectations
	mockRepo.On("HasAnySubscription", msisdn).Return(true, nil) // User has subscriptions (any product)
	mockRepo.On("DeleteSubscriptionRecord", msisdn).Return(nil) // Cleanup succeeds

	// Execute cleanup directly
	service.handleInvalidMSISDNCleanup(msisdn, productId, requestID)

	// Wait for completion
	time.Sleep(100 * time.Millisecond)

	// Verify expectations
	mockRepo.AssertExpectations(t)

	// Verify that HasAnySubscription was called (product-independent check)
	mockRepo.AssertCalled(t, "HasAnySubscription", msisdn)

	// Verify that DeleteSubscriptionRecord was called (removes ALL subscriptions)
	mockRepo.AssertCalled(t, "DeleteSubscriptionRecord", msisdn)
}

// TestIntegrationBatchProcessing tests batch processing of multiple INVALID_MSISDN responses
func TestIntegrationBatchProcessing(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := &MockSubscriptionRepositoryComplete{}

	service := &SubscriptionService{
		logger: logger,
		repo:   mockRepo,
	}

	// Create test data with multiple INVALID_MSISDN responses
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
	time.Sleep(300 * time.Millisecond)

	// Verify expectations
	mockRepo.AssertExpectations(t)
}

// TestIntegrationMetricsAndMonitoring tests that metrics are properly tracked
func TestIntegrationMetricsAndMonitoring(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	metrics := monitoring.NewInvalidMSISDNMetrics(logger)

	// Test initial state
	assert.Equal(t, int64(0), metrics.TotalInvalidMSISDNsDetected)
	assert.Equal(t, int64(0), metrics.TotalSubscriptionsCleaned)

	// Simulate some operations
	metrics.RecordInvalidMSISDNDetected()
	metrics.RecordInvalidMSISDNDetected()
	metrics.RecordSubscriptionCleaned(50 * time.Millisecond)
	metrics.RecordSubscriptionCleaned(75 * time.Millisecond)
	metrics.RecordCleanupFailure("database_error", assert.AnError)

	// Verify metrics
	assert.Equal(t, int64(2), metrics.TotalInvalidMSISDNsDetected)
	assert.Equal(t, int64(2), metrics.TotalSubscriptionsCleaned)
	assert.Equal(t, int64(1), metrics.TotalCleanupFailures)

	// Test success rate calculation
	successRate := metrics.GetSuccessRate()
	assert.InDelta(t, 66.67, successRate, 0.01) // 2 success, 1 failure = 66.67%

	// Test metrics snapshot
	snapshot := metrics.GetMetricsSnapshot()
	assert.NotNil(t, snapshot)
	assert.Equal(t, int64(2), snapshot["total_invalid_msisdns_detected"])
	assert.Equal(t, int64(2), snapshot["total_subscriptions_cleaned"])
	assert.Equal(t, int64(1), snapshot["total_cleanup_failures"])

	// Test reset functionality
	metrics.Reset()
	assert.Equal(t, int64(0), metrics.TotalInvalidMSISDNsDetected)
	assert.Equal(t, int64(0), metrics.TotalSubscriptionsCleaned)
}

// TestIntegrationErrorHandling tests error scenarios and recovery
func TestIntegrationErrorHandling(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := &MockSubscriptionRepositoryComplete{}

	service := &SubscriptionService{
		logger: logger,
		repo:   mockRepo,
	}

	// Test scenario: Database error during subscription check
	msisdn := "233123456792"
	productId := 789
	requestID := "test-request-id-error"

	// Set up mock expectations for error scenario
	mockRepo.On("HasAnySubscription", msisdn).Return(false, assert.AnError)

	// Execute cleanup (should handle error gracefully)
	service.handleInvalidMSISDNCleanup(msisdn, productId, requestID)

	// Wait for completion
	time.Sleep(100 * time.Millisecond)

	// Verify expectations
	mockRepo.AssertExpectations(t)

	// Test scenario: Database error during deletion (with retries)
	msisdn2 := "233123456793"
	productId2 := 790
	requestID2 := "test-request-id-retry"

	// Set up mock expectations for retry scenario
	mockRepo.On("HasAnySubscription", msisdn2).Return(true, nil)
	mockRepo.On("DeleteSubscriptionRecord", msisdn2).Return(assert.AnError).Times(3) // All retries fail

	// Execute cleanup (should retry and eventually fail)
	service.handleInvalidMSISDNCleanup(msisdn2, productId2, requestID2)

	// Wait for completion (including retries)
	time.Sleep(600 * time.Millisecond)

	// Verify expectations
	mockRepo.AssertExpectations(t)
}

// TestIntegrationConfiguration tests that the system works with different configurations
func TestIntegrationConfiguration(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := &MockSubscriptionRepositoryComplete{}

	service := &SubscriptionService{
		logger: logger,
		repo:   mockRepo,
	}

	// Test with different MSISDN formats
	testCases := []string{
		"233123456789",
		"233123456790",
		"233123456791",
	}

	for _, msisdn := range testCases {
		// Set up mock expectations
		mockRepo.On("HasAnySubscription", msisdn).Return(true, nil)
		mockRepo.On("DeleteSubscriptionRecord", msisdn).Return(nil)

		// Execute cleanup
		service.handleInvalidMSISDNCleanup(msisdn, 123, "test-request-id")

		// Wait for completion
		time.Sleep(100 * time.Millisecond)
	}

	// Verify all expectations were met
	mockRepo.AssertExpectations(t)
}

// TestIntegrationPerformance tests performance characteristics
func TestIntegrationPerformance(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := &MockSubscriptionRepositoryComplete{}

	service := &SubscriptionService{
		logger: logger,
		repo:   mockRepo,
	}

	// Test batch processing performance
	startTime := time.Now()

	// Create large batch
	responses := make([]*domain.MTResponse, 100)
	requests := make([]domain.MTRequest, 100)

	for i := 0; i < 100; i++ {
		responses[i] = &domain.MTResponse{
			Code:    "INVALID_MSISDN",
			Message: "Invalid MSISDN",
			ResponseData: map[string]interface{}{
				"subscriptionResult": "INVALID_MSISDN",
			},
			RequestID: fmt.Sprintf("test-request-id-%d", i),
		}

		requests[i] = domain.MTRequest{
			ProductID:          100 + i,
			PricepointID:       200 + i,
			UserIdentifier:     fmt.Sprintf("233123456%03d", i),
			UserIdentifierType: "MSISDN",
			EntryChannel:       "WEB",
			MoTransactionUUID:  fmt.Sprintf("test-tx-id-%d", i),
		}

		// Set up mock expectations
		msisdn := fmt.Sprintf("233123456%03d", i)
		mockRepo.On("CreateInvalidMSISDNLog", mock.AnythingOfType("*domain.InvalidMSISDNLog")).Return(nil)
		mockRepo.On("HasAnySubscription", msisdn).Return(true, nil)
		mockRepo.On("DeleteSubscriptionRecord", msisdn).Return(nil)
	}

	partnerId := 789

	// Process batch
	service.BatchHandleInvalidMSISDNs(responses, requests, partnerId)

	// Wait for completion
	time.Sleep(500 * time.Millisecond)

	duration := time.Since(startTime)

	// Verify expectations
	mockRepo.AssertExpectations(t)

	// Performance assertion: Should complete within reasonable time
	assert.Less(t, duration, 2*time.Second, "Batch processing should complete within 2 seconds")
}
