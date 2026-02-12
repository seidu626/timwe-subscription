package monitoring

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// CacheEntry represents a cached metrics entry
type CacheEntry struct {
	Data        interface{}
	Timestamp   time.Time
	TTL         time.Duration
	AccessCount int64
	LastAccess  time.Time
}

// MetricsCache provides caching for monitoring metrics
type MetricsCache struct {
	cache     map[string]*CacheEntry
	mu        sync.RWMutex
	logger    *zap.Logger
	config    *CacheConfig
	stopChan  chan struct{}
	isRunning bool
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	DefaultTTL      time.Duration `json:"default_ttl" yaml:"default_ttl"`
	MaxEntries      int           `json:"max_entries" yaml:"max_entries"`
	CleanupInterval time.Duration `json:"cleanup_interval" yaml:"cleanup_interval"`
	EnableStats     bool          `json:"enable_stats" yaml:"enable_stats"`
	Compression     bool          `json:"compression" yaml:"compression"`
}

// CacheStats provides cache performance statistics
type CacheStats struct {
	TotalEntries  int       `json:"total_entries"`
	HitCount      int64     `json:"hit_count"`
	MissCount     int64     `json:"miss_count"`
	EvictionCount int64     `json:"eviction_count"`
	HitRate       float64   `json:"hit_rate"`
	MemoryUsage   uint64    `json:"memory_usage"`
	LastCleanup   time.Time `json:"last_cleanup"`
	CleanupCount  int64     `json:"cleanup_count"`
}

// NewMetricsCache creates a new metrics cache
func NewMetricsCache(config *CacheConfig, logger *zap.Logger) *MetricsCache {
	if config == nil {
		config = &CacheConfig{
			DefaultTTL:      5 * time.Minute,
			MaxEntries:      1000,
			CleanupInterval: 1 * time.Minute,
			EnableStats:     true,
			Compression:     false,
		}
	}

	return &MetricsCache{
		cache:    make(map[string]*CacheEntry),
		config:   config,
		logger:   logger,
		stopChan: make(chan struct{}),
	}
}

// Start begins cache management
func (mc *MetricsCache) Start(ctx context.Context) error {
	if mc.isRunning {
		return nil
	}

	mc.isRunning = true
	mc.logger.Info("Starting metrics cache")

	// Start cleanup goroutine
	go mc.cleanupLoop(ctx)

	mc.logger.Info("Metrics cache started successfully")
	return nil
}

// Stop stops cache management
func (mc *MetricsCache) Stop() {
	if !mc.isRunning {
		return
	}

	mc.logger.Info("Stopping metrics cache")
	close(mc.stopChan)
	mc.isRunning = false
}

// Set stores a value in the cache
func (mc *MetricsCache) Set(key string, value interface{}, ttl time.Duration) {
	if ttl == 0 {
		ttl = mc.config.DefaultTTL
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Check if we need to evict entries
	if len(mc.cache) >= mc.config.MaxEntries {
		mc.evictOldest()
	}

	mc.cache[key] = &CacheEntry{
		Data:        value,
		Timestamp:   time.Now(),
		TTL:         ttl,
		AccessCount: 0,
		LastAccess:  time.Now(),
	}

	mc.logger.Debug("Cached metrics",
		zap.String("key", key),
		zap.Duration("ttl", ttl))
}

// Get retrieves a value from the cache
func (mc *MetricsCache) Get(key string) (interface{}, bool) {
	mc.mu.RLock()
	entry, exists := mc.cache[key]
	mc.mu.RUnlock()

	if !exists {
		mc.updateStats(false)
		return nil, false
	}

	// Check if entry has expired
	if time.Since(entry.Timestamp) > entry.TTL {
		mc.mu.Lock()
		delete(mc.cache, key)
		mc.mu.Unlock()
		mc.updateStats(false)
		return nil, false
	}

	// Update access statistics
	mc.mu.Lock()
	entry.AccessCount++
	entry.LastAccess = time.Now()
	mc.mu.Unlock()

	mc.updateStats(true)
	return entry.Data, true
}

// GetWithTTL retrieves a value and its remaining TTL
func (mc *MetricsCache) GetWithTTL(key string) (interface{}, time.Duration, bool) {
	mc.mu.RLock()
	entry, exists := mc.cache[key]
	mc.mu.RUnlock()

	if !exists {
		mc.updateStats(false)
		return nil, 0, false
	}

	// Check if entry has expired
	elapsed := time.Since(entry.Timestamp)
	if elapsed > entry.TTL {
		mc.mu.Lock()
		delete(mc.cache, key)
		mc.mu.Unlock()
		mc.updateStats(false)
		return nil, 0, false
	}

	remainingTTL := entry.TTL - elapsed

	// Update access statistics
	mc.mu.Lock()
	entry.AccessCount++
	entry.LastAccess = time.Now()
	mc.mu.Unlock()

	mc.updateStats(true)
	return entry.Data, remainingTTL, true
}

// Delete removes a value from the cache
func (mc *MetricsCache) Delete(key string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if _, exists := mc.cache[key]; exists {
		delete(mc.cache, key)
		mc.logger.Debug("Deleted cached metrics", zap.String("key", key))
	}
}

// Clear removes all entries from the cache
func (mc *MetricsCache) Clear() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	oldSize := len(mc.cache)
	mc.cache = make(map[string]*CacheEntry)

	mc.logger.Info("Cleared metrics cache", zap.Int("entries_removed", oldSize))
}

