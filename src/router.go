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
	cfg := r.configMgr.Get()
	routes, exists := cfg.Models[alias]
	if !exists {
		return nil, nil
	}

	sorted := make([]ModelRoute, len(routes))
	copy(sorted, routes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})

	var result []ResolvedRoute
	for _, route := range sorted {
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
		result = append(result, ResolvedRoute{
			BackendName: backend.Name,
			BackendURL:  backend.URL,
			Model:       route.Model,
		})
	}
	return result, nil
}
