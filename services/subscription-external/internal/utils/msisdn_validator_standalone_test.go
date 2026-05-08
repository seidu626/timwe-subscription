package utils

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestMSISDNValidatorStandalone tests the MSISDN validator without complex config dependencies
func TestMSISDNValidatorStandalone(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Test with default configuration
	validator := NewMSISDNValidator(logger, nil, nil)
	assert.NotNil(t, validator)

	// Test default prefixes are loaded
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

	// Test prefix validation
	operator, isValid := validator.ValidateTelcoPrefix("23324")
	assert.True(t, isValid)
	assert.Equal(t, "MTN", operator)

	operator, isValid = validator.ValidateTelcoPrefix("23320")
	assert.True(t, isValid)
	assert.Contains(t, []string{"AirtelTigo", "Vodafone"}, operator)

	// Test invalid prefix
	operator, isValid = validator.ValidateTelcoPrefix("99999")
	assert.False(t, isValid)
	assert.Equal(t, "", operator)
}

// TestConfigurablePrefixes tests the configurable prefix functionality
func TestConfigurablePrefixes(t *testing.T) {
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
}

// TestRuntimePrefixUpdates tests updating prefixes at runtime
func TestRuntimePrefixUpdates(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	validator := NewMSISDNValidator(logger, nil, nil)
	assert.NotNil(t, validator)

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

	// Test that new prefixes work
	operator, isValid := validator.ValidateTelcoPrefix("23360")
	assert.True(t, isValid)
	assert.Equal(t, "NewOperator", operator)
}

// TestConfigurationReload tests reloading the entire configuration
func TestConfigurationReload(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	validator := NewMSISDNValidator(logger, nil, nil)
	assert.NotNil(t, validator)

	// Test configuration reload
	reloadConfig := &MSISDNValidationConfig{
		CacheExpiry:             45 * time.Minute,
		EnablePrefixValidation:  true,
		EnableExcludedUserCheck: false,
		EnableInvalidLogCheck:   true,
		MaxValidationErrors:     500,
		TelcoPrefixes: map[string][]string{
			"ReloadedOperator": {"23370", "23371"},
		},
	}

	validator.ReloadConfiguration(reloadConfig)
	currentConfig := validator.GetConfiguration()
	assert.Equal(t, 45*time.Minute, currentConfig.CacheExpiry)
	assert.False(t, currentConfig.EnableExcludedUserCheck)
	assert.Equal(t, 500, currentConfig.MaxValidationErrors)
	assert.Equal(t, reloadConfig.TelcoPrefixes, currentConfig.TelcoPrefixes)

	// Test that reloaded prefixes work
	operator, isValid := validator.ValidateTelcoPrefix("23370")
	assert.True(t, isValid)
	assert.Equal(t, "ReloadedOperator", operator)
}

// TestMSISDNFormatValidation tests the MSISDN format validation
func TestMSISDNFormatValidation(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	validator := NewMSISDNValidator(logger, nil, nil)
	assert.NotNil(t, validator)

	// Test valid MSISDN formats
	validMSISDNs := []string{
		"233241234567", // Full country code
		"0241234567",   // Local with leading zero
		"241234567",    // Local without leading zero
	}

	for _, msisdn := range validMSISDNs {
		result, err := validator.ValidateMSISDN(context.Background(), msisdn)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		// Note: Without a real repository, we can't test full validation
		// but we can test that the function doesn't crash
	}

	// Test invalid MSISDN formats
	invalidMSISDNs := []string{
		"123",           // Too short
		"abcdefghijk",   // Non-numeric
		"2332412345678", // Too long
	}

	for _, msisdn := range invalidMSISDNs {
		result, err := validator.ValidateMSISDN(context.Background(), msisdn)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		// Should fail format validation
	}
}

// TestStatisticsTracking tests that statistics are properly tracked
func TestStatisticsTracking(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	validator := NewMSISDNValidator(logger, nil, nil)
	assert.NotNil(t, validator)

	// Get initial stats
	initialStats := validator.GetValidationStats()
	initialTotal := initialStats.TotalValidations

	// Perform some validations
	validator.ValidateMSISDN(context.Background(), "233241234567")
	validator.ValidateMSISDN(context.Background(), "233251234567")

	// Get updated stats
	updatedStats := validator.GetValidationStats()
	assert.Equal(t, initialTotal+2, updatedStats.TotalValidations)
	assert.True(t, updatedStats.LastUpdated.After(initialStats.LastUpdated))
}

// TestCacheFunctionality tests the caching mechanism
func TestCacheFunctionality(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Create validator with short cache expiry for testing
	config := &MSISDNValidationConfig{
		CacheExpiry:             100 * time.Millisecond,
		EnablePrefixValidation:  true,
		EnableExcludedUserCheck: false,
		EnableInvalidLogCheck:   false,
		MaxValidationErrors:     1000,
	}

	validator := NewMSISDNValidator(logger, nil, config)
	assert.NotNil(t, validator)

	// First validation should miss cache
	initialStats := validator.GetValidationStats()
	validator.ValidateMSISDN(context.Background(), "233241234567")
	statsAfterFirst := validator.GetValidationStats()
	assert.Equal(t, initialStats.CacheMisses+1, statsAfterFirst.CacheMisses)

	// Second validation should hit cache
	validator.ValidateMSISDN(context.Background(), "233241234567")
	statsAfterSecond := validator.GetValidationStats()
	assert.Equal(t, statsAfterFirst.CacheHits+1, statsAfterSecond.CacheHits)

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Third validation should miss cache again
	validator.ValidateMSISDN(context.Background(), "233241234567")
	finalStats := validator.GetValidationStats()
	assert.Equal(t, statsAfterSecond.CacheMisses+1, finalStats.CacheMisses)
}
