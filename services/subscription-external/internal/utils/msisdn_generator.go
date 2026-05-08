package utils

import (
	"context"
	"fmt"
	mrand "math/rand/v2" // Use math/rand/v2 for Go 1.22+
	"strings"
	"sync"
	"time"

	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription-external/internal/repository"
	"go.uber.org/zap"
)

// MSISDNCache represents a thread-safe cache for MSISDN validation results
type MSISDNCache struct {
	validMSISDNS   sync.Map // Cache for valid MSISDNS
	invalidMSISDNS sync.Map // Cache for invalid MSISDNS (Premier/Staff/Invalid logs)
	logger         *zap.Logger
}

// MSISDNPool represents a pool of sample MSISDNs for pattern-based generation
type MSISDNPool struct {
	tigoSamples []string
	mu          sync.RWMutex
}

// Global MSISDN pool for pattern-based generation
var globalMSISDNPool = &MSISDNPool{
	// Sample Tigo/Airtel numbers from the userbase
	// These are used as templates for generating similar patterns
	tigoSamples: []string{
		"233561075653", "233561234567", "233561345678", "233561456789",
		"233561567890", "233561678901", "233561789012", "233561890123",
		"233561901234", "233561012345", "233561123456", "233561234567",
		"233561345678", "233561456789", "233561567890", "233561678901",
		"233561789012", "233561890123", "233561901234", "233561012345",
		// Add more samples from different prefixes
		"233578123456", "233578234567", "233578345678", "233578456789",
		"233242123456", "233242234567", "233242345678", "233242456789",
		"233307123456", "233307234567", "233307345678", "233307456789",
		"233245123456", "233245234567", "233245345678", "233245456789",
		"233247123456", "233247234567", "233247345678", "233247456789",
		"233576123456", "233576234567", "233576345678", "233576456789",
		"233271123456", "233271234567", "233271345678", "233271456789",
		"233273123456", "233273234567", "233273345678", "233273456789",
		"233571123456", "233571234567", "233571345678", "233571456789",
		"233277123456", "233277234567", "233277345678", "233277456789",
	},
}

// NewMSISDNCache creates a new MSISDN cache instance
func NewMSISDNCache(logger *zap.Logger) *MSISDNCache {
	return &MSISDNCache{
		logger: logger,
	}
}

// CacheResult stores a validation result in the appropriate cache
func (c *MSISDNCache) CacheResult(msisdn string, isValid bool) {
	if isValid {
		c.validMSISDNS.Store(msisdn, time.Now())
	} else {
		c.invalidMSISDNS.Store(msisdn, time.Now())
	}
}

// IsCachedValid checks if an MSISDN is cached as valid
func (c *MSISDNCache) IsCachedValid(msisdn string) (bool, bool) {
	if _, found := c.validMSISDNS.Load(msisdn); found {
		return true, true
	}
	if _, found := c.invalidMSISDNS.Load(msisdn); found {
		return false, true
	}
	return false, false
}

// Cleanup removes old cache entries (older than 24 hours)
func (c *MSISDNCache) Cleanup() {
	cutoff := time.Now().Add(-24 * time.Hour)

	c.validMSISDNS.Range(func(key, value interface{}) bool {
		if timestamp, ok := value.(time.Time); ok && timestamp.Before(cutoff) {
			c.validMSISDNS.Delete(key)
		}
		return true
	})

	c.invalidMSISDNS.Range(func(key, value interface{}) bool {
		if timestamp, ok := value.(time.Time); ok && timestamp.Before(cutoff) {
			c.invalidMSISDNS.Delete(key)
		}
		return true
	})
}

// Global cache instance
var globalMSISDNCache = NewMSISDNCache(nil)

// secureRandomInt returns a random integer in the range [min, max)
func secureRandomInt(min, max int) (int, error) {
	if min >= max {
		return 0, fmt.Errorf("invalid range: min (%d) must be less than max (%d)", min, max)
	}
	width := max - min
	n := mrand.IntN(width)
	return min + n, nil
}

