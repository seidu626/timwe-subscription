package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/seidu626/subscription-manager/subscription-external/internal/monitoring"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// MockUserBaseRepositoryForBlacklisted is a mock for testing BLACKLISTED functionality
type MockUserBaseRepositoryForBlacklisted struct {
	mock.Mock
}

func (m *MockUserBaseRepositoryForBlacklisted) InsertUserRecords(ctx context.Context, users []*domain.UserBase) error {
	args := m.Called(ctx, users)
	return args.Error(0)
}

func (m *MockUserBaseRepositoryForBlacklisted) GetBlacklistedMSISDNS(ctx context.Context, msisdns []string) ([]string, error) {
	args := m.Called(ctx, msisdns)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockUserBaseRepositoryForBlacklisted) IsExcludedUser(msisdn string) (bool, error) {
	args := m.Called(msisdn)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserBaseRepositoryForBlacklisted) LoadExclusionList() (map[string]bool, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]bool), args.Error(1)
}

func (m *MockUserBaseRepositoryForBlacklisted) FilterMSISDNS(msisdns []string) ([]string, error) {
	args := m.Called(msisdns)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockUserBaseRepositoryForBlacklisted) GenerateBatchMSISDNSFast(ctx context.Context, count int, productId int, shortcode string, entryChannel string) ([]string, error) {
	args := m.Called(ctx, count, productId, shortcode, entryChannel)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockUserBaseRepositoryForBlacklisted) GetExistingMSISDNS(ctx context.Context, msisdns []string) ([]string, error) {
	args := m.Called(ctx, msisdns)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockUserBaseRepositoryForBlacklisted) GetInvalidMSISDNS(ctx context.Context, msisdns []string) ([]string, error) {
	args := m.Called(ctx, msisdns)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockUserBaseRepositoryForBlacklisted) GetInvalidMSISDNSOptimized(ctx context.Context, msisdns []string) ([]string, error) {
	args := m.Called(ctx, msisdns)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockUserBaseRepositoryForBlacklisted) GetInvalidMSISDNSFast(ctx context.Context, msisdn string) (bool, error) {
	args := m.Called(ctx, msisdn)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserBaseRepositoryForBlacklisted) GetInvalidMSISDNSStats(ctx context.Context) (map[string]interface{}, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockUserBaseRepositoryForBlacklisted) GetPremierMSISDNS(ctx context.Context, msisdns []string) ([]string, error) {
	args := m.Called(ctx, msisdns)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockUserBaseRepositoryForBlacklisted) GetStaffMSISDNS(ctx context.Context, msisdns []string) ([]string, error) {
	args := m.Called(ctx, msisdns)
	return args.Get(0).([]string), args.Error(1)
}

// TestEnhancedBlacklistedUserHandling tests the complete enhanced BLACKLISTED user handling flow
func TestEnhancedBlacklistedUserHandling(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := &MockSubscriptionRepositoryComplete{}
	mockUserBaseRepo := &MockUserBaseRepositoryForBlacklisted{}

	service := &SubscriptionService{
		logger:             logger,
		repo:               mockRepo,
		UserBaseRepository: mockUserBaseRepo,
	}

	// Test scenario: BLACKLISTED response received during opt-in
	msisdn := "233123456789"
	productId := 123
	requestID := "test-request-id-123"
	partnerId := 789

	// Set up mock expectations for the complete flow
	mockUserBaseRepo.On("InsertUserRecords", mock.Anything, mock.AnythingOfType("[]*domain.UserBase")).Return(nil)
	mockRepo.On("DeleteSubscriptionRecord", msisdn).Return(nil)

	// Create test response
	response := &domain.MTResponse{
		Code:    "BLACKLISTED",
		Message: "User is blacklisted",
		ResponseData: map[string]interface{}{
			"subscriptionResult": "BLACKLISTED",
			"subscriptionError":  "null",
		},
		RequestID: requestID,
	}

	// Execute the enhanced flow
	service.handleBlacklistedUserEnhanced(msisdn, productId, requestID, partnerId, response)

	// Wait for async operations to complete
	time.Sleep(200 * time.Millisecond)

	// Verify all mock expectations were met
	mockRepo.AssertExpectations(t)
	mockUserBaseRepo.AssertExpectations(t)
}

// TestBlacklistedUserRetryLogic tests the retry logic for blacklisted user operations
func TestBlacklistedUserRetryLogic(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := &MockSubscriptionRepositoryComplete{}
	mockUserBaseRepo := &MockUserBaseRepositoryForBlacklisted{}

	service := &SubscriptionService{
		logger:             logger,
		repo:               mockRepo,
		UserBaseRepository: mockUserBaseRepo,
	}

	// Test scenario: Userbase insertion fails initially, then succeeds
	msisdn := "233123456790"
	productId := 456
	requestID := "test-request-id-456"
	partnerId := 790

	// Set up mock expectations for retry scenario
	mockUserBaseRepo.On("InsertUserRecords", mock.Anything, mock.AnythingOfType("[]*domain.UserBase")).
		Return(assert.AnError).Once() // First attempt fails
	mockUserBaseRepo.On("InsertUserRecords", mock.Anything, mock.AnythingOfType("[]*domain.UserBase")).
		Return(nil).Once() // Second attempt succeeds
	mockRepo.On("DeleteSubscriptionRecord", msisdn).Return(nil)

	response := &domain.MTResponse{
		Code:      "BLACKLISTED",
		Message:   "User is blacklisted",
		RequestID: requestID,
	}

	// Execute the enhanced flow
	service.handleBlacklistedUserEnhanced(msisdn, productId, requestID, partnerId, response)

	// Wait for completion (including retries)
	time.Sleep(600 * time.Millisecond)

	// Verify expectations
	mockRepo.AssertExpectations(t)
	mockUserBaseRepo.AssertExpectations(t)
}

