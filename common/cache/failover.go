package cached

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisMode string

const (
	RedisModePrimary  RedisMode = "primary"
	RedisModeFallback RedisMode = "fallback"
)

type RedisClient interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, exp time.Duration) error
	MGet(ctx context.Context, keys ...string) ([]interface{}, error)
	Exists(ctx context.Context, keys ...string) (int64, error)
	SetNX(ctx context.Context, key string, value interface{}, exp time.Duration) (bool, error)
	Del(ctx context.Context, keys ...string) (int64, error)
	Expire(ctx context.Context, key string, exp time.Duration) (bool, error)
	Keys(ctx context.Context, pattern string) ([]string, error)
	Scan(ctx context.Context, cursor uint64, match string, count int64) ([]string, uint64, error)
	HSet(ctx context.Context, key string, values ...interface{}) (int64, error)
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	Ping(ctx context.Context) (string, error)
	Mode() RedisMode
	Close() error
}

type FailoverRedisClient struct {
	primary         *redis.Client
	fallback        *memoryRedisStore
	mode            RedisMode
	fallbackEnabled bool
	checkInterval   time.Duration
	pingTimeout     time.Duration
	logTransitions  bool

	mu     sync.RWMutex
	stopCh chan struct{}
	wg     sync.WaitGroup
}

type failoverOptions struct {
	fallbackEnabled bool
	checkInterval   time.Duration
	pingTimeout     time.Duration
	logTransitions  bool
}

func NewFailoverRedisClient(redisOptions *redis.Options) RedisClient {
	if redisOptions == nil {
		redisOptions = &redis.Options{Addr: "localhost:6379", DB: 0}
	}

	opts := loadFailoverOptionsFromEnv()
	client := &FailoverRedisClient{
		primary:         redis.NewClient(redisOptions),
		fallback:        newMemoryRedisStore(),
		fallbackEnabled: opts.fallbackEnabled,
		checkInterval:   opts.checkInterval,
		pingTimeout:     opts.pingTimeout,
		logTransitions:  opts.logTransitions,
		stopCh:          make(chan struct{}),
	}

	if err := client.pingPrimary(); err != nil {
		if client.fallbackEnabled {
			client.mode = RedisModeFallback
			log.Printf("WARN: Redis unavailable at startup (%v), using in-memory fallback", err)
		} else {
			client.mode = RedisModePrimary
			log.Printf("WARN: Redis unavailable at startup (%v), fallback disabled", err)
		}
	} else {
		client.mode = RedisModePrimary
		log.Printf("INFO: Redis connection established, using primary backend")
	}

	client.wg.Add(1)
	go client.monitorPrimary()

	return client
}

func loadFailoverOptionsFromEnv() failoverOptions {
	return failoverOptions{
		fallbackEnabled: getEnvBool("APP_CACHE_FALLBACK_ENABLED", true),
		checkInterval:   getEnvDuration("APP_CACHE_FAILOVER_CHECK_INTERVAL", 5*time.Second),
		pingTimeout:     getEnvDuration("APP_CACHE_FAILOVER_PING_TIMEOUT", 1*time.Second),
		logTransitions:  getEnvBool("APP_CACHE_FAILOVER_LOG_TRANSITIONS", true),
	}
}

func (c *FailoverRedisClient) monitorPrimary() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := c.pingPrimary()
			if err == nil {
				if c.Mode() == RedisModeFallback {
					c.setMode(RedisModePrimary, "redis_recovered", nil)
				}
			} else {
				if c.fallbackEnabled && c.Mode() == RedisModePrimary {
					c.setMode(RedisModeFallback, "redis_unreachable", err)
				}
			}
		case <-c.stopCh:
			return
		}
	}
}

func (c *FailoverRedisClient) Mode() RedisMode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.mode
}

func (c *FailoverRedisClient) Close() error {
	select {
	case <-c.stopCh:
		// already closed
	default:
		close(c.stopCh)
	}
	c.wg.Wait()
	return c.primary.Close()
}

func (c *FailoverRedisClient) Ping(ctx context.Context) (string, error) {
	if c.Mode() == RedisModeFallback {
		return c.fallback.Ping(ctx)
	}
	res, err := c.primary.Ping(ctx).Result()
	if err != nil && c.shouldFailover(err) {
		c.setMode(RedisModeFallback, "ping_error", err)
		return c.fallback.Ping(ctx)
	}
	return res, err
}