// validateMSISDN checks if an MSISDN is valid by checking multiple criteria
func validateMSISDN(ctx context.Context, repo repository.UserBaseRepositoryInterface, msisdn string) (bool, error) {
	// Check cache first
	if isValid, cached := globalMSISDNCache.IsCachedValid(msisdn); cached {
		return isValid, nil
	}

	// Check if MSISDN is Premier, Staff, or Blacklisted
	isExcluded, err := repo.IsExcludedUser(msisdn)
	if err != nil {
		return false, fmt.Errorf("error checking excluded user status: %v", err)
	}
	if isExcluded {
		globalMSISDNCache.CacheResult(msisdn, false)
		return false, nil
	}

	// Check if MSISDN exists in invalid_msisdn_logs table using optimized method
	// Try the fast method first, fall back to optimized if needed
	if fastRepo, ok := repo.(interface {
		GetInvalidMSISDNSFast(context.Context, string) (bool, error)
	}); ok {
		invalid, err := fastRepo.GetInvalidMSISDNSFast(ctx, msisdn)
		if err != nil {
			return false, fmt.Errorf("error checking invalid MSISDN logs: %v", err)
		}
		if invalid {
			globalMSISDNCache.CacheResult(msisdn, false)
			return false, nil
		}
	} else {
		// Fall back to original method if fast method not available
		invalidMSISDNS, err := repo.GetInvalidMSISDNS(ctx, []string{msisdn})
		if err != nil {
			return false, fmt.Errorf("error checking invalid MSISDN logs: %v", err)
		}
		if len(invalidMSISDNS) > 0 {
			globalMSISDNCache.CacheResult(msisdn, false)
			return false, nil
		}
	}

	// MSISDN is valid
	globalMSISDNCache.CacheResult(msisdn, true)
	return true, nil
}

// generateMSISDN generates a single MSISDN with the given prefix
// Enhanced to follow patterns from real Tigo userbase
func generateMSISDN(prefix string) (string, error) {
	// Generate a 6-digit suffix following patterns from real data
	// Most Tigo numbers have patterns like: 075653, 234567, 345678
	// These appear to follow sequential patterns in blocks

	// Choose generation strategy
	strategy, err := secureRandomInt(0, 3)
	if err != nil {
		return "", err
	}

	var suffix int
	switch strategy {
	case 0:
		// Sequential pattern (like 234567, 345678)
		base, err := secureRandomInt(100000, 900000)
		if err != nil {
			return "", err
		}
		suffix = base + (base % 111111) // Create sequential-like patterns

	case 1:
		// Block pattern (like 075653, 075654, 075655)
		block, err := secureRandomInt(0, 999)
		if err != nil {
			return "", err
		}
		subblock, err := secureRandomInt(0, 999)
		if err != nil {
			return "", err
		}
		suffix = block*1000 + subblock

	default:
		// Random pattern
		suffix, err = secureRandomInt(100000, 1000000)
		if err != nil {
			return "", err
		}
	}

	// Ensure suffix is 6 digits
	if suffix >= 1000000 {
		suffix = suffix % 1000000
	}
	if suffix < 100000 {
		suffix += 100000
	}

	return fmt.Sprintf("%s%06d", prefix, suffix), nil
}

