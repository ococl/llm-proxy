package proxy

import (
	"math"
	"math/rand"
	"time"
)

// BackoffStrategy defines the retry backoff behavior
type BackoffStrategy struct {
	// Initial delay before first retry
	InitialDelay time.Duration
	// Maximum delay between retries
	MaxDelay time.Duration
	// Multiplier for exponential backoff (e.g., 2.0 for doubling)
	Multiplier float64
	// Jitter factor (0.0-1.0) to add randomness
	JitterFactor float64
	// Maximum number of retries
	MaxRetries int
}

// DefaultBackoffStrategy returns a production-ready backoff strategy
func DefaultBackoffStrategy() *BackoffStrategy {
	return &BackoffStrategy{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		JitterFactor: 0.1,
		MaxRetries:   3,
	}
}

// CalculateDelay calculates the delay for the given retry attempt
func (b *BackoffStrategy) CalculateDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	if attempt > b.MaxRetries {
		return 0
	}

	// Exponential backoff: delay = initial * (multiplier ^ (attempt - 1))
	delay := float64(b.InitialDelay) * math.Pow(b.Multiplier, float64(attempt-1))

	// Cap at max delay
	if delay > float64(b.MaxDelay) {
		delay = float64(b.MaxDelay)
	}

	// Add jitter to avoid thundering herd
	if b.JitterFactor > 0 {
		jitter := delay * b.JitterFactor * (rand.Float64() - 0.5) * 2 // -jitterFactor to +jitterFactor
		delay += jitter
		if delay < 0 {
			delay = 0
		}
	}

	return time.Duration(delay)
}

// ShouldRetry determines if a retry should be attempted
func (b *BackoffStrategy) ShouldRetry(attempt int) bool {
	return attempt > 0 && attempt <= b.MaxRetries
}

// NewBackoffStrategy creates a custom backoff strategy
func NewBackoffStrategy(initialDelay, maxDelay time.Duration, multiplier, jitterFactor float64, maxRetries int) *BackoffStrategy {
	return &BackoffStrategy{
		InitialDelay: initialDelay,
		MaxDelay:     maxDelay,
		Multiplier:   multiplier,
		JitterFactor: jitterFactor,
		MaxRetries:   maxRetries,
	}
}
