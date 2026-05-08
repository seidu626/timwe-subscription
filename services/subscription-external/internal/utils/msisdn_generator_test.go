package utils

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockUserBaseRepository is a mock implementation for testing
type MockUserBaseRepository struct {
	mock.Mock
}

func (m *MockUserBaseRepository) IsPremierOrStaff(msisdn string) (bool, error) {
	args := m.Called(msisdn)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserBaseRepository) GetInvalidMSISDNS(ctx context.Context, msisdns []string) ([]string, error) {
	args := m.Called(ctx, msisdns)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockUserBaseRepository) FilterMSISDNS(msisdns []string) ([]string, error) {
	args := m.Called(msisdns)
	return args.Get(0).([]string), args.Error(1)
}

// Add missing interface methods
func (m *MockUserBaseRepository) IsExcludedUser(msisdn string) (bool, error) {
	args := m.Called(msisdn)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserBaseRepository) LoadExclusionList() (map[string]bool, error) {
	args := m.Called()
	return args.Get(0).(map[string]bool), args.Error(1)
}

func (m *MockUserBaseRepository) GetExistingMSISDNS(ctx context.Context, msisdns []string) ([]string, error) {
	args := m.Called(ctx, msisdns)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockUserBaseRepository) InsertUserRecords(ctx context.Context, records []*domain.UserBase) error {
	args := m.Called(ctx, records)
	return args.Error(0)
}

func (m *MockUserBaseRepository) GetInvalidMSISDNSOptimized(ctx context.Context, msisdns []string) ([]string, error) {
	args := m.Called(ctx, msisdns)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockUserBaseRepository) GetInvalidMSISDNSFast(ctx context.Context, msisdn string) (bool, error) {
	args := m.Called(ctx, msisdn)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserBaseRepository) GetInvalidMSISDNSStats(ctx context.Context) (map[string]interface{}, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockUserBaseRepository) GetBlacklistedMSISDNS(ctx context.Context, msisdns []string) ([]string, error) {
	args := m.Called(ctx, msisdns)
	return args.Get(0).([]string), args.Error(1)
}

func TestLoadTigoUserbaseData(t *testing.T) {
	// Test loading sample data
	sampleData := []string{
		"233561075653",
		"233561234567",
		"233578345678",
		"233242456789",
	}

	// Load samples into the pool
	LoadMSISDNSamples(sampleData)

	// Verify samples were loaded
	assert.True(t, len(globalMSISDNPool.tigoSamples) > 0)
}

