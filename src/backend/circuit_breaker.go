package backend

import (
	"sync"
	"time"
)

type CircuitState int

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

type CircuitBreaker struct {
	mu sync.RWMutex

	state           CircuitState
	failureCount    int
	successCount    int
	lastFailureTime time.Time
	lastStateChange time.Time

	failureThreshold int
	successThreshold int
	openTimeout      time.Duration
	halfOpenMaxReqs  int
	halfOpenReqCount int
}

func NewCircuitBreaker(failureThreshold, successThreshold int, openTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            StateClosed,
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		openTimeout:      openTimeout,
		halfOpenMaxReqs:  3,
	}
}

func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()
	currentState := cb.state

	if currentState == StateOpen {
		if time.Since(cb.lastStateChange) >= cb.openTimeout {
			cb.state = StateHalfOpen
			cb.halfOpenReqCount = 0
			cb.successCount = 0
			cb.failureCount = 0
			currentState = StateHalfOpen
		} else {
			cb.mu.Unlock()
			return ErrCircuitOpen
		}
	}

	if currentState == StateHalfOpen {
		if cb.halfOpenReqCount >= cb.halfOpenMaxReqs {
			cb.mu.Unlock()
			return ErrCircuitOpen
		}
		cb.halfOpenReqCount++
	}

	cb.mu.Unlock()

	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.onFailure()
	} else {
		cb.onSuccess()
	}

	return err
}

func (cb *CircuitBreaker) onSuccess() {
	cb.failureCount = 0

	if cb.state == StateHalfOpen {
		cb.successCount++
		if cb.successCount >= cb.successThreshold {
			cb.state = StateClosed
			cb.lastStateChange = time.Now()
			cb.halfOpenReqCount = 0
		}
	}
}

func (cb *CircuitBreaker) onFailure() {
	cb.failureCount++
	cb.lastFailureTime = time.Now()

	if cb.state == StateHalfOpen {
		cb.state = StateOpen
		cb.lastStateChange = time.Now()
		cb.halfOpenReqCount = 0
		return
	}

	if cb.failureCount >= cb.failureThreshold {
		cb.state = StateOpen
		cb.lastStateChange = time.Now()
	}
}

func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

func (cb *CircuitBreaker) IsAvailable() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.state == StateOpen {
		if time.Since(cb.lastStateChange) >= cb.openTimeout {
			return true
		}
		return false
	}

	if cb.state == StateHalfOpen && cb.halfOpenReqCount >= cb.halfOpenMaxReqs {
		return false
	}

	return true
}

var ErrCircuitOpen = &CircuitOpenError{}

type CircuitOpenError struct{}

func (e *CircuitOpenError) Error() string {
	return "circuit breaker is open"
}

type CircuitBreakerManager struct {
	breakers map[string]*CircuitBreaker
	mu       sync.RWMutex

	failureThreshold int
	successThreshold int
	openTimeout      time.Duration
}

func NewCircuitBreakerManager(failureThreshold, successThreshold int, openTimeout time.Duration) *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers:         make(map[string]*CircuitBreaker),
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		openTimeout:      openTimeout,
	}
}

func (m *CircuitBreakerManager) GetBreaker(key string) *CircuitBreaker {
	m.mu.RLock()
	breaker, exists := m.breakers[key]
	m.mu.RUnlock()

	if exists {
		return breaker
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if breaker, exists := m.breakers[key]; exists {
		return breaker
	}

	breaker = NewCircuitBreaker(m.failureThreshold, m.successThreshold, m.openTimeout)
	m.breakers[key] = breaker
	return breaker
}

func (m *CircuitBreakerManager) IsAvailable(key string) bool {
	breaker := m.GetBreaker(key)
	return breaker.IsAvailable()
}

func (m *CircuitBreakerManager) GetState(key string) CircuitState {
	breaker := m.GetBreaker(key)
	return breaker.GetState()
}
