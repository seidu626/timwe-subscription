package utils

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/sony/gobreaker"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// NetworkResilientClient provides enhanced HTTP client with resilience patterns
type NetworkResilientClient struct {
	client         *fasthttp.Client
	circuitBreaker *gobreaker.CircuitBreaker
	logger         *zap.Logger
	config         *NetworkConfig
}

// NetworkConfig contains configuration for network resilience
type NetworkConfig struct {
	MaxRetries              int
	BaseRetryDelay          time.Duration
	MaxRetryDelay           time.Duration
	ConnectionTimeout       time.Duration
	ReadTimeout             time.Duration
	WriteTimeout            time.Duration
	MaxConnsPerHost         int
	MaxIdleConnDuration     time.Duration
	CircuitBreakerThreshold int
	CircuitBreakerTimeout   time.Duration
	JitterEnabled           bool
}

// DefaultNetworkConfig returns sensible defaults for network configuration
func DefaultNetworkConfig() *NetworkConfig {
	return &NetworkConfig{
		MaxRetries:              5, // Increased from 3 to 5 for better resilience
		BaseRetryDelay:          200 * time.Millisecond,
		MaxRetryDelay:           30 * time.Second,
		ConnectionTimeout:       10 * time.Second,
		ReadTimeout:             30 * time.Second,
		WriteTimeout:            30 * time.Second,
		MaxConnsPerHost:         200, // Increased connection pool
		MaxIdleConnDuration:     60 * time.Second,
		CircuitBreakerThreshold: 3,
		CircuitBreakerTimeout:   30 * time.Second,
		JitterEnabled:           true,
	}
}

// NewNetworkResilientClient creates a new resilient HTTP client
func NewNetworkResilientClient(logger *zap.Logger, config *NetworkConfig) *NetworkResilientClient {
	if config == nil {
		config = DefaultNetworkConfig()
	}

	// Configure fasthttp client with enhanced settings
	client := &fasthttp.Client{
		// Connection settings
		MaxConnsPerHost:     config.MaxConnsPerHost,
		MaxIdleConnDuration: config.MaxIdleConnDuration,
		MaxConnDuration:     5 * time.Minute, // Force connection refresh
		MaxConnWaitTimeout:  config.ConnectionTimeout,

		// Timeout settings
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,

		// Performance optimizations
		MaxResponseBodySize:      10 * 1024 * 1024, // 10MB max response
		DisablePathNormalizing:   true,
		NoDefaultUserAgentHeader: true,

		// Custom dialer for better connection control
		Dial: func(addr string) (net.Conn, error) {
			return fasthttp.DialTimeout(addr, config.ConnectionTimeout)
		},
	}

	// Configure circuit breaker with more sophisticated settings
	cbSettings := gobreaker.Settings{
		Name:        "NetworkResilientClient",
		MaxRequests: 10,
		Interval:    60 * time.Second,
		Timeout:     config.CircuitBreakerTimeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Trip circuit breaker on consecutive failures or high failure rate
			if counts.ConsecutiveFailures >= uint32(config.CircuitBreakerThreshold) {
				return true
			}

			// Also trip on high failure rate (e.g., 80% failure rate with minimum requests)
			if counts.Requests >= 10 {
				failureRate := float64(counts.TotalFailures) / float64(counts.Requests)
				return failureRate >= 0.8
			}

			return false
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			logger.Info("Circuit breaker state change",
				zap.String("name", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()),
				zap.String("timestamp", time.Now().Format(time.RFC3339)))
		},
	}
	circuitBreaker := gobreaker.NewCircuitBreaker(cbSettings)

	return &NetworkResilientClient{
		client:         client,
		circuitBreaker: circuitBreaker,
		logger:         logger,
		config:         config,
	}
}

// DoWithRetry executes HTTP request with retry logic and circuit breaker
func (c *NetworkResilientClient) DoWithRetry(ctx context.Context, req *fasthttp.Request, res *fasthttp.Response) error {
	return c.doWithRetryInternal(ctx, req, res, 0)
}

