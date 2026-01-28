package usecase

import (
	domainerror "llm-proxy/domain/error"
	"llm-proxy/domain/port"
	"sort"
)

// RouteResolveUseCase handles route resolution.
type RouteResolveUseCase struct {
	config          port.ConfigProvider
	backendRepo     port.BackendRepository
	fallbackAliases map[string][]string
}

// NewRouteResolveUseCase creates a new route resolve use case.
func NewRouteResolveUseCase(
	config port.ConfigProvider,
	backendRepo port.BackendRepository,
	fallbackAliases map[string][]string,
) *RouteResolveUseCase {
	return &RouteResolveUseCase{
		config:          config,
		backendRepo:     backendRepo,
		fallbackAliases: fallbackAliases,
	}
}

// Resolve resolves a model alias to a list of routes.
func (uc *RouteResolveUseCase) Resolve(alias string) ([]*port.Route, error) {
	cfg := uc.config.Get()
	modelAlias := cfg.Models[alias]
	if modelAlias == nil || !modelAlias.IsEnabled() {
		return nil, domainerror.NewUnknownModel(alias)
	}

	var routes []*port.Route
	for _, routeCfg := range modelAlias.Routes {
		if !routeCfg.Enabled {
			continue
		}

		backend := uc.backendRepo.GetByName(routeCfg.Backend)
		if backend == nil || !backend.IsEnabled() {
			continue
		}

		routes = append(routes, &port.Route{
			Backend:   backend,
			Model:     routeCfg.Model,
			Priority:  routeCfg.Priority,
			Protocol:  backend.Protocol(),
			Enabled:   routeCfg.Enabled,
			Reasoning: routeCfg.Reasoning,
		})
	}

	if len(routes) == 0 {
		return nil, domainerror.NewUnknownModel(alias)
	}

	// 按 priority 升序排序（数值越小优先级越高）
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].Priority < routes[j].Priority
	})

	return routes, nil
}

// GetFallbackAliases returns fallback aliases for a given model.
func (uc *RouteResolveUseCase) GetFallbackAliases(alias string) []string {
	return uc.fallbackAliases[alias]
}

// HasFallback returns true if the alias has fallback configuration.
func (uc *RouteResolveUseCase) HasFallback(alias string) bool {
	_, ok := uc.fallbackAliases[alias]
	return ok
}
