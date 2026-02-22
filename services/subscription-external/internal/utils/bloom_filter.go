package utils

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bits-and-blooms/bloom/v3"
	cached "github.com/seidu626/subscription-manager/common/cache"
	"go.uber.org/zap"
)

// MSISDNBloomFilter provides ultra-fast negative lookups for invalid MSISDNs
// Uses Bloom Filter to quickly determine if an MSISDN might be invalid
// False positives are acceptable (will fall back to database check)
type MSISDNBloomFilter struct {
	filter *bloom.BloomFilter
	mutex  sync.RWMutex
	redis  cached.RedisClient
	logger *zap.Logger

	// Configuration
	expectedItems     uint
	falsePositiveRate float64
	ttl               time.Duration

	// Statistics
	stats struct {
		itemsAdded     uint64
		cacheHits      uint64
		cacheMisses    uint64
		falsePositives uint64
	}
}

// NewMSISDNBloomFilter creates a new Bloom Filter for MSISDN validation
func NewMSISDNBloomFilter(expectedItems uint, falsePositiveRate float64, redisClient cached.RedisClient, logger *zap.Logger) *MSISDNBloomFilter {
	bf := &MSISDNBloomFilter{
		filter:            bloom.NewWithEstimates(expectedItems, falsePositiveRate),
		redis:             redisClient,
		logger:            logger,
		expectedItems:     expectedItems,
		falsePositiveRate: falsePositiveRate,
		ttl:               24 * time.Hour, // Cache for 24 hours
	}

	// Load existing data from Redis if available
	if bf.redis != nil {
		go bf.loadFromRedis()
	}

	return bf
}

// Add adds an MSISDN to the Bloom Filter
func (bf *MSISDNBloomFilter) Add(msisdn string) {
	bf.mutex.Lock()
	defer bf.mutex.Unlock()

	bf.filter.AddString(msisdn)
	bf.stats.itemsAdded++

	// Also cache in Redis for persistence
	bf.cacheInRedis(msisdn)
}

// AddBatch adds multiple MSISDNs to the Bloom Filter efficiently
func (bf *MSISDNBloomFilter) AddBatch(msisdns []string) {
	bf.mutex.Lock()
	defer bf.mutex.Unlock()

	for _, msisdn := range msisdns {
		bf.filter.AddString(msisdn)
		bf.stats.itemsAdded++
	}

	// Batch cache in Redis
	bf.cacheBatchInRedis(msisdns)
}

// MightContain checks if an MSISDN might be in the Bloom Filter
// Returns true if the MSISDN might be invalid (false positives possible)
// Returns false if the MSISDN is definitely not invalid
func (bf *MSISDNBloomFilter) MightContain(msisdn string) bool {
	bf.mutex.RLock()
	defer bf.mutex.RUnlock()

	return bf.filter.TestString(msisdn)
}

// CheckMSISDN performs a comprehensive check with fallback to Redis and database
// Returns true if MSISDN is invalid, false if valid
func (bf *MSISDNBloomFilter) CheckMSISDN(ctx context.Context, msisdn string, dbCheck func(string) (bool, error)) (bool, error) {
	// Step 1: Check Bloom Filter (fastest)
	if !bf.MightContain(msisdn) {
		bf.stats.cacheHits++
		return false, nil // Definitely not invalid
	}

	// Step 2: Check Redis cache (fast)
	invalid, err := bf.checkRedisCache(ctx, msisdn)
	if err == nil && invalid {
		bf.stats.cacheHits++
		return true, nil // Found in cache
	}

	// Step 3: Fall back to database check (slowest but accurate)
	bf.stats.cacheMisses++
	invalid, err = dbCheck(msisdn)
	if err != nil {
		return false, fmt.Errorf("database check failed: %w", err)
	}

	// If found in database, add to Bloom Filter for future fast lookups
	if invalid {
		bf.Add(msisdn)
		bf.stats.falsePositives++ // This was a false positive in Bloom Filter
	}

	return invalid, nil
}

