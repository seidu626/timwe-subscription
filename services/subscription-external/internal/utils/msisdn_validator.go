package utils

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"sync"

	"github.com/seidu626/subscription-manager/subscription-external/internal/repository"
	"go.uber.org/zap"
)

// GhanaTelecomPrefixes defines default valid prefixes for Ghana telecom operators
// These are used as fallback if no prefixes are configured
var DefaultGhanaTelecomPrefixes = map[string][]string{
	"MTN": {
		"23324", "23325", "23354", "23355", "23359",
	},
	"AirtelTigo": {
		"23320", "23327", "23328", "23356", "23357", "23350",
		"23326", "23346", "23347", "23348", "23349",
	},
	"Vodafone": {
		"23323", "23333", "23320", "23350", "23351", "23352", "23353",
	},
	"Glo": {
		"23323", "23358",
	},
}

// MSISDNValidator provides comprehensive MSISDN validation
type MSISDNValidator struct {
	logger        *zap.Logger
	repo          repository.UserBaseRepositoryInterface
	cache         *MSISDNValidationCache
	config        *MSISDNValidationConfig
	stats         *MSISDNValidationStats
	telcoPrefixes map[string][]string // Configurable telecom prefixes
}

// MSISDNValidationCache provides caching for validation results
type MSISDNValidationCache struct {
	validCache   map[string]time.Time
	invalidCache map[string]time.Time
	cacheExpiry  time.Duration
	mu           sync.RWMutex // Protect concurrent access to cache maps
}

// ValidationResult contains the result of MSISDN validation
type ValidationResult struct {
	IsValid         bool
	ErrorReason     string
	Operator        string
	FormattedMSISDN string
}

// MSISDNValidationConfig provides configuration for MSISDN validation
type MSISDNValidationConfig struct {
	CacheExpiry             time.Duration
	EnablePrefixValidation  bool
	EnableExcludedUserCheck bool
	EnableInvalidLogCheck   bool
	MaxValidationErrors     int
	TelcoPrefixes           map[string][]string // Optional: Override default prefixes
}

// MSISDNValidationStats tracks validation performance and outcomes
type MSISDNValidationStats struct {
	TotalValidations     int64
	ValidMSISDNs         int64
	InvalidMSISDNs       int64
	ValidationErrors     int64
	CacheHits            int64
	CacheMisses          int64
	PreventedAPICalls    int64
	ValidationLatency    time.Duration
	InvalidReasons       map[string]int64
	OperatorDistribution map[string]int64
	LastUpdated          time.Time
	mu                   sync.RWMutex
}

// DefaultMSISDNValidationConfig returns sensible defaults
func DefaultMSISDNValidationConfig() *MSISDNValidationConfig {
	return &MSISDNValidationConfig{
		CacheExpiry:             30 * time.Minute,
		EnablePrefixValidation:  true,
		EnableExcludedUserCheck: true,
		EnableInvalidLogCheck:   true,
		MaxValidationErrors:     1000,
	}
}

// NewMSISDNValidator creates a new MSISDN validator
func NewMSISDNValidator(logger *zap.Logger, repo repository.UserBaseRepositoryInterface, config *MSISDNValidationConfig) *MSISDNValidator {
	if config == nil {
		config = DefaultMSISDNValidationConfig()
	}

	// Initialize with default prefixes
	telcoPrefixes := DefaultGhanaTelecomPrefixes

	// Override with configured prefixes if provided
	if config.TelcoPrefixes != nil && len(config.TelcoPrefixes) > 0 {
		telcoPrefixes = config.TelcoPrefixes
		logger.Info("Using configured telecom prefixes",
			zap.Int("operator_count", len(config.TelcoPrefixes)))
	} else {
		logger.Info("Using default Ghana telecom prefixes",
			zap.Int("operator_count", len(DefaultGhanaTelecomPrefixes)))
	}

	return &MSISDNValidator{
		logger: logger,
		repo:   repo,
		config: config,
		cache: &MSISDNValidationCache{
			validCache:   make(map[string]time.Time),
			invalidCache: make(map[string]time.Time),
			cacheExpiry:  config.CacheExpiry,
			mu:           sync.RWMutex{},
		},
		stats: &MSISDNValidationStats{
			InvalidReasons:       make(map[string]int64),
			OperatorDistribution: make(map[string]int64),
			LastUpdated:          time.Now(),
		},
		telcoPrefixes: telcoPrefixes,
	}
}

