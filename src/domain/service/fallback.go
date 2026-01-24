package service

import (
	"math/rand"
	"time"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
)

// FallbackStrategy implements the fallback logic for routing.
type FallbackStrategy struct {
	cooldownMgr       port.CooldownProvider
	fallbackAliases   map[string][]entity.ModelAlias
	enableBackoff     bool
	backoffInitial    time.Duration
	backoffMax        time.Duration
	backoffMultiplier float64
	backoffJitter     float64
	maxRetries        int
}

// NewFallbackStrategy creates a new fallback strategy.
func NewFallbackStrategy(
	cooldownMgr port.CooldownProvider,
	fallbackAliases map[string][]entity.ModelAlias,
	backoffConfig entity.RetryConfig,
) *FallbackStrategy {
	return &FallbackStrategy{
		cooldownMgr:       cooldownMgr,
		fallbackAliases:   fallbackAliases,
		enableBackoff:     backoffConfig.EnableBackoff,
		backoffInitial:    backoffConfig.GetBackoffInitialDelay(),
		backoffMax:        backoffConfig.GetBackoffMaxDelay(),
		backoffMultiplier: backoffConfig.GetBackoffMultiplier(),
		backoffJitter:     backoffConfig.GetBackoffJitter(),
		maxRetries:        backoffConfig.GetMaxRetries(),
	}
}

// ShouldRetry 根据错误类型和重试次数决定是否应该重试。
// 对于认证错误(401)、权限错误(403)、客户端错误(400)等不应重试。
// 对于服务器错误(500)、服务不可用(503)、超时等可以重试。
func (fs *FallbackStrategy) ShouldRetry(attempt int, lastErr error) bool {
	// 首先检查重试次数是否超过最大值
	if attempt >= fs.maxRetries {
		return false
	}

	// 如果没有错误信息,允许重试
	if lastErr == nil {
		return true
	}

	// 根据错误消息判断错误类型,决定是否重试
	errMsg := lastErr.Error()

	// 客户端错误(4xx)通常不应重试,除非是速率限制(429)可能需要等待后重试
	if isClientError(errMsg) && !isRateLimitError(errMsg) {
		return false
	}

	// 其他情况允许重试
	return true
}

// isClientError 判断是否为客户端错误(4xx)
func isClientError(errMsg string) bool {
	// 检查常见的客户端错误状态码
	clientErrorPatterns := []string{
		"401", "Unauthorized",
		"403", "Forbidden",
		"400", "Bad Request",
		"404", "Not Found",
		"422", "Unprocessable Entity",
	}

	for _, pattern := range clientErrorPatterns {
		if contains(errMsg, pattern) {
			return true
		}
	}
	return false
}

// isRateLimitError 判断是否为速率限制错误(429)
func isRateLimitError(errMsg string) bool {
	rateLimitPatterns := []string{
		"429", "Too Many Requests",
		"rate limit",
	}

	for _, pattern := range rateLimitPatterns {
		if contains(errMsg, pattern) {
			return true
		}
	}
	return false
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

// containsHelper 辅助函数,检查子串是否存在于主字符串中
func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// GetBackoffDelay returns the delay before the next retry.
func (fs *FallbackStrategy) GetBackoffDelay(attempt int) time.Duration {
	if !fs.enableBackoff || attempt == 0 {
		return 0
	}

	delay := float64(fs.backoffInitial)
	for i := 1; i < attempt; i++ {
		delay *= fs.backoffMultiplier
	}
	if delay > float64(fs.backoffMax) {
		delay = float64(fs.backoffMax)
	}

	// Add jitter
	jitterRange := delay * fs.backoffJitter
	delay = delay - jitterRange + (rand.Float64() * jitterRange * 2)

	return time.Duration(delay)
}

// GetMaxRetries returns the maximum number of retries.
func (fs *FallbackStrategy) GetMaxRetries() int {
	return fs.maxRetries
}

// FilterAvailableRoutes filters routes that are not in cooldown.
func (fs *FallbackStrategy) FilterAvailableRoutes(routes []*port.Route) []*port.Route {
	var available []*port.Route
	for _, route := range routes {
		backendName := route.Backend.Name()
		modelName := route.Model
		if !fs.cooldownMgr.IsCoolingDown(backendName, modelName) {
			available = append(available, route)
		}
	}
	return available
}

// GetFallbackRoutes returns fallback routes for the given alias.
func (fs *FallbackStrategy) GetFallbackRoutes(
	originalAlias string,
	routeResolver port.RouteResolver,
) ([]*port.Route, error) {
	fallbackAliases, ok := fs.fallbackAliases[originalAlias]
	if !ok {
		return nil, nil
	}

	var allRoutes []*port.Route
	for _, alias := range fallbackAliases {
		routes, err := routeResolver.Resolve(alias.String())
		if err != nil {
			continue
		}
		allRoutes = append(allRoutes, routes...)
	}

	return allRoutes, nil
}

// GetNextRetryDelay calculates the next retry delay.
func (fs *FallbackStrategy) GetNextRetryDelay(attempt int) time.Duration {
	if !fs.enableBackoff {
		return 0
	}
	return fs.GetBackoffDelay(attempt)
}