// generateFromPool generates an MSISDN based on patterns from the sample pool
func generateFromPool(prefix string) (string, error) {
	globalMSISDNPool.mu.RLock()
	defer globalMSISDNPool.mu.RUnlock()

	if len(globalMSISDNPool.tigoSamples) == 0 {
		return generateMSISDN(prefix)
	}

	// Select a random sample as template
	sampleIdx, err := secureRandomInt(0, len(globalMSISDNPool.tigoSamples))
	if err != nil {
		return generateMSISDN(prefix)
	}

	sample := globalMSISDNPool.tigoSamples[sampleIdx]
	if len(sample) < 12 {
		return generateMSISDN(prefix)
	}

	// Extract the last 6 digits and modify them slightly
	if len(sample) < 12 {
		return generateMSISDN(prefix)
	}
	lastSix := sample[len(sample)-6:]

	// Convert to number and add small random variation
	var baseNum int
	_, _ = fmt.Sscanf(lastSix, "%d", &baseNum)

	variation, err := secureRandomInt(-1000, 1000)
	if err != nil {
		return generateMSISDN(prefix)
	}

	newSuffix := baseNum + variation
	if newSuffix < 100000 {
		newSuffix = 100000 + (newSuffix % 100000)
	}
	if newSuffix >= 1000000 {
		newSuffix = newSuffix % 1000000
	}

	return fmt.Sprintf("%s%06d", prefix, newSuffix), nil
}

// GenerateRandomMSISDN generates a random MSISDN ensuring it doesn't belong to Premier/Staff users or invalid logs
// Enhanced with pattern-based generation using real Tigo userbase data
func GenerateRandomMSISDN(telco string, config *config.Config, repo repository.UserBaseRepositoryInterface) (string, error) {
	return GenerateRandomMSISDNWithContext(context.Background(), telco, config, repo)
}

// GenerateRandomMSISDNWithContext generates a random MSISDN with context for better control
func GenerateRandomMSISDNWithContext(ctx context.Context, telco string, config *config.Config, repo repository.UserBaseRepositoryInterface) (string, error) {
	normalizedTelco := strings.ToLower(telco)

	// Map common telco names to configuration keys
	telcoMapping := map[string]string{
		"tigo":       "AirtelTigo",
		"airtel":     "AirtelTigo",
		"airteltigo": "AirtelTigo",
		"mtn":        "MTN",
		"vodafone":   "Vodafone",
	}

	configKey := normalizedTelco
	if mapped, exists := telcoMapping[normalizedTelco]; exists {
		configKey = mapped
	}

	prefixes, exists := config.Application.TelcoPrefixes[configKey]
	if !exists {
		// Try with original telco name as fallback
		prefixes, exists = config.Application.TelcoPrefixes[normalizedTelco]
		if !exists {
			return "", fmt.Errorf("invalid telco: %s", telco)
		}
	}

	if len(prefixes) == 0 {
		return "", fmt.Errorf("no prefixes configured for telco: %s", telco)
	}

	maxAttempts := 100 // Prevent infinite loops
	attempts := 0

	for attempts < maxAttempts {
		attempts++

		// Generate a random prefix with weighted selection for common prefixes
		// Prefixes like 233561, 233578, 233242 are more common in real data
		prefixIndex, err := selectWeightedPrefix(prefixes)
		if err != nil {
			prefixIndex, err = secureRandomInt(0, len(prefixes))
			if err != nil {
				return "", fmt.Errorf("failed to select random prefix: %v", err)
			}
		}
		prefix := prefixes[prefixIndex]

		// Generate MSISDN using pattern-based generation 70% of the time
		var msisdn string
		usePool, _ := secureRandomInt(0, 10)
		if usePool < 7 {
			msisdn, err = generateFromPool(prefix)
		} else {
			msisdn, err = generateMSISDN(prefix)
		}

		if err != nil {
			continue // Try again with different random values
		}

		// Validate the generated MSISDN
		isValid, err := validateMSISDN(ctx, repo, msisdn)
		if err != nil {
			// Log error but continue trying
			if globalMSISDNCache.logger != nil {
				globalMSISDNCache.logger.Warn("Error validating MSISDN, retrying",
					zap.String("msisdn", msisdn),
					zap.Error(err))
			}
			continue
		}

		if isValid {
			return msisdn, nil
		}
	}

	return "", fmt.Errorf("failed to generate valid MSISDN after %d attempts", maxAttempts)
}

