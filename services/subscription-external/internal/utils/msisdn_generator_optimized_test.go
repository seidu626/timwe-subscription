package utils

import (
	"context"
	"testing"
	"time"

	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// MockRepositoryForOptimized is a simplified mock for testing the optimized generator
type MockRepositoryForOptimized struct {
	mock.Mock
}

func (m *MockRepositoryForOptimized) IsExcludedUser(msisdn string) (bool, error) {
	args := m.Called(msisdn)
	return args.Bool(0), args.Error(1)
}

func (m *MockRepositoryForOptimized) FilterMSISDNS(msisdns []string) ([]string, error) {
	args := m.Called(msisdns)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockRepositoryForOptimized) LoadExclusionList() (map[string]bool, error) {
	args := m.Called()
	return args.Get(0).(map[string]bool), args.Error(1)
}

func (m *MockRepositoryForOptimized) GetExistingMSISDNS(ctx context.Context, msisdns []string) ([]string, error) {
	args := m.Called(ctx, msisdns)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockRepositoryForOptimized) InsertUserRecords(ctx context.Context, records []*domain.UserBase) error {
	args := m.Called(ctx, records)
	return args.Error(0)
}

func (m *MockRepositoryForOptimized) GetInvalidMSISDNS(ctx context.Context, msisdns []string) ([]string, error) {
	args := m.Called(ctx, msisdns)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockRepositoryForOptimized) GetInvalidMSISDNSOptimized(ctx context.Context, msisdns []string) ([]string, error) {
	args := m.Called(ctx, msisdns)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockRepositoryForOptimized) GetInvalidMSISDNSFast(ctx context.Context, msisdn string) (bool, error) {
	args := m.Called(ctx, msisdn)
	return args.Bool(0), args.Error(1)
}

func (m *MockRepositoryForOptimized) GetInvalidMSISDNSStats(ctx context.Context) (map[string]interface{}, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockRepositoryForOptimized) GetBlacklistedMSISDNS(ctx context.Context, msisdns []string) ([]string, error) {
	args := m.Called(ctx, msisdns)
	return args.Get(0).([]string), args.Error(1)
}

func TestOptimizedMSISDNGeneratorCreation(t *testing.T) {
	// Test that we can create an optimized generator
	logger, _ := zap.NewDevelopment()

	// Create a mock repository
	mockRepo := new(MockRepositoryForOptimized)

	// Create bloom filter (nil for this test)
	var bloomFilter *MSISDNBloomFilter

	// Create generator
	generator := NewOptimizedMSISDNGenerator(
		bloomFilter,
		mockRepo,
		logger,
		100, // batch size
		10,  // max concurrent
	)

	assert.NotNil(t, generator)
	assert.Equal(t, 100, generator.batchSize)
	assert.Equal(t, 10, generator.maxConcurrent)
	assert.True(t, generator.cacheEnabled)
}

func TestOptimizedMSISDNGeneratorStats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := new(MockRepositoryForOptimized)

	generator := NewOptimizedMSISDNGenerator(
		nil, // no bloom filter
		mockRepo,
		logger,
		50,
		5,
	)

	// Get initial stats
	stats := generator.GetStats()
	assert.NotNil(t, stats)
	assert.Equal(t, int64(0), stats["generated"])
	assert.Equal(t, int64(0), stats["validated"])
	assert.Equal(t, 50, stats["batch_size"])
	assert.Equal(t, 5, stats["max_concurrent"])
	assert.True(t, stats["cache_enabled"].(bool))
}

func TestOptimizedMSISDNGeneratorConfiguration(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := new(MockRepositoryForOptimized)

	generator := NewOptimizedMSISDNGenerator(
		nil,
		mockRepo,
		logger,
		100,
		10,
	)

	// Test configuration update
	generator.SetConfiguration(200, 20, false)

	stats := generator.GetStats()
	assert.Equal(t, 200, stats["batch_size"])
	assert.Equal(t, 20, stats["max_concurrent"])
	assert.False(t, stats["cache_enabled"].(bool))
}

func TestOptimizedMSISDNGeneratorResetStats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := new(MockRepositoryForOptimized)

	generator := NewOptimizedMSISDNGenerator(
		nil,
		mockRepo,
		logger,
		100,
		10,
	)

	// Update some stats manually
	generator.mutex.Lock()
	generator.stats.generated = 100
	generator.stats.validated = 50
	generator.stats.generationTime = time.Second
	generator.mutex.Unlock()

	// Verify stats were set
	stats := generator.GetStats()
	assert.Equal(t, int64(100), stats["generated"])
	assert.Equal(t, int64(50), stats["validated"])

	// Reset stats
	generator.ResetStats()

	// Verify stats were reset
	stats = generator.GetStats()
	assert.Equal(t, int64(0), stats["generated"])
	assert.Equal(t, int64(0), stats["validated"])
}

func TestOptimizedMSISDNGeneratorTelcoPrefixes(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := new(MockRepositoryForOptimized)

	generator := NewOptimizedMSISDNGenerator(
		nil,
		mockRepo,
		logger,
		100,
		10,
	)

	// Test telco prefix generation
	prefixes := generator.getTelcoPrefixes("mtn", nil)
	assert.NotEmpty(t, prefixes)
	assert.Contains(t, prefixes, "23324")

	prefixes = generator.getTelcoPrefixes("vodafone", nil)
	assert.NotEmpty(t, prefixes)
	assert.Contains(t, prefixes, "23320")

	prefixes = generator.getTelcoPrefixes("airteltigo", nil)
	assert.NotEmpty(t, prefixes)
	assert.Contains(t, prefixes, "233270")

	// Test unknown telco
	prefixes = generator.getTelcoPrefixes("unknown", nil)
	assert.NotEmpty(t, prefixes)
	assert.Contains(t, prefixes, "233")
}

