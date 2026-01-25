package service

import (
	"math/rand"
	"sync"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
)

// Logging field constants for consistent field names.
const (
	FieldStrategy          = "strategy"
	FieldAvailableBackends = "available_backends"
	FieldPriority          = "priority"
	FieldSelectionMethod   = "selection_method"
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
		lb.logger.Warn("无路由可用",
			port.Int(FieldTotalRoutes, 0))
		return nil
	}

	if len(routes) == 1 {
		backend := routes[0].Backend
		lb.logger.Debug("单路由直达",
			port.String("backend", backend.Name()),
			port.Int(FieldPriority, routes[0].Priority))
		return backend
	}

	lb.logger.Debug("开始选择后端",
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
		lb.logger.Debug("后端选择",
			port.String("backend", backend.Name()),
			port.String(FieldSelectionMethod, string(lb.strategy)))
	} else {
		lb.logger.Warn("后端选择失败",
			port.Int(FieldTotalRoutes, len(routes)),
			port.String(FieldStrategy, string(lb.strategy)))
	}

	return backend
}

func (lb *LoadBalancer) selectRandom(routes []*port.Route) *entity.Backend {
	enabled := filterEnabledBackends(routes)
	if len(enabled) == 0 {
		lb.logger.Warn("随机选择无可用后端",
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
		lb.logger.Warn("轮询无可用后端",
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
		lb.logger.Warn("最少请求无可用后端",
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
		lb.logger.Warn("优先级过滤后无路由",
			port.Int(FieldTotalRoutes, len(routes)),
			port.Int(FieldPriority, highestPriority))
		return nil
	}

	if len(topRoutes) == 1 {
		backend := topRoutes[0].Backend
		lb.logger.Debug("单后端优先",
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
		wlb.logger.Warn("加权LB未配置后端")
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
		wlb.logger.Warn("无可用正权重后端",
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
			wlb.logger.Debug("加权选择完成",
				port.String("backend", wb.Backend.Name()),
				port.Int("weight", wb.Weight),
				port.Int("total_weight", totalWeight))
			return wb.Backend
		}
	}

	return nil
}
