package metrics

import (
	"testing"
	"time"

	"llm-proxy/domain/port"
)

func TestPrometheusMetricsAdapter_ImplementsInterface(t *testing.T) {
	// 确保 PrometheusMetricsAdapter 实现了 port.MetricsProvider 接口
	var _ port.MetricsProvider = (*PrometheusMetricsAdapter)(nil)
}

func TestPrometheusMetricsAdapter_New(t *testing.T) {
	adapter := NewPrometheusMetricsAdapter()
	if adapter == nil {
		t.Error("NewPrometheusMetricsAdapter should return a non-nil adapter")
	}
	if adapter.metrics == nil {
		t.Error("adapter.metrics should not be nil")
	}
}

func TestPrometheusMetricsAdapter_IncRequestsTotal(t *testing.T) {
	adapter := NewPrometheusMetricsAdapter()

	// 测试正常调用不应 panic
	adapter.IncRequestsTotal("test-backend")

	// 验证计数器已增加
	snapshot := adapter.GetSnapshot()
	requestsTotal, ok := snapshot["requests_total"].(map[string]int64)
	if !ok {
		t.Fatal("snapshot should contain requests_total map")
	}
	if requestsTotal["test-backend"] != 1 {
		t.Errorf("requests_total[test-backend] = %d, want 1", requestsTotal["test-backend"])
	}

	// 再次调用验证递增
	adapter.IncRequestsTotal("test-backend")
	snapshot = adapter.GetSnapshot()
	requestsTotal = snapshot["requests_total"].(map[string]int64)
	if requestsTotal["test-backend"] != 2 {
		t.Errorf("requests_total[test-backend] = %d, want 2", requestsTotal["test-backend"])
	}
}

func TestPrometheusMetricsAdapter_RecordDuration(t *testing.T) {
	adapter := NewPrometheusMetricsAdapter()

	// 测试正常调用不应 panic
	adapter.RecordDuration("test-backend", 100*time.Millisecond)

	// 验证持续时间已记录
	snapshot := adapter.GetSnapshot()
	avgDurations, ok := snapshot["avg_duration_ms"].(map[string]float64)
	if !ok {
		t.Fatal("snapshot should contain avg_duration_ms map")
	}
	if avgDurations["test-backend"] != 100 {
		t.Errorf("avg_duration_ms[test-backend] = %f, want 100", avgDurations["test-backend"])
	}
}

func TestPrometheusMetricsAdapter_IncBackendErrors(t *testing.T) {
	adapter := NewPrometheusMetricsAdapter()

	// 测试正常调用不应 panic
	adapter.IncBackendErrors("test-backend")

	// 验证错误计数器已增加
	snapshot := adapter.GetSnapshot()
	backendErrors, ok := snapshot["backend_errors"].(map[string]int64)
	if !ok {
		t.Fatal("snapshot should contain backend_errors map")
	}
	if backendErrors["test-backend"] != 1 {
		t.Errorf("backend_errors[test-backend] = %d, want 1", backendErrors["test-backend"])
	}
}

func TestPrometheusMetricsAdapter_SetCircuitBreakerState(t *testing.T) {
	adapter := NewPrometheusMetricsAdapter()

	// 测试正常调用不应 panic
	adapter.SetCircuitBreakerState("test-backend", 1) // 1 表示开启状态

	// 验证熔断器状态已设置
	snapshot := adapter.GetSnapshot()
	circuitBreakerState, ok := snapshot["circuit_breaker_state"].(map[string]int)
	if !ok {
		t.Fatal("snapshot should contain circuit_breaker_state map")
	}
	if circuitBreakerState["test-backend"] != 1 {
		t.Errorf("circuit_breaker_state[test-backend] = %d, want 1", circuitBreakerState["test-backend"])
	}
}

func TestPrometheusMetricsAdapter_IncActiveRequests(t *testing.T) {
	adapter := NewPrometheusMetricsAdapter()

	// 测试正常调用不应 panic
	adapter.IncActiveRequests()

	// 验证活动请求计数已增加
	snapshot := adapter.GetSnapshot()
	activeRequests, ok := snapshot["active_requests"].(int64)
	if !ok {
		t.Fatal("snapshot should contain active_requests")
	}
	if activeRequests != 1 {
		t.Errorf("active_requests = %d, want 1", activeRequests)
	}
}

