package main

import (
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Backend struct {
	Name    string `yaml:"name"`
	URL     string `yaml:"url"`
	APIKey  string `yaml:"api_key,omitempty"`
	Enabled *bool  `yaml:"enabled,omitempty"`
}

func (b *Backend) IsEnabled() bool {
	return b.Enabled == nil || *b.Enabled
}

type ModelRoute struct {
	Backend  string `yaml:"backend"`
	Model    string `yaml:"model"`
	Priority int    `yaml:"priority"`
	Enabled  *bool  `yaml:"enabled,omitempty"`
}

func (r *ModelRoute) IsEnabled() bool {
	return r.Enabled == nil || *r.Enabled
}

type ModelAlias struct {
	Enabled *bool        `yaml:"enabled,omitempty"`
	Routes  []ModelRoute `yaml:"routes"`
}

func (m *ModelAlias) IsEnabled() bool {
	return m.Enabled == nil || *m.Enabled
}

type Fallback struct {
	CooldownSeconds int                 `yaml:"cooldown_seconds"`
	MaxRetries      int                 `yaml:"max_retries"`
	AliasFallback   map[string][]string `yaml:"alias_fallback,omitempty"`
}

type Detection struct {
	ErrorCodes    []string `yaml:"error_codes"`
	ErrorPatterns []string `yaml:"error_patterns"`
}

type Logging struct {
	Level         string `yaml:"level"`
	ConsoleLevel  string `yaml:"console_level,omitempty"`
	RequestDir    string `yaml:"request_dir"`
	ErrorDir      string `yaml:"error_dir"`
	GeneralFile   string `yaml:"general_file"`
	SeparateFiles bool   `yaml:"separate_files"`
	MaskSensitive *bool  `yaml:"mask_sensitive,omitempty"`
	EnableMetrics bool   `yaml:"enable_metrics"`
	MaxFileSizeMB int    `yaml:"max_file_size_mb"`
}

func (l *Logging) ShouldMaskSensitive() bool {
	return l.MaskSensitive == nil || *l.MaskSensitive
}

type Config struct {
	Listen      string                 `yaml:"listen"`
	ProxyAPIKey string                 `yaml:"proxy_api_key"`
	Backends    []Backend              `yaml:"backends"`
	Models      map[string]*ModelAlias `yaml:"models"`
	Fallback    Fallback               `yaml:"fallback"`
	Detection   Detection              `yaml:"detection"`
	Logging     Logging                `yaml:"logging"`
}

func (c *Config) GetListen() string {
	if c.Listen == "" {
		return ":8765"
	}
	return c.Listen
}

type ConfigManager struct {
	config     *Config
	configPath string
	lastMod    time.Time
	mu         sync.RWMutex
}

func NewConfigManager(path string) (*ConfigManager, error) {
	cm := &ConfigManager{configPath: path}
	if err := cm.load(); err != nil {
		return nil, err
	}
	return cm, nil
}

func (cm *ConfigManager) load() error {
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}
	stat, _ := os.Stat(cm.configPath)
	cm.config = &cfg
	cm.lastMod = stat.ModTime()
	return nil
}

func (cm *ConfigManager) Get() *Config {
	cm.mu.RLock()
	stat, err := os.Stat(cm.configPath)
	if err != nil || stat.ModTime().Equal(cm.lastMod) {
		defer cm.mu.RUnlock()
		return cm.config
	}
	cm.mu.RUnlock()

	cm.mu.Lock()
	defer cm.mu.Unlock()
	// Double check after acquiring write lock
	stat, _ = os.Stat(cm.configPath)
	if stat.ModTime().Equal(cm.lastMod) {
		return cm.config
	}
	if err := cm.tryReload(); err != nil {
		LogGeneral("WARN", "配置重载失败: %v，继续使用旧配置", err)
	}
	return cm.config
}

func (cm *ConfigManager) tryReload() error {
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}
	stat, _ := os.Stat(cm.configPath)
	cm.config = &cfg
	cm.lastMod = stat.ModTime()
	LogGeneral("INFO", "配置重载成功")
	return nil
}

func (cm *ConfigManager) GetBackend(name string) *Backend {
	cfg := cm.Get()
	for i := range cfg.Backends {
		if cfg.Backends[i].Name == name {
			return &cfg.Backends[i]
		}
	}
	return nil
}
