package main

import (
	"bytes"
	"encoding/json"
	"io"
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

		// 从请求体中提取模型信息
		ip := extractIP(r)

		// 对于某些路径，我们可能无法直接获得模型信息，因此只对特定路径进行模型级限流
		var model string
		if r.URL.Path == "/v1/chat/completions" || r.URL.Path == "/v1/completions" {
			// 读取请求体以提取模型信息
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				WriteJSONError(w, ErrBadRequest, http.StatusBadRequest, "")
				return
			}

			// 重新设置请求体
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

			// 解析模型
			var reqBody map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
				// 如果无法解析请求体，仍然进行全局和IP限流
				model = ""
			} else {
				if modelVal, ok := reqBody["model"].(string); ok {
					model = modelVal
				}
			}
		}

		if !rl.Allow(ip, model) {
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