func (c *FailoverRedisClient) Get(ctx context.Context, key string) (string, error) {
	if c.Mode() == RedisModeFallback {
		return c.fallback.Get(ctx, key)
	}
	res, err := c.primary.Get(ctx, key).Result()
	if err != nil && !errors.Is(err, redis.Nil) && c.shouldFailover(err) {
		c.setMode(RedisModeFallback, "get_error", err)
		return c.fallback.Get(ctx, key)
	}
	return res, err
}

func (c *FailoverRedisClient) Set(ctx context.Context, key string, value interface{}, exp time.Duration) error {
	if c.Mode() == RedisModeFallback {
		return c.fallback.Set(ctx, key, value, exp)
	}
	err := c.primary.Set(ctx, key, value, exp).Err()
	if err != nil && c.shouldFailover(err) {
		c.setMode(RedisModeFallback, "set_error", err)
		return c.fallback.Set(ctx, key, value, exp)
	}
	return err
}

func (c *FailoverRedisClient) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	if c.Mode() == RedisModeFallback {
		return c.fallback.MGet(ctx, keys...)
	}
	res, err := c.primary.MGet(ctx, keys...).Result()
	if err != nil && c.shouldFailover(err) {
		c.setMode(RedisModeFallback, "mget_error", err)
		return c.fallback.MGet(ctx, keys...)
	}
	return res, err
}

func (c *FailoverRedisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	if c.Mode() == RedisModeFallback {
		return c.fallback.Exists(ctx, keys...)
	}
	res, err := c.primary.Exists(ctx, keys...).Result()
	if err != nil && c.shouldFailover(err) {
		c.setMode(RedisModeFallback, "exists_error", err)
		return c.fallback.Exists(ctx, keys...)
	}
	return res, err
}

func (c *FailoverRedisClient) SetNX(ctx context.Context, key string, value interface{}, exp time.Duration) (bool, error) {
	if c.Mode() == RedisModeFallback {
		return c.fallback.SetNX(ctx, key, value, exp)
	}
	res, err := c.primary.SetNX(ctx, key, value, exp).Result()
	if err != nil && c.shouldFailover(err) {
		c.setMode(RedisModeFallback, "setnx_error", err)
		return c.fallback.SetNX(ctx, key, value, exp)
	}
	return res, err
}

func (c *FailoverRedisClient) Del(ctx context.Context, keys ...string) (int64, error) {
	if c.Mode() == RedisModeFallback {
		return c.fallback.Del(ctx, keys...)
	}
	res, err := c.primary.Del(ctx, keys...).Result()
	if err != nil && c.shouldFailover(err) {
		c.setMode(RedisModeFallback, "del_error", err)
		return c.fallback.Del(ctx, keys...)
	}
	return res, err
}

func (c *FailoverRedisClient) Expire(ctx context.Context, key string, exp time.Duration) (bool, error) {
	if c.Mode() == RedisModeFallback {
		return c.fallback.Expire(ctx, key, exp)
	}
	res, err := c.primary.Expire(ctx, key, exp).Result()
	if err != nil && c.shouldFailover(err) {
		c.setMode(RedisModeFallback, "expire_error", err)
		return c.fallback.Expire(ctx, key, exp)
	}
	return res, err
}

func (c *FailoverRedisClient) Keys(ctx context.Context, pattern string) ([]string, error) {
	if c.Mode() == RedisModeFallback {
		return c.fallback.Keys(ctx, pattern)
	}
	res, err := c.primary.Keys(ctx, pattern).Result()
	if err != nil && c.shouldFailover(err) {
		c.setMode(RedisModeFallback, "keys_error", err)
		return c.fallback.Keys(ctx, pattern)
	}
	return res, err
}

func (c *FailoverRedisClient) Scan(ctx context.Context, cursor uint64, match string, count int64) ([]string, uint64, error) {
	if c.Mode() == RedisModeFallback {
		return c.fallback.Scan(ctx, cursor, match, count)
	}
	res, next, err := c.primary.Scan(ctx, cursor, match, count).Result()
	if err != nil && c.shouldFailover(err) {
		c.setMode(RedisModeFallback, "scan_error", err)
		return c.fallback.Scan(ctx, cursor, match, count)
	}
	return res, next, err
}

