package utils

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/seidu626/subscription-manager/common/config"
	"github.com/seidu626/subscription-manager/subscription-external/internal/repository"
	"go.uber.org/zap"
)

// OptimizedMSISDNGenerator provides high-performance MSISDN generation
// using Bloom Filter for ultra-fast negative lookups
type OptimizedMSISDNGenerator struct {
	bloomFilter *MSISDNBloomFilter
	repo        repository.UserBaseRepositoryInterface
	logger      *zap.Logger

	// Configuration
	batchSize     int
	maxConcurrent int
	cacheEnabled  bool

	// Statistics
	stats struct {
		generated      int64
		validated      int64
		bloomHits      int64
		bloomMisses    int64
		generationTime time.Duration
	}

	// Thread safety
	mutex sync.RWMutex
}

// NewOptimizedMSISDNGenerator creates a new optimized MSISDN generator
func NewOptimizedMSISDNGenerator(
	bloomFilter *MSISDNBloomFilter,
	repo repository.UserBaseRepositoryInterface,
	logger *zap.Logger,
	batchSize int,
	maxConcurrent int,
) *OptimizedMSISDNGenerator {
	// Validate parameters
	if repo == nil {
		panic("repository cannot be nil")
	}
	if logger == nil {
		panic("logger cannot be nil")
	}
	if batchSize <= 0 {
		batchSize = 100 // Default batch size
		logger.Warn("Invalid batch size, using default", zap.Int("provided", batchSize), zap.Int("default", 100))
	}
	if maxConcurrent <= 0 {
		maxConcurrent = 10 // Default concurrency
		logger.Warn("Invalid max concurrent, using default", zap.Int("provided", maxConcurrent), zap.Int("default", 10))
	}
	if maxConcurrent > 1000 {
		maxConcurrent = 1000 // Cap maximum concurrency
		logger.Warn("Max concurrent too high, capping at 1000", zap.Int("provided", maxConcurrent), zap.Int("capped", 1000))
	}

	return &OptimizedMSISDNGenerator{
		bloomFilter:   bloomFilter,
		repo:          repo,
		logger:        logger,
		batchSize:     batchSize,
		maxConcurrent: maxConcurrent,
		cacheEnabled:  true,
		stats: struct {
			generated      int64
			validated      int64
			bloomHits      int64
			bloomMisses    int64
			generationTime time.Duration
		}{},
	}
}

// GenerateRandomMSISDNOptimized generates a single valid MSISDN using optimized validation
func (g *OptimizedMSISDNGenerator) GenerateRandomMSISDNOptimized(
	ctx context.Context,
	telco string,
	config *config.Config,
) (string, error) {
	startTime := time.Now()
	defer func() {
		g.mutex.Lock()
		g.stats.generationTime += time.Since(startTime)
		g.stats.generated++
		g.mutex.Unlock()
	}()

	// Get telco prefixes from config (you'll need to implement this based on your config structure)
	prefixes := g.getTelcoPrefixes(telco, config)
	if len(prefixes) == 0 {
		return "", fmt.Errorf("no prefixes found for telco: %s", telco)
	}

	maxAttempts := 100 // Prevent infinite loops
	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Generate random MSISDN
		msisdn, err := g.generateRandomMSISDN(prefixes)
		if err != nil {
			continue
		}

		// Use Bloom Filter for ultra-fast validation
		if g.bloomFilter != nil {
			// Check if MSISDN might be invalid (false positives are acceptable)
			if !g.bloomFilter.MightContain(msisdn) {
				// Definitely not invalid, check if it's excluded user
				isExcluded, err := g.repo.IsExcludedUser(msisdn)
				if err != nil {
					g.logger.Warn("Failed to check excluded user status",
						zap.String("msisdn", msisdn), zap.Error(err))
					continue
				}

				if !isExcluded {
					g.mutex.Lock()
					g.stats.bloomHits++
					g.mutex.Unlock()
					return msisdn, nil
				}
			} else {
				g.mutex.Lock()
				g.stats.bloomMisses++
				g.mutex.Unlock()
			}
		}

		// Fall back to full validation if Bloom Filter is not available
		if g.bloomFilter == nil {
			valid, err := g.validateMSISDNFull(ctx, msisdn)
			if err != nil {
				continue
			}
			if valid {
				return msisdn, nil
			}
		}
	}

	return "", fmt.Errorf("failed to generate valid MSISDN after %d attempts", maxAttempts)
}