// GenerateRandomMSISDNNoValidate generates an MSISDN without any repository-backed validation
// It is intended to be used by batched generators that perform bulk validation later
func GenerateRandomMSISDNNoValidate(telco string, config *config.Config) (string, error) {
	normalizedTelco := strings.ToLower(telco)

	// Map common telco names to configuration keys
	telcoMapping := map[string]string{
		"tigo":       "AirtelTigo",
		"airtel":     "AirtelTigo",
		"airteltigo": "AirtelTigo",
		"mtn":        "MTN",
		"vodafone":   "Vodafone",
	}

	configKey := normalizedTelco
	if mapped, exists := telcoMapping[normalizedTelco]; exists {
		configKey = mapped
	}

	prefixes, exists := config.Application.TelcoPrefixes[configKey]
	if !exists {
		// Try with original telco name as fallback
		prefixes, exists = config.Application.TelcoPrefixes[normalizedTelco]
		if !exists {
			return "", fmt.Errorf("invalid telco: %s", telco)
		}
	}
	if len(prefixes) == 0 {
		return "", fmt.Errorf("no prefixes configured for telco: %s", telco)
	}

	// Generate a random prefix with weighted selection for common prefixes
	prefixIndex, err := selectWeightedPrefix(prefixes)
	if err != nil {
		prefixIndex, err = secureRandomInt(0, len(prefixes))
		if err != nil {
			return "", fmt.Errorf("failed to select random prefix: %v", err)
		}
	}
	prefix := prefixes[prefixIndex]

	// Generate MSISDN using pattern-based generation 70% of the time
	var msisdn string
	usePool, _ := secureRandomInt(0, 10)
	if usePool < 7 {
		msisdn, err = generateFromPool(prefix)
	} else {
		msisdn, err = generateMSISDN(prefix)
	}
	if err != nil {
		return "", err
	}
	return msisdn, nil
}

// GenerateBatchMSISDNSFast generates MSISDNs using batched validation to minimize DB calls
// Strategy:
// 1) Generate candidate MSISDNs without validation
// 2) Filter against a preloaded exclusion set (Premier/Staff) in-memory
// 3) Batch-check invalid_msisdn_logs for remaining candidates
// 4) Repeat until the requested count is satisfied
func GenerateBatchMSISDNSFast(ctx context.Context, telco string, count int, config *config.Config, repo repository.UserBaseRepositoryInterface) ([]string, error) {
	if count <= 0 {
		return []string{}, nil
	}

	// Load exclusion set once (Premier/Staff)
	exclusionSet, err := repo.LoadExclusionList()
	if err != nil {
		// Proceed without exclusion set if it fails; downstream checks will still handle invalid logs
		exclusionSet = nil
	}

	resultSet := make(map[string]struct{}, count*2)
	result := make([]string, 0, count)
	generatedSet := make(map[string]struct{}, count*3)

	// Tunables for chunk sizes
	chunkSize := count
	if chunkSize < 500 {
		chunkSize = 500
	}
	if chunkSize > 5000 {
		chunkSize = 5000
	}

	for len(result) < count {
		// 1) Generate a chunk of unique candidates
		candidates := make([]string, 0, chunkSize)
		for len(candidates) < chunkSize {
			msisdn, genErr := GenerateRandomMSISDNNoValidate(telco, config)
			if genErr != nil {
				continue
			}
			if _, seen := generatedSet[msisdn]; seen {
				continue
			}
			generatedSet[msisdn] = struct{}{}
			// Skip if in exclusion set
			if exclusionSet != nil {
				if _, excluded := exclusionSet[msisdn]; excluded {
					continue
				}
			}
			candidates = append(candidates, msisdn)
		}

		// 2) Batch-check invalid logs and filter
		var invalidSet map[string]struct{}
		if optRepo, ok := repo.(interface {
			GetInvalidMSISDNSOptimized(context.Context, []string) ([]string, error)
		}); ok {
			invalid, err := optRepo.GetInvalidMSISDNSOptimized(ctx, candidates)
			if err != nil {
				return nil, fmt.Errorf("error checking invalid MSISDN logs: %v", err)
			}
			invalidSet = make(map[string]struct{}, len(invalid))
			for _, m := range invalid {
				invalidSet[m] = struct{}{}
			}
		} else {
			// Fall back to original method if optimized method not available
			invalid, err := repo.GetInvalidMSISDNS(ctx, candidates)
			if err != nil {
				return nil, fmt.Errorf("error checking invalid MSISDN logs: %v", err)
			}
			invalidSet = make(map[string]struct{}, len(invalid))
			for _, m := range invalid {
				invalidSet[m] = struct{}{}
			}
		}

		// 3) Batch-check blacklisted MSISDNs and filter
		blacklisted, err := repo.GetBlacklistedMSISDNS(ctx, candidates)
		if err != nil {
			return nil, fmt.Errorf("error checking blacklisted MSISDNs: %v", err)
		}
		blacklistedSet := make(map[string]struct{}, len(blacklisted))
		for _, m := range blacklisted {
			blacklistedSet[m] = struct{}{}
		}

		for _, msisdn := range candidates {
			if _, bad := invalidSet[msisdn]; bad {
				continue
			}
			if _, blacklisted := blacklistedSet[msisdn]; blacklisted {
				continue
			}
			if _, already := resultSet[msisdn]; already {
				continue
			}
			resultSet[msisdn] = struct{}{}
			result = append(result, msisdn)
			if len(result) >= count {
				break
			}
		}
	}

	return result[:count], nil
}

