package usecase

import (
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
func (rs *RetryStrategy) ShouldRetry(attempt int, lastErr error) bool {
	return attempt < rs.maxRetries
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
