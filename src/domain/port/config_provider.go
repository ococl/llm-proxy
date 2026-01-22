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