// GenerateBatchMSISDNSConcurrently generates a batch of MSISDNS concurrently with improved efficiency
func GenerateBatchMSISDNSConcurrently(telco string, count int, config *config.Config, repo repository.UserBaseRepositoryInterface) ([]string, error) {
	return GenerateBatchMSISDNSConcurrentlyWithContext(context.Background(), telco, count, config, repo)
}

// GenerateBatchMSISDNSConcurrentlyWithContext generates a batch of MSISDNS with context
func GenerateBatchMSISDNSConcurrentlyWithContext(ctx context.Context, telco string, count int, config *config.Config, repo repository.UserBaseRepositoryInterface) ([]string, error) {
	// Fast path using batched validation to avoid per-MSISDN DB calls
	return GenerateBatchMSISDNSFast(ctx, telco, count, config, repo)
}

// GenerateBatchMSISDNSWithValidation generates MSISDNS with comprehensive validation
func GenerateBatchMSISDNSWithValidation(ctx context.Context, telco string, count int, config *config.Config, repo repository.UserBaseRepositoryInterface) ([]string, error) {
	// Generate initial batch
	msisdns, err := GenerateBatchMSISDNSConcurrentlyWithContext(ctx, telco, count, config, repo)
	if err != nil {
		return nil, err
	}

	// Perform batch validation to ensure all MSISDNS are truly valid
	validMSISDNS, err := validateBatchMSISDNS(ctx, repo, msisdns)
	if err != nil {
		return nil, fmt.Errorf("batch validation failed: %v", err)
	}

	// If we don't have enough valid MSISDNS, generate more
	if len(validMSISDNS) < count {
		additionalNeeded := count - len(validMSISDNS)
		additionalMSISDNS, err := GenerateBatchMSISDNSWithValidation(ctx, telco, additionalNeeded, config, repo)
		if err != nil {
			return nil, err
		}
		validMSISDNS = append(validMSISDNS, additionalMSISDNS...)
	}

	// Return only the requested count
	if len(validMSISDNS) > count {
		validMSISDNS = validMSISDNS[:count]
	}

	return validMSISDNS, nil
}

