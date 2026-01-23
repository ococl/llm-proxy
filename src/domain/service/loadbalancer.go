package service

import (
	"math/rand"
	"sync"
	"time"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
)

// Logging field constants for consistent field names.
const (
	FieldStrategy          = "strategy"
	FieldAvailableBackends = "available_backends"
	FieldPriority          = "priority"
	FieldSelectionMethod   = "selection_method"
	FieldCircuitState      = "circuit_state"
	FieldFailureCount      = "failure_count"
	FieldSuccessCount      = "success_count"
	FieldTotalRoutes       = "total_routes"
	FieldFilteredRoutes    = "filtered_routes"
)

type LoadBalancingStrategy string

const (
	StrategyRandom        LoadBalancingStrategy = "random"
	StrategyRoundRobin    LoadBalancingStrategy = "round_robin"
	StrategyLeastRequests LoadBalancingStrategy = "least_requests"
	StrategyWeighted      LoadBalancingStrategy = "weighted"
)

type LoadBalancer struct {
	strategy LoadBalancingStrategy
	logger   port.Logger
	mu       sync.Mutex
}

func NewLoadBalancer(strategy LoadBalancingStrategy) *LoadBalancer {
	if strategy == "" {
		strategy = StrategyRandom
	}
	return &LoadBalancer{
		strategy: strategy,
		logger:   &port.NopLogger{},
	}
}

func (lb *LoadBalancer) WithLogger(logger port.Logger) *LoadBalancer {
	lb.logger = logger
	return lb
}

func (lb *LoadBalancer) Select(routes []*port.Route) *entity.Backend {
	if len(routes) == 0 {
		lb.logger.Warn("no routes available for selection",
			port.Int(FieldTotalRoutes, 0))
		return nil
	}

	if len(routes) == 1 {
		backend := routes[0].Backend
		lb.logger.Debug("single route available, selected directly",
			port.String("backend", backend.Name()),
			port.Int(FieldPriority, routes[0].Priority))
		return backend
	}

	lb.logger.Debug("selecting backend",
		port.Int(FieldTotalRoutes, len(routes)),
		port.String(FieldStrategy, string(lb.strategy)))

	var backend *entity.Backend
	switch lb.strategy {
	case StrategyRandom:
		backend = lb.selectRandom(routes)
	case StrategyRoundRobin:
		backend = lb.selectRoundRobin(routes)
	case StrategyLeastRequests:
		backend = lb.selectLeastRequests(routes)
	case StrategyWeighted:
		backend = lb.selectWeighted(routes)
	default:
		backend = lb.selectRandom(routes)
	}

	if backend != nil {
		lb.logger.Debug("backend selected",
			port.String("backend", backend.Name()),
			port.String(FieldSelectionMethod, string(lb.strategy)))
	} else {
		lb.logger.Warn("backend selection returned nil",
			port.Int(FieldTotalRoutes, len(routes)),
			port.String(FieldStrategy, string(lb.strategy)))
	}

	return backend
}

func (lb *LoadBalancer) selectRandom(routes []*port.Route) *entity.Backend {
	enabled := filterEnabledBackends(routes)
	if len(enabled) == 0 {
		lb.logger.Warn("no enabled backends available",
			port.Int(FieldTotalRoutes, len(routes)),
			port.Int(FieldFilteredRoutes, 0))
		return nil
	}
	idx := rand.Intn(len(enabled))
	return enabled[idx]
}

func (lb *LoadBalancer) selectRoundRobin(routes []*port.Route) *entity.Backend {
	enabled := filterEnabledBackends(routes)
	if len(enabled) == 0 {
		lb.logger.Warn("no enabled backends available",
			port.Int(FieldTotalRoutes, len(routes)),
			port.Int(FieldFilteredRoutes, 0))
		return nil
	}
	idx := rand.Intn(len(enabled))
	return enabled[idx]
}

func (lb *LoadBalancer) selectLeastRequests(routes []*port.Route) *entity.Backend {
	enabled := filterEnabledBackends(routes)
	if len(enabled) == 0 {
		lb.logger.Warn("no enabled backends available",
			port.Int(FieldTotalRoutes, len(routes)),
			port.Int(FieldFilteredRoutes, 0))
		return nil
	}
	return enabled[rand.Intn(len(enabled))]
}

