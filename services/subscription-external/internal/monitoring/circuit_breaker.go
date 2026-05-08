package monitoring

import (
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
)

// CircuitBreakerState represents the state of the circuit breaker
type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name            string
	maxFailures     int
	timeout         time.Duration
	resetTimeout    time.Duration
	state           CircuitBreakerState
	failures        int
	lastFailureTime time.Time
	nextAttempt     time.Time
	mu              sync.RWMutex
	logger          *zap.Logger
}

// CircuitBreakerConfig holds configuration for circuit breaker
type CircuitBreakerConfig struct {
	Name         string
	MaxFailures  int
	Timeout      time.Duration
	ResetTimeout time.Duration
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config CircuitBreakerConfig, logger *zap.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		name:         config.Name,
		maxFailures:  config.MaxFailures,
		timeout:      config.Timeout,
		resetTimeout: config.ResetTimeout,
		state:        StateClosed,
		logger:       logger,
	}
}

// Execute runs the given function with circuit breaker protection
func (cb *CircuitBreaker) Execute(operation func() error) error {
	if !cb.canExecute() {
		return errors.New("circuit breaker is open")
	}

	// Execute the operation with timeout
	done := make(chan error, 1)
	go func() {
		done <- operation()
	}()

	select {
	case err := <-done:
		return cb.handleResult(err)
	case <-time.After(cb.timeout):
		cb.handleResult(errors.New("operation timeout"))
		return errors.New("operation timeout")
	}
}

// canExecute checks if the operation can be executed
func (cb *CircuitBreaker) canExecute() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		return time.Now().After(cb.nextAttempt)
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// handleResult processes the result of an operation
func (cb *CircuitBreaker) handleResult(err error) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.onFailure()
		return err
	}

	cb.onSuccess()
	return nil
}

// onFailure handles operation failure
func (cb *CircuitBreaker) onFailure() {
	cb.failures++
	cb.lastFailureTime = time.Now()

	cb.logger.Warn("Circuit breaker operation failed",
		zap.String("name", cb.name),
		zap.Int("failures", cb.failures),
		zap.Int("max_failures", cb.maxFailures))

	if cb.failures >= cb.maxFailures {
		cb.setState(StateOpen)
		cb.nextAttempt = time.Now().Add(cb.resetTimeout)

		cb.logger.Error("Circuit breaker opened",
			zap.String("name", cb.name),
			zap.Time("next_attempt", cb.nextAttempt))
	}
}

// onSuccess handles operation success
func (cb *CircuitBreaker) onSuccess() {
	if cb.state == StateHalfOpen {
		cb.setState(StateClosed)
		cb.logger.Info("Circuit breaker closed",
			zap.String("name", cb.name))
	}
	cb.failures = 0
}

// setState changes the circuit breaker state
func (cb *CircuitBreaker) setState(state CircuitBreakerState) {
	if cb.state != state {
		oldState := cb.state
		cb.state = state

		cb.logger.Info("Circuit breaker state changed",
			zap.String("name", cb.name),
			zap.String("old_state", cb.stateString(oldState)),
			zap.String("new_state", cb.stateString(state)))
	}

	if state == StateOpen {
		cb.nextAttempt = time.Now().Add(cb.resetTimeout)
	} else if state == StateHalfOpen {
		// Reset failure count when entering half-open state
		cb.failures = 0
	}
}

// stateString returns string representation of state
func (cb *CircuitBreaker) stateString(state CircuitBreakerState) string {
	switch state {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns circuit breaker statistics
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"name":              cb.name,
		"state":             cb.stateString(cb.state),
		"failures":          cb.failures,
		"max_failures":      cb.maxFailures,
		"last_failure_time": cb.lastFailureTime,
		"next_attempt":      cb.nextAttempt,
		"timeout":           cb.timeout,
		"reset_timeout":     cb.resetTimeout,
	}
}

// Reset manually resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.setState(StateClosed)
	cb.failures = 0
	cb.lastFailureTime = time.Time{}
	cb.nextAttempt = time.Time{}

	cb.logger.Info("Circuit breaker manually reset",
		zap.String("name", cb.name))
}
