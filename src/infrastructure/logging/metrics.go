package logging

import (
	"sync"
	"time"
)

type PrometheusMetrics struct {
	RequestsTotal       map[string]int64
	RequestDurationMs   map[string][]float64
	BackendErrors       map[string]int64
	CircuitBreakerState map[string]int
	ActiveRequests      int64
	mu                  sync.RWMutex
}

func NewPrometheusMetrics() *PrometheusMetrics {
	return &PrometheusMetrics{
		RequestsTotal:       make(map[string]int64),
		RequestDurationMs:   make(map[string][]float64),
		BackendErrors:       make(map[string]int64),
		CircuitBreakerState: make(map[string]int),
	}
}

func (m *PrometheusMetrics) IncRequestsTotal(backend string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RequestsTotal[backend]++
}

func (m *PrometheusMetrics) RecordDuration(backend string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RequestDurationMs[backend] = append(m.RequestDurationMs[backend], float64(duration.Milliseconds()))
}

func (m *PrometheusMetrics) IncBackendErrors(backend string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.BackendErrors[backend]++
}

func (m *PrometheusMetrics) SetCircuitBreakerState(backend string, state int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CircuitBreakerState[backend] = state
}

func (m *PrometheusMetrics) IncActiveRequests() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ActiveRequests++
}

func (m *PrometheusMetrics) DecActiveRequests() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ActiveRequests--
}

func (m *PrometheusMetrics) GetSnapshot() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := map[string]interface{}{
		"requests_total":        m.RequestsTotal,
		"backend_errors":        m.BackendErrors,
		"circuit_breaker_state": m.CircuitBreakerState,
		"active_requests":       m.ActiveRequests,
	}

	avgDurations := make(map[string]float64)
	for backend, durations := range m.RequestDurationMs {
		if len(durations) > 0 {
			sum := 0.0
			for _, d := range durations {
				sum += d
			}
			avgDurations[backend] = sum / float64(len(durations))
		}
	}
	snapshot["avg_duration_ms"] = avgDurations

	return snapshot
}

var globalMetrics *PrometheusMetrics
var metricsOnce sync.Once

func GetGlobalMetrics() *PrometheusMetrics {
	metricsOnce.Do(func() {
		globalMetrics = NewPrometheusMetrics()
	})
	return globalMetrics
}