func TestGenerateMSISDNWithPatterns(t *testing.T) {
	// Setup mock repository
	mockRepo := new(MockUserBaseRepository)
	mockRepo.On("IsPremierOrStaff", mock.Anything).Return(false, nil)
	mockRepo.On("IsExcludedUser", mock.Anything).Return(false, nil)
	mockRepo.On("GetInvalidMSISDNS", mock.Anything, mock.Anything).Return([]string{}, nil)
	mockRepo.On("GetInvalidMSISDNSFast", mock.Anything, mock.Anything).Return(false, nil)
	mockRepo.On("LoadExclusionList").Return(map[string]bool{}, nil)
	mockRepo.On("GetInvalidMSISDNSOptimized", mock.Anything, mock.Anything).Return([]string{}, nil)
	mockRepo.On("GetBlacklistedMSISDNS", mock.Anything, mock.Anything).Return([]string{}, nil)

	// Setup config
	cfg := &config.Config{
		Application: struct {
			Environment    config.Environment  `mapstructure:"ENVIRONMENT"`
			Port           int                 `mapstructure:"PORT"`
			AllowedOrigins []string            `mapstructure:"ALLOWED_ORIGINS"`
			TelcoPrefixes  map[string][]string `mapstructure:"TELCO_PREFIXES"`
			TIMWE          struct {
				Host                   string        `mapstructure:"HOST"`
				BaseURL                string        `mapstructure:"BASE_URL"`
				APIKey                 string        `mapstructure:"API_KEY"`
				MTAPIKey               string        `mapstructure:"MT_API_KEY"`
				Psk                    string        `mapstructure:"PSK"`
				PartnerServiceID       string        `mapstructure:"PARTNER_SERVICE_ID"`
				PartnerRoleID          string        `mapstructure:"PARTNER_ROLE_ID"`
				Realm                  string        `mapstructure:"REALM"`
				AuthenticationKey      string        `mapstructure:"AUTHENTICATION_KEY"`
				MCC                    string        `mapstructure:"MCC"`
				MNC                    string        `mapstructure:"MNC"`
				Timeout                time.Duration `mapstructure:"TIMEOUT"`
				MaxConnections         int           `mapstructure:"MAX_CONNECTIONS"`
				ChargeRetryMaxDuration time.Duration `mapstructure:"CHARGE_RETRY_MAX_DURATION"`
				ChargeRetryBaseDelay   time.Duration `mapstructure:"CHARGE_RETRY_BASE_DELAY"`
				ChargeRetryMaxDelay    time.Duration `mapstructure:"CHARGE_RETRY_MAX_DELAY"`
				CBMaxRequests          int           `mapstructure:"CB_MAX_REQUESTS"`
				CBTimeout              time.Duration `mapstructure:"CB_TIMEOUT"`
				CBInterval             time.Duration `mapstructure:"CB_INTERVAL"`
				CBMinRequests          int           `mapstructure:"CB_MIN_REQUESTS"`
				CBFailureRateThreshold float64       `mapstructure:"CB_FAILURE_RATE_THRESHOLD"`
				CBConsecutiveFailures  int           `mapstructure:"CB_CONSECUTIVE_FAILURES"`
			} `mapstructure:"TIMWE_MA"`
			HTTP struct {
				ReadTimeout      time.Duration `mapstructure:"READ_TIMEOUT"`
				WriteTimeout     time.Duration `mapstructure:"WRITE_TIMEOUT"`
				IdleTimeout      time.Duration `mapstructure:"IDLE_TIMEOUT"`
				MaxRequestBodyMB int           `mapstructure:"MAX_REQUEST_BODY_MB"`
				Concurrency      int           `mapstructure:"CONCURRENCY"`
			} `mapstructure:"HTTP"`
			Batch struct {
				MaxWorkersPerJob    int `mapstructure:"MAX_WORKERS_PER_JOB"`
				MaxConcurrentOptins int `mapstructure:"MAX_CONCURRENT_OPTINS"`
				TargetQPS           int `mapstructure:"TARGET_QPS"`
			} `mapstructure:"BATCH"`
			MSISDNGenerator struct {
				Enabled            bool          `mapstructure:"ENABLED"`
				BatchSize          int           `mapstructure:"BATCH_SIZE"`
				MaxConcurrent      int           `mapstructure:"MAX_CONCURRENT"`
				MaxMSISDNCount     int           `mapstructure:"MAX_MSISDN_COUNT"`
				CacheEnabled       bool          `mapstructure:"CACHE_ENABLED"`
				BloomFilterEnabled bool          `mapstructure:"BLOOM_FILTER_ENABLED"`
				FalsePositiveRate  float64       `mapstructure:"FALSE_POSITIVE_RATE"`
				ValidationTimeout  time.Duration `mapstructure:"VALIDATION_TIMEOUT"`
				GenerationTimeout  time.Duration `mapstructure:"GENERATION_TIMEOUT"`
				WorkerPoolSize     int           `mapstructure:"WORKER_POOL_SIZE"`
				ChannelBufferSize  int           `mapstructure:"CHANNEL_BUFFER_SIZE"`
				FallbackToDatabase bool          `mapstructure:"FALLBACK_TO_DATABASE"`
				MaxRetryAttempts   int           `mapstructure:"MAX_RETRY_ATTEMPTS"`
				PreloadEnabled     bool          `mapstructure:"PRELOAD_ENABLED"`
				PreloadBatchSize   int           `mapstructure:"PRELOAD_BATCH_SIZE"`
			} `mapstructure:"MSISDN_GENERATOR"`
			MSISDNValidation struct {
				CacheExpiry             time.Duration       `mapstructure:"CACHE_EXPIRY"`
				EnablePrefixValidation  bool                `mapstructure:"ENABLE_PREFIX_VALIDATION"`
				EnableExcludedUserCheck bool                `mapstructure:"ENABLE_EXCLUDED_USER_CHECK"`
				EnableInvalidLogCheck   bool                `mapstructure:"ENABLE_INVALID_LOG_CHECK"`
				MaxValidationErrors     int                 `mapstructure:"MAX_VALIDATION_ERRORS"`
				TelcoPrefixes           map[string][]string `mapstructure:"TELCO_PREFIXES"`
			} `mapstructure:"MSISDN_VALIDATION"`
			NetworkResilience struct {
				MaxRetries              int           `mapstructure:"MAX_RETRIES"`
				BaseRetryDelay          time.Duration `mapstructure:"BASE_RETRY_DELAY"`
				MaxRetryDelay           time.Duration `mapstructure:"MAX_RETRY_DELAY"`
				ConnectionTimeout       time.Duration `mapstructure:"CONNECTION_TIMEOUT"`
				ReadTimeout             time.Duration `mapstructure:"READ_TIMEOUT"`
				WriteTimeout            time.Duration `mapstructure:"WRITE_TIMEOUT"`
				MaxConnsPerHost         int           `mapstructure:"MAX_CONNS_PER_HOST"`
				MaxIdleConnDuration     time.Duration `mapstructure:"MAX_IDLE_CONN_DURATION"`
				CircuitBreakerThreshold int           `mapstructure:"CIRCUIT_BREAKER_THRESHOLD"`
				CircuitBreakerTimeout   time.Duration `mapstructure:"CIRCUIT_BREAKER_TIMEOUT"`
				JitterEnabled           bool          `mapstructure:"JITTER_ENABLED"`
			} `mapstructure:"NETWORK_RESILIENCE"`
			EnhancedMonitoring struct {
				EnableAutomatedRecovery bool          `mapstructure:"ENABLE_AUTOMATED_RECOVERY"`
				RecoveryCooldown        time.Duration `mapstructure:"RECOVERY_COOLDOWN"`
				MaxRecoveryAttempts     int           `mapstructure:"MAX_RECOVERY_ATTEMPTS"`
				HealthCheckInterval     time.Duration `mapstructure:"HEALTH_CHECK_INTERVAL"`
				AlertCooldown           time.Duration `mapstructure:"ALERT_COOLDOWN"`
				EnableRealTimeMetrics   bool          `mapstructure:"ENABLE_REAL_TIME_METRICS"`
			} `mapstructure:"ENHANCED_MONITORING"`
			Log struct {
				Path    string `mapstructure:"PATH"`
				Rolling struct {
					Enabled           bool `mapstructure:"ENABLED"`
					MaxSize           int  `mapstructure:"MAX_SIZE"`
					MaxAge            int  `mapstructure:"MAX_AGE"`
					MaxBackups        int  `mapstructure:"MAX_BACKUPS"`
					Compress          bool `mapstructure:"COMPRESS"`
					CompressThreshold int  `mapstructure:"COMPRESS_THRESHOLD"`
					LocalTime         bool `mapstructure:"LOCAL_TIME"`
				} `mapstructure:"ROLLING"`
			}
			Key struct {
				Default string `mapstructure:"DEFAULT"`
				Rsa     struct {
					Public  string `mapstructure:"PUBLIC"`
					Private string `mapstructure:"PRIVATE"`
				}
			}
			Graceful struct {
				MaxSecond time.Duration `mapstructure:"MAX_SECOND"`
			} `mapstructure:"GRACEFUL"`
		}{
			TelcoPrefixes: map[string][]string{
				"AirtelTigo": {"233561", "233578", "233242", "233307", "233245"},
				"MTN":        {"233540", "233550", "233244", "233240"},
				"Vodafone":   {"233201", "233202", "233203"},
			},
			MSISDNGenerator: struct {
				Enabled            bool          `mapstructure:"ENABLED"`
				BatchSize          int           `mapstructure:"BATCH_SIZE"`
				MaxConcurrent      int           `mapstructure:"MAX_CONCURRENT"`
				MaxMSISDNCount     int           `mapstructure:"MAX_MSISDN_COUNT"`
				CacheEnabled       bool          `mapstructure:"CACHE_ENABLED"`
				BloomFilterEnabled bool          `mapstructure:"BLOOM_FILTER_ENABLED"`
				FalsePositiveRate  float64       `mapstructure:"FALSE_POSITIVE_RATE"`
				ValidationTimeout  time.Duration `mapstructure:"VALIDATION_TIMEOUT"`
				GenerationTimeout  time.Duration `mapstructure:"GENERATION_TIMEOUT"`
				WorkerPoolSize     int           `mapstructure:"WORKER_POOL_SIZE"`
				ChannelBufferSize  int           `mapstructure:"CHANNEL_BUFFER_SIZE"`
				FallbackToDatabase bool          `mapstructure:"FALLBACK_TO_DATABASE"`
				MaxRetryAttempts   int           `mapstructure:"MAX_RETRY_ATTEMPTS"`
				PreloadEnabled     bool          `mapstructure:"PRELOAD_ENABLED"`
				PreloadBatchSize   int           `mapstructure:"PRELOAD_BATCH_SIZE"`
			}{
				Enabled:            true,
				BatchSize:          1000,
				MaxConcurrent:      50,
				MaxMSISDNCount:     1000000,
				CacheEnabled:       true,
				BloomFilterEnabled: true,
				FalsePositiveRate:  0.01,
				ValidationTimeout:  30 * time.Second,
				GenerationTimeout:  60 * time.Second,
				WorkerPoolSize:     100,
				ChannelBufferSize:  2000,
				FallbackToDatabase: true,
				MaxRetryAttempts:   3,
				PreloadEnabled:     true,
				PreloadBatchSize:   10000,
			},
			MSISDNValidation: struct {
				CacheExpiry             time.Duration       `mapstructure:"CACHE_EXPIRY"`
				EnablePrefixValidation  bool                `mapstructure:"ENABLE_PREFIX_VALIDATION"`
				EnableExcludedUserCheck bool                `mapstructure:"ENABLE_EXCLUDED_USER_CHECK"`
				EnableInvalidLogCheck   bool                `mapstructure:"ENABLE_INVALID_LOG_CHECK"`
				MaxValidationErrors     int                 `mapstructure:"MAX_VALIDATION_ERRORS"`
				TelcoPrefixes           map[string][]string `mapstructure:"TELCO_PREFIXES"`
			}{
				CacheExpiry:             30 * time.Minute,
				EnablePrefixValidation:  true,
				EnableExcludedUserCheck: true,
				EnableInvalidLogCheck:   true,
				MaxValidationErrors:     1000,
				TelcoPrefixes: map[string][]string{
					"AirtelTigo": {"233561", "233578", "233242", "233307", "233245"},
					"MTN":        {"233540", "233550", "233244", "233240"},
					"Vodafone":   {"233201", "233202", "233203"},
				},
			},
		},
	}

	// Test generating for Tigo
	msisdn, err := GenerateRandomMSISDN("tigo", cfg, mockRepo)
	assert.NoError(t, err)
	assert.NotEmpty(t, msisdn)
	assert.Equal(t, 12, len(msisdn))
	assert.True(t, strings.HasPrefix(msisdn, "233"))

	// Test generating for Airtel (should map to AirtelTigo)
	msisdn, err = GenerateRandomMSISDN("airtel", cfg, mockRepo)
	assert.NoError(t, err)
	assert.NotEmpty(t, msisdn)

	// Test batch generation
	batch, err := GenerateBatchMSISDNSConcurrently("tigo", 10, cfg, mockRepo)
	assert.NoError(t, err)
	assert.Equal(t, 10, len(batch))

	// Check uniqueness
	uniqueMap := make(map[string]bool)
	for _, m := range batch {
		uniqueMap[m] = true
	}
	assert.Equal(t, 10, len(uniqueMap), "Generated MSISDNs should be unique")
}