func TestPrometheusMetricsAdapter_DecActiveRequests(t *testing.T) {
	adapter := NewPrometheusMetricsAdapter()

	// 重置活动请求计数（因为可能有其他测试增加了计数）
	snapshot := adapter.GetSnapshot()
	if activeRequests, ok := snapshot["active_requests"].(int64); ok && activeRequests > 0 {
		// 减少到0
		for i := int64(0); i < activeRequests; i++ {
			adapter.DecActiveRequests()
		}
	}

	// 增加两次
	adapter.IncActiveRequests()
	adapter.IncActiveRequests()

	// 减少一次
	adapter.DecActiveRequests()

	// 验证活动请求计数已减少
	snapshot = adapter.GetSnapshot()
	activeRequests, ok := snapshot["active_requests"].(int64)
	if !ok {
		t.Fatal("snapshot should contain active_requests")
	}
	if activeRequests != 1 {
		t.Errorf("active_requests = %d, want 1", activeRequests)
	}
}

func TestPrometheusMetricsAdapter_GetSnapshot(t *testing.T) {
	adapter := NewPrometheusMetricsAdapter()

	// 测试空快照
	snapshot := adapter.GetSnapshot()
	if snapshot == nil {
		t.Error("GetSnapshot should return a non-nil map")
	}

	// 验证快照包含所有预期的键
	expectedKeys := []string{
		"requests_total",
		"backend_errors",
		"circuit_breaker_state",
		"active_requests",
		"avg_duration_ms",
	}
	for _, key := range expectedKeys {
		if _, ok := snapshot[key]; !ok {
			t.Errorf("snapshot should contain key %q", key)
		}
	}
}

func TestPrometheusMetricsAdapter_MultipleBackends(t *testing.T) {
	adapter := NewPrometheusMetricsAdapter()

	// 模拟多个后端的请求
	adapter.IncRequestsTotal("openai")
	adapter.IncRequestsTotal("openai")
	adapter.IncRequestsTotal("anthropic")
	adapter.IncRequestsTotal("anthropic")
	adapter.IncRequestsTotal("anthropic")

	// 验证每个后端的计数
	snapshot := adapter.GetSnapshot()
	requestsTotal := snapshot["requests_total"].(map[string]int64)

	if requestsTotal["openai"] != 2 {
		t.Errorf("requests_total[openai] = %d, want 2", requestsTotal["openai"])
	}
	if requestsTotal["anthropic"] != 3 {
		t.Errorf("requests_total[anthropic] = %d, want 3", requestsTotal["anthropic"])
	}
}

func TestPrometheusMetricsAdapter_ConcurrentAccess(t *testing.T) {
	adapter := NewPrometheusMetricsAdapter()

	// 并发测试
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				adapter.IncRequestsTotal("concurrent-backend")
				adapter.IncActiveRequests()
				adapter.DecActiveRequests()
			}
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证最终状态
	snapshot := adapter.GetSnapshot()
	requestsTotal := snapshot["requests_total"].(map[string]int64)
	if requestsTotal["concurrent-backend"] != 1000 {
		t.Errorf("requests_total[concurrent-backend] = %d, want 1000", requestsTotal["concurrent-backend"])
	}
}

func TestNopMetricsProvider(t *testing.T) {
	// 测试 NopMetricsProvider 的所有方法都不 panic
	provider := &port.NopMetricsProvider{}

	provider.IncRequestsTotal("test")
	provider.RecordDuration("test", time.Second)
	provider.IncBackendErrors("test")
	provider.SetCircuitBreakerState("test", 1)
	provider.IncActiveRequests()
	provider.DecActiveRequests()

	snapshot := provider.GetSnapshot()
	if snapshot != nil {
		t.Error("NopMetricsProvider.GetSnapshot should return nil")
	}
}
