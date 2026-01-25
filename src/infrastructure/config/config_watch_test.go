package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestManager_Watch(t *testing.T) {
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
`

	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}

	mgr, err := NewManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	notifyChan := mgr.Watch()
	defer mgr.StopWatch()

	cfg := mgr.Get()
	if cfg.Listen != ":8080" {
		t.Errorf("Initial config Listen = %q, want %q", cfg.Listen, ":8080")
	}

	updatedConfig := `
listen: ":9090"
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
`

	time.Sleep(100 * time.Millisecond)

	if err := os.WriteFile(configPath, []byte(updatedConfig), 0644); err != nil {
		t.Fatalf("Failed to write updated config: %v", err)
	}

	select {
	case <-notifyChan:
		t.Log("Received config change notification")
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for config change notification")
	}

	time.Sleep(100 * time.Millisecond)

	cfg = mgr.Get()
	if cfg.Listen != ":9090" {
		t.Errorf("Updated config Listen = %q, want %q", cfg.Listen, ":9090")
	}
}

func TestManager_Watch_LoggingChange(t *testing.T) {
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
  console_level: "info"
`

	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}

	mgr, err := NewManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	callbackCalled := false
	LoggingConfigChangedFunc = func(c *Config) error {
		callbackCalled = true
		return nil
	}
	defer func() { LoggingConfigChangedFunc = nil }()

	notifyChan := mgr.Watch()
	defer mgr.StopWatch()

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
  level: "debug"
  console_level: "debug"
`

	time.Sleep(100 * time.Millisecond)

	if err := os.WriteFile(configPath, []byte(updatedConfig), 0644); err != nil {
		t.Fatalf("Failed to write updated config: %v", err)
	}

	select {
	case <-notifyChan:
		t.Log("Received config change notification")
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for config change notification")
	}

	time.Sleep(100 * time.Millisecond)

	if !callbackCalled {
		t.Error("LoggingConfigChangedFunc was not called")
	}

	cfg := mgr.Get()
	if cfg.Logging.Level != "debug" {
		t.Errorf("Updated logging level = %q, want %q", cfg.Logging.Level, "debug")
	}
}

func TestManager_StopWatch(t *testing.T) {
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
`

	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}

	mgr, err := NewManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	notifyChan := mgr.Watch()

	mgr.StopWatch()

	time.Sleep(100 * time.Millisecond)

	updatedConfig := `
listen: ":9090"
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
`

	if err := os.WriteFile(configPath, []byte(updatedConfig), 0644); err != nil {
		t.Fatalf("Failed to write updated config: %v", err)
	}

	select {
	case <-notifyChan:
		t.Error("Received notification after StopWatch was called")
	case <-time.After(3 * time.Second):
		t.Log("No notification received after StopWatch (expected)")
	}
}
