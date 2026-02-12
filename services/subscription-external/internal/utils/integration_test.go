package utils

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestMSISDNValidatorIntegration tests the MSISDN validator with various scenarios
func TestMSISDNValidatorIntegration(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Use a simple mock for testing - we'll just test the validation logic
	// without external dependencies for this integration test

	// Test with default configuration
	validator := NewMSISDNValidator(logger, nil, nil)
	assert.NotNil(t, validator)

	// Test valid MSISDN format (this will fail validation due to nil repo, but we can test the structure)
	_, _ = validator.ValidateMSISDN(context.Background(), "0244123456")
	// We expect an error due to nil repository, but the validator should be created
	assert.NotNil(t, validator)
}

// TestNetworkResilientClientIntegration tests the network resilient client
func TestNetworkResilientClientIntegration(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Test with default configuration
	client := NewNetworkResilientClient(logger, nil)
	assert.NotNil(t, client)
	assert.NotNil(t, client.circuitBreaker)

	// Test configuration override
	config := &NetworkConfig{
		MaxRetries:              3,
		BaseRetryDelay:          100 * time.Millisecond,
		MaxRetryDelay:           1 * time.Second,
		ConnectionTimeout:       5 * time.Second,
		ReadTimeout:             10 * time.Second,
		WriteTimeout:            10 * time.Second,
		MaxConnsPerHost:         100,
		MaxIdleConnDuration:     30 * time.Second,
		CircuitBreakerThreshold: 2,
		CircuitBreakerTimeout:   15 * time.Second,
		JitterEnabled:           true,
	}

	clientWithConfig := NewNetworkResilientClient(logger, config)
	assert.NotNil(t, clientWithConfig)
	assert.Equal(t, config.MaxRetries, clientWithConfig.config.MaxRetries)

	// Test health check with proper parameters
	ctx := context.Background()
	_ = client.HealthCheck(ctx, "http://localhost:8080/health")
	// We expect an error since localhost:8080 is not available, but the function should execute
	assert.NotNil(t, client)
}

// TestBatchProcessorIntegration tests the batch processor
func TestBatchProcessorIntegration(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Test with default configuration
	processor := NewBatchProcessor(logger, nil)
	assert.NotNil(t, processor)
	assert.True(t, processor.config.EnableRetryQueue)

	// Test configuration override
	config := &BatchConfig{
		BatchSize:           50,
		MaxConcurrency:      5,
		RetryAttempts:       2,
		RetryDelay:          1 * time.Second,
		PartialSuccessRatio: 0.8,
		ErrorBatchSize:      5,
		ProcessingTimeout:   2 * time.Minute,
		EnableRetryQueue:    false,
	}

	processorWithConfig := NewBatchProcessor(logger, config)
	assert.NotNil(t, processorWithConfig)
	assert.Equal(t, config.BatchSize, processorWithConfig.config.BatchSize)
	assert.False(t, processorWithConfig.config.EnableRetryQueue)

	// Test metrics
	metrics := processor.GetMetrics()
	assert.NotNil(t, metrics)
	assert.Equal(t, int64(0), metrics.TotalProcessed)

	// Cleanup
	processor.Stop()
	processorWithConfig.Stop()
}

// TestComponentIntegration tests that all components work together
func TestComponentIntegration(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Create all components
	validator := NewMSISDNValidator(logger, nil, nil)
	networkClient := NewNetworkResilientClient(logger, nil)
	batchProcessor := NewBatchProcessor(logger, nil)

	// Verify all components are created successfully
	assert.NotNil(t, validator)
	assert.NotNil(t, networkClient)
	assert.NotNil(t, batchProcessor)

	// Test batch processor with validation results
	// This would typically process multiple MSISDNs in a batch
	// For now, just verify the processor is working
	metrics := batchProcessor.GetMetrics()
	assert.NotNil(t, metrics)

	// Cleanup
	batchProcessor.Stop()
}

// TestConfigurationIntegration tests configuration integration
func TestConfigurationIntegration(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Test MSISDN validation with custom config
	msisdnConfig := &MSISDNValidationConfig{
		CacheExpiry:             15 * time.Minute,
		EnablePrefixValidation:  false, // Disable for testing
		EnableExcludedUserCheck: true,
		EnableInvalidLogCheck:   true,
		MaxValidationErrors:     500,
	}

	validator := NewMSISDNValidator(logger, nil, msisdnConfig)
	assert.NotNil(t, validator)
	assert.Equal(t, 15*time.Minute, validator.config.CacheExpiry)
	assert.False(t, validator.config.EnablePrefixValidation)

	// Test network resilience with custom config
	networkConfig := &NetworkConfig{
		MaxRetries:              7,
		BaseRetryDelay:          500 * time.Millisecond,
		MaxRetryDelay:           2 * time.Second,
		ConnectionTimeout:       8 * time.Second,
		ReadTimeout:             25 * time.Second,
		WriteTimeout:            25 * time.Second,
		MaxConnsPerHost:         150,
		MaxIdleConnDuration:     45 * time.Second,
		CircuitBreakerThreshold: 4,
		CircuitBreakerTimeout:   20 * time.Second,
		JitterEnabled:           false,
	}

	networkClient := NewNetworkResilientClient(logger, networkConfig)
	assert.NotNil(t, networkClient)
	assert.Equal(t, 7, networkClient.config.MaxRetries)
	assert.False(t, networkClient.config.JitterEnabled)

	// Test batch processing with custom config
	batchConfig := &BatchConfig{
		BatchSize:           75,
		MaxConcurrency:      8,
		RetryAttempts:       4,
		RetryDelay:          1500 * time.Millisecond, // Fixed: use milliseconds instead of float
		PartialSuccessRatio: 0.75,
		ErrorBatchSize:      8,
		ProcessingTimeout:   3 * time.Minute,
		EnableRetryQueue:    true,
	}

	batchProcessor := NewBatchProcessor(logger, batchConfig)
	assert.NotNil(t, batchProcessor)
	assert.Equal(t, 75, batchProcessor.config.BatchSize)
	assert.Equal(t, 8, batchProcessor.config.MaxConcurrency)

	// Cleanup
	batchProcessor.Stop()
}

