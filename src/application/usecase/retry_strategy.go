package usecase

import (
	"strings"
	"time"
)

// RetryStrategy implements configurable retry logic.
type RetryStrategy struct {
	maxRetries        int
	enableBackoff     bool
	backoffInitial    time.Duration
	backoffMax        time.Duration
	backoffMultiplier float64
	backoffJitter     float64
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
	}
}

// ShouldRetry implements port.RetryStrategy.
// 根据错误类型和重试次数决定是否应该重试。
// 对于认证错误(401)、权限错误(403)、客户端错误(400)等不应重试。
// 对于服务器错误(500)、服务不可用(503)、超时等可以重试。
func (rs *RetryStrategy) ShouldRetry(attempt int, lastErr error) bool {
	// 首先检查重试次数是否超过最大值
	if attempt >= rs.maxRetries {
		return false
	}

	// 如果没有错误信息，允许重试
	if lastErr == nil {
		return true
	}

	// 根据错误消息判断错误类型，决定是否重试
	errMsg := lastErr.Error()

	// 客户端错误(4xx)通常不应重试，除非是速率限制(429)可能需要等待后重试
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
