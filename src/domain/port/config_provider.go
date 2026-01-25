package port

import (
	"time"

	"llm-proxy/domain/entity"
)

type ConfigProvider interface {
	Get() *Config
	Watch() <-chan struct{}
	GetBackend(name string) *entity.Backend
	GetModelAlias(alias string) *ModelAlias
}

type MetricsProvider interface {
	IncRequestsTotal(backend string)
	RecordDuration(backend string, duration time.Duration)
	IncBackendErrors(backend string)
	SetCircuitBreakerState(backend string, state int)
	IncActiveRequests()
	DecActiveRequests()
	GetSnapshot() map[string]interface{}
}

type CooldownProvider interface {
	IsCoolingDown(backend, model string) bool
	SetCooldown(backend, model string, duration time.Duration)
	ClearExpired()
}

type RetryStrategy interface {
	ShouldRetry(attempt int, lastErr error) bool
	GetDelay(attempt int) time.Duration
	GetMaxRetries() int
}

// NopMetricsProvider is a no-op implementation of MetricsProvider for testing.
type NopMetricsProvider struct{}

// IncRequestsTotal does nothing.
func (n *NopMetricsProvider) IncRequestsTotal(backend string) {}

// RecordDuration does nothing.
func (n *NopMetricsProvider) RecordDuration(backend string, duration time.Duration) {}

// IncBackendErrors does nothing.
func (n *NopMetricsProvider) IncBackendErrors(backend string) {}

// SetCircuitBreakerState does nothing.
func (n *NopMetricsProvider) SetCircuitBreakerState(backend string, state int) {}

// IncActiveRequests does nothing.
func (n *NopMetricsProvider) IncActiveRequests() {}

// DecActiveRequests does nothing.
func (n *NopMetricsProvider) DecActiveRequests() {}

// GetSnapshot returns an empty map.
func (n *NopMetricsProvider) GetSnapshot() map[string]interface{} { return nil }