func (c *FailoverRedisClient) HSet(ctx context.Context, key string, values ...interface{}) (int64, error) {
	if c.Mode() == RedisModeFallback {
		return c.fallback.HSet(ctx, key, values...)
	}
	res, err := c.primary.HSet(ctx, key, values...).Result()
	if err != nil && c.shouldFailover(err) {
		c.setMode(RedisModeFallback, "hset_error", err)
		return c.fallback.HSet(ctx, key, values...)
	}
	return res, err
}

func (c *FailoverRedisClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	if c.Mode() == RedisModeFallback {
		return c.fallback.HGetAll(ctx, key)
	}
	res, err := c.primary.HGetAll(ctx, key).Result()
	if err != nil && c.shouldFailover(err) {
		c.setMode(RedisModeFallback, "hgetall_error", err)
		return c.fallback.HGetAll(ctx, key)
	}
	return res, err
}

func (c *FailoverRedisClient) pingPrimary() error {
	ctx, cancel := context.WithTimeout(context.Background(), c.pingTimeout)
	defer cancel()
	return c.primary.Ping(ctx).Err()
}

func (c *FailoverRedisClient) setMode(mode RedisMode, reason string, err error) {
	if mode == RedisModeFallback && !c.fallbackEnabled {
		return
	}

	c.mu.Lock()
	prev := c.mode
	if prev == mode {
		c.mu.Unlock()
		return
	}
	c.mode = mode
	c.mu.Unlock()

	if !c.logTransitions {
		return
	}

	if err != nil {
		log.Printf("WARN: Redis mode transition: %s -> %s reason=%s err=%v", prev, mode, reason, err)
		return
	}
	log.Printf("INFO: Redis mode transition: %s -> %s reason=%s", prev, mode, reason)
}

func (c *FailoverRedisClient) shouldFailover(err error) bool {
	if !c.fallbackEnabled || err == nil {
		return false
	}
	errText := strings.ToLower(err.Error())
	if strings.Contains(errText, "connection refused") ||
		strings.Contains(errText, "no such host") ||
		strings.Contains(errText, "i/o timeout") ||
		strings.Contains(errText, "network is unreachable") ||
		strings.Contains(errText, "broken pipe") ||
		strings.Contains(errText, "eof") {
		return true
	}
	return false
}

