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

	// 收集所有可用的优先级并排序（从小到大，数字越小优先级越高）
	priorities := collectPriorities(routes)
	if len(priorities) == 0 {
		lb.logger.Warn("无可用优先级",
			port.Int(FieldTotalRoutes, len(routes)))
		return nil
	}

	// 按优先级从高到低遍历，直到找到可用的后端组
	for _, priority := range priorities {
		topRoutes := lb.filterByPriority(routes, priority)

		if len(topRoutes) > 0 {
			// 在该优先级组内选择第一个（因为路由已按配置顺序排序）
			backend := topRoutes[0].Backend
			lb.logger.Debug("优先级选择完成",
				port.String("backend", backend.Name()),
				port.Int(FieldPriority, priority),
				port.Int(FieldTotalRoutes, len(routes)))
			return backend
		}

		// 该优先级无可用路由，继续尝试下一优先级
	}

	lb.logger.Warn("所有优先级均无可用路由",
		port.Int(FieldTotalRoutes, len(routes)),
		port.Int("priority_count", len(priorities)))
	return nil
}

// collectPriorities 收集所有路由的优先级并按升序排序
func collectPriorities(routes []*port.Route) []int {
	prioritySet := make(map[int]struct{})
	for _, route := range routes {
		if route.IsEnabled() {
			prioritySet[route.Priority] = struct{}{}
		}
	}

	if len(prioritySet) == 0 {
		return nil
	}

	priorities := make([]int, 0, len(prioritySet))
	for p := range prioritySet {
		priorities = append(priorities, p)
	}

	// 按升序排序，数字越小优先级越高
	for i := 0; i < len(priorities)-1; i++ {
		for j := i + 1; j < len(priorities); j++ {
			if priorities[j] < priorities[i] {
				priorities[i], priorities[j] = priorities[j], priorities[i]
			}
		}
	}

	return priorities
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