// UpdateTelcoPrefixes updates the telecom prefixes from configuration
func (v *MSISDNValidator) UpdateTelcoPrefixes(prefixes map[string][]string) {
	v.stats.mu.Lock()
	defer v.stats.mu.Unlock()

	if prefixes != nil && len(prefixes) > 0 {
		v.telcoPrefixes = prefixes
		v.logger.Info("Updated telecom prefixes from configuration",
			zap.Int("operator_count", len(prefixes)))

		// Log the new prefixes for debugging
		for operator, operatorPrefixes := range prefixes {
			v.logger.Debug("Operator prefixes updated",
				zap.String("operator", operator),
				zap.Strings("prefixes", operatorPrefixes))
		}
	} else {
		// Fallback to default prefixes
		v.telcoPrefixes = DefaultGhanaTelecomPrefixes
		v.logger.Info("Falling back to default Ghana telecom prefixes")
	}
}

// GetTelcoPrefixes returns the current telecom prefix configuration
func (v *MSISDNValidator) GetTelcoPrefixes() map[string][]string {
	v.stats.mu.RLock()
	defer v.stats.mu.RUnlock()

	// Return a copy to avoid race conditions
	prefixes := make(map[string][]string)
	for operator, operatorPrefixes := range v.telcoPrefixes {
		prefixes[operator] = make([]string, len(operatorPrefixes))
		copy(prefixes[operator], operatorPrefixes)
	}

	return prefixes
}

// ValidateTelcoPrefix validates if a given prefix is valid for any operator
func (v *MSISDNValidator) ValidateTelcoPrefix(prefix string) (string, bool) {
	v.stats.mu.RLock()
	defer v.stats.mu.RUnlock()

	for operator, prefixes := range v.telcoPrefixes {
		for _, validPrefix := range prefixes {
			if validPrefix == prefix {
				return operator, true
			}
		}
	}
	return "", false
}

// ValidateMSISDN validates an MSISDN with comprehensive checks
func (v *MSISDNValidator) ValidateMSISDN(ctx context.Context, msisdn string) (*ValidationResult, error) {
	// Add timeout to context if not already present
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	startTime := time.Now()
	defer func() {
		v.updateStats(msisdn, time.Since(startTime))
	}()

	// Check cache first
	if cached := v.getCachedResult(msisdn); cached != nil {
		v.stats.mu.Lock()
		v.stats.CacheHits++
		v.stats.mu.Unlock()
		return cached, nil
	}

	v.stats.mu.Lock()
	v.stats.CacheMisses++
	v.stats.mu.Unlock()

	// Initialize result
	result := &ValidationResult{
		IsValid:         false,
		FormattedMSISDN: msisdn,
		Operator:        "Unknown",
		ErrorReason:     "",
	}

	// Step 1: Format validation
	formattedMSISDN, err := v.formatMSISDN(msisdn)
	if err != nil {
		result.IsValid = false
		result.ErrorReason = fmt.Sprintf("Format validation failed: %v", err)
		v.cacheResult(msisdn, result)
		v.recordInvalidReason("format_validation_failed")
		return result, nil
	}
	result.FormattedMSISDN = formattedMSISDN

	// Step 2: Ghana telecom prefix validation (if enabled)
	if v.config.EnablePrefixValidation {
		operator, isValidPrefix := v.validateGhanaTelecomPrefix(formattedMSISDN)
		if !isValidPrefix {
			result.IsValid = false
			result.ErrorReason = "Invalid Ghana telecom prefix"
			v.cacheResult(msisdn, result)
			v.recordInvalidReason("invalid_ghana_prefix")
			return result, nil
		}
		result.Operator = operator
		v.recordOperatorDistribution(operator)
	}

	// Step 3: Check against excluded users (if enabled)
	if v.config.EnableExcludedUserCheck && v.repo != nil {
		isExcluded, err := v.repo.IsExcludedUser(formattedMSISDN)
		if err != nil {
			v.logger.Warn("Failed to check excluded user status",
				zap.String("msisdn", formattedMSISDN),
				zap.Error(err))
			// Don't fail validation due to database errors, but log the issue
		} else if isExcluded {
			result.IsValid = false
			result.ErrorReason = "MSISDN is excluded (Premier/Staff/Blacklisted)"
			v.cacheResult(msisdn, result)
			v.recordInvalidReason("excluded_user")
			return result, nil
		}
	}

	// Step 4: Check against invalid MSISDN logs (if enabled)
	if v.config.EnableInvalidLogCheck && v.repo != nil {
		if fastRepo, ok := v.repo.(interface {
			GetInvalidMSISDNSFast(context.Context, string) (bool, error)
		}); ok {
			invalid, err := fastRepo.GetInvalidMSISDNSFast(ctx, formattedMSISDN)
			if err != nil {
				v.logger.Warn("Failed to check invalid MSISDN logs",
					zap.String("msisdn", formattedMSISDN),
					zap.Error(err))
			} else if invalid {
				result.IsValid = false
				result.ErrorReason = "MSISDN found in invalid logs"
				v.cacheResult(msisdn, result)
				v.recordInvalidReason("found_in_invalid_logs")
				return result, nil
			}
		}
	}

	// All validations passed
	result.IsValid = true
	v.cacheResult(msisdn, result)
	return result, nil
}