func TestMSISDNDataLoader(t *testing.T) {
	loader := NewMSISDNDataLoader()

	// Test with sample data
	testSamples := []string{
		"233561075653",
		"233561234567",
		"233578345678",
		"233242456789",
		"233307567890",
	}

	// Manually add samples for testing
	loader.samples = testSamples

	// Test GetSamples
	samples := loader.GetSamples()
	assert.Equal(t, 5, len(samples))

	// Test GetSamplesByPrefix
	prefix561 := loader.GetSamplesByPrefix("233561")
	assert.Equal(t, 2, len(prefix561))

	// Test GetUniquePrefixes
	prefixes := loader.GetUniquePrefixes()
	assert.Equal(t, 4, len(prefixes))
	assert.Contains(t, prefixes, "233561")
	assert.Contains(t, prefixes, "233578")
	assert.Contains(t, prefixes, "233242")
	assert.Contains(t, prefixes, "233307")

	// Test GetPrefixDistribution
	distribution := loader.GetPrefixDistribution()
	assert.Equal(t, 2, distribution["233561"])
	assert.Equal(t, 1, distribution["233578"])
	assert.Equal(t, 1, distribution["233242"])
	assert.Equal(t, 1, distribution["233307"])
}

func TestWeightedPrefixSelection(t *testing.T) {
	prefixes := []string{"233561", "233578", "233242", "233307", "233245"}

	// Test weighted selection multiple times
	distribution := make(map[string]int)
	for i := 0; i < 1000; i++ {
		idx, err := selectWeightedPrefix(prefixes)
		assert.NoError(t, err)
		assert.True(t, idx >= 0 && idx < len(prefixes))
		distribution[prefixes[idx]]++
	}

	// 233561 should have the highest count due to its weight
	assert.True(t, distribution["233561"] > distribution["233307"])
}