// GenerateBatchMSISDNSOptimized generates multiple MSISDNs efficiently using concurrent processing
func (g *OptimizedMSISDNGenerator) GenerateBatchMSISDNSOptimized(
	ctx context.Context,
	telco string,
	count int,
	config *config.Config,
) ([]string, error) {
	if count <= 0 {
		return []string{}, nil
	}

	// Add a reasonable timeout to prevent infinite blocking
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	startTime := time.Now()
	defer func() {
		g.mutex.Lock()
		g.stats.generationTime += time.Since(startTime)
		g.stats.generated += int64(count)
		g.mutex.Unlock()
	}()

	// Validate configuration
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Get the maximum allowed count from configuration, with fallback to 10000
	maxCount := 10000 // Default fallback
	if config.Application.MSISDNGenerator.MaxMSISDNCount > 0 {
		maxCount = config.Application.MSISDNGenerator.MaxMSISDNCount
	}

	// Limit the maximum count to prevent resource exhaustion
	if count > maxCount {
		return nil, fmt.Errorf("count too large (%d), maximum allowed is %d", count, maxCount)
	}

	// Use worker pool for concurrent generation
	results := make(chan string, count)
	errors := make(chan error, count)

	// Create worker pool with limited concurrency
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, g.maxConcurrent)

	// Create a job queue to distribute work
	jobs := make(chan int, count)

	// Start a fixed number of workers instead of one per job
	numWorkers := g.maxConcurrent
	if numWorkers > count {
		numWorkers = count
	}

	g.logger.Info("Starting MSISDN batch generation",
		zap.Int("requested", count),
		zap.Int("workers", numWorkers),
		zap.Int("maxConcurrent", g.maxConcurrent))

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for range jobs {
				// Check context cancellation
				select {
				case <-ctx.Done():
					return
				default:
				}

				// Acquire semaphore (this should never block since we limit workers)
				select {
				case semaphore <- struct{}{}:
				case <-ctx.Done():
					return
				}

				msisdn, err := g.GenerateRandomMSISDNOptimized(ctx, telco, config)

				// Release semaphore immediately after work
				<-semaphore

				if err != nil {
					select {
					case errors <- fmt.Errorf("worker %d failed: %w", workerID, err):
					case <-ctx.Done():
						return
					}
					continue
				}

				select {
				case results <- msisdn:
				case <-ctx.Done():
					return
				}
			}
		}(i)
	}

	// Feed jobs to workers
	go func() {
		defer close(jobs)
		for i := 0; i < count; i++ {
			select {
			case jobs <- i:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for all workers to complete in a separate goroutine
	go func() {
		wg.Wait()
		close(results)
		close(errors)
	}()

	// Collect results with timeout protection
	var msisdns []string
	timeout := time.After(30 * time.Second) // 30 second timeout

	for {
		select {
		case msisdn, ok := <-results:
			if !ok {
				// Channel closed, all workers completed
				if len(msisdns) == 0 {
					return nil, fmt.Errorf("no MSISDNs were generated successfully")
				}
				g.logger.Info("MSISDN batch generation completed successfully",
					zap.Int("requested", count),
					zap.Int("generated", len(msisdns)),
					zap.Duration("duration", time.Since(startTime)))
				return msisdns, nil
			}
			msisdns = append(msisdns, msisdn)
			if len(msisdns) >= count {
				return msisdns, nil
			}
		case err, ok := <-errors:
			if !ok {
				// Channel closed, continue with results
				continue
			}
			if len(msisdns) == 0 {
				return nil, fmt.Errorf("all MSISDN generation attempts failed: %w", err)
			}
			g.logger.Warn("Some MSISDN generation attempts failed",
				zap.Int("requested", count),
				zap.Int("generated", len(msisdns)),
				zap.Error(err))
		case <-timeout:
			// Timeout reached
			if len(msisdns) == 0 {
				return nil, fmt.Errorf("MSISDN generation timed out after 30 seconds")
			}
			g.logger.Warn("MSISDN generation timed out",
				zap.Int("requested", count),
				zap.Int("generated", len(msisdns)))
			return msisdns, nil
		case <-ctx.Done():
			// Context cancelled
			return nil, fmt.Errorf("MSISDN generation cancelled: %w", ctx.Err())
		}
	}
}

// GenerateBatchMSISDNSWithSmartValidation generates MSISDNs with intelligent batch validation
func (g *OptimizedMSISDNGenerator) GenerateBatchMSISDNSWithSmartValidation(
	ctx context.Context,
	telco string,
	count int,
	config *config.Config,
) ([]string, error) {
	if count <= 0 {
		return []string{}, nil
	}

	// Generate more candidates than needed to account for invalid ones
	candidateMultiplier := 3
	if g.bloomFilter != nil {
		candidateMultiplier = 2 // Bloom Filter reduces invalid candidates
	}

	candidates := make([]string, 0, count*candidateMultiplier)
	prefixes := g.getTelcoPrefixes(telco, config)

	// Generate candidates
	for len(candidates) < count*candidateMultiplier {
		msisdn, err := g.generateRandomMSISDN(prefixes)
		if err != nil {
			continue
		}
		candidates = append(candidates, msisdn)
	}

	// Use Bloom Filter for first-pass filtering
	if g.bloomFilter != nil {
		candidates = g.filterWithBloomFilter(candidates)
	}

	// If we have enough candidates after Bloom Filter, validate them
	if len(candidates) >= count {
		validMSISDNS, err := g.validateBatchMSISDNS(ctx, candidates[:count])
		if err != nil {
			return nil, err
		}
		return validMSISDNS, nil
	}

	// Fall back to full validation for remaining candidates
	validMSISDNS, err := g.validateBatchMSISDNS(ctx, candidates)
	if err != nil {
		return nil, err
	}

	// If we still don't have enough, generate more
	if len(validMSISDNS) < count {
		additional, err := g.GenerateBatchMSISDNSWithSmartValidation(
			ctx, telco, count-len(validMSISDNS), config)
		if err != nil {
			return validMSISDNS, err
		}
		validMSISDNS = append(validMSISDNS, additional...)
	}

	return validMSISDNS, nil
}

// filterWithBloomFilter filters candidates using Bloom Filter for ultra-fast negative lookups
func (g *OptimizedMSISDNGenerator) filterWithBloomFilter(candidates []string) []string {
	if g.bloomFilter == nil {
		return candidates
	}

	var filtered []string
	for _, msisdn := range candidates {
		// If Bloom Filter says definitely not invalid, include it
		if !g.bloomFilter.MightContain(msisdn) {
			filtered = append(filtered, msisdn)
		}
	}

	g.mutex.Lock()
	g.stats.bloomHits += int64(len(filtered))
	g.stats.bloomMisses += int64(len(candidates) - len(filtered))
	g.mutex.Unlock()

	return filtered
}

// validateMSISDNFull performs full validation including database checks
func (g *OptimizedMSISDNGenerator) validateMSISDNFull(ctx context.Context, msisdn string) (bool, error) {
	// Check if MSISDN is excluded user
	isExcluded, err := g.repo.IsExcludedUser(msisdn)
	if err != nil {
		return false, err
	}
	if isExcluded {
		return false, nil
	}

	// Check if MSISDN exists in invalid logs
	invalidMSISDNS, err := g.repo.GetInvalidMSISDNSFast(ctx, msisdn)
	if err != nil {
		return false, err
	}
	if invalidMSISDNS {
		return false, nil
	}

	return true, nil
}

// validateBatchMSISDNS validates multiple MSISDNs efficiently
func (g *OptimizedMSISDNGenerator) validateBatchMSISDNS(ctx context.Context, msisdns []string) ([]string, error) {
	if len(msisdns) == 0 {
		return []string{}, nil
	}

	// Use optimized repository method for batch validation
	validMSISDNS, err := g.repo.FilterMSISDNS(msisdns)
	if err != nil {
		return nil, err
	}

	// Check for invalid MSISDNs in logs
	invalidMSISDNS, err := g.repo.GetInvalidMSISDNSOptimized(ctx, validMSISDNS)
	if err != nil {
		return nil, err
	}

	// Remove invalid MSISDNs
	invalidSet := make(map[string]bool)
	for _, msisdn := range invalidMSISDNS {
		invalidSet[msisdn] = true
	}

	var finalValidMSISDNS []string
	for _, msisdn := range validMSISDNS {
		if !invalidSet[msisdn] {
			finalValidMSISDNS = append(finalValidMSISDNS, msisdn)
		}
	}

	g.mutex.Lock()
	g.stats.validated += int64(len(finalValidMSISDNS))
	g.mutex.Unlock()

	return finalValidMSISDNS, nil
}

// generateRandomMSISDN generates a random MSISDN with the given prefixes
func (g *OptimizedMSISDNGenerator) generateRandomMSISDN(prefixes []string) (string, error) {
	if len(prefixes) == 0 {
		return "", fmt.Errorf("no prefixes provided")
	}

	// Select random prefix
	prefix := prefixes[rand.Intn(len(prefixes))]
	prefixLength := len(prefix)

	// Calculate required suffix length to make total MSISDN exactly 12 digits
	// Ghana MSISDNs should be exactly 12 digits: 233 (country code) + 9 digits
	requiredSuffixLength := 12 - prefixLength

	// Validate prefix length (Ghana prefixes are 3, 5, or 6 digits)
	if prefixLength != 3 && prefixLength != 5 && prefixLength != 6 {
		return "", fmt.Errorf("invalid prefix length %d for prefix %s, must be 3, 5, or 6 digits", prefixLength, prefix)
	}

	if requiredSuffixLength < 3 || requiredSuffixLength > 9 {
		return "", fmt.Errorf("calculated suffix length %d is invalid for prefix %s (length %d)", requiredSuffixLength, prefix, prefixLength)
	}

	// Generate random suffix with exact required length
	suffix := ""
	for i := 0; i < requiredSuffixLength; i++ {
		suffix += fmt.Sprintf("%d", rand.Intn(10))
	}

	msisdn := prefix + suffix

	// Double-check the final length
	if len(msisdn) != 12 {
		return "", fmt.Errorf("generated MSISDN length %d is not 12: prefix=%s (len=%d), suffix=%s (len=%d), msisdn=%s",
			len(msisdn), prefix, prefixLength, suffix, requiredSuffixLength, msisdn)
	}

	return msisdn, nil
}

// getTelcoPrefixes extracts telco prefixes from config
func (g *OptimizedMSISDNGenerator) getTelcoPrefixes(telco string, config *config.Config) []string {
	if config == nil {
		// Fallback to hardcoded prefixes if config is not available
		return g.getFallbackPrefixes(telco)
	}

	// Normalize telco name for config lookup
	normalizedTelco := g.normalizeTelcoName(telco)

	// Try to get prefixes from config
	if prefixes, exists := config.Application.TelcoPrefixes[normalizedTelco]; exists && len(prefixes) > 0 {
		return prefixes
	}

	// Fallback to hardcoded prefixes if not found in config
	return g.getFallbackPrefixes(telco)
}

// normalizeTelcoName normalizes telco names for consistent config lookup
func (g *OptimizedMSISDNGenerator) normalizeTelcoName(telco string) string {
	switch strings.ToLower(telco) {
	case "tigo", "airtel", "airteltigo":
		return "AirtelTigo"
	case "mtn":
		return "MTN"
	case "vodafone":
		return "Vodafone"
	default:
		return telco
	}
}

// getFallbackPrefixes provides hardcoded prefixes as fallback
func (g *OptimizedMSISDNGenerator) getFallbackPrefixes(telco string) []string {
	switch strings.ToLower(telco) {
	case "mtn":
		return []string{"23324", "23354", "23355"}
	case "vodafone":
		return []string{"23320", "23350"}
	case "tigo", "airtel", "airteltigo":
		return []string{"233278", "233203", "233578", "233242", "233307", "233245", "233247", "233576", "233271", "233273", "233571", "233277", "233233", "233544", "233561", "233579", "233317", "233243", "233265", "233246", "233285", "233270", "233267", "233542", "233572", "233276", "233264", "233200", "233556", "233263", "233274", "233577", "233261", "233275", "233268", "233272", "233269", "233337", "233543", "233208", "233262", "233575", "233573", "233570", "233249", "233232", "233244", "233574", "233279", "233266", "233260", "233236", "233560", "233240", "233546", "233549"}
	default:
		return []string{"233"} // Default Ghana prefix
	}
}

// PreloadBloomFilter loads existing invalid MSISDNs into the Bloom Filter for better performance
func (g *OptimizedMSISDNGenerator) PreloadBloomFilter(ctx context.Context) error {
	if g.bloomFilter == nil {
		g.logger.Info("Bloom Filter not available, skipping preload")
		return nil
	}

	g.logger.Info("Starting Bloom Filter preload...")
	startTime := time.Now()

	// Get a sample of invalid MSISDNs to preload
	// We'll use a reasonable batch size to avoid memory issues
	batchSize := 10000
	totalLoaded := 0

	for {
		// Get batch of invalid MSISDNs
		invalidMSISDNS, err := g.repo.GetInvalidMSISDNS(ctx, make([]string, batchSize))
		if err != nil {
			g.logger.Error("Failed to get invalid MSISDNs for preload", zap.Error(err))
			break
		}

		if len(invalidMSISDNS) == 0 {
			break // No more invalid MSISDNs
		}

		// Add to Bloom Filter
		g.bloomFilter.AddBatch(invalidMSISDNS)
		totalLoaded += len(invalidMSISDNS)

		g.logger.Debug("Preloaded batch",
			zap.Int("batchSize", len(invalidMSISDNS)),
			zap.Int("totalLoaded", totalLoaded))

		// Check if we should continue
		if len(invalidMSISDNS) < batchSize {
			break // Last batch
		}
	}

	elapsed := time.Since(startTime)
	g.logger.Info("Bloom Filter preload completed",
		zap.Int("totalLoaded", totalLoaded),
		zap.Duration("elapsed", elapsed))

	return nil
}

// GetStats returns current statistics about the generator
func (g *OptimizedMSISDNGenerator) GetStats() map[string]interface{} {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	avgGenerationTime := time.Duration(0)
	if g.stats.generated > 0 {
		avgGenerationTime = g.stats.generationTime / time.Duration(g.stats.generated)
	}

	bloomHitRate := float64(0)
	if g.stats.bloomHits+g.stats.bloomMisses > 0 {
		bloomHitRate = float64(g.stats.bloomHits) / float64(g.stats.bloomHits+g.stats.bloomMisses)
	}

	return map[string]interface{}{
		"generated":             g.stats.generated,
		"validated":             g.stats.validated,
		"bloom_hits":            g.stats.bloomHits,
		"bloom_misses":          g.stats.bloomMisses,
		"bloom_hit_rate":        bloomHitRate,
		"total_generation_time": g.stats.generationTime.String(),
		"avg_generation_time":   avgGenerationTime.String(),
		"batch_size":            g.batchSize,
		"max_concurrent":        g.maxConcurrent,
		"cache_enabled":         g.cacheEnabled,
	}
}

// ResetStats resets all statistics
func (g *OptimizedMSISDNGenerator) ResetStats() {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	g.stats.generated = 0
	g.stats.validated = 0
	g.stats.bloomHits = 0
	g.stats.bloomMisses = 0
	g.stats.generationTime = 0
}

// SetConfiguration updates generator configuration
func (g *OptimizedMSISDNGenerator) SetConfiguration(batchSize, maxConcurrent int, cacheEnabled bool) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	g.batchSize = batchSize
	g.maxConcurrent = maxConcurrent
	g.cacheEnabled = cacheEnabled
}

// GetPerformanceMetrics returns detailed performance metrics
func (g *OptimizedMSISDNGenerator) GetPerformanceMetrics() map[string]interface{} {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	avgGenerationTime := time.Duration(0)
	if g.stats.generated > 0 {
		avgGenerationTime = g.stats.generationTime / time.Duration(g.stats.generated)
	}

	bloomHitRate := float64(0)
	if g.stats.bloomHits+g.stats.bloomMisses > 0 {
		bloomHitRate = float64(g.stats.bloomHits) / float64(g.stats.bloomHits+g.stats.bloomMisses)
	}

	throughput := float64(0)
	if g.stats.generationTime > 0 {
		throughput = float64(g.stats.generated) / g.stats.generationTime.Seconds()
	}

	return map[string]interface{}{
		"generated":                     g.stats.generated,
		"validated":                     g.stats.validated,
		"bloom_hits":                    g.stats.bloomHits,
		"bloom_misses":                  g.stats.bloomMisses,
		"bloom_hit_rate":                bloomHitRate,
		"total_generation_time":         g.stats.generationTime.String(),
		"avg_generation_time":           avgGenerationTime.String(),
		"throughput_msisdns_per_second": throughput,
		"batch_size":                    g.batchSize,
		"max_concurrent":                g.maxConcurrent,
		"cache_enabled":                 g.cacheEnabled,
		"bloom_filter_enabled":          g.bloomFilter != nil,
	}
}

// TunePerformance automatically tunes generator parameters based on current performance
func (g *OptimizedMSISDNGenerator) TunePerformance() {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	// Calculate current throughput
	currentThroughput := float64(0)
	if g.stats.generationTime > 0 {
		currentThroughput = float64(g.stats.generated) / g.stats.generationTime.Seconds()
	}

	// Target throughput (adjust based on your requirements)
	targetThroughput := 1000.0 // 1000 MSISDNs per second

	// Adjust batch size based on throughput
	if currentThroughput < targetThroughput*0.8 && g.batchSize < 5000 {
		// Increase batch size for better throughput
		newBatchSize := int(float64(g.batchSize) * 1.2)
		if newBatchSize > 5000 {
			newBatchSize = 5000
		}
		g.logger.Info("Increasing batch size for better throughput",
			zap.Int("old", g.batchSize),
			zap.Int("new", newBatchSize),
			zap.Float64("current_throughput", currentThroughput))
		g.batchSize = newBatchSize
	} else if currentThroughput > targetThroughput*1.2 && g.batchSize > 100 {
		// Decrease batch size if we're exceeding target
		newBatchSize := int(float64(g.batchSize) * 0.9)
		if newBatchSize < 100 {
			newBatchSize = 100
		}
		g.logger.Info("Decreasing batch size to optimize performance",
			zap.Int("old", g.batchSize),
			zap.Int("new", newBatchSize),
			zap.Float64("current_throughput", currentThroughput))
		g.batchSize = newBatchSize
	}

	// Adjust concurrency based on performance
	if currentThroughput < targetThroughput*0.7 && g.maxConcurrent < 200 {
		// Increase concurrency for better performance
		newConcurrency := int(float64(g.maxConcurrent) * 1.1)
		if newConcurrency > 200 {
			newConcurrency = 200
		}
		g.logger.Info("Increasing concurrency for better performance",
			zap.Int("old", g.maxConcurrent),
			zap.Int("new", newConcurrency),
			zap.Float64("current_throughput", currentThroughput))
		g.maxConcurrent = newConcurrency
	} else if currentThroughput > targetThroughput*1.3 && g.maxConcurrent > 10 {
		// Decrease concurrency if we're exceeding target significantly
		newConcurrency := int(float64(g.maxConcurrent) * 0.95)
		if newConcurrency < 10 {
			newConcurrency = 10
		}
		g.logger.Info("Decreasing concurrency to optimize resource usage",
			zap.Int("old", g.maxConcurrent),
			zap.Int("new", newConcurrency),
			zap.Float64("current_throughput", currentThroughput))
		g.maxConcurrent = newConcurrency
	}
}

// GetDetailedStats returns comprehensive statistics including performance metrics
func (g *OptimizedMSISDNGenerator) GetDetailedStats() map[string]interface{} {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	avgGenerationTime := time.Duration(0)
	if g.stats.generated > 0 {
		avgGenerationTime = g.stats.generationTime / time.Duration(g.stats.generated)
	}

	bloomHitRate := float64(0)
	if g.stats.bloomHits+g.stats.bloomMisses > 0 {
		bloomHitRate = float64(g.stats.bloomHits) / float64(g.stats.bloomHits+g.stats.bloomMisses)
	}

	throughput := float64(0)
	if g.stats.generationTime > 0 {
		throughput = float64(g.stats.generated) / g.stats.generationTime.Seconds()
	}

	// Calculate efficiency metrics
	efficiency := float64(0)
	if g.stats.generated > 0 {
		efficiency = float64(g.stats.validated) / float64(g.stats.generated) * 100
	}

	return map[string]interface{}{
		"generated":                     g.stats.generated,
		"validated":                     g.stats.validated,
		"bloom_hits":                    g.stats.bloomHits,
		"bloom_misses":                  g.stats.bloomMisses,
		"bloom_hit_rate":                bloomHitRate,
		"total_generation_time":         g.stats.generationTime.String(),
		"avg_generation_time":           avgGenerationTime.String(),
		"throughput_msisdns_per_second": throughput,
		"efficiency_percentage":         efficiency,
		"batch_size":                    g.batchSize,
		"max_concurrent":                g.maxConcurrent,
		"cache_enabled":                 g.cacheEnabled,
		"bloom_filter_enabled":          g.bloomFilter != nil,
		"performance_tuning": map[string]interface{}{
			"target_throughput":  1000.0,
			"current_throughput": throughput,
			"performance_ratio":  throughput / 1000.0,
		},
	}
}
