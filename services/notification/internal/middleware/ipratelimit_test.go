package middleware

import (
	"net"
	"testing"
	"time"

	"github.com/valyala/fasthttp"
)

// fixedClock is a deterministic Clock for tests.
type fixedClock struct {
	t time.Time
}

func (c *fixedClock) Now() time.Time { return c.t }
func (c *fixedClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

func newFixedClock() *fixedClock {
	return &fixedClock{t: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}
}

// TestIPRateLimiter_Disabled verifies that limit=0 always allows.
func TestIPRateLimiter_Disabled(t *testing.T) {
	l := NewIPRateLimiter(0, newFixedClock())
	for i := 0; i < 1000; i++ {
		if !l.Allow("1.2.3.4") {
			t.Fatalf("disabled limiter denied request %d", i)
		}
	}
}

// TestIPRateLimiter_LimitEnforced verifies that requests beyond the bucket are denied.
func TestIPRateLimiter_LimitEnforced(t *testing.T) {
	clk := newFixedClock()
	const limit = 3.0
	l := NewIPRateLimiter(limit, clk)

	// First 3 requests should be allowed.
	for i := 0; i < int(limit); i++ {
		if !l.Allow("10.0.0.1") {
			t.Fatalf("request %d should be allowed (within limit)", i+1)
		}
	}
	// 4th request should be denied (time has not advanced).
	if l.Allow("10.0.0.1") {
		t.Fatal("4th request should be denied")
	}
}

// TestIPRateLimiter_RefillAfterTime verifies that tokens refill after time passes.
func TestIPRateLimiter_RefillAfterTime(t *testing.T) {
	clk := newFixedClock()
	const limit = 2.0
	l := NewIPRateLimiter(limit, clk)

	// Consume all tokens.
	l.Allow("10.0.0.2")
	l.Allow("10.0.0.2")
	if l.Allow("10.0.0.2") {
		t.Fatal("should be denied after bucket empty")
	}

	// Advance time by 1 full minute → bucket refills to max.
	clk.Advance(time.Minute)

	// Should be allowed again (2 tokens available).
	if !l.Allow("10.0.0.2") {
		t.Fatal("should be allowed after refill")
	}
	if !l.Allow("10.0.0.2") {
		t.Fatal("second request should be allowed after refill")
	}
	// Third should be denied again.
	if l.Allow("10.0.0.2") {
		t.Fatal("third request should be denied after second refill consumption")
	}
}

// TestIPRateLimiter_PerIPIsolation verifies different IPs have independent buckets.
func TestIPRateLimiter_PerIPIsolation(t *testing.T) {
	clk := newFixedClock()
	l := NewIPRateLimiter(1, clk)

	// IP A exhausts its bucket.
	if !l.Allow("192.168.1.1") {
		t.Fatal("IP A first request should be allowed")
	}
	if l.Allow("192.168.1.1") {
		t.Fatal("IP A second request should be denied")
	}

	// IP B should be unaffected.
	if !l.Allow("192.168.1.2") {
		t.Fatal("IP B should still be allowed")
	}
}

// TestIPRateLimiter_PartialRefill verifies partial minute refill is proportional.
func TestIPRateLimiter_PartialRefill(t *testing.T) {
	clk := newFixedClock()
	// Limit: 60 req/min = 1 req/second equivalent.
	l := NewIPRateLimiter(60, clk)

	// Consume all 60 tokens.
	for i := 0; i < 60; i++ {
		l.Allow("10.1.1.1")
	}
	if l.Allow("10.1.1.1") {
		t.Fatal("should be denied after 60 requests")
	}

	// Advance 1 second (= 1/60 of a minute) → should refill exactly 1 token.
	clk.Advance(time.Second)

	if !l.Allow("10.1.1.1") {
		t.Fatal("should be allowed after 1 second (1 token refilled)")
	}
	if l.Allow("10.1.1.1") {
		t.Fatal("should be denied after consuming the 1 refilled token")
	}
}

// TestSourceIP_XForwardedFor verifies that the first XFF entry is used.
func TestSourceIP_XForwardedFor(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.1, 172.16.0.1")
	ip := sourceIP(ctx)
	if ip != "203.0.113.5" {
		t.Fatalf("expected 203.0.113.5, got %q", ip)
	}
}

// TestSourceIP_FallbackToRemote verifies RemoteAddr fallback when no XFF header.
func TestSourceIP_FallbackToRemote(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.SetRemoteAddr(&net.TCPAddr{IP: net.ParseIP("198.51.100.7"), Port: 12345})
	ip := sourceIP(ctx)
	if ip != "198.51.100.7" {
		t.Fatalf("expected 198.51.100.7, got %q", ip)
	}
}

// TestCallbackIPRateLimitMiddleware_DisabledPassthrough confirms disabled limiter is passthrough.
func TestCallbackIPRateLimitMiddleware_DisabledPassthrough(t *testing.T) {
	called := false
	next := func(ctx *fasthttp.RequestCtx) { called = true }

	wrapped := CallbackIPRateLimitMiddleware(next, NewIPRateLimiter(0, newFixedClock()))
	// Should be the same function (no wrapping).
	ctx := &fasthttp.RequestCtx{}
	ctx.SetRemoteAddr(&net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 80})
	wrapped(ctx)
	if !called {
		t.Fatal("next handler should have been called for disabled limiter")
	}
}

// TestCallbackIPRateLimitMiddleware_Returns429 verifies 429 response when limited.
func TestCallbackIPRateLimitMiddleware_Returns429(t *testing.T) {
	clk := newFixedClock()
	limiter := NewIPRateLimiter(1, clk)

	handlerCalled := 0
	next := func(ctx *fasthttp.RequestCtx) { handlerCalled++ }

	wrapped := CallbackIPRateLimitMiddleware(next, limiter)

	ctx1 := &fasthttp.RequestCtx{}
	ctx1.SetRemoteAddr(&net.TCPAddr{IP: net.ParseIP("5.5.5.5"), Port: 1234})
	wrapped(ctx1)
	if handlerCalled != 1 {
		t.Fatalf("first request should reach handler, got %d calls", handlerCalled)
	}

	ctx2 := &fasthttp.RequestCtx{}
	ctx2.SetRemoteAddr(&net.TCPAddr{IP: net.ParseIP("5.5.5.5"), Port: 1234})
	wrapped(ctx2)
	if handlerCalled != 1 {
		t.Fatalf("second request should be blocked, handler should still have 1 call, got %d", handlerCalled)
	}
	if ctx2.Response.StatusCode() != fasthttp.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", ctx2.Response.StatusCode())
	}
}