func TestPatternBasedGeneration(t *testing.T) {
	// Test generateFromPool
	prefix := "233561"

	// Add some samples to the pool
	LoadMSISDNSamples([]string{
		"233561075653",
		"233561234567",
		"233561345678",
	})

	msisdn, err := generateFromPool(prefix)
	assert.NoError(t, err)
	assert.NotEmpty(t, msisdn)
	assert.True(t, strings.HasPrefix(msisdn, prefix))
	assert.Equal(t, 12, len(msisdn))
}

func TestValidMSISDNFormat(t *testing.T) {
	testCases := []struct {
		msisdn   string
		expected bool
	}{
		{"233561075653", true},   // Valid Tigo number
		{"233540123456", true},   // Valid MTN number
		{"233201987654", true},   // Valid Vodafone number
		{"23356107565", false},   // Too short
		{"2335610756531", false}, // Too long
		{"123561075653", false},  // Wrong country code
		{"23356107565a", false},  // Contains letter
		{"", false},              // Empty
	}

	for _, tc := range testCases {
		result := isValidMSISDNFormat(tc.msisdn)
		assert.Equal(t, tc.expected, result, "Failed for MSISDN: %s", tc.msisdn)
	}
}

func TestPatternDetection(t *testing.T) {
	// Test sequential pattern
	assert.True(t, isSequential("123456"))
	assert.True(t, isSequential("234567"))
	assert.False(t, isSequential("135790"))

	// Test repeating pattern
	assert.True(t, isRepeating("111222"))
	assert.True(t, isRepeating("555666"))
	assert.False(t, isRepeating("123456"))

	// Test block pattern
	assert.True(t, isBlockPattern("123123"))
	assert.True(t, isBlockPattern("456457"))
	assert.False(t, isBlockPattern("123456"))
}