// TestBatchBlacklistedUserProcessing tests batch processing of multiple BLACKLISTED responses
func TestBatchBlacklistedUserProcessing(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := &MockSubscriptionRepositoryComplete{}
	mockUserBaseRepo := &MockUserBaseRepositoryForBlacklisted{}

	service := &SubscriptionService{
		logger:             logger,
		repo:               mockRepo,
		UserBaseRepository: mockUserBaseRepo,
	}

	// Create test data with multiple BLACKLISTED responses
	responses := []*domain.MTResponse{
		{
			Code:    "BLACKLISTED",
			Message: "User is blacklisted",
			ResponseData: map[string]interface{}{
				"subscriptionResult": "BLACKLISTED",
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
			Code:    "BLACKLISTED",
			Message: "User is blacklisted",
			ResponseData: map[string]interface{}{
				"subscriptionError": "BLACKLISTED",
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

	// Set up mock expectations for the two blacklisted users
	mockUserBaseRepo.On("InsertUserRecords", mock.Anything, mock.AnythingOfType("[]*domain.UserBase")).Return(nil).Times(2)
	mockRepo.On("DeleteSubscriptionRecord", "233123456789").Return(nil)
	mockRepo.On("DeleteSubscriptionRecord", "233123456791").Return(nil)

	// Process batch
	service.BatchHandleBlacklistedUsers(responses, requests, partnerId)

	// Wait for async operations to complete
	time.Sleep(300 * time.Millisecond)

	// Verify expectations
	mockRepo.AssertExpectations(t)
	mockUserBaseRepo.AssertExpectations(t)
}

// TestBlacklistedUserAuditLogging tests the audit logging functionality
func TestBlacklistedUserAuditLogging(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := &MockSubscriptionRepositoryComplete{}
	mockUserBaseRepo := &MockUserBaseRepositoryForBlacklisted{}

	service := &SubscriptionService{
		logger:             logger,
		repo:               mockRepo,
		UserBaseRepository: mockUserBaseRepo,
	}

	// Test scenario: Audit log creation
	msisdn := "233123456792"
	productId := 789
	requestID := "test-request-id-audit"
	partnerId := 791

	// Set up mock expectations
	mockUserBaseRepo.On("InsertUserRecords", mock.Anything, mock.AnythingOfType("[]*domain.UserBase")).Return(nil)
	mockRepo.On("DeleteSubscriptionRecord", msisdn).Return(nil)

	response := &domain.MTResponse{
		Code:      "BLACKLISTED",
		Message:   "User is blacklisted",
		RequestID: requestID,
	}

	// Execute the enhanced flow
	service.handleBlacklistedUserEnhanced(msisdn, productId, requestID, partnerId, response)

	// Wait for completion
	time.Sleep(100 * time.Millisecond)

	// Verify expectations
	mockRepo.AssertExpectations(t)
	mockUserBaseRepo.AssertExpectations(t)
}

// TestBlacklistedUserMetrics tests the metrics collection functionality
func TestBlacklistedUserMetrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	metrics := monitoring.NewBlacklistedMetrics(logger)

	// Test initial state
	assert.Equal(t, int64(0), metrics.TotalBlacklistedUsersDetected)
	assert.Equal(t, int64(0), metrics.TotalUserbaseInsertions)
	assert.Equal(t, int64(0), metrics.TotalSubscriptionsCleaned)

	// Simulate some operations
	metrics.RecordBlacklistedUserDetected()
	metrics.RecordBlacklistedUserDetected()
	metrics.RecordUserbaseInsertion(50 * time.Millisecond)
	metrics.RecordUserbaseInsertion(75 * time.Millisecond)
	metrics.RecordSubscriptionCleaned(100 * time.Millisecond)
	metrics.RecordOperationFailure("database_error", assert.AnError)
	metrics.RecordAuditLogCreated()

	// Verify metrics
	assert.Equal(t, int64(2), metrics.TotalBlacklistedUsersDetected)
	assert.Equal(t, int64(2), metrics.TotalUserbaseInsertions)
	assert.Equal(t, int64(1), metrics.TotalSubscriptionsCleaned)
	assert.Equal(t, int64(1), metrics.TotalOperationFailures)
	assert.Equal(t, int64(1), metrics.TotalAuditLogsCreated)

	// Test success rate calculation
	successRate := metrics.GetSuccessRate()
	assert.Equal(t, 75.0, successRate) // 3 success, 1 failure = 75%

	// Test metrics snapshot
	snapshot := metrics.GetMetricsSnapshot()
	assert.NotNil(t, snapshot)
	assert.Equal(t, int64(2), snapshot["total_blacklisted_users_detected"])
	assert.Equal(t, int64(2), snapshot["total_userbase_insertions"])
	assert.Equal(t, int64(1), snapshot["total_subscriptions_cleaned"])
	assert.Equal(t, int64(1), snapshot["total_operation_failures"])
	assert.Equal(t, int64(1), snapshot["total_audit_logs_created"])

	// Test reset functionality
	metrics.Reset()
	assert.Equal(t, int64(0), metrics.TotalBlacklistedUsersDetected)
	assert.Equal(t, int64(0), metrics.TotalUserbaseInsertions)
	assert.Equal(t, int64(0), metrics.TotalSubscriptionsCleaned)
}

// TestBlacklistedUserErrorHandling tests error scenarios and recovery
func TestBlacklistedUserErrorHandling(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := &MockSubscriptionRepositoryComplete{}
	mockUserBaseRepo := &MockUserBaseRepositoryForBlacklisted{}

	service := &SubscriptionService{
		logger:             logger,
		repo:               mockRepo,
		UserBaseRepository: mockUserBaseRepo,
	}

	// Test scenario: Database error during userbase insertion
	msisdn := "233123456793"
	productId := 792
	requestID := "test-request-id-error"
	partnerId := 793

	// Set up mock expectations for error scenario
	mockUserBaseRepo.On("InsertUserRecords", mock.Anything, mock.AnythingOfType("[]*domain.UserBase")).
		Return(assert.AnError).Times(3) // All retries fail

	response := &domain.MTResponse{
		Code:      "BLACKLISTED",
		Message:   "User is blacklisted",
		RequestID: requestID,
	}

	// Execute the enhanced flow (should fail after retries)
	service.handleBlacklistedUserEnhanced(msisdn, productId, requestID, partnerId, response)

	// Wait for completion (including retries)
	time.Sleep(600 * time.Millisecond)

	// Verify expectations
	mockUserBaseRepo.AssertExpectations(t)
}

// TestBlacklistedUserConfiguration tests that the system works with different configurations
func TestBlacklistedUserConfiguration(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := &MockSubscriptionRepositoryComplete{}
	mockUserBaseRepo := &MockUserBaseRepositoryForBlacklisted{}

	service := &SubscriptionService{
		logger:             logger,
		repo:               mockRepo,
		UserBaseRepository: mockUserBaseRepo,
	}

	// Test with different MSISDN formats
	testCases := []string{
		"233123456789",
		"233123456790",
		"233123456791",
	}

	for _, msisdn := range testCases {
		// Set up mock expectations
		mockUserBaseRepo.On("InsertUserRecords", mock.Anything, mock.AnythingOfType("[]*domain.UserBase")).Return(nil)
		mockRepo.On("DeleteSubscriptionRecord", msisdn).Return(nil)

		response := &domain.MTResponse{
			Code:      "BLACKLISTED",
			Message:   "User is blacklisted",
			RequestID: "test-request-id",
		}

		// Execute the enhanced flow
		service.handleBlacklistedUserEnhanced(msisdn, 123, "test-request-id", 789, response)

		// Wait for completion
		time.Sleep(100 * time.Millisecond)
	}

	// Verify all expectations were met
	mockRepo.AssertExpectations(t)
	mockUserBaseRepo.AssertExpectations(t)
}

// TestBlacklistedUserPerformance tests performance characteristics
func TestBlacklistedUserPerformance(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := &MockSubscriptionRepositoryComplete{}
	mockUserBaseRepo := &MockUserBaseRepositoryForBlacklisted{}

	service := &SubscriptionService{
		logger:             logger,
		repo:               mockRepo,
		UserBaseRepository: mockUserBaseRepo,
	}

	// Test batch processing performance
	startTime := time.Now()

	// Create large batch
	responses := make([]*domain.MTResponse, 50)
	requests := make([]domain.MTRequest, 50)

	for i := 0; i < 50; i++ {
		responses[i] = &domain.MTResponse{
			Code:    "BLACKLISTED",
			Message: "User is blacklisted",
			ResponseData: map[string]interface{}{
				"subscriptionResult": "BLACKLISTED",
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
		mockUserBaseRepo.On("InsertUserRecords", mock.Anything, mock.AnythingOfType("[]*domain.UserBase")).Return(nil)
		mockRepo.On("DeleteSubscriptionRecord", msisdn).Return(nil)
	}

	partnerId := 789

	// Process batch
	service.BatchHandleBlacklistedUsers(responses, requests, partnerId)

	// Wait for completion
	time.Sleep(500 * time.Millisecond)

	duration := time.Since(startTime)

	// Verify expectations
	mockRepo.AssertExpectations(t)
	mockUserBaseRepo.AssertExpectations(t)

	// Performance assertion: Should complete within reasonable time
	assert.Less(t, duration, 2*time.Second, "Batch processing should complete within 2 seconds")
}