// formatMSISDN ensures MSISDN is in correct format (233xxxxxxxxx)
func (v *MSISDNValidator) formatMSISDN(msisdn string) (string, error) {
	// Remove any non-digit characters
	re := regexp.MustCompile(`\D`)
	cleanMSISDN := re.ReplaceAllString(msisdn, "")

	// Handle different input formats
	switch {
	case strings.HasPrefix(cleanMSISDN, "233"):
		// Already has country code
		if len(cleanMSISDN) != 12 {
			return "", fmt.Errorf("invalid length for MSISDN with country code: %d digits", len(cleanMSISDN))
		}
		return cleanMSISDN, nil

	case strings.HasPrefix(cleanMSISDN, "0"):
		// Local format with leading zero (0xxxxxxxxx)
		if len(cleanMSISDN) != 10 {
			return "", fmt.Errorf("invalid length for local MSISDN: %d digits", len(cleanMSISDN))
		}
		// Remove leading zero and add country code
		return "233" + cleanMSISDN[1:], nil

	case len(cleanMSISDN) == 9:
		// Local format without leading zero (xxxxxxxxx)
		return "233" + cleanMSISDN, nil

	default:
		return "", fmt.Errorf("unrecognized MSISDN format: %s", cleanMSISDN)
	}
}

// validateGhanaTelecomPrefix checks if MSISDN has valid Ghana telecom prefix
func (v *MSISDNValidator) validateGhanaTelecomPrefix(msisdn string) (string, bool) {
	if len(msisdn) < 7 {
		return "", false
	}

	// Check 5-digit prefixes first (more specific)
	prefix5 := msisdn[:5]
	for operator, prefixes := range v.telcoPrefixes {
		for _, validPrefix := range prefixes {
			if len(validPrefix) == 5 && prefix5 == validPrefix {
				return operator, true
			}
		}
	}

	// Check 4-digit prefixes
	prefix4 := msisdn[:4]
	for operator, prefixes := range v.telcoPrefixes {
		for _, validPrefix := range prefixes {
			if len(validPrefix) == 4 && prefix4 == validPrefix {
				return operator, true
			}
		}
	}

	return "", false
}

