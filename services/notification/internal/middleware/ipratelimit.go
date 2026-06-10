package middleware

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

// Clock is an interface for time operations, injectable for deterministic tests.
type Clock interface {
	Now() time.Time
}

// realClock is the production Clock implementation.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// RealClock is the default production clock.
var RealClock Clock = realClock{}

// ipBucket holds the per-IP token-bucket state.
type ipBucket struct {
	mu       sync.Mutex
	tokens   float64
	lastSeen time.Time
}

// IPRateLimiter is a per-IP token-bucket rate limiter.
// Safe for concurrent use. When limit is 0, all requests are allowed.
type IPRateLimiter struct {
	// maxPerMin is the bucket capacity and refill rate (tokens per minute).
	// 0 means disabled.
	maxPerMin float64
	mu        sync.Mutex
	buckets   map[string]*ipBucket
	clock     Clock
}

// NewIPRateLimiterFromEnv creates an IPRateLimiter configured from the
// CALLBACK_RATE_LIMIT_PER_MIN environment variable.
//   - Unset or "0" → disabled (all requests pass).
//   - Positive integer → that many requests per minute per source IP.
func NewIPRateLimiterFromEnv(clock Clock) *IPRateLimiter {
	limit := 0.0
	if raw := strings.TrimSpace(os.Getenv("CALLBACK_RATE_LIMIT_PER_MIN")); raw != "" {
		if v, err := strconv.ParseFloat(raw, 64); err == nil && v > 0 {
			limit = v
		}
	}
	return NewIPRateLimiter(limit, clock)
}

// NewIPRateLimiter creates an IPRateLimiter with a given limit (requests/min).
// limit=0 disables rate limiting.
func NewIPRateLimiter(limitPerMin float64, clock Clock) *IPRateLimiter {
	if clock == nil {
		clock = RealClock
	}
	return &IPRateLimiter{
		maxPerMin: limitPerMin,
		buckets:   make(map[string]*ipBucket),
		clock:     clock,
	}
}

// Allow returns true if the request from ip is within the rate limit.
// When the limiter is disabled (maxPerMin == 0), it always returns true.
func (l *IPRateLimiter) Allow(ip string) bool {
	if l.maxPerMin == 0 {
		return true
	}

	l.mu.Lock()
	b, ok := l.buckets[ip]
	if !ok {
		b = &ipBucket{tokens: l.maxPerMin, lastSeen: l.clock.Now()}
		l.buckets[ip] = b
	}
	l.mu.Unlock()

	b.mu.Lock()
	defer b.mu.Unlock()

	now := l.clock.Now()
	elapsed := now.Sub(b.lastSeen).Minutes()
	b.lastSeen = now

	// Refill tokens proportional to elapsed time.
	b.tokens += elapsed * l.maxPerMin
	if b.tokens > l.maxPerMin {
		b.tokens = l.maxPerMin
	}

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// sourceIP returns the real client IP from X-Forwarded-For (first hop only,
// as the gateway is the only trusted proxy) or falls back to RemoteAddr.
func sourceIP(ctx *fasthttp.RequestCtx) string {
	xff := strings.TrimSpace(string(ctx.Request.Header.Peek("X-Forwarded-For")))
	if xff != "" {
		// Take only the first entry — the gateway MUST be the direct upstream,
		// so the first entry is the real client IP injected by the gateway.
		if idx := strings.IndexByte(xff, ','); idx != -1 {
			xff = strings.TrimSpace(xff[:idx])
		}
		if xff != "" {
			return xff
		}
	}
	return ctx.RemoteIP().String()
}

// CallbackIPRateLimitMiddleware wraps next with per-IP rate limiting.
// When the limiter is disabled (nil or maxPerMin == 0), returns next unchanged.
func CallbackIPRateLimitMiddleware(next fasthttp.RequestHandler, limiter *IPRateLimiter) fasthttp.RequestHandler {
	if limiter == nil || limiter.maxPerMin == 0 {
		return next
	}
	return func(ctx *fasthttp.RequestCtx) {
		ip := sourceIP(ctx)
		if !limiter.Allow(ip) {
			ctx.SetStatusCode(fasthttp.StatusTooManyRequests)
			ctx.SetContentType("application/json")
			ctx.SetBodyString(`{"message":"rate limit exceeded","code":"RATE_LIMITED","inError":"true"}`)
			return
		}
		next(ctx)
	}
}