func TestOptimizedMSISDNGeneratorRandomMSISDN(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := new(MockRepositoryForOptimized)

	generator := NewOptimizedMSISDNGenerator(
		nil,
		mockRepo,
		logger,
		100,
		10,
	)

	prefixes := []string{"23324", "23354"}

	// Test random MSISDN generation
	msisdn, err := generator.generateRandomMSISDN(prefixes)
	assert.NoError(t, err)
	assert.NotEmpty(t, msisdn)
	assert.Equal(t, 12, len(msisdn), "MSISDN should be exactly 12 digits, got: %s (length: %d)", msisdn, len(msisdn))

	// Verify it starts with one of the prefixes
	validPrefix := false
	for _, prefix := range prefixes {
		if len(msisdn) >= len(prefix) && msisdn[:len(prefix)] == prefix {
			validPrefix = true
			break
		}
	}
	assert.True(t, validPrefix, "MSISDN should start with a valid prefix")

	// Test with different prefix lengths
	longPrefixes := []string{"233278", "233203"} // 6-digit prefixes
	msisdn2, err := generator.generateRandomMSISDN(longPrefixes)
	assert.NoError(t, err)
	assert.Equal(t, 12, len(msisdn2), "MSISDN with 6-digit prefix should be exactly 12 digits")

	// Test edge case with 3-digit prefix
	shortPrefixes := []string{"233"} // 3-digit prefix
	msisdn3, err := generator.generateRandomMSISDN(shortPrefixes)
	assert.NoError(t, err)
	assert.Equal(t, 12, len(msisdn3), "MSISDN with 3-digit prefix should be exactly 12 digits")
}

func TestOptimizedMSISDNGeneratorMSISDNLengthValidation(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := new(MockRepositoryForOptimized)

	generator := NewOptimizedMSISDNGenerator(
		nil,
		mockRepo,
		logger,
		100,
		10,
	)

	// Test with valid prefixes that should generate 12-digit MSISDNs
	testCases := []struct {
		name           string
		prefixes       []string
		expectedLength int
	}{
		{"3-digit prefix", []string{"233"}, 12},
		{"5-digit prefix", []string{"23324", "23354"}, 12},
		{"6-digit prefix", []string{"233278", "233203"}, 12},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			msisdn, err := generator.generateRandomMSISDN(tc.prefixes)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedLength, len(msisdn),
				"MSISDN should be exactly %d digits for %s, got: %s (length: %d)",
				tc.expectedLength, tc.name, msisdn, len(msisdn))
		})
	}

	// Test with invalid prefix lengths
	invalidPrefixes := []string{"23", "2332", "2332789"} // Too short or too long
	for _, prefix := range invalidPrefixes {
		_, err := generator.generateRandomMSISDN([]string{prefix})
		if !assert.Error(t, err, "Should error with invalid prefix length: %s", prefix) {
			continue
		}
		assert.Contains(t, err.Error(), "invalid prefix length")
	}
}

func TestOptimizedMSISDNGeneratorBloomFilterIntegration(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockRepo := new(MockRepositoryForOptimized)

	// Create a bloom filter
	bloomFilter := NewMSISDNBloomFilter(1000, 0.01, nil, logger)

	generator := NewOptimizedMSISDNGenerator(
		bloomFilter,
		mockRepo,
		logger,
		100,
		10,
	)

	// Add some MSISDNs to bloom filter
	bloomFilter.Add("23324123456")
	bloomFilter.Add("23354123456")

	// Test bloom filter integration
	assert.NotNil(t, generator.bloomFilter)

	// Test that bloom filter can detect added MSISDNs
	assert.True(t, generator.bloomFilter.MightContain("23324123456"))
	assert.True(t, generator.bloomFilter.MightContain("23354123456"))
	assert.False(t, generator.bloomFilter.MightContain("23399999999"))
}

func TestGenerateBatchMSISDNSOptimized_DeadlockPrevention(t *testing.T) {
	// Test with a large count to ensure no deadlock occurs
	logger, _ := zap.NewDevelopment()
	mockRepo := new(MockRepositoryForOptimized)

	generator := NewOptimizedMSISDNGenerator(
		nil, // no bloom filter
		mockRepo,
		logger,
		100, // batch size
		5,   // low max concurrent to test worker pool
	)

	mockRepo.On("IsExcludedUser", mock.Anything).Return(false, nil)
	mockRepo.On("GetInvalidMSISDNSFast", mock.Anything, mock.Anything).Return(false, nil)

	// Test with count much larger than maxConcurrent to trigger the old deadlock scenario
	ctx := context.Background()
	msisdns, err := generator.GenerateBatchMSISDNSOptimized(ctx, "tigo", 100, &config.Config{})

	// Should not deadlock and should return results
	assert.NoError(t, err)
	assert.Len(t, msisdns, 100)

	// Verify that the function returns quickly (no deadlock)
	// This test ensures the worker pool pattern works correctly
}