// getCachedResult retrieves cached validation result
func (v *MSISDNValidator) getCachedResult(msisdn string) *ValidationResult {
	now := time.Now()

	// Check valid cache
	v.cache.mu.RLock()
	validTime, exists := v.cache.validCache[msisdn]
	if exists && now.Sub(validTime) < v.cache.cacheExpiry {
		v.cache.mu.RUnlock()
		return &ValidationResult{
			IsValid:         true,
			FormattedMSISDN: msisdn,
		}
	}
	if exists {
		// Need to delete expired entry - upgrade to write lock
		v.cache.mu.RUnlock()
		v.cache.mu.Lock()
		delete(v.cache.validCache, msisdn)
		v.cache.mu.Unlock()
	} else {
		v.cache.mu.RUnlock()
	}

	// Check invalid cache
	v.cache.mu.RLock()
	invalidTime, exists := v.cache.invalidCache[msisdn]
	if exists && now.Sub(invalidTime) < v.cache.cacheExpiry {
		v.cache.mu.RUnlock()
		return &ValidationResult{
			IsValid:     false,
			ErrorReason: "Cached invalid result",
		}
	}
	if exists {
		// Need to delete expired entry - upgrade to write lock
		v.cache.mu.RUnlock()
		v.cache.mu.Lock()
		delete(v.cache.invalidCache, msisdn)
		v.cache.mu.Unlock()
	} else {
		v.cache.mu.RUnlock()
	}

	return nil
}

// updateStats updates validation statistics
func (v *MSISDNValidator) updateStats(msisdn string, latency time.Duration) {
	v.stats.mu.Lock()
	defer v.stats.mu.Unlock()

	v.stats.TotalValidations++
	v.stats.ValidationLatency = latency
	v.stats.LastUpdated = time.Now()
}

// recordInvalidReason records the reason for invalid MSISDN
func (v *MSISDNValidator) recordInvalidReason(reason string) {
	v.stats.mu.Lock()
	defer v.stats.mu.Unlock()

	v.stats.InvalidMSISDNs++
	v.stats.InvalidReasons[reason]++
}

// recordOperatorDistribution records operator distribution for valid MSISDNs
func (v *MSISDNValidator) recordOperatorDistribution(operator string) {
	if operator == "" {
		return
	}

	v.stats.mu.Lock()
	defer v.stats.mu.Unlock()

	v.stats.ValidMSISDNs++
	v.stats.OperatorDistribution[operator]++
}

// cacheResult stores validation result in cache
func (v *MSISDNValidator) cacheResult(msisdn string, result *ValidationResult) {
	now := time.Now()
	v.cache.mu.Lock()
	defer v.cache.mu.Unlock()

	if result.IsValid {
		v.cache.validCache[msisdn] = now
	} else {
		v.cache.invalidCache[msisdn] = now
	}

	// Clean up old cache entries periodically
	v.cleanupCache()
}

// cleanupCache removes expired cache entries
// NOTE: This method assumes the caller holds the cache lock (v.cache.mu)
func (v *MSISDNValidator) cleanupCache() {
	now := time.Now()

	// Clean valid cache
	for msisdn, timestamp := range v.cache.validCache {
		if now.Sub(timestamp) > v.cache.cacheExpiry {
			delete(v.cache.validCache, msisdn)
		}
	}

	// Clean invalid cache
	for msisdn, timestamp := range v.cache.invalidCache {
		if now.Sub(timestamp) > v.cache.cacheExpiry {
			delete(v.cache.invalidCache, msisdn)
		}
	}
}

// BatchValidateMSISDNs validates multiple MSISDNs efficiently
func (v *MSISDNValidator) BatchValidateMSISDNs(ctx context.Context, msisdns []string) (map[string]*ValidationResult, error) {
	results := make(map[string]*ValidationResult)

	for _, msisdn := range msisdns {
		result, err := v.ValidateMSISDN(ctx, msisdn)
		if err != nil {
			return nil, fmt.Errorf("failed to validate MSISDN %s: %w", msisdn, err)
		}
		results[msisdn] = result
	}

	return results, nil
}

