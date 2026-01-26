package middleware

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	adapter_config "llm-proxy/adapter/config"
	"llm-proxy/infrastructure/config"
)

// TestRateLimiter_HotReload 测试限流器的热重载功能
func TestRateLimiter_HotReload(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	initialConfig := `
listen: ":8080"
proxy_api_key: "test-key"
backends:
  - name: "test-backend"
    url: "http://localhost:8080"
models:
  "test/model":
    routes:
      - backend: "test-backend"
        model: "test-model"
        priority: 1
fallback:
  cooldown_seconds: 60
  max_retries: 3
detection:
  error_codes: ["4xx", "5xx"]
logging:
  level: "info"
rate_limit:
  enabled: true
  global_rps: 10.0
  burst_factor: 1.0
`

	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}

	mgr, err := config.NewManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	adapter := adapter_config.NewConfigAdapter(mgr)
	rl := NewRateLimiter(adapter)

	// 验证初始配置
	cfg := rl.configGetter()
	if cfg.GlobalRPS != 10.0 {
		t.Errorf("Initial global RPS = %f, want %f", cfg.GlobalRPS, 10.0)
	}

	// 启动监控
	notifyChan := mgr.Watch()
	defer mgr.StopWatch()

	// 修改配置
	updatedConfig := `
listen: ":8080"
proxy_api_key: "test-key"
backends:
  - name: "test-backend"
    url: "http://localhost:8080"
models:
  "test/model":
    routes:
      - backend: "test-backend"
        model: "test-model"
        priority: 1
fallback:
  cooldown_seconds: 60
  max_retries: 3
detection:
  error_codes: ["4xx", "5xx"]
logging:
  level: "info"
rate_limit:
  enabled: true
  global_rps: 100.0
  burst_factor: 2.0
`

	time.Sleep(100 * time.Millisecond)

	if err := os.WriteFile(configPath, []byte(updatedConfig), 0644); err != nil {
		t.Fatalf("Failed to write updated config: %v", err)
	}

	// 等待配置变更通知
	select {
	case <-notifyChan:
		t.Log("Received config change notification")
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for config change notification")
	}

	// 手动调用 Update 来模拟热重载
	rl.Update()

	// 验证更新后的配置
	cfg = rl.configGetter()
	if cfg.GlobalRPS != 100.0 {
		t.Errorf("Updated global RPS = %f, want %f", cfg.GlobalRPS, 100.0)
	}
	if cfg.BurstFactor != 2.0 {
		t.Errorf("Updated burst factor = %f, want %f", cfg.BurstFactor, 2.0)
	}
}

// TestConcurrencyLimiter_HotReload 测试并发限制器的热重载功能
func TestConcurrencyLimiter_HotReload(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	initialConfig := `
listen: ":8080"
proxy_api_key: "test-key"
backends:
  - name: "test-backend"
    url: "http://localhost:8080"
models:
  "test/model":
    routes:
      - backend: "test-backend"
        model: "test-model"
        priority: 1
fallback:
  cooldown_seconds: 60
  max_retries: 3
detection:
  error_codes: ["4xx", "5xx"]
logging:
  level: "info"
concurrency:
  enabled: true
  max_requests: 10
  max_queue_size: 100
`

	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}

	mgr, err := config.NewManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	adapter := adapter_config.NewConfigAdapter(mgr)
	cl := NewConcurrencyLimiter(adapter)

	// 验证初始配置
	cfg := cl.configGetter()
	if cfg.MaxRequests != 10 {
		t.Errorf("Initial max requests = %d, want %d", cfg.MaxRequests, 10)
	}

	// 启动监控
	notifyChan := mgr.Watch()
	defer mgr.StopWatch()

	// 修改配置
	updatedConfig := `
listen: ":8080"
proxy_api_key: "test-key"
backends:
  - name: "test-backend"
    url: "http://localhost:8080"
models:
  "test/model":
    routes:
      - backend: "test-backend"
        model: "test-model"
        priority: 1
fallback:
  cooldown_seconds: 60
  max_retries: 3
detection:
  error_codes: ["4xx", "5xx"]
logging:
  level: "info"
concurrency:
  enabled: true
  max_requests: 50
  max_queue_size: 200
`

	time.Sleep(100 * time.Millisecond)

	if err := os.WriteFile(configPath, []byte(updatedConfig), 0644); err != nil {
		t.Fatalf("Failed to write updated config: %v", err)
	}

	// 等待配置变更通知
	select {
	case <-notifyChan:
		t.Log("Received config change notification")
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for config change notification")
	}

	// 手动调用 Update 来模拟热重载
	cl.Update()

	// 验证更新后的配置
	cfg = cl.configGetter()
	if cfg.MaxRequests != 50 {
		t.Errorf("Updated max requests = %d, want %d", cfg.MaxRequests, 50)
	}
	if cfg.MaxQueueSize != 200 {
		t.Errorf("Updated max queue size = %d, want %d", cfg.MaxQueueSize, 200)
	}
}

// TestRateLimiter_Update_DisablesLimiter 测试 Update 方法禁用限流器
func TestRateLimiter_Update_DisablesLimiter(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	enabledConfig := `
listen: ":8080"
backends:
  - name: "test-backend"
    url: "http://localhost:8080"
rate_limit:
  enabled: true
  global_rps: 10.0
`

	if err := os.WriteFile(configPath, []byte(enabledConfig), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	mgr, err := config.NewManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	adapter := adapter_config.NewConfigAdapter(mgr)
	rl := NewRateLimiter(adapter)

	// 验证启用状态
	cfg := rl.configGetter()
	if !cfg.Enabled {
		t.Error("Expected rate limiter to be enabled")
	}

	// 禁用限流器
	disabledConfig := `
listen: ":8080"
backends:
  - name: "test-backend"
    url: "http://localhost:8080"
rate_limit:
  enabled: false
`

	if err := os.WriteFile(configPath, []byte(disabledConfig), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// 等待配置加载
	time.Sleep(100 * time.Millisecond)

	// 调用 Update
	rl.Update()

	// 验证禁用状态
	cfg = rl.configGetter()
	if cfg.Enabled {
		t.Error("Expected rate limiter to be disabled after update")
	}
}

// TestConcurrencyLimiter_Update_DisablesLimiter 测试 Update 方法禁用并发限制器
func TestConcurrencyLimiter_Update_DisablesLimiter(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	enabledConfig := `
listen: ":8080"
backends:
  - name: "test-backend"
    url: "http://localhost:8080"
concurrency:
  enabled: true
  max_requests: 10
`

	if err := os.WriteFile(configPath, []byte(enabledConfig), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	mgr, err := config.NewManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	adapter := adapter_config.NewConfigAdapter(mgr)
	cl := NewConcurrencyLimiter(adapter)

	// 验证启用状态
	cfg := cl.configGetter()
	if !cfg.Enabled {
		t.Error("Expected concurrency limiter to be enabled")
	}

	// 禁用并发限制器
	disabledConfig := `
listen: ":8080"
backends:
  - name: "test-backend"
    url: "http://localhost:8080"
concurrency:
  enabled: false
`

	if err := os.WriteFile(configPath, []byte(disabledConfig), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// 等待配置加载
	time.Sleep(100 * time.Millisecond)

	// 调用 Update
	cl.Update()

	// 验证禁用状态
	cfg = cl.configGetter()
	if cfg.Enabled {
		t.Error("Expected concurrency limiter to be disabled after update")
	}
}