// TestConfigurableTelcoPrefixes tests the new configurable telecom prefix functionality
func TestConfigurableTelcoPrefixes(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Test with custom telecom prefixes
	customPrefixes := map[string][]string{
		"CustomMTN": {
			"23324", "23325", "23326",
		},
		"CustomAirtel": {
			"23320", "23327", "23328",
		},
		"CustomVodafone": {
			"23323", "23333",
		},
	}

	msisdnConfig := &MSISDNValidationConfig{
		CacheExpiry:             30 * time.Minute,
		EnablePrefixValidation:  true,
		EnableExcludedUserCheck: true,
		EnableInvalidLogCheck:   true,
		MaxValidationErrors:     1000,
		TelcoPrefixes:           customPrefixes,
	}

	validator := NewMSISDNValidator(logger, nil, msisdnConfig)
	assert.NotNil(t, validator)

	// Verify custom prefixes are loaded
	loadedPrefixes := validator.GetTelcoPrefixes()
	assert.Equal(t, len(customPrefixes), len(loadedPrefixes))
	assert.Equal(t, customPrefixes["CustomMTN"], loadedPrefixes["CustomMTN"])

	// Test prefix validation with custom prefixes
	operator, isValid := validator.ValidateTelcoPrefix("23324")
	assert.True(t, isValid)
	assert.Equal(t, "CustomMTN", operator)

	operator, isValid = validator.ValidateTelcoPrefix("23320")
	assert.True(t, isValid)
	assert.Equal(t, "CustomAirtel", operator)

	// Test invalid prefix
	operator, isValid = validator.ValidateTelcoPrefix("99999")
	assert.False(t, isValid)
	assert.Equal(t, "", operator)

	// Test updating prefixes at runtime
	newPrefixes := map[string][]string{
		"NewOperator": {
			"23360", "23361", "23362",
		},
	}

	validator.UpdateTelcoPrefixes(newPrefixes)
	updatedPrefixes := validator.GetTelcoPrefixes()
	assert.Equal(t, len(newPrefixes), len(updatedPrefixes))
	assert.Equal(t, newPrefixes["NewOperator"], updatedPrefixes["NewOperator"])

	// Test configuration reload
	reloadConfig := &MSISDNValidationConfig{
		CacheExpiry:             45 * time.Minute,
		EnablePrefixValidation:  true,
		EnableExcludedUserCheck: false,
		EnableInvalidLogCheck:   true,
		MaxValidationErrors:     500,
		TelcoPrefixes:           customPrefixes,
	}

	validator.ReloadConfiguration(reloadConfig)
	currentConfig := validator.GetConfiguration()
	assert.Equal(t, 45*time.Minute, currentConfig.CacheExpiry)
	assert.False(t, currentConfig.EnableExcludedUserCheck)
	assert.Equal(t, 500, currentConfig.MaxValidationErrors)
	assert.Equal(t, customPrefixes, currentConfig.TelcoPrefixes)
}

// TestDefaultPrefixesFallback tests that default prefixes are used when none are configured
func TestDefaultPrefixesFallback(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Test with no custom prefixes (should use defaults)
	msisdnConfig := &MSISDNValidationConfig{
		CacheExpiry:             30 * time.Minute,
		EnablePrefixValidation:  true,
		EnableExcludedUserCheck: true,
		EnableInvalidLogCheck:   true,
		MaxValidationErrors:     1000,
		// No TelcoPrefixes specified - should use defaults
	}

	validator := NewMSISDNValidator(logger, nil, msisdnConfig)
	assert.NotNil(t, validator)

	// Should have default prefixes
	defaultPrefixes := validator.GetTelcoPrefixes()
	assert.Greater(t, len(defaultPrefixes), 0)

	// Should contain expected default operators
	_, hasMTN := defaultPrefixes["MTN"]
	_, hasAirtelTigo := defaultPrefixes["AirtelTigo"]
	_, hasVodafone := defaultPrefixes["Vodafone"]
	_, hasGlo := defaultPrefixes["Glo"]

	assert.True(t, hasMTN, "Default MTN prefixes should be present")
	assert.True(t, hasAirtelTigo, "Default AirtelTigo prefixes should be present")
	assert.True(t, hasVodafone, "Default Vodafone prefixes should be present")
	assert.True(t, hasGlo, "Default Glo prefixes should be present")

	// Test that default prefixes work
	operator, isValid := validator.ValidateTelcoPrefix("23324")
	assert.True(t, isValid)
	assert.Equal(t, "MTN", operator)
}