// checkRedisCache checks if MSISDN is cached in Redis
func (bf *MSISDNBloomFilter) checkRedisCache(ctx context.Context, msisdn string) (bool, error) {
	if bf.redis == nil {
		return false, fmt.Errorf("redis client is not configured")
	}
	key := fmt.Sprintf("invalid_msisdn:%s", msisdn)
	exists, err := bf.redis.Exists(ctx, key)
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// cacheInRedis caches an MSISDN in Redis
func (bf *MSISDNBloomFilter) cacheInRedis(msisdn string) {
	if bf.redis == nil {
		return
	}
	key := fmt.Sprintf("invalid_msisdn:%s", msisdn)
	ctx := context.Background()

	err := bf.redis.Set(ctx, key, "1", bf.ttl)
	if err != nil {
		bf.logger.Warn("Failed to cache MSISDN in Redis",
			zap.String("msisdn", msisdn),
			zap.Error(err))
	}
}

// cacheBatchInRedis caches multiple MSISDNs in Redis efficiently
func (bf *MSISDNBloomFilter) cacheBatchInRedis(msisdns []string) {
	if len(msisdns) == 0 {
		return
	}
	if bf.redis == nil {
		return
	}

	ctx := context.Background()
	for _, msisdn := range msisdns {
		key := fmt.Sprintf("invalid_msisdn:%s", msisdn)
		if err := bf.redis.Set(ctx, key, "1", bf.ttl); err != nil {
			bf.logger.Warn("Failed to cache MSISDN in batch",
				zap.String("msisdn", msisdn),
				zap.Error(err))
		}
	}
}

// loadFromRedis loads existing invalid MSISDNs from Redis into the Bloom Filter
func (bf *MSISDNBloomFilter) loadFromRedis() {
	if bf.redis == nil {
		return
	}
	ctx := context.Background()
	pattern := "invalid_msisdn:*"

	// Scan Redis for all invalid MSISDN keys
	var cursor uint64
	var keys []string
	var err error

	for {
		keys, cursor, err = bf.redis.Scan(ctx, cursor, pattern, 1000)
		if err != nil {
			bf.logger.Error("Failed to scan Redis for invalid MSISDNs", zap.Error(err))
			return
		}

		// Extract MSISDNs from keys and add to Bloom Filter
		for _, key := range keys {
			if len(key) > 13 { // "invalid_msisdn:" is 13 chars
				msisdn := key[13:] // Remove "invalid_msisdn:" prefix
				bf.Add(msisdn)
			}
		}

		if cursor == 0 {
			break
		}
	}

	bf.logger.Info("Loaded invalid MSISDNs from Redis into Bloom Filter",
		zap.Uint64("itemsLoaded", bf.stats.itemsAdded))
}

// GetStats returns current statistics about the Bloom Filter
func (bf *MSISDNBloomFilter) GetStats() map[string]interface{} {
	bf.mutex.RLock()
	defer bf.mutex.RUnlock()

	// Calculate hit rate safely
	hitRate := float64(0)
	if bf.stats.cacheHits+bf.stats.cacheMisses > 0 {
		hitRate = float64(bf.stats.cacheHits) / float64(bf.stats.cacheHits+bf.stats.cacheMisses)
	}

	return map[string]interface{}{
		"expected_items":      bf.expectedItems,
		"false_positive_rate": bf.falsePositiveRate,
		"items_added":         bf.stats.itemsAdded,
		"cache_hits":          bf.stats.cacheHits,
		"cache_misses":        bf.stats.cacheMisses,
		"false_positives":     bf.stats.falsePositives,
		"filter_size":         bf.filter.Cap(),
		"filter_count":        bf.stats.itemsAdded, // Use our tracked count instead
		"hit_rate":            hitRate,
	}
}

// Reset clears the Bloom Filter and statistics
func (bf *MSISDNBloomFilter) Reset() {
	bf.mutex.Lock()
	defer bf.mutex.Unlock()

	bf.filter = bloom.NewWithEstimates(bf.expectedItems, bf.falsePositiveRate)
	bf.stats.itemsAdded = 0
	bf.stats.cacheHits = 0
	bf.stats.cacheMisses = 0
	bf.stats.falsePositives = 0

	bf.logger.Info("Bloom Filter reset")
}

// Optimize resizes the Bloom Filter based on current usage
func (bf *MSISDNBloomFilter) Optimize() {
	bf.mutex.Lock()
	defer bf.mutex.Unlock()

	currentCount := uint(bf.stats.itemsAdded) // Convert to uint for comparison
	if currentCount > bf.expectedItems*2 {
		// Resize to accommodate more items
		newExpectedItems := currentCount * 2
		newFilter := bloom.NewWithEstimates(newExpectedItems, bf.falsePositiveRate)

		// Note: We can't easily migrate existing items, so we'll start fresh
		// In production, you might want to implement a migration strategy
		bf.filter = newFilter
		bf.expectedItems = newExpectedItems

		bf.logger.Info("Bloom Filter resized",
			zap.Uint("newExpectedItems", newExpectedItems))
	}
}

// Export exports the Bloom Filter data for backup/restore
func (bf *MSISDNBloomFilter) Export() ([]byte, error) {
	bf.mutex.RLock()
	defer bf.mutex.RUnlock()

	return bf.filter.MarshalBinary()
}

// Import imports Bloom Filter data from backup
func (bf *MSISDNBloomFilter) Import(data []byte) error {
	bf.mutex.Lock()
	defer bf.mutex.Unlock()

	newFilter := &bloom.BloomFilter{}
	err := newFilter.UnmarshalBinary(data)
	if err != nil {
		return fmt.Errorf("failed to unmarshal Bloom Filter data: %w", err)
	}

	bf.filter = newFilter
	bf.logger.Info("Bloom Filter imported from backup")
	return nil
}

// Close performs cleanup operations
func (bf *MSISDNBloomFilter) Close() error {
	bf.logger.Info("Closing MSISDN Bloom Filter", zap.Any("final_stats", bf.GetStats()))
	return nil
}
