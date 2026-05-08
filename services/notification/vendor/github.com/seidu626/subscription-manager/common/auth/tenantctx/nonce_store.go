package tenantctx

import (
	"strings"
	"sync"
	"time"
)

type MemoryNonceStore struct {
	mu    sync.Mutex
	seen  map[string]time.Time
	clock func() time.Time
}

func NewMemoryNonceStore() *MemoryNonceStore {
	return &MemoryNonceStore{
		seen:  make(map[string]time.Time),
		clock: time.Now,
	}
}

func (s *MemoryNonceStore) Use(nonce string, expiresAt time.Time) bool {
	nonce = strings.TrimSpace(nonce)
	if nonce == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock().UTC()
	for key, expiry := range s.seen {
		if !expiry.After(now) {
			delete(s.seen, key)
		}
	}
	if expiry, ok := s.seen[nonce]; ok && expiry.After(now) {
		return false
	}
	s.seen[nonce] = expiresAt.UTC()
	return true
}
