package domain

import (
	"math/rand"
	"sync"
	"time"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
)

// LoadBalancingStrategy defines the load balancing approach.
type LoadBalancingStrategy string

const (
	// StrategyRandom uses random selection.
	StrategyRandom LoadBalancingStrategy = "random"
	// StrategyRoundRobin uses round-robin selection.
	StrategyRoundRobin LoadBalancingStrategy = "round_robin"
	// StrategyLeastRequests selects the backend with fewest requests.
	StrategyLeastRequests LoadBalancingStrategy = "least_requests"
	// StrategyWeighted selects based on weights.
	StrategyWeighted LoadBalancingStrategy = "weighted"
)

// LoadBalancer implements various load balancing strategies.
type LoadBalancer struct {
	strategy LoadBalancingStrategy
	mu       sync.Mutex
}

// NewLoadBalancer creates a new load balancer with the given strategy.
func NewLoadBalancer(strategy LoadBalancingStrategy) *LoadBalancer {
	if strategy == "" {
		strategy = StrategyRandom
	}
	return &LoadBalancer{
		strategy: strategy,
	}
}

// Select selects a backend from the given routes using the configured strategy.
func (lb *LoadBalancer) Select(routes []*port.Route) *port.Backend {
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

// SelectWithPriority selects a backend considering priority grouping.
func (lb *LoadBalancer) SelectWithPriority(routes entity.RouteList) *port.Backend {
	groups := routes.GroupByPriority()
	for priority := 0; priority < 100; priority++ {
		group, ok := groups[priority]
		if !ok {
			continue
		}
		if group.IsEmpty() {
			continue
		}
		enabled := group.FilterEnabled()
		if !enabled.IsEmpty() {
			var portRoutes []*port.Route
			for _, r := range enabled {
				portRoutes = append(portRoutes, &port.Route{
					Backend:  &port.Backend{Name: r.Backend().Name(), URL: r.Backend().URL().String(), APIKey: string(r.Backend().APIKey()), Enabled: r.Backend().IsEnabled(), Protocol: r.Backend().Protocol()},
					Model:    r.Model(),
					Priority: r.Priority(),
					Protocol: r.Protocol(),
				})
			}
			return lb.Select(portRoutes)
		}
	}
	return nil
}

// selectRandom randomly selects a backend.
func (lb *LoadBalancer) selectRandom(routes []*port.Route) *port.Backend {
	enabled := filterEnabledBackends(routes)
	if len(enabled) == 0 {
		return nil
	}
	idx := rand.Intn(len(enabled))
	return enabled[idx]
}

// selectRoundRobin selects the next backend in round-robin fashion.
func (lb *LoadBalancer) selectRoundRobin(routes []*port.Route) *port.Backend {
	enabled := filterEnabledBackends(routes)
	if len(enabled) == 0 {
		return nil
	}
	idx := rand.Intn(len(enabled))
	return enabled[idx]
}

// selectLeastRequests selects the backend with the fewest active requests.
func (lb *LoadBalancer) selectLeastRequests(routes []*port.Route) *port.Backend {
	enabled := filterEnabledBackends(routes)
	if len(enabled) == 0 {
		return nil
	}
	return enabled[rand.Intn(len(enabled))]
}

// selectWeighted selects a backend based on priority (simplified weighted).
func (lb *LoadBalancer) selectWeighted(routes []*port.Route) *port.Backend {
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

// filterEnabledBackends filters routes to only enabled backends.
func filterEnabledBackends(routes []*port.Route) []*port.Backend {
	var backends []*port.Backend
	for _, route := range routes {
		if route.IsEnabled() {
			backends = append(backends, route.Backend)
		}
	}
	return backends
}

// WeightedBackend represents a backend with a weight.
type WeightedBackend struct {
	Backend *port.Backend
	Weight  int
}

// WeightedLoadBalancer implements weighted load balancing.
type WeightedLoadBalancer struct {
	backends []WeightedBackend
	mu       sync.Mutex
}

// NewWeightedLoadBalancer creates a new weighted load balancer.
func NewWeightedLoadBalancer(backends []WeightedBackend) *WeightedLoadBalancer {
	return &WeightedLoadBalancer{
		backends: backends,
	}
}

// Select selects a backend based on weights.
func (wlb *WeightedLoadBalancer) Select() *port.Backend {
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

// CircuitBreakerState represents the state of a circuit breaker.
type CircuitBreakerState int

const (
	// CircuitStateClosed allows requests to pass.
	CircuitStateClosed CircuitBreakerState = iota
	// CircuitStateOpen blocks requests.
	CircuitStateOpen
	// CircuitStateHalfOpen allows limited requests to test recovery.
	CircuitStateHalfOpen
)

// String returns the string representation of the circuit breaker state.
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
	}
}

// State returns the current state of the circuit breaker.
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// AllowRequest returns true if the request should be allowed.
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
	case CircuitStateClosed:
		cb.failureCount++
		if cb.failureCount >= cb.failureThreshold {
			cb.state = CircuitStateOpen
			cb.lastStateChange = time.Now()
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