// doWithRetryInternal handles the actual retry logic
func (c *NetworkResilientClient) doWithRetryInternal(ctx context.Context, req *fasthttp.Request, res *fasthttp.Response, attempt int) error {
	// Check if we've exceeded max retries
	if attempt >= c.config.MaxRetries {
		return fmt.Errorf("max retries (%d) exceeded", c.config.MaxRetries)
	}

	// Apply jitter to prevent thundering herd
	if attempt > 0 {
		delay := c.calculateRetryDelay(attempt)
		if c.config.JitterEnabled {
			jitter := time.Duration(rand.Intn(int(delay.Milliseconds()/2))) * time.Millisecond
			delay += jitter
		}

		c.logger.Debug("Retrying request after delay",
			zap.Int("attempt", attempt),
			zap.Duration("delay", delay))

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	// Execute request through circuit breaker
	err := c.executeWithCircuitBreaker(req, res)
	if err != nil {
		// Determine if error is retryable
		if c.isRetryableError(err) && attempt < c.config.MaxRetries-1 {
			c.logger.Warn("Retryable error occurred, will retry",
				zap.Int("attempt", attempt+1),
				zap.Error(err))
			return c.doWithRetryInternal(ctx, req, res, attempt+1)
		}
		return err
	}

	// Check HTTP status code for retryable errors
	if c.isRetryableStatusCode(res.StatusCode()) && attempt < c.config.MaxRetries-1 {
		c.logger.Warn("Retryable status code, will retry",
			zap.Int("attempt", attempt+1),
			zap.Int("statusCode", res.StatusCode()))
		return c.doWithRetryInternal(ctx, req, res, attempt+1)
	}

	return nil
}

// executeWithCircuitBreaker executes request through circuit breaker
func (c *NetworkResilientClient) executeWithCircuitBreaker(req *fasthttp.Request, res *fasthttp.Response) error {
	result, err := c.circuitBreaker.Execute(func() (interface{}, error) {
		return nil, c.client.Do(req, res)
	})

	if err != nil {
		return err
	}

	return result.(error)
}

// calculateRetryDelay calculates exponential backoff delay
func (c *NetworkResilientClient) calculateRetryDelay(attempt int) time.Duration {
	delay := time.Duration(math.Pow(2, float64(attempt))) * c.config.BaseRetryDelay
	if delay > c.config.MaxRetryDelay {
		delay = c.config.MaxRetryDelay
	}
	return delay
}

// isRetryableError determines if an error should trigger a retry
func (c *NetworkResilientClient) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Network-level errors that are typically retryable
	retryableErrors := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"no route to host",
		"network is unreachable",
		"temporary failure",
		"i/o timeout",
		"deadline exceeded",
		"broken pipe",
		"connection aborted",
	}

	errLower := strings.ToLower(errStr)
	for _, retryableErr := range retryableErrors {
		if strings.Contains(errLower, retryableErr) {
			return true
		}
	}

	// Check for specific network error types
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout() || netErr.Temporary()
	}

	return false
}

// isRetryableStatusCode determines if HTTP status code should trigger retry
func (c *NetworkResilientClient) isRetryableStatusCode(statusCode int) bool {
	// Retry on server errors and specific client errors
	retryableStatusCodes := []int{
		500, // Internal Server Error
		502, // Bad Gateway
		503, // Service Unavailable
		504, // Gateway Timeout
		408, // Request Timeout
		429, // Too Many Requests
	}

	for _, code := range retryableStatusCodes {
		if statusCode == code {
			return true
		}
	}

	return false
}

// GetStats returns client statistics
func (c *NetworkResilientClient) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"circuit_breaker_state": c.circuitBreaker.State().String(),
		"max_retries":           c.config.MaxRetries,
		"base_retry_delay":      c.config.BaseRetryDelay.String(),
		"max_conns_per_host":    c.config.MaxConnsPerHost,
		"connection_timeout":    c.config.ConnectionTimeout.String(),
		"read_timeout":          c.config.ReadTimeout.String(),
		"write_timeout":         c.config.WriteTimeout.String(),
	}
}

// Close closes the underlying HTTP client
func (c *NetworkResilientClient) Close() error {
	// fasthttp.Client doesn't have a Close method, but we can log the shutdown
	c.logger.Info("Network resilient client shutting down")
	return nil
}

// HealthCheck performs a basic health check
func (c *NetworkResilientClient) HealthCheck(ctx context.Context, url string) error {
	req := fasthttp.AcquireRequest()
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(res)

	req.SetRequestURI(url)
	req.Header.SetMethod("GET")
	req.Header.Set("User-Agent", "HealthCheck/1.0")

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	err := c.DoWithRetry(ctx, req, res)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	if res.StatusCode() >= 400 {
		return fmt.Errorf("health check returned status %d", res.StatusCode())
	}

	return nil
}
