package main

import (
	"sort"
)

type Router struct {
	configMgr *ConfigManager
	cooldown  *CooldownManager
}

func NewRouter(cfg *ConfigManager, cd *CooldownManager) *Router {
	return &Router{configMgr: cfg, cooldown: cd}
}

type ResolvedRoute struct {
	BackendName string
	BackendURL  string
	Model       string
}

func (r *Router) Resolve(alias string) ([]ResolvedRoute, error) {
	return r.resolveWithVisited(alias, make(map[string]bool))
}

func (r *Router) resolveWithVisited(alias string, visited map[string]bool) ([]ResolvedRoute, error) {
	if visited[alias] {
		LogGeneral("WARN", "Circular fallback detected for alias: %s", alias)
		return nil, nil
	}
	visited[alias] = true

	cfg := r.configMgr.Get()
	var result []ResolvedRoute

	modelAlias, exists := cfg.Models[alias]
	if exists && modelAlias != nil && modelAlias.IsEnabled() {
		sorted := make([]ModelRoute, len(modelAlias.Routes))
		copy(sorted, modelAlias.Routes)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Priority < sorted[j].Priority
		})

		for _, route := range sorted {
			if !route.IsEnabled() {
				continue
			}
			key := r.cooldown.Key(route.Backend, route.Model)
			if r.cooldown.IsCoolingDown(key) {
				LogGeneral("DEBUG", "Skipping %s (cooling down)", key)
				continue
			}
			backend := r.configMgr.GetBackend(route.Backend)
			if backend == nil {
				LogGeneral("WARN", "Backend not found: %s", route.Backend)
				continue
			}
			if !backend.IsEnabled() {
				LogGeneral("DEBUG", "Skipping disabled backend: %s", route.Backend)
				continue
			}
			result = append(result, ResolvedRoute{
				BackendName: backend.Name,
				BackendURL:  backend.URL,
				Model:       route.Model,
			})
		}
	}

	fallbackRoutes := r.collectFallbackRoutes(alias, visited)
	result = append(result, fallbackRoutes...)

	return result, nil
}

func (r *Router) collectFallbackRoutes(alias string, visited map[string]bool) []ResolvedRoute {
	cfg := r.configMgr.Get()
	fallbacks, exists := cfg.Fallback.AliasFallback[alias]
	if !exists || len(fallbacks) == 0 {
		return nil
	}

	var result []ResolvedRoute
	for _, fallbackAlias := range fallbacks {
		routes, _ := r.resolveWithVisited(fallbackAlias, visited)
		if len(routes) > 0 {
			LogGeneral("DEBUG", "Including fallback routes from %s for %s", fallbackAlias, alias)
			result = append(result, routes...)
		}
	}
	return result
}
