package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"llm-proxy/infrastructure/config"
)

func TestConfigAdapter_GetBackend(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `
listen: ":8080"
backends:
  - name: openai
    url: https://api.openai.com/v1
    api_key: sk-test
    enabled: true
    protocol: openai
  - name: anthropic
    url: https://api.anthropic.com/v1
    api_key: sk-ant-test
    enabled: true
    protocol: anthropic
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	manager, err := config.NewManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	adapter := NewConfigAdapter(manager)

	backend := adapter.GetBackend("openai")
	if backend == nil {
		t.Error("GetBackend should return a backend for existing name")
	}
	if backend.Name() != "openai" {
		t.Errorf("Backend name = %v, want %v", backend.Name(), "openai")
	}
	if backend.URL().String() != "https://api.openai.com/v1" {
		t.Errorf("Backend URL = %v, want %v", backend.URL(), "https://api.openai.com/v1")
	}

	missingBackend := adapter.GetBackend("nonexistent")
	if missingBackend != nil {
		t.Error("GetBackend should return nil for non-existing name")
	}
}

func TestConfigAdapter_GetBackends(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `
listen: ":8080"
backends:
  - name: openai
    url: https://api.openai.com/v1
    api_key: sk-test
    enabled: true
  - name: disabled-backend
    url: https://disabled.example.com
    api_key: sk-disabled
    enabled: false
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	manager, err := config.NewManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	adapter := NewConfigAdapter(manager)
	backends := adapter.GetBackends()

	if len(backends) != 1 {
		t.Errorf("GetBackends returned %d backends, want 1", len(backends))
	}

	if len(backends) > 0 && backends[0].Name() != "openai" {
		t.Errorf("First backend name = %v, want %v", backends[0].Name(), "openai")
	}
}

func TestConfigAdapter_GetModelAlias(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `
listen: ":8080"
backends:
  - name: openai
    url: https://api.openai.com/v1
    api_key: sk-test
    enabled: true
models:
  gpt-4:
    enabled: true
    routes:
      - backend: openai
        model: gpt-4-turbo
        priority: 1
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	manager, err := config.NewManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	adapter := NewConfigAdapter(manager)

	alias := adapter.GetModelAlias("gpt-4")
	if alias == nil {
		t.Error("GetModelAlias should return an alias for existing name")
	}
	if !alias.IsEnabled() {
		t.Error("Model alias should be enabled")
	}

	missingAlias := adapter.GetModelAlias("nonexistent")
	if missingAlias != nil {
		t.Error("GetModelAlias should return nil for non-existing name")
	}
}

func TestConfigAdapter_Watch(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `listen: ":8080"`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	manager, err := config.NewManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	adapter := NewConfigAdapter(manager)
	ch := adapter.Watch()

	// Watch 现在应该返回非 nil 通道
	if ch == nil {
		t.Error("Watch should return a non-nil channel")
	}

	// 测试配置变更通知
	select {
	case <-ch:
		// 收到配置变更信号
	case <-time.After(3 * time.Second):
		t.Error("Did not receive config change signal within timeout")
	}
}

func TestBackendRepository_GetAll(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `
listen: ":8080"
backends:
  - name: openai
    url: https://api.openai.com/v1
    api_key: sk-test
    enabled: true
  - name: anthropic
    url: https://api.anthropic.com/v1
    api_key: sk-ant-test
    enabled: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	manager, err := config.NewManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	adapter := NewConfigAdapter(manager)
	repo := NewBackendRepository(adapter)

	backends := repo.GetAll()
	if len(backends) != 2 {
		t.Errorf("GetAll returned %d backends, want 2", len(backends))
	}
}

func TestBackendRepository_GetByName(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `
listen: ":8080"
backends:
  - name: openai
    url: https://api.openai.com/v1
    api_key: sk-test
    enabled: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	manager, err := config.NewManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	adapter := NewConfigAdapter(manager)
	repo := NewBackendRepository(adapter)

	backend := repo.GetByName("openai")
	if backend == nil {
		t.Error("GetByName should return a backend for existing name")
	}

	missingBackend := repo.GetByName("nonexistent")
	if missingBackend != nil {
		t.Error("GetByName should return nil for non-existing name")
	}
}

func TestBackendRepository_GetEnabled(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `
listen: ":8080"
backends:
  - name: openai
    url: https://api.openai.com/v1
    api_key: sk-test
    enabled: true
  - name: disabled
    url: https://disabled.example.com
    api_key: sk-test
    enabled: false
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	manager, err := config.NewManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	adapter := NewConfigAdapter(manager)
	repo := NewBackendRepository(adapter)

	enabled := repo.GetEnabled()
	if len(enabled) != 1 {
		t.Errorf("GetEnabled returned %d backends, want 1", len(enabled))
	}

	if len(enabled) > 0 && !enabled[0].IsEnabled() {
		t.Error("GetEnabled should only return enabled backends")
	}
}

func TestBackendRepository_GetByNames(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `
listen: ":8080"
backends:
  - name: openai
    url: https://api.openai.com/v1
    api_key: sk-test
    enabled: true
  - name: anthropic
    url: https://api.anthropic.com/v1
    api_key: sk-ant-test
    enabled: true
  - name: local
    url: http://localhost:8080/v1
    api_key: sk-local
    enabled: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	manager, err := config.NewManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	adapter := NewConfigAdapter(manager)
	repo := NewBackendRepository(adapter)

	backends := repo.GetByNames([]string{"openai", "anthropic"})
	if len(backends) != 2 {
		t.Errorf("GetByNames returned %d backends, want 2", len(backends))
	}

	backends = repo.GetByNames([]string{"openai", "nonexistent"})
	if len(backends) != 1 {
		t.Errorf("GetByNames should filter out non-existing names, got %d", len(backends))
	}
}
