package domain

import (
	"math/rand"
	"sync"
	"time"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
)

type LoadBalancingStrategy string

const (
	StrategyRandom            LoadBalancingStrategy = "random"
	StrategyRoundRobin        LoadBalancingStrategy = "round_robin"
	StrategyLeastRequests     LoadBalancingStrategy = "least_requests"
	StrategyWeighted          LoadBalancingStrategy = "weighted"
)

type LoadBalancer struct {
	strategy LoadBalancingStrategy
	mu       sync.Mutex
}

func NewLoadBalancer(strategy LoadBalancingStrategy) *LoadBalancer {
	if strategy == "" {
		strategy = StrategyRandom
	}
	return &LoadBalancer{
		strategy: strategy,
	}
}

func (lb *LoadBalancer) Select(routes []*port.Route) *entity.Backend {
	if len(routes) == 0 {
		return nil
	}
	if len(routes) == 1 {
		return routes[0].Backend
	}

	switch lb.strategy {
	case StrategyRandom:
		return lb.selectRandom(routes)
	case StrategyRoundRobin:
		return lb.selectRoundRobin(routes)
	case StrategyLeastRequests:
		return lb.selectLeastRequests(routes)
	case StrategyWeighted:
		return lb.selectWeighted(routes)
	default:
		return lb.selectRandom(routes)
	}
}

func (lb *LoadBalancer) selectRandom(routes []*port.Route) *entity.Backend {
	enabled := filterEnabledBackends(routes)
	if len(enabled) == 0 {
		return nil
	}
	idx := rand.Intn(len(enabled))
	return enabled[idx]
}

func (lb *LoadBalancer) selectRoundRobin(routes []*port.Route) *entity.Backend {
	enabled := filterEnabledBackends(routes)
	if len(enabled) == 0 {
		return nil
	}
	idx := rand.Intn(len(enabled))
	return enabled[idx]
}

func (lb *LoadBalancer) selectLeastRequests(routes []*port.Route) *entity.Backend {
	enabled := filterEnabledBackends(routes)
	if len(enabled) == 0 {
		return nil
	}
	return enabled[rand.Intn(len(enabled))]
}

func (lb *LoadBalancer) selectWeighted(routes []*port.Route) *entity.Backend {
	if len(routes) == 0 {
		return nil
	}
	best := routes[0]
	for _, route := range routes {
		if route.IsEnabled() && route.Priority < best.Priority {
			best = route
		}
	}
	return best.Backend
}

func filterEnabledBackends(routes []*port.Route) []*entity.Backend {
	var backends []*entity.Backend
	for _, route := range routes {
		if route.IsEnabled() {
			backends = append(backends, route.Backend)
		}
	}
	return backends
}

type WeightedBackend struct {
	Backend *entity.Backend
	Weight  int
}

type WeightedLoadBalancer struct {
	backends []WeightedBackend
	mu       sync.Mutex
}

func NewWeightedLoadBalancer(backends []WeightedBackend) *WeightedLoadBalancer {
	return &WeightedLoadBalancer{
		backends: backends,
	}
}

func (wlb *WeightedLoadBalancer) Select() *entity.Backend {
	wlb.mu.Lock()
	defer wlb.mu.Unlock()

	if len(wlb.backends) == 0 {
		return nil
	}

	totalWeight := 0
	for _, wb := range wlb.backends {
		if wb.Backend.IsEnabled() {
			totalWeight += wb.Weight
		}
	}

	if totalWeight == 0 {
		return nil
	}

	randWeight := rand.Intn(totalWeight)
	currentWeight := 0

	for _, wb := range wlb.backends {
		if !wb.Backend.IsEnabled() {
			continue
		}
		currentWeight += wb.Weight
		if randWeight < currentWeight {
			return wb.Backend
		}
	}

	return nil
}

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

type CircuitBreaker struct {
	state            CircuitBreakerState
	failureCount     int
	successCount     int
	failureThreshold int
	successThreshold int
	openTimeout      time.Duration
	lastStateChange  time.Time
	mu               sync.RWMutex
}

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
	}
}

func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

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
			return true
		}
		return false
	case CircuitStateHalfOpen:
		return true
	default:
		return true
	}
}

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
		}
	case CircuitStateClosed:
		cb.failureCount = 0
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitStateHalfOpen:
		cb.state = CircuitStateOpen
		cb.lastStateChange = time.Now()
	case CircuitStateClosed:
		cb.failureCount++
		if cb.failureCount >= cb.failureThreshold {
			cb.state = CircuitStateOpen
			cb.lastStateChange = time.Now()
		}
	}
}

func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = CircuitStateClosed
	cb.failureCount = 0
	cb.successCount = 0
	cb.lastStateChange = time.Now()
}
