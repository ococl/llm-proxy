package main

import (
	"net"
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	global    *rate.Limiter
	perIP     map[string]*rate.Limiter
	perModel  map[string]*rate.Limiter
	mu        sync.RWMutex
	config    *RateLimitConfig
	configMgr *ConfigManager
}

func NewRateLimiter(configMgr *ConfigManager) *RateLimiter {
	cfg := configMgr.Get().RateLimit
	rl := &RateLimiter{
		perIP:     make(map[string]*rate.Limiter),
		perModel:  make(map[string]*rate.Limiter),
		configMgr: configMgr,
		config:    &cfg,
	}
	if cfg.Enabled {
		burst := int(cfg.GetGlobalRPS() * cfg.GetBurstFactor())
		rl.global = rate.NewLimiter(rate.Limit(cfg.GetGlobalRPS()), burst)
	}
	return rl
}

func (rl *RateLimiter) getIPLimiter(ip string) *rate.Limiter {
	rl.mu.RLock()
	limiter, exists := rl.perIP[ip]
	rl.mu.RUnlock()
	if exists {
		return limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()
	if limiter, exists = rl.perIP[ip]; exists {
		return limiter
	}
	cfg := rl.config
	burst := int(cfg.GetPerIPRPS() * cfg.GetBurstFactor())
	limiter = rate.NewLimiter(rate.Limit(cfg.GetPerIPRPS()), burst)
	rl.perIP[ip] = limiter
	return limiter
}

func (rl *RateLimiter) getModelLimiter(model string) *rate.Limiter {
	rl.mu.RLock()
	limiter, exists := rl.perModel[model]
	rl.mu.RUnlock()
	if exists {
		return limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()
	if limiter, exists = rl.perModel[model]; exists {
		return limiter
	}
	cfg := rl.config
	rps := cfg.GetGlobalRPS()
	if modelRPS, ok := cfg.PerModelRPS[model]; ok && modelRPS > 0 {
		rps = modelRPS
	}
	burst := int(rps * cfg.GetBurstFactor())
	limiter = rate.NewLimiter(rate.Limit(rps), burst)
	rl.perModel[model] = limiter
	return limiter
}

func (rl *RateLimiter) Allow(ip, model string) bool {
	cfg := rl.configMgr.Get().RateLimit
	if !cfg.Enabled {
		return true
	}
	if rl.global != nil && !rl.global.Allow() {
		return false
	}
	if ip != "" && !rl.getIPLimiter(ip).Allow() {
		return false
	}
	if model != "" && !rl.getModelLimiter(model).Allow() {
		return false
	}
	return true
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := rl.configMgr.Get().RateLimit
		if !cfg.Enabled {
			next.ServeHTTP(w, r)
			return
		}
		ip := extractIP(r)
		if !rl.Allow(ip, "") {
			WriteJSONError(w, ErrRateLimited, http.StatusTooManyRequests, "")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
