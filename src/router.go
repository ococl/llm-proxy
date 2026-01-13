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
	modelAlias, exists := cfg.Models[alias]
	if !exists || modelAlias == nil || !modelAlias.IsEnabled() {
		return nil, nil
	}

	sorted := make([]ModelRoute, len(modelAlias.Routes))
	copy(sorted, modelAlias.Routes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})

	var result []ResolvedRoute
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
	return result, nil
}