// GetValidationStats returns current validation statistics
func (v *MSISDNValidator) GetValidationStats() *MSISDNValidationStats {
	v.stats.mu.RLock()
	defer v.stats.mu.RUnlock()

	// Create a copy to avoid race conditions
	stats := &MSISDNValidationStats{
		TotalValidations:     v.stats.TotalValidations,
		ValidMSISDNs:         v.stats.ValidMSISDNs,
		InvalidMSISDNs:       v.stats.InvalidMSISDNs,
		ValidationErrors:     v.stats.ValidationErrors,
		CacheHits:            v.stats.CacheHits,
		CacheMisses:          v.stats.CacheMisses,
		PreventedAPICalls:    v.stats.PreventedAPICalls,
		ValidationLatency:    v.stats.ValidationLatency,
		InvalidReasons:       make(map[string]int64),
		OperatorDistribution: make(map[string]int64),
		LastUpdated:          v.stats.LastUpdated,
	}

	// Copy maps
	for k, v := range v.stats.InvalidReasons {
		stats.InvalidReasons[k] = v
	}
	for k, v := range v.stats.OperatorDistribution {
		stats.OperatorDistribution[k] = v
	}

	return stats
}

// IsValidMSISDNFormat performs basic format validation without database checks
func IsValidMSISDNFormat(msisdn string) bool {
	// Remove any non-digit characters
	re := regexp.MustCompile(`\D`)
	cleanMSISDN := re.ReplaceAllString(msisdn, "")

	// Check if it's a valid format
	switch {
	case strings.HasPrefix(cleanMSISDN, "233") && len(cleanMSISDN) == 12:
		return true
	case strings.HasPrefix(cleanMSISDN, "0") && len(cleanMSISDN) == 10:
		return true
	case len(cleanMSISDN) == 9:
		return true
	default:
		return false
	}
}

// ReloadConfiguration reloads the MSISDN validation configuration
func (v *MSISDNValidator) ReloadConfiguration(config *MSISDNValidationConfig) {
	v.stats.mu.Lock()
	defer v.stats.mu.Unlock()

	if config == nil {
		v.logger.Warn("Attempted to reload with nil configuration, keeping current config")
		return
	}

	// Update cache expiry
	if config.CacheExpiry > 0 {
		v.cache.mu.Lock()
		v.cache.cacheExpiry = config.CacheExpiry
		v.cache.mu.Unlock()
		v.logger.Info("Updated cache expiry", zap.Duration("new_expiry", config.CacheExpiry))
	}

	// Update validation flags
	v.config.EnablePrefixValidation = config.EnablePrefixValidation
	v.config.EnableExcludedUserCheck = config.EnableExcludedUserCheck
	v.config.EnableInvalidLogCheck = config.EnableInvalidLogCheck
	v.config.MaxValidationErrors = config.MaxValidationErrors

	// Update telecom prefixes if provided
	if config.TelcoPrefixes != nil && len(config.TelcoPrefixes) > 0 {
		v.telcoPrefixes = config.TelcoPrefixes
		v.logger.Info("Reloaded telecom prefixes from configuration",
			zap.Int("operator_count", len(config.TelcoPrefixes)))

		// Log the new prefixes for debugging
		for operator, operatorPrefixes := range config.TelcoPrefixes {
			v.logger.Debug("Operator prefixes reloaded",
				zap.String("operator", operator),
				zap.Strings("prefixes", operatorPrefixes))
		}
	}

	// Clear cache when configuration changes
	v.clearCache()
	v.logger.Info("Configuration reloaded and cache cleared")
}

// clearCache clears all cached validation results
func (v *MSISDNValidator) clearCache() {
	v.cache.mu.Lock()
	defer v.cache.mu.Unlock()

	v.cache.validCache = make(map[string]time.Time)
	v.cache.invalidCache = make(map[string]time.Time)
}

// GetConfiguration returns a copy of the current configuration
func (v *MSISDNValidator) GetConfiguration() *MSISDNValidationConfig {
	v.stats.mu.RLock()
	defer v.stats.mu.RUnlock()

	v.cache.mu.RLock()
	cacheExpiry := v.cache.cacheExpiry
	v.cache.mu.RUnlock()

	return &MSISDNValidationConfig{
		CacheExpiry:             cacheExpiry,
		EnablePrefixValidation:  v.config.EnablePrefixValidation,
		EnableExcludedUserCheck: v.config.EnableExcludedUserCheck,
		EnableInvalidLogCheck:   v.config.EnableInvalidLogCheck,
		MaxValidationErrors:     v.config.MaxValidationErrors,
		TelcoPrefixes:           v.GetTelcoPrefixes(),
	}
}
