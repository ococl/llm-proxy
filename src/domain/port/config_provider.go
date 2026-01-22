package port

import "time"

// ConfigProvider interface for configuration access.
// This abstracts the configuration loading for better testability.
type ConfigProvider interface {
	// Get returns the current configuration.
	Get() *Config
	// Watch returns a channel that signals when configuration changes.
	Watch() <-chan struct{}
	// GetBackend returns a backend by name.
	GetBackend(name string) *Backend
	// GetModelAlias returns a model alias configuration.
	GetModelAlias(alias string) *ModelAlias
}

// MetricsProvider interface for collecting application metrics.
type MetricsProvider interface {
	// IncRequestsTotal increments the request counter for a backend.
	IncRequestsTotal(backend string)
	// RecordDuration records the duration of a request.
	RecordDuration(backend string, duration time.Duration)
	// IncBackendErrors increments the error counter for a backend.
	IncBackendErrors(backend string)
	// SetCircuitBreakerState sets the circuit breaker state for a backend.
	SetCircuitBreakerState(backend string, state int)
	// IncActiveRequests increments the active request counter.
	IncActiveRequests()
	// DecActiveRequests decrements the active request counter.
	DecActiveRequests()
	// GetSnapshot returns a snapshot of current metrics.
	GetSnapshot() map[string]interface{}
}

// CooldownProvider interface for backend cooldown management.
type CooldownProvider interface {
	// IsCoolingDown returns true if the backend is in cooldown for the given model.
	IsCoolingDown(backend, model string) bool
	// SetCooldown sets a cooldown period for a backend/model combination.
	SetCooldown(backend, model string, duration time.Duration)
	// ClearExpired removes expired cooldown entries.
	ClearExpired()
}
