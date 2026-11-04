package service

import (
	"sync"
	"time"
)

// productOwnerCacheTTL bounds how long a product's owner is reused. Ownership
// changes are rare admin actions; a new owner is seen within this window.
const productOwnerCacheTTL = time.Minute

// productOwnerCacheMax caps the entries kept; the cache is cleared when full.
const productOwnerCacheMax = 4096

type productOwnerEntry struct {
	tenantID string
	err      error
	expires  time.Time
}

type productOwnerCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	now     func() time.Time
	entries map[int]productOwnerEntry
}

func newProductOwnerCache(ttl time.Duration) *productOwnerCache {
	return &productOwnerCache{ttl: ttl, now: time.Now, entries: make(map[int]productOwnerEntry)}
}

// get returns a live entry. A nil cache never hits.
func (c *productOwnerCache) get(productID int) (string, error, bool) {
	if c == nil {
		return "", nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[productID]
	if !ok || !c.now().Before(entry.expires) {
		return "", nil, false
	}
	return entry.tenantID, entry.err, true
}

func (c *productOwnerCache) put(productID int, tenantID string, err error) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= productOwnerCacheMax {
		clear(c.entries)
	}
	c.entries[productID] = productOwnerEntry{tenantID: tenantID, err: err, expires: c.now().Add(c.ttl)}
}
