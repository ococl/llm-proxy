package usecase

import (
	"fmt"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/error"
	"llm-proxy/domain/port"
)

// RouteResolveUseCase handles route resolution.
type RouteResolveUseCase struct {
	config       port.ConfigProvider
	fallbackCfg  FallbackConfig
}

// FallbackConfig represents fallback alias configuration.
type FallbackConfig struct {
	Aliases map[string][]string
}

// NewRouteResolveUseCase creates a new route resolve use case.
func NewRouteResolveUseCase(config port.ConfigProvider, fallbackCfg FallbackConfig) *RouteResolveUseCase {
	return &RouteResolveUseCase{
		config:      config,
		fallbackCfg: fallbackCfg,
	}
}

// Resolve resolves a model alias to a list of routes.
func (uc *RouteResolveUseCase) Resolve(alias string) ([]*port.Route, error) {
	cfg := uc.config.Get()
	modelAlias := cfg.Models[alias]
	if modelAlias == nil || !modelAlias.IsEnabled() {
		return nil, error.NewUnknownModel(alias)
	}

	var routes []*port.Route
	for _, routeCfg := range modelAlias.Routes {
		if !routeCfg.IsEnabled() {
			continue
		}

		backend := uc.config.GetBackend(routeCfg.BackendName)
		if backend == nil {
			continue
		}

		route := entity.NewRoute(
			convertToDomainBackend(backend),
			routeCfg.Model,
			routeCfg.Priority,
			routeCfg.IsEnabled(),
		)
		if routeCfg.Protocol != "" {
			route = route.WithProtocol(port.Protocol(routeCfg.Protocol))
		}

		routes = append(routes, &port.Route{
			Backend:   convertToDomainBackend(backend),
			Model:     routeCfg.Model,
			Priority:  routeCfg.Priority,
			Protocol:  route.GetProtocol(),
		})
	}

	if len(routes) == 0 {
		return nil, error.NewUnknownModel(alias)
	}

	return routes, nil
}

// GetFallbackAliases returns fallback aliases for a given model.
func (uc *RouteResolveUseCase) GetFallbackAliases(alias string) []string {
	return uc.fallbackCfg.Aliases[alias]
}

// HasFallback returns true if the alias has fallback configuration.
func (uc *RouteResolveUseCase) HasFallback(alias string) bool {
	_, ok := uc.fallbackCfg.Aliases[alias]
	return ok
}

// convertToDomainBackend converts config.Backend to port.Backend.
func convertToDomainBackend(b *port.Backend) *port.Backend {
	if b == nil {
		return nil
	}
	return &port.Backend{
		Name:     b.Name,
		URL:      b.URL,
		APIKey:   b.APIKey,
		Enabled:  b.Enabled,
		Protocol: b.Protocol,
	}
}