package service

import (
	"sync"
	"time"

	"llm-proxy/domain/port"
)

// 日志字段常量
const (
	FieldCircuitState = "circuit_state"
	FieldFailureCount = "failure_count"
	FieldSuccessCount = "success_count"
)

type CircuitBreakerState int

const (
	CircuitStateClosed CircuitBreakerState = iota
	CircuitStateOpen
	CircuitStateHalfOpen
)

func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitStateClosed:
		return "closed"
	case CircuitStateOpen:
		return "open"
	case CircuitStateHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

// CircuitBreaker implements the circuit breaker pattern.
type CircuitBreaker struct {
	state            CircuitBreakerState
	failureCount     int
	successCount     int
	failureThreshold int
	successThreshold int
	openTimeout      time.Duration
	lastStateChange  time.Time
	logger           port.Logger
	mu               sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(
	failureThreshold int,
	successThreshold int,
	openTimeout time.Duration,
) *CircuitBreaker {
	if failureThreshold <= 0 {
		failureThreshold = 5
	}
	if successThreshold <= 0 {
		successThreshold = 2
	}
	if openTimeout <= 0 {
		openTimeout = 60 * time.Second
	}
	return &CircuitBreaker{
		state:            CircuitStateClosed,
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		openTimeout:      openTimeout,
		lastStateChange:  time.Now(),
		logger:           &port.NopLogger{},
	}
}

// WithLogger sets the logger.
func (cb *CircuitBreaker) WithLogger(logger port.Logger) *CircuitBreaker {
	cb.logger = logger
	return cb
}

// State returns the current state.
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// AllowRequest checks if a request should be allowed.
func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitStateClosed:
		return true
	case CircuitStateOpen:
		if time.Since(cb.lastStateChange) > cb.openTimeout {
			cb.state = CircuitStateHalfOpen
			cb.successCount = 0
			cb.failureCount = 0
			cb.lastStateChange = time.Now()
			cb.logger.Info("熔断器转为半开",
				port.String(FieldCircuitState, cb.state.String()))
			return true
		}
		return false
	case CircuitStateHalfOpen:
		return true
	default:
		return true
	}
}

// RecordSuccess records a successful request.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitStateHalfOpen:
		cb.successCount++
		if cb.successCount >= cb.successThreshold {
			cb.state = CircuitStateClosed
			cb.failureCount = 0
			cb.lastStateChange = time.Now()
			cb.logger.Info("熔断器恢复关闭",
				port.String(FieldCircuitState, cb.state.String()),
				port.Int(FieldSuccessCount, cb.successCount))
		}
	case CircuitStateClosed:
		cb.failureCount = 0
	}
}

// RecordFailure records a failed request.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitStateHalfOpen:
		cb.state = CircuitStateOpen
		cb.lastStateChange = time.Now()
		cb.logger.Warn("熔断器半开失败转打开",
			port.String(FieldCircuitState, cb.state.String()))
	case CircuitStateClosed:
		cb.failureCount++
		if cb.failureCount >= cb.failureThreshold {
			cb.state = CircuitStateOpen
			cb.lastStateChange = time.Now()
			cb.logger.Warn("熔断器达阈值打开",
				port.String(FieldCircuitState, cb.state.String()),
				port.Int(FieldFailureCount, cb.failureCount))
		}
	}
}

// Reset resets the circuit breaker to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = CircuitStateClosed
	cb.failureCount = 0
	cb.successCount = 0
	cb.lastStateChange = time.Now()
}
