package main

import (
	"math/rand"
	"sort"
	"time"
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
		LogGeneral("WARN", "检测到循环回退: 别名=%s", alias)
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

		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		for i := 0; i < len(sorted); {
			j := i + 1
			for j < len(sorted) && sorted[j].Priority == sorted[i].Priority {
				j++
			}
			if j-i > 1 {
				rng.Shuffle(j-i, func(a, b int) {
					sorted[i+a], sorted[i+b] = sorted[i+b], sorted[i+a]
				})
			}
			i = j
		}

		for _, route := range sorted {
			if !route.IsEnabled() {
				continue
			}
			key := r.cooldown.Key(route.Backend, route.Model)
			if r.cooldown.IsCoolingDown(key) {
				LogGeneral("DEBUG", "跳过冷却中的后端: %s", key)
				continue
			}
			backend := r.configMgr.GetBackend(route.Backend)
			if backend == nil {
				LogGeneral("WARN", "后端不存在: %s", route.Backend)
				continue
			}
			if !backend.IsEnabled() {
				LogGeneral("DEBUG", "跳过已禁用的后端: %s", route.Backend)
				continue
			}
			result = append(result, ResolvedRoute{
				BackendName: backend.Name,
				BackendURL:  backend.URL,
				Model:       route.Model,
			})
		}
	}

	visitedCopy := make(map[string]bool, len(visited)+1)
	for k, v := range visited {
		visitedCopy[k] = v
	}
	fallbackRoutes := r.collectFallbackRoutes(alias, visitedCopy)
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
		routes, err := r.resolveWithVisited(fallbackAlias, visited)
		if err != nil {
			LogGeneral("WARN", "解析回退别名 %s 失败: %v", fallbackAlias, err)
			continue
		}
		if len(routes) > 0 {
			LogGeneral("DEBUG", "添加回退路由: %s -> %s", alias, fallbackAlias)
			result = append(result, routes...)
		}
	}
	return result
}
