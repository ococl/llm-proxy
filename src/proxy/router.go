package proxy

import (
	"math/rand"
	"sort"
	"time"

	"llm-proxy/backend"
	"llm-proxy/config"
	"llm-proxy/logging"
)

type Router struct {
	configMgr *config.Manager
	cooldown  *backend.CooldownManager
}

func NewRouter(cfg *config.Manager, cd *backend.CooldownManager) *Router {
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
		logging.ProxySugar.Warnw("检测到循环回退", "alias", alias)
		return nil, nil
	}
	visited[alias] = true

	cfg := r.configMgr.Get()
	var result []ResolvedRoute

	modelAlias, exists := cfg.Models[alias]
	if exists && modelAlias != nil && modelAlias.IsEnabled() {
		sorted := make([]config.ModelRoute, len(modelAlias.Routes))
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
				logging.ProxySugar.Debugw("跳过冷却中的后端", "key", key)
				continue
			}
			bkend := r.configMgr.GetBackend(route.Backend)
			if bkend == nil {
				logging.ProxySugar.Warnw("后端不存在", "backend", route.Backend)
				continue
			}
			if !bkend.IsEnabled() {
				logging.ProxySugar.Debugw("跳过已禁用的后端", "backend", route.Backend)
				continue
			}
			if result == nil {
				result = make([]ResolvedRoute, 0, len(sorted))
			}
			result = append(result, ResolvedRoute{
				BackendName: bkend.Name,
				BackendURL:  bkend.URL,
				Model:       route.Model,
			})
		}
	}

	visitedCopy := make(map[string]bool, len(visited)+1)
	for k, v := range visited {
		visitedCopy[k] = v
	}
	fallbackRoutes := r.collectFallbackRoutes(alias, visitedCopy)
	if len(fallbackRoutes) > 0 {
		if result == nil {
			result = fallbackRoutes
		} else {
			result = append(result, fallbackRoutes...)
		}
	}

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
			logging.ProxySugar.Warnw("解析回退别名失败", "fallbackAlias", alias, "error", err)
			continue
		}
		if len(routes) > 0 {
			logging.FileOnlySugar.Debugw("添加回退路由", "alias", alias, "fallbackAlias", fallbackAlias)
			result = append(result, routes...)
		}
	}
	return result
}