// validateBatchMSISDNS performs batch validation of MSISDNS
func validateBatchMSISDNS(ctx context.Context, repo repository.UserBaseRepositoryInterface, msisdns []string) ([]string, error) {
	if len(msisdns) == 0 {
		return []string{}, nil
	}

	var validMSISDNS []string

	// Check for Premier/Staff MSISDNS in batch
	premierStaffMSISDNS, err := repo.FilterMSISDNS(msisdns)
	if err != nil {
		return nil, fmt.Errorf("error filtering Premier/Staff MSISDNS: %v", err)
	}

	// Create a set of valid MSISDNS (not Premier/Staff)
	validSet := make(map[string]bool)
	for _, msisdn := range premierStaffMSISDNS {
		validSet[msisdn] = true
	}

	// Check for invalid MSISDNS in logs using optimized method when available
	if optRepo, ok := repo.(interface {
		GetInvalidMSISDNSOptimized(context.Context, []string) ([]string, error)
	}); ok {
		invalidMSISDNS, err := optRepo.GetInvalidMSISDNSOptimized(ctx, msisdns)
		if err != nil {
			return nil, fmt.Errorf("error checking invalid MSISDN logs: %v", err)
		}

		// Remove invalid MSISDNS from valid set
		for _, msisdn := range invalidMSISDNS {
			delete(validSet, msisdn)
		}
	} else {
		// Fall back to original method if optimized method not available
		invalidMSISDNS, err := repo.GetInvalidMSISDNS(ctx, msisdns)
		if err != nil {
			return nil, fmt.Errorf("error checking invalid MSISDN logs: %v", err)
		}

		// Remove invalid MSISDNS from valid set
		for _, msisdn := range invalidMSISDNS {
			delete(validSet, msisdn)
		}
	}

	// Convert set back to slice
	for msisdn := range validSet {
		validMSISDNS = append(validMSISDNS, msisdn)
	}

	return validMSISDNS, nil
}

// SetLogger sets the logger for the global MSISDN cache
func SetMSISDNCacheLogger(logger *zap.Logger) {
	globalMSISDNCache.logger = logger
}

// CleanupCache cleans up old entries from the global cache
func CleanupMSISDNCache() {
	globalMSISDNCache.Cleanup()
}

// selectWeightedPrefix selects a prefix with weighted probability based on real data distribution
func selectWeightedPrefix(prefixes []string) (int, error) {
	// Common Tigo/Airtel prefixes based on real data analysis
	weights := map[string]int{
		"233561": 30, // Most common in sample data
		"233578": 20,
		"233242": 15,
		"233307": 10,
		"233245": 10,
		"233247": 8,
		"233576": 7,
		"233271": 5,
		"233273": 5,
		"233571": 5,
		"233277": 5,
	}

	// Build weighted selection
	totalWeight := 0
	prefixWeights := make([]int, len(prefixes))
	for i, prefix := range prefixes {
		if weight, exists := weights[prefix]; exists {
			prefixWeights[i] = weight
			totalWeight += weight
		} else {
			prefixWeights[i] = 1
			totalWeight += 1
		}
	}

	// Select based on weight
	if totalWeight == 0 {
		return 0, fmt.Errorf("no valid weights")
	}

	target, err := secureRandomInt(0, totalWeight)
	if err != nil {
		return 0, err
	}

	cumulative := 0
	for i, weight := range prefixWeights {
		cumulative += weight
		if target < cumulative {
			return i, nil
		}
	}

	return len(prefixes) - 1, nil
}

// LoadMSISDNSamples loads sample MSISDNs into the global pool
func LoadMSISDNSamples(samples []string) {
	globalMSISDNPool.mu.Lock()
	defer globalMSISDNPool.mu.Unlock()

	if len(samples) > 0 {
		globalMSISDNPool.tigoSamples = append(globalMSISDNPool.tigoSamples, samples...)
	}
}