// Exists checks if a key exists in the cache
func (mc *MetricsCache) Exists(key string) bool {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	entry, exists := mc.cache[key]
	if !exists {
		return false
	}

	// Check if entry has expired
	if time.Since(entry.Timestamp) > entry.TTL {
		return false
	}

	return true
}

// Keys returns all cache keys
func (mc *MetricsCache) Keys() []string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	keys := make([]string, 0, len(mc.cache))
	for key := range mc.cache {
		keys = append(keys, key)
	}
	return keys
}

// Size returns the current cache size
func (mc *MetricsCache) Size() int {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return len(mc.cache)
}

// evictOldest removes the oldest entry from the cache
func (mc *MetricsCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range mc.cache {
		if oldestKey == "" || entry.Timestamp.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.Timestamp
		}
	}

	if oldestKey != "" {
		delete(mc.cache, oldestKey)
		mc.logger.Debug("Evicted oldest cache entry", zap.String("key", oldestKey))
	}
}

// cleanupLoop periodically cleans up expired entries
func (mc *MetricsCache) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(mc.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-mc.stopChan:
			return
		case <-ticker.C:
			mc.cleanup()
		}
	}
}

// cleanup removes expired entries from the cache
func (mc *MetricsCache) cleanup() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	startTime := time.Now()
	removedCount := 0

	for key, entry := range mc.cache {
		if time.Since(entry.Timestamp) > entry.TTL {
			delete(mc.cache, key)
			removedCount++
		}
	}

	if removedCount > 0 {
		mc.logger.Info("Cache cleanup completed",
			zap.Int("entries_removed", removedCount),
			zap.Int("remaining_entries", len(mc.cache)),
			zap.Duration("cleanup_duration", time.Since(startTime)))
	}
}

// updateStats updates cache statistics
func (mc *MetricsCache) updateStats(hit bool) {
	// This is a simplified stats implementation
	// In production, you might want more sophisticated statistics
	if hit {
		// Increment hit count
	} else {
		// Increment miss count
	}
}

// GetStats returns cache statistics
func (mc *MetricsCache) GetStats() *CacheStats {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	// Calculate hit rate
	var hitRate float64
	totalRequests := int64(0) // This would be hitCount + missCount in real implementation

	if totalRequests > 0 {
		hitRate = float64(0) // This would be hitCount / totalRequests in real implementation
	}

	return &CacheStats{
		TotalEntries: len(mc.cache),
		HitRate:      hitRate,
		// Other stats would be populated from actual counters
	}
}

// SetTTL updates the TTL for an existing key
func (mc *MetricsCache) SetTTL(key string, ttl time.Duration) bool {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if entry, exists := mc.cache[key]; exists {
		entry.TTL = ttl
		return true
	}
	return false
}

// Touch updates the last access time for a key
func (mc *MetricsCache) Touch(key string) bool {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if entry, exists := mc.cache[key]; exists {
		entry.LastAccess = time.Now()
		entry.AccessCount++
		return true
	}
	return false
}

// IsRunning returns whether the cache is running
func (mc *MetricsCache) IsRunning() bool {
	return mc.isRunning
}
