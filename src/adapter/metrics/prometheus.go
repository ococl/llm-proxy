package metrics

import (
	"time"

	"llm-proxy/domain/port"
	"llm-proxy/infrastructure/logging"
)

// PrometheusMetricsAdapter 将 infrastructure/logging 的 PrometheusMetrics 适配为 port.MetricsProvider 接口
type PrometheusMetricsAdapter struct {
	metrics *logging.PrometheusMetrics
}

// NewPrometheusMetricsAdapter 创建一个新的 Prometheus 指标适配器
func NewPrometheusMetricsAdapter() *PrometheusMetricsAdapter {
	return &PrometheusMetricsAdapter{
		metrics: logging.GetGlobalMetrics(),
	}
}

// IncRequestsTotal 增加请求总数计数器
func (a *PrometheusMetricsAdapter) IncRequestsTotal(backend string) {
	a.metrics.IncRequestsTotal(backend)
}

// RecordDuration 记录请求持续时间（毫秒）
func (a *PrometheusMetricsAdapter) RecordDuration(backend string, duration time.Duration) {
	a.metrics.RecordDuration(backend, duration)
}

// IncBackendErrors 增加后端错误计数器
func (a *PrometheusMetricsAdapter) IncBackendErrors(backend string) {
	a.metrics.IncBackendErrors(backend)
}

// SetCircuitBreakerState 设置熔断器状态
func (a *PrometheusMetricsAdapter) SetCircuitBreakerState(backend string, state int) {
	a.metrics.SetCircuitBreakerState(backend, state)
}

// IncActiveRequests 增加活动请求计数
func (a *PrometheusMetricsAdapter) IncActiveRequests() {
	a.metrics.IncActiveRequests()
}

// DecActiveRequests 减少活动请求计数
func (a *PrometheusMetricsAdapter) DecActiveRequests() {
	a.metrics.DecActiveRequests()
}

// GetSnapshot 获取当前指标快照
func (a *PrometheusMetricsAdapter) GetSnapshot() map[string]interface{} {
	return a.metrics.GetSnapshot()
}

// 确保 PrometheusMetricsAdapter 实现 port.MetricsProvider 接口
var _ port.MetricsProvider = (*PrometheusMetricsAdapter)(nil)
