package domain

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

// ShouldRetry determines if a retry should be attempted.
func (fs *FallbackStrategy) ShouldRetry(attempt int, lastErr error) bool {
	return attempt < fs.maxRetries
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