func getEnvBool(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	if v == "1" || v == "true" || v == "yes" || v == "on" {
		return true
	}
	if v == "0" || v == "false" || v == "no" || v == "off" {
		return false
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

type memoryEntry struct {
	value     string
	hash      map[string]string
	expiresAt time.Time
}

type memoryRedisStore struct {
	mu      sync.RWMutex
	entries map[string]memoryEntry
}

func newMemoryRedisStore() *memoryRedisStore {
	return &memoryRedisStore{entries: make(map[string]memoryEntry)}
}

func (m *memoryRedisStore) Ping(context.Context) (string, error) { return "PONG", nil }

func (m *memoryRedisStore) Set(_ context.Context, key string, value interface{}, exp time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupExpiredLocked()

	entry := memoryEntry{value: stringify(value)}
	if exp > 0 {
		entry.expiresAt = time.Now().Add(exp)
	}
	m.entries[key] = entry
	return nil
}

func (m *memoryRedisStore) Get(_ context.Context, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupExpiredLocked()

	entry, ok := m.entries[key]
	if !ok || entry.hash != nil {
		return "", redis.Nil
	}
	return entry.value, nil
}

func (m *memoryRedisStore) MGet(_ context.Context, keys ...string) ([]interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupExpiredLocked()

	res := make([]interface{}, len(keys))
	for i, key := range keys {
		entry, ok := m.entries[key]
		if !ok || entry.hash != nil {
			res[i] = nil
			continue
		}
		res[i] = entry.value
	}
	return res, nil
}

func (m *memoryRedisStore) Exists(_ context.Context, keys ...string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupExpiredLocked()

	var count int64
	for _, key := range keys {
		if _, ok := m.entries[key]; ok {
			count++
		}
	}
	return count, nil
}

func (m *memoryRedisStore) SetNX(_ context.Context, key string, value interface{}, exp time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupExpiredLocked()

	if _, ok := m.entries[key]; ok {
		return false, nil
	}

	entry := memoryEntry{value: stringify(value)}
	if exp > 0 {
		entry.expiresAt = time.Now().Add(exp)
	}
	m.entries[key] = entry
	return true, nil
}

func (m *memoryRedisStore) Del(_ context.Context, keys ...string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var deleted int64
	for _, key := range keys {
		if _, ok := m.entries[key]; ok {
			delete(m.entries, key)
			deleted++
		}
	}
	return deleted, nil
}

func (m *memoryRedisStore) Expire(_ context.Context, key string, exp time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupExpiredLocked()

	entry, ok := m.entries[key]
	if !ok {
		return false, nil
	}
	if exp <= 0 {
		entry.expiresAt = time.Time{}
	} else {
		entry.expiresAt = time.Now().Add(exp)
	}
	m.entries[key] = entry
	return true, nil
}

func (m *memoryRedisStore) Keys(_ context.Context, pattern string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupExpiredLocked()

	keys := make([]string, 0, len(m.entries))
	for key := range m.entries {
		if matchesPattern(pattern, key) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

func (m *memoryRedisStore) Scan(ctx context.Context, cursor uint64, match string, count int64) ([]string, uint64, error) {
	keys, err := m.Keys(ctx, match)
	if err != nil {
		return nil, 0, err
	}
	if count <= 0 {
		count = 10
	}
	if cursor >= uint64(len(keys)) {
		return []string{}, 0, nil
	}

	start := int(cursor)
	end := start + int(count)
	if end >= len(keys) {
		return keys[start:], 0, nil
	}
	return keys[start:end], uint64(end), nil
}

func (m *memoryRedisStore) HSet(_ context.Context, key string, values ...interface{}) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupExpiredLocked()

	entry, ok := m.entries[key]
	if !ok || entry.hash == nil {
		entry = memoryEntry{hash: make(map[string]string), expiresAt: entry.expiresAt}
	}

	var added int64
	if len(values) == 1 {
		switch v := values[0].(type) {
		case map[string]interface{}:
			for k, raw := range v {
				if _, exists := entry.hash[k]; !exists {
					added++
				}
				entry.hash[k] = stringify(raw)
			}
		case map[string]string:
			for k, raw := range v {
				if _, exists := entry.hash[k]; !exists {
					added++
				}
				entry.hash[k] = raw
			}
		default:
			return 0, fmt.Errorf("unsupported HSet map type %T", values[0])
		}
	} else {
		if len(values)%2 != 0 {
			return 0, fmt.Errorf("HSet expects even number of key/value pairs")
		}
		for i := 0; i < len(values); i += 2 {
			field := stringify(values[i])
			if _, exists := entry.hash[field]; !exists {
				added++
			}
			entry.hash[field] = stringify(values[i+1])
		}
	}

	m.entries[key] = entry
	return added, nil
}

func (m *memoryRedisStore) HGetAll(_ context.Context, key string) (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupExpiredLocked()

	entry, ok := m.entries[key]
	if !ok || entry.hash == nil {
		return map[string]string{}, nil
	}

	copyMap := make(map[string]string, len(entry.hash))
	for k, v := range entry.hash {
		copyMap[k] = v
	}
	return copyMap, nil
}

func (m *memoryRedisStore) cleanupExpiredLocked() {
	now := time.Now()
	for key, entry := range m.entries {
		if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
			delete(m.entries, key)
		}
	}
}

func matchesPattern(pattern, key string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	ok, err := path.Match(pattern, key)
	if err != nil {
		return key == pattern
	}
	return ok
}

func stringify(v interface{}) string {
	switch value := v.(type) {
	case nil:
		return ""
	case string:
		return value
	case []byte:
		return string(value)
	case fmt.Stringer:
		return value.String()
	case int:
		return strconv.Itoa(value)
	case int64:
		return strconv.FormatInt(value, 10)
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case bool:
		if value {
			return "1"
		}
		return "0"
	default:
		return fmt.Sprint(value)
	}
}
