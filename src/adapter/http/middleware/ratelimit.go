package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"sync"

	domainerror "llm-proxy/domain/error"
	"llm-proxy/domain/port"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	global       *rate.Limiter
	perIP        map[string]*rate.Limiter
	perModel     map[string]*rate.Limiter
	mu           sync.RWMutex
	configGetter func() port.RateLimitConfig
}

// NewRateLimiter creates a new rate limiter with the given config provider.
// The limiter will dynamically update when the configuration changes.
func NewRateLimiter(configProvider port.ConfigProvider) *RateLimiter {
	configGetter := func() port.RateLimitConfig {
		return configProvider.Get().RateLimit
	}
	cfg := configGetter()
	rl := &RateLimiter{
		perIP:        make(map[string]*rate.Limiter),
		perModel:     make(map[string]*rate.Limiter),
		configGetter: configGetter,
	}
	// 初始化全局限流器
	if cfg.Enabled {
		burst := int(cfg.GlobalRPS * cfg.BurstFactor)
		rl.global = rate.NewLimiter(rate.Limit(cfg.GlobalRPS), burst)
	}
	return rl
}

// Update 更新限流器配置，当配置变更时调用此方法
func (rl *RateLimiter) Update() {
	cfg := rl.configGetter()
	rl.mu.Lock()
	// 更新全局限流器
	if cfg.Enabled {
		burst := int(cfg.GlobalRPS * cfg.BurstFactor)
		rl.global = rate.NewLimiter(rate.Limit(cfg.GlobalRPS), burst)
	} else {
		rl.global = nil
	}
	// 清除缓存的 perIP 和 perModel limiter，下次请求时重新创建
	rl.perIP = make(map[string]*rate.Limiter)
	rl.perModel = make(map[string]*rate.Limiter)
	rl.mu.Unlock()
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
	cfg := rl.configGetter()
	burst := int(cfg.PerIPRPS * cfg.BurstFactor)
	limiter = rate.NewLimiter(rate.Limit(cfg.PerIPRPS), burst)
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
	cfg := rl.configGetter()
	rps := cfg.GlobalRPS
	if modelRPS, ok := cfg.PerModelRPS[model]; ok && modelRPS > 0 {
		rps = modelRPS
	}
	burst := int(rps * cfg.BurstFactor)
	limiter = rate.NewLimiter(rate.Limit(rps), burst)
	rl.perModel[model] = limiter
	return limiter
}

func (rl *RateLimiter) Allow(ip, model string) bool {
	cfg := rl.configGetter()
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
		cfg := rl.configGetter()
		if !cfg.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		ip := ExtractIP(r)

		var model string
		if r.URL.Path == "/v1/chat/completions" || r.URL.Path == "/v1/completions" {
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				domainerror.WriteBadRequest(w, "无法读取请求体")
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

			var reqBody map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &reqBody); err == nil {
				if modelVal, ok := reqBody["model"].(string); ok {
					model = modelVal
				}
			}
		}

		if !rl.Allow(ip, model) {
			domainerror.WriteRateLimited(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func ExtractIP(r *http.Request) string {
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
