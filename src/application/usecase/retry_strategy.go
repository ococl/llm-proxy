package usecase

import (
	"fmt"
	"strings"
	"time"

	"llm-proxy/domain/port"
)

// RetryStrategy implements configurable retry logic.
type RetryStrategy struct {
	maxRetries        int
	enableBackoff     bool
	backoffInitial    time.Duration
	backoffMax        time.Duration
	backoffMultiplier float64
	backoffJitter     float64
	// 错误回退配置
	errorFallback *port.ErrorFallbackConfig
}

// NewRetryStrategy creates a new retry strategy.
func NewRetryStrategy(
	maxRetries int,
	enableBackoff bool,
	backoffInitial,
	backoffMax time.Duration,
	backoffMultiplier,
	backoffJitter float64,
) *RetryStrategy {
	return NewRetryStrategyWithFallback(maxRetries, enableBackoff, backoffInitial, backoffMax, backoffMultiplier, backoffJitter, nil)
}

// NewRetryStrategyWithFallback creates a retry strategy with error fallback config.
func NewRetryStrategyWithFallback(
	maxRetries int,
	enableBackoff bool,
	backoffInitial,
	backoffMax time.Duration,
	backoffMultiplier,
	backoffJitter float64,
	errorFallback *port.ErrorFallbackConfig,
) *RetryStrategy {
	if maxRetries <= 0 {
		maxRetries = 3
	}
	if backoffInitial <= 0 {
		backoffInitial = 100 * time.Millisecond
	}
	if backoffMax <= 0 {
		backoffMax = 5 * time.Second
	}
	if backoffMultiplier <= 0 {
		backoffMultiplier = 2.0
	}
	if backoffJitter < 0 || backoffJitter > 1 {
		backoffJitter = 0.1
	}

	return &RetryStrategy{
		maxRetries:        maxRetries,
		enableBackoff:     enableBackoff,
		backoffInitial:    backoffInitial,
		backoffMax:        backoffMax,
		backoffMultiplier: backoffMultiplier,
		backoffJitter:     backoffJitter,
		errorFallback:     errorFallback,
	}
}

// ShouldRetry implements port.RetryStrategy.
// 根据错误类型和重试次数决定是否应该回退。
// - 5xx 错误：启用 server_error 回退时立即回退
// - 4xx 错误：401/403/429 或匹配 patterns 关键词时立即回退
// - 其他情况：不回退（返回 false）
func (rs *RetryStrategy) ShouldRetry(attempt int, lastErr error) bool {
	// 首先检查重试次数是否超过最大值
	if attempt >= rs.maxRetries {
		return false
	}

	// 如果没有错误信息，不回退
	if lastErr == nil {
		return false
	}

	// 根据错误消息判断错误类型，决定是否回退
	errMsg := lastErr.Error()

	// 检查是否应该回退
	return rs.shouldFallback(errMsg)
}

// shouldFallback 判断错误消息是否应该触发回退
func (rs *RetryStrategy) shouldFallback(errMsg string) bool {
	// 如果没有配置，使用默认行为
	if rs.errorFallback == nil {
		return rs.defaultFallback(errMsg)
	}

	// 检查是否是服务器错误（5xx）
	if rs.errorFallback.ServerError.Enabled && isServerError(errMsg) {
		return true
	}

	// 检查是否是客户端错误（4xx）且需要回退
	if rs.errorFallback.ClientError.Enabled {
		// 检查状态码是否在列表中
		if containsStatusCode(errMsg, rs.errorFallback.ClientError.StatusCodes) {
			return true
		}

		// 检查错误消息是否包含配置的关键词
		for _, pattern := range rs.errorFallback.ClientError.Patterns {
			if strings.Contains(strings.ToLower(errMsg), strings.ToLower(pattern)) {
				return true
			}
		}
	}

	return false
}

// defaultFallback 默认回退逻辑（向后兼容）
func (rs *RetryStrategy) defaultFallback(errMsg string) bool {
	// 5xx 错误默认回退
	if isServerError(errMsg) {
		return true
	}

	// 客户端错误(4xx)不回退，除非是速率限制(429)
	if isClientError(errMsg) && !isRateLimitError(errMsg) {
		return false
	}

	// 速率限制可以重试
	if isRateLimitError(errMsg) {
		return true
	}

	return false
}

// containsStatusCode 检查错误消息中是否包含指定的状态码
func containsStatusCode(errMsg string, codes []int) bool {
	for _, code := range codes {
		codeStr := fmt.Sprintf("%d", code)
		if strings.Contains(errMsg, codeStr) {
			return true
		}
	}
	return false
}

// isServerError 判断是否为服务器错误(5xx)
func isServerError(errMsg string) bool {
	serverErrorPatterns := []string{
		"500", "Internal Server Error",
		"502", "Bad Gateway",
		"503", "Service Unavailable",
		"504", "Gateway Timeout",
	}

	for _, pattern := range serverErrorPatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}
	return false
}

// isClientError 判断是否为客户端错误(4xx)
func isClientError(errMsg string) bool {
	clientErrorPatterns := []string{
		"401", "Unauthorized",
		"403", "Forbidden",
		"400", "Bad Request",
		"404", "Not Found",
		"422", "Unprocessable Entity",
	}

	for _, pattern := range clientErrorPatterns {
		if strings.Contains(errMsg, pattern) {
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
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}
	return false
}

// GetDelay implements port.RetryStrategy.
func (rs *RetryStrategy) GetDelay(attempt int) time.Duration {
	if !rs.enableBackoff || attempt == 0 {
		return 0
	}

	delay := float64(rs.backoffInitial)
	for i := 1; i < attempt; i++ {
		delay *= rs.backoffMultiplier
	}
	if delay > float64(rs.backoffMax) {
		delay = float64(rs.backoffMax)
	}

	// Add jitter
	jitterRange := delay * rs.backoffJitter
	delay = delay - jitterRange + (float64(time.Now().UnixNano()%1000) / 1000 * jitterRange * 2)

	return time.Duration(delay)
}

// GetMaxRetries implements port.RetryStrategy.
func (rs *RetryStrategy) GetMaxRetries() int {
	return rs.maxRetries
}

// DefaultRetryStrategy returns a default retry strategy.
func DefaultRetryStrategy() *RetryStrategy {
	return NewRetryStrategy(
		3,                    // maxRetries
		true,                 // enableBackoff
		100*time.Millisecond, // backoffInitial
		5*time.Second,        // backoffMax
		2.0,                  // backoffMultiplier
		0.1,                  // backoffJitter
	)
}