func BenchmarkGenerateMSISDN(b *testing.B) {
	mockRepo := new(MockUserBaseRepository)
	mockRepo.On("IsPremierOrStaff", mock.Anything).Return(false, nil)
	mockRepo.On("IsExcludedUser", mock.Anything).Return(false, nil)
	mockRepo.On("GetInvalidMSISDNS", mock.Anything, mock.Anything).Return([]string{}, nil)
	mockRepo.On("GetInvalidMSISDNSFast", mock.Anything, mock.Anything).Return(false, nil)
	mockRepo.On("LoadExclusionList").Return(map[string]bool{}, nil)
	mockRepo.On("GetInvalidMSISDNSOptimized", mock.Anything, mock.Anything).Return([]string{}, nil)
	mockRepo.On("GetBlacklistedMSISDNS", mock.Anything, mock.Anything).Return([]string{}, nil)

	cfg := &config.Config{
		Application: struct {
			Environment    config.Environment  `mapstructure:"ENVIRONMENT"`
			Port           int                 `mapstructure:"PORT"`
			AllowedOrigins []string            `mapstructure:"ALLOWED_ORIGINS"`
			TelcoPrefixes  map[string][]string `mapstructure:"TELCO_PREFIXES"`
			TIMWE          struct {
				Host                   string        `mapstructure:"HOST"`
				BaseURL                string        `mapstructure:"BASE_URL"`
				APIKey                 string        `mapstructure:"API_KEY"`
				MTAPIKey               string        `mapstructure:"MT_API_KEY"`
				Psk                    string        `mapstructure:"PSK"`
				PartnerServiceID       string        `mapstructure:"PARTNER_SERVICE_ID"`
				PartnerRoleID          string        `mapstructure:"PARTNER_ROLE_ID"`
				Realm                  string        `mapstructure:"REALM"`
				AuthenticationKey      string        `mapstructure:"AUTHENTICATION_KEY"`
				MCC                    string        `mapstructure:"MCC"`
				MNC                    string        `mapstructure:"MNC"`
				Timeout                time.Duration `mapstructure:"TIMEOUT"`
				MaxConnections         int           `mapstructure:"MAX_CONNECTIONS"`
				ChargeRetryMaxDuration time.Duration `mapstructure:"CHARGE_RETRY_MAX_DURATION"`
				ChargeRetryBaseDelay   time.Duration `mapstructure:"CHARGE_RETRY_BASE_DELAY"`
				ChargeRetryMaxDelay    time.Duration `mapstructure:"CHARGE_RETRY_MAX_DELAY"`
				CBMaxRequests          int           `mapstructure:"CB_MAX_REQUESTS"`
				CBTimeout              time.Duration `mapstructure:"CB_TIMEOUT"`
				CBInterval             time.Duration `mapstructure:"CB_INTERVAL"`
				CBMinRequests          int           `mapstructure:"CB_MIN_REQUESTS"`
				CBFailureRateThreshold float64       `mapstructure:"CB_FAILURE_RATE_THRESHOLD"`
				CBConsecutiveFailures  int           `mapstructure:"CB_CONSECUTIVE_FAILURES"`
			} `mapstructure:"TIMWE_MA"`
			HTTP struct {
				ReadTimeout      time.Duration `mapstructure:"READ_TIMEOUT"`
				WriteTimeout     time.Duration `mapstructure:"WRITE_TIMEOUT"`
				IdleTimeout      time.Duration `mapstructure:"IDLE_TIMEOUT"`
				MaxRequestBodyMB int           `mapstructure:"MAX_REQUEST_BODY_MB"`
				Concurrency      int           `mapstructure:"CONCURRENCY"`
			} `mapstructure:"HTTP"`
			Batch struct {
				MaxWorkersPerJob    int `mapstructure:"MAX_WORKERS_PER_JOB"`
				MaxConcurrentOptins int `mapstructure:"MAX_CONCURRENT_OPTINS"`
				TargetQPS           int `mapstructure:"TARGET_QPS"`
			} `mapstructure:"BATCH"`
			MSISDNGenerator struct {
				Enabled            bool          `mapstructure:"ENABLED"`
				BatchSize          int           `mapstructure:"BATCH_SIZE"`
				MaxConcurrent      int           `mapstructure:"MAX_CONCURRENT"`
				MaxMSISDNCount     int           `mapstructure:"MAX_MSISDN_COUNT"`
				CacheEnabled       bool          `mapstructure:"CACHE_ENABLED"`
				BloomFilterEnabled bool          `mapstructure:"BLOOM_FILTER_ENABLED"`
				FalsePositiveRate  float64       `mapstructure:"FALSE_POSITIVE_RATE"`
				ValidationTimeout  time.Duration `mapstructure:"VALIDATION_TIMEOUT"`
				GenerationTimeout  time.Duration `mapstructure:"GENERATION_TIMEOUT"`
				WorkerPoolSize     int           `mapstructure:"WORKER_POOL_SIZE"`
				ChannelBufferSize  int           `mapstructure:"CHANNEL_BUFFER_SIZE"`
				FallbackToDatabase bool          `mapstructure:"FALLBACK_TO_DATABASE"`
				MaxRetryAttempts   int           `mapstructure:"MAX_RETRY_ATTEMPTS"`
				PreloadEnabled     bool          `mapstructure:"PRELOAD_ENABLED"`
				PreloadBatchSize   int           `mapstructure:"PRELOAD_BATCH_SIZE"`
			} `mapstructure:"MSISDN_GENERATOR"`
			MSISDNValidation struct {
				CacheExpiry             time.Duration       `mapstructure:"CACHE_EXPIRY"`
				EnablePrefixValidation  bool                `mapstructure:"ENABLE_PREFIX_VALIDATION"`
				EnableExcludedUserCheck bool                `mapstructure:"ENABLE_EXCLUDED_USER_CHECK"`
				EnableInvalidLogCheck   bool                `mapstructure:"ENABLE_INVALID_LOG_CHECK"`
				MaxValidationErrors     int                 `mapstructure:"MAX_VALIDATION_ERRORS"`
				TelcoPrefixes           map[string][]string `mapstructure:"TELCO_PREFIXES"`
			} `mapstructure:"MSISDN_VALIDATION"`
			NetworkResilience struct {
				MaxRetries              int           `mapstructure:"MAX_RETRIES"`
				BaseRetryDelay          time.Duration `mapstructure:"BASE_RETRY_DELAY"`
				MaxRetryDelay           time.Duration `mapstructure:"MAX_RETRY_DELAY"`
				ConnectionTimeout       time.Duration `mapstructure:"CONNECTION_TIMEOUT"`
				ReadTimeout             time.Duration `mapstructure:"READ_TIMEOUT"`
				WriteTimeout            time.Duration `mapstructure:"WRITE_TIMEOUT"`
				MaxConnsPerHost         int           `mapstructure:"MAX_CONNS_PER_HOST"`
				MaxIdleConnDuration     time.Duration `mapstructure:"MAX_IDLE_CONN_DURATION"`
				CircuitBreakerThreshold int           `mapstructure:"CIRCUIT_BREAKER_THRESHOLD"`
				CircuitBreakerTimeout   time.Duration `mapstructure:"CIRCUIT_BREAKER_TIMEOUT"`
				JitterEnabled           bool          `mapstructure:"JITTER_ENABLED"`
			} `mapstructure:"NETWORK_RESILIENCE"`
			EnhancedMonitoring struct {
				EnableAutomatedRecovery bool          `mapstructure:"ENABLE_AUTOMATED_RECOVERY"`
				RecoveryCooldown        time.Duration `mapstructure:"RECOVERY_COOLDOWN"`
				MaxRecoveryAttempts     int           `mapstructure:"MAX_RECOVERY_ATTEMPTS"`
				HealthCheckInterval     time.Duration `mapstructure:"HEALTH_CHECK_INTERVAL"`
				AlertCooldown           time.Duration `mapstructure:"ALERT_COOLDOWN"`
				EnableRealTimeMetrics   bool          `mapstructure:"ENABLE_REAL_TIME_METRICS"`
			} `mapstructure:"ENHANCED_MONITORING"`
			Log struct {
				Path    string `mapstructure:"PATH"`
				Rolling struct {
					Enabled           bool `mapstructure:"ENABLED"`
					MaxSize           int  `mapstructure:"MAX_SIZE"`
					MaxAge            int  `mapstructure:"MAX_AGE"`
					MaxBackups        int  `mapstructure:"MAX_BACKUPS"`
					Compress          bool `mapstructure:"COMPRESS"`
					CompressThreshold int  `mapstructure:"COMPRESS_THRESHOLD"`
					LocalTime         bool `mapstructure:"LOCAL_TIME"`
				} `mapstructure:"ROLLING"`
			}
			Key struct {
				Default string `mapstructure:"DEFAULT"`
				Rsa     struct {
					Public  string `mapstructure:"PUBLIC"`
					Private string `mapstructure:"PRIVATE"`
				}
			}
			Graceful struct {
				MaxSecond time.Duration `mapstructure:"MAX_SECOND"`
			} `mapstructure:"GRACEFUL"`
		}{
			TelcoPrefixes: map[string][]string{
				"AirtelTigo": {"233561", "233578", "233242"},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GenerateRandomMSISDN("tigo", cfg, mockRepo)
	}
}

// Example usage function showing how to integrate the enhanced generator
func ExampleUsage() {
	// Load real Tigo userbase data
	err := LoadTigoUserbaseData("/path/to/Tigo_Userbase.csv")
	if err != nil {
		fmt.Printf("Failed to load userbase data: %v\n", err)
		// Generator will still work without loaded data
	}

	// Analyze patterns in the data (optional)
	analysis, err := AnalyzeMSISDNPatterns("/path/to/Tigo_Userbase.csv")
	if err == nil {
		fmt.Printf("Loaded %d samples with %d unique prefixes\n",
			analysis.TotalSamples, len(analysis.UniquePrefixes))
		fmt.Printf("Prefix distribution:\n")
		for prefix, count := range analysis.PrefixDistribution {
			fmt.Printf("  %s: %d\n", prefix, count)
		}
	}

	// Now use the generator as normal
	// It will use the loaded patterns for more realistic generation
}

// Usage demonstrates how to use the MSISDN generator
func Usage() {
	ExampleUsage()
}