func (lb *LoadBalancer) selectWeighted(routes []*port.Route) *entity.Backend {
	if len(routes) == 0 {
		return nil
	}

	highestPriority := lb.findHighestPriority(routes)
	topRoutes := lb.filterByPriority(routes, highestPriority)

	if len(topRoutes) == 0 {
		lb.logger.Warn("no routes after priority filtering",
			port.Int(FieldTotalRoutes, len(routes)),
			port.Int(FieldPriority, highestPriority))
		return nil
	}

	if len(topRoutes) == 1 {
		backend := topRoutes[0].Backend
		lb.logger.Debug("single backend in priority group",
			port.String("backend", backend.Name()),
			port.Int(FieldPriority, highestPriority))
		return backend
	}

	return lb.selectRandomBackend(topRoutes)
}

func (lb *LoadBalancer) findHighestPriority(routes []*port.Route) int {
	minPriority := routes[0].Priority
	for _, route := range routes {
		if route.IsEnabled() && route.Priority < minPriority {
			minPriority = route.Priority
		}
	}
	return minPriority
}

func (lb *LoadBalancer) filterByPriority(routes []*port.Route, priority int) []*port.Route {
	var filtered []*port.Route
	for _, route := range routes {
		if route.IsEnabled() && route.Priority == priority {
			filtered = append(filtered, route)
		}
	}
	return filtered
}

func (lb *LoadBalancer) selectRandomBackend(routes []*port.Route) *entity.Backend {
	idx := rand.Intn(len(routes))
	return routes[idx].Backend
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
	logger   port.Logger
	mu       sync.Mutex
}

func NewWeightedLoadBalancer(backends []WeightedBackend) *WeightedLoadBalancer {
	return &WeightedLoadBalancer{
		backends: backends,
		logger:   &port.NopLogger{},
	}
}

func (wlb *WeightedLoadBalancer) WithLogger(logger port.Logger) *WeightedLoadBalancer {
	wlb.logger = logger
	return wlb
}

func (wlb *WeightedLoadBalancer) Select() *entity.Backend {
	wlb.mu.Lock()
	defer wlb.mu.Unlock()

	if len(wlb.backends) == 0 {
		wlb.logger.Warn("no backends configured in weighted load balancer")
		return nil
	}

	totalWeight := 0
	enabledCount := 0
	for _, wb := range wlb.backends {
		if wb.Backend.IsEnabled() {
			totalWeight += wb.Weight
			enabledCount++
		}
	}

	if totalWeight == 0 {
		wlb.logger.Warn("no enabled backends with positive weight",
			port.Int(FieldTotalCount, len(wlb.backends)),
			port.Int("enabled_count", enabledCount))
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
			wlb.logger.Debug("weighted backend selected",
				port.String("backend", wb.Backend.Name()),
				port.Int("weight", wb.Weight),
				port.Int("total_weight", totalWeight))
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
	logger           port.Logger
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
		logger:           &port.NopLogger{},
	}
}

func (cb *CircuitBreaker) WithLogger(logger port.Logger) *CircuitBreaker {
	cb.logger = logger
	return cb
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
			cb.logger.Info("circuit breaker transitioned to half-open",
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
			cb.logger.Info("circuit breaker closed after successful recovery",
				port.String(FieldCircuitState, cb.state.String()),
				port.Int(FieldSuccessCount, cb.successCount))
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
		cb.logger.Warn("circuit breaker opened from half-open after failure",
			port.String(FieldCircuitState, cb.state.String()))
	case CircuitStateClosed:
		cb.failureCount++
		if cb.failureCount >= cb.failureThreshold {
			cb.state = CircuitStateOpen
			cb.lastStateChange = time.Now()
			cb.logger.Warn("circuit breaker opened due to failure threshold",
				port.String(FieldCircuitState, cb.state.String()),
				port.Int(FieldFailureCount, cb.failureCount))
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
