package main

import (
	"fmt"
	"net/url"
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
	BaseDir       string `yaml:"base_dir,omitempty"`
	RequestDir    string `yaml:"request_dir"`
	ErrorDir      string `yaml:"error_dir"`
	GeneralFile   string `yaml:"general_file"`
	SeparateFiles bool   `yaml:"separate_files"`
	MaskSensitive *bool  `yaml:"mask_sensitive,omitempty"`
	EnableMetrics bool   `yaml:"enable_metrics"`
	MaxFileSizeMB int    `yaml:"max_file_size_mb"`
	MaxAgeDays    int    `yaml:"max_age_days,omitempty"`
	MaxBackups    int    `yaml:"max_backups,omitempty"`
	Compress      bool   `yaml:"compress,omitempty"`
	Format        string `yaml:"format,omitempty"`
	Colorize      *bool  `yaml:"colorize,omitempty"`
	ConsoleStyle  string `yaml:"console_style,omitempty"`
	ConsoleFormat string `yaml:"console_format,omitempty"`
	DebugMode     bool   `yaml:"debug_mode,omitempty"`
	Async         bool   `yaml:"async"`
	BufferSize    int    `yaml:"buffer_size"`
	FlushInterval int    `yaml:"flush_interval,omitempty"`
	DropOnFull    bool   `yaml:"drop_on_full"`
	// 日志轮转配置
	RotateBySize bool   `yaml:"rotate_by_size,omitempty"`
	RotateByTime bool   `yaml:"rotate_by_time,omitempty"`
	TimeRotation string `yaml:"time_rotation,omitempty"`
	// 详细脱敏配置
	DetailedMasking *bool `yaml:"detailed_masking,omitempty"`
	// 特殊处理配置
	ProblematicBackends []string `yaml:"problematic_backends,omitempty"` // 针对特定供应商的特殊处理
}

func (l *Logging) ShouldMaskSensitive() bool {
	return l.MaskSensitive == nil || *l.MaskSensitive
}

func (l *Logging) ShouldAsync() bool {
	return l.Async
}

func (l *Logging) GetBufferSize() int {
	if l.BufferSize <= 0 {
		return 10000
	}
	return l.BufferSize
}

func (l *Logging) GetConsoleFormat() string {
	if l.ConsoleFormat == "" {
		return "markdown"
	}
	return l.ConsoleFormat
}

func (l *Logging) GetFormat() string {
	if l.Format == "" {
		return "json"
	}
	return l.Format
}

func (l *Logging) ShouldDropOnFull() bool {
	return l.DropOnFull
}

func (l *Logging) GetFlushInterval() int {
	if l.FlushInterval <= 0 {
		return 5
	}
	return l.FlushInterval
}

func (l *Logging) GetMaxFileSizeMB() int {
	if l.MaxFileSizeMB <= 0 {
		return 100
	}
	return l.MaxFileSizeMB
}

func (l *Logging) GetMaxAgeDays() int {
	if l.MaxAgeDays <= 0 {
		return 7
	}
	return l.MaxAgeDays
}

func (l *Logging) GetMaxBackups() int {
	if l.MaxBackups <= 0 {
		return 10
	}
	return l.MaxBackups
}

func (l *Logging) ShouldRotateBySize() bool {
	return l.RotateBySize || l.MaxFileSizeMB > 0
}

func (l *Logging) ShouldRotateByTime() bool {
	return l.RotateByTime || l.MaxAgeDays > 0
}

func (l *Logging) GetTimeRotation() string {
	if l.TimeRotation == "" {
		return "daily"
	}
	return l.TimeRotation
}

func (l *Logging) ShouldUseDetailedMasking() bool {
	return l.DetailedMasking != nil && *l.DetailedMasking
}

func (l *Logging) GetBaseDir() string {
	if l.BaseDir == "" {
		return "./logs"
	}
	return l.BaseDir
}

func (l *Logging) GetLevel() string {
	if l.Level == "" {
		return "info"
	}
	return l.Level
}

func (l *Logging) GetConsoleLevel() string {
	if l.ConsoleLevel == "" {
		return l.GetLevel()
	}
	return l.ConsoleLevel
}

func (l *Logging) GetColorize() bool {
	return l.Colorize == nil || *l.Colorize
}

func (l *Logging) GetConsoleStyle() string {
	if l.ConsoleStyle == "" {
		return "compact"
	}
	return l.ConsoleStyle
}

type Config struct {
	Listen      string                 `yaml:"listen"`
	ProxyAPIKey string                 `yaml:"proxy_api_key"`
	Backends    []Backend              `yaml:"backends"`
	Models      map[string]*ModelAlias `yaml:"models"`
	Fallback    Fallback               `yaml:"fallback"`
	Detection   Detection              `yaml:"detection"`
	Logging     Logging                `yaml:"logging"`
	Timeout     TimeoutConfig          `yaml:"timeout"`
	RateLimit   RateLimitConfig        `yaml:"rate_limit"`
	Concurrency ConcurrencyConfig      `yaml:"concurrency"`
}

type TimeoutConfig struct {
	ConnectTimeout time.Duration `yaml:"connect_timeout"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout"`
	TotalTimeout   time.Duration `yaml:"total_timeout"`
}

type RateLimitConfig struct {
	Enabled     bool               `yaml:"enabled"`
	GlobalRPS   float64            `yaml:"global_rps"`
	PerIPRPS    float64            `yaml:"per_ip_rps"`
	PerModelRPS map[string]float64 `yaml:"per_model_rps"`
	BurstFactor float64            `yaml:"burst_factor"`
}

type ConcurrencyConfig struct {
	Enabled         bool          `yaml:"enabled"`
	MaxRequests     int           `yaml:"max_requests"`
	MaxQueueSize    int           `yaml:"max_queue_size"`
	QueueTimeout    time.Duration `yaml:"queue_timeout"`
	PerBackendLimit int           `yaml:"per_backend_limit"`
}

func (r *RateLimitConfig) GetGlobalRPS() float64 {
	if r.GlobalRPS <= 0 {
		return 1000
	}
	return r.GlobalRPS
}

func (r *RateLimitConfig) GetPerIPRPS() float64 {
	if r.PerIPRPS <= 0 {
		return 100
	}
	return r.PerIPRPS
}

func (r *RateLimitConfig) GetBurstFactor() float64 {
	if r.BurstFactor <= 0 {
		return 1.5
	}
	return r.BurstFactor
}

func (c *ConcurrencyConfig) GetMaxRequests() int {
	if c.MaxRequests <= 0 {
		return 500
	}
	return c.MaxRequests
}

func (c *ConcurrencyConfig) GetMaxQueueSize() int {
	if c.MaxQueueSize <= 0 {
		return 1000
	}
	return c.MaxQueueSize
}

func (c *ConcurrencyConfig) GetQueueTimeout() time.Duration {
	if c.QueueTimeout <= 0 {
		return 30 * time.Second
	}
	return c.QueueTimeout
}

func (c *ConcurrencyConfig) GetPerBackendLimit() int {
	if c.PerBackendLimit <= 0 {
		return 100
	}
	return c.PerBackendLimit
}

func (t *TimeoutConfig) GetConnectTimeout() time.Duration {
	if t.ConnectTimeout <= 0 {
		return 10 * time.Second
	}
	return t.ConnectTimeout
}

func (t *TimeoutConfig) GetReadTimeout() time.Duration {
	if t.ReadTimeout <= 0 {
		return 60 * time.Second
	}
	return t.ReadTimeout
}

func (t *TimeoutConfig) GetWriteTimeout() time.Duration {
	if t.WriteTimeout <= 0 {
		return 60 * time.Second
	}
	return t.WriteTimeout
}

func (t *TimeoutConfig) GetTotalTimeout() time.Duration {
	if t.TotalTimeout <= 0 {
		return 10 * time.Minute
	}
	return t.TotalTimeout
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
	stat, err := os.Stat(cm.configPath)
	if err != nil {
		return err
	}
	cm.config = &cfg
	cm.lastMod = stat.ModTime()
	return nil
}

func (cm *ConfigManager) Get() *Config {
	cm.mu.RLock()
	cfg := cm.config
	cm.mu.RUnlock()

	stat, err := os.Stat(cm.configPath)
	if err != nil {
		return cfg
	}
	if stat.ModTime().Equal(cm.lastMod) {
		return cfg
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()
	if stat, err = os.Stat(cm.configPath); err != nil {
		SystemSugar.Warnw("检查配置文件失败，继续使用旧配置", "error", err)
		return cm.config
	}
	if stat.ModTime().Equal(cm.lastMod) {
		return cm.config
	}
	if err := cm.tryReload(); err != nil {
		SystemSugar.Warnw("配置重载失败，继续使用旧配置", "error", err)
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
	stat, err := os.Stat(cm.configPath)
	if err != nil {
		return err
	}

	// 检测日志配置是否变化，需要重新初始化日志器
	oldCfg := cm.config
	loggingChanged := oldCfg == nil || loggingConfigChanged(&oldCfg.Logging, &cfg.Logging)

	cm.config = &cfg
	cm.lastMod = stat.ModTime()

	if loggingChanged {
		if err := reinitializeLoggers(&cfg); err != nil {
			SystemSugar.Warnw("日志配置重载失败", "error", err)
		} else {
			SystemSugar.Info("日志配置已重载")
		}
	}

	SystemSugar.Info("配置重载成功")
	return nil
}

// loggingConfigChanged 检测日志配置是否发生变化
func loggingConfigChanged(old, new *Logging) bool {
	if old.Level != new.Level || old.ConsoleLevel != new.ConsoleLevel {
		return true
	}
	if old.DebugMode != new.DebugMode {
		return true
	}
	if old.GetColorize() != new.GetColorize() {
		return true
	}
	if old.ConsoleStyle != new.ConsoleStyle || old.ConsoleFormat != new.ConsoleFormat {
		return true
	}
	if old.Format != new.Format || old.BaseDir != new.BaseDir {
		return true
	}
	if old.ShouldMaskSensitive() != new.ShouldMaskSensitive() {
		return true
	}
	if old.ShouldUseDetailedMasking() != new.ShouldUseDetailedMasking() {
		return true
	}
	if old.MaxFileSizeMB != new.MaxFileSizeMB || old.MaxAgeDays != new.MaxAgeDays {
		return true
	}
	if old.MaxBackups != new.MaxBackups || old.Compress != new.Compress {
		return true
	}
	return false
}

// reinitializeLoggers 重新初始化日志器
func reinitializeLoggers(cfg *Config) error {
	// 先关闭旧的日志器
	ShutdownLoggers()
	// 重新初始化
	return InitLoggers(cfg)
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

func ValidateConfig(cfg *Config) []error {
	var errors []error

	if cfg.Listen == "" {
		errors = append(errors, fmt.Errorf("listen 地址不能为空"))
	}

	if len(cfg.Backends) == 0 {
		errors = append(errors, fmt.Errorf("必须配置至少一个后端"))
	}

	backendNames := make(map[string]bool)
	for i, backend := range cfg.Backends {
		if backend.Name == "" {
			errors = append(errors, fmt.Errorf("后端 #%d 缺少名称", i+1))
		} else {
			if backendNames[backend.Name] {
				errors = append(errors, fmt.Errorf("后端名称重复: %s", backend.Name))
			}
			backendNames[backend.Name] = true
		}

		if backend.URL == "" {
			errors = append(errors, fmt.Errorf("后端 %s 缺少 URL", backend.Name))
		} else {
			if _, err := url.ParseRequestURI(backend.URL); err != nil {
				errors = append(errors, fmt.Errorf("后端 %s URL 格式无效: %v", backend.Name, err))
			}
		}
	}

	for alias, modelAlias := range cfg.Models {
		if modelAlias == nil {
			continue
		}
		for i, route := range modelAlias.Routes {
			if route.Backend == "" {
				errors = append(errors, fmt.Errorf("模型别名 %s 的路由 #%d 缺少后端引用", alias, i+1))
			} else if !backendNames[route.Backend] {
				errors = append(errors, fmt.Errorf("模型别名 %s 引用的后端 %s 不存在", alias, route.Backend))
			}
			if route.Model == "" {
				errors = append(errors, fmt.Errorf("模型别名 %s 的路由 #%d 缺少模型名称", alias, i+1))
			}
		}
	}

	if cfg.Fallback.CooldownSeconds <= 0 {
		cfg.Fallback.CooldownSeconds = 60
	}
	if cfg.Fallback.MaxRetries <= 0 {
		cfg.Fallback.MaxRetries = 3
	}

	if err := validateTimeouts(cfg); err != nil {
		errors = append(errors, err)
	}
	if err := validateNumericRanges(cfg); err != nil {
		errors = append(errors, err)
	}
	if err := validateURLSchemes(cfg); err != nil {
		errors = append(errors, err)
	}
	if err := validateRateLimitConfig(cfg); err != nil {
		errors = append(errors, err)
	}
	if err := validateConcurrencyConfig(cfg); err != nil {
		errors = append(errors, err)
	}

	return errors
}

func validateTimeouts(cfg *Config) error {
	if cfg.Timeout.ConnectTimeout < 1*time.Second {
		return fmt.Errorf("connect_timeout 太短: %v (最小 1秒)", cfg.Timeout.ConnectTimeout)
	}
	if cfg.Timeout.ConnectTimeout > 5*time.Minute {
		return fmt.Errorf("connect_timeout 太长: %v (最大 5分钟)", cfg.Timeout.ConnectTimeout)
	}
	if cfg.Timeout.ReadTimeout < 1*time.Second {
		return fmt.Errorf("read_timeout 太短: %v (最小 1秒)", cfg.Timeout.ReadTimeout)
	}
	if cfg.Timeout.ReadTimeout > 10*time.Minute {
		return fmt.Errorf("read_timeout 太长: %v (最大 10分钟)", cfg.Timeout.ReadTimeout)
	}
	if cfg.Timeout.TotalTimeout < 1*time.Second {
		return fmt.Errorf("total_timeout 太短: %v (最小 1秒)", cfg.Timeout.TotalTimeout)
	}
	if cfg.Timeout.TotalTimeout > 30*time.Minute {
		return fmt.Errorf("total_timeout 太长: %v (最大 30分钟)", cfg.Timeout.TotalTimeout)
	}
	return nil
}

func validateNumericRanges(cfg *Config) error {
	if cfg.RateLimit.Enabled {
		if cfg.RateLimit.GlobalRPS > 10000 {
			return fmt.Errorf("global_rps 过大: %v (最大 10000)", cfg.RateLimit.GlobalRPS)
		}
		if cfg.RateLimit.GlobalRPS < 1 {
			return fmt.Errorf("global_rps 过小: %v (最小 1)", cfg.RateLimit.GlobalRPS)
		}
		if cfg.RateLimit.PerIPRPS > 1000 {
			return fmt.Errorf("per_ip_rps 过大: %v (最大 1000)", cfg.RateLimit.PerIPRPS)
		}
		if cfg.RateLimit.BurstFactor < 1 || cfg.RateLimit.BurstFactor > 3 {
			return fmt.Errorf("burst_factor 范围无效: %v (应在 1-3 之间)", cfg.RateLimit.BurstFactor)
		}
	}
	if cfg.Concurrency.Enabled {
		if cfg.Concurrency.MaxRequests > 10000 {
			return fmt.Errorf("max_requests 过大: %v (最大 10000)", cfg.Concurrency.MaxRequests)
		}
		if cfg.Concurrency.MaxRequests < 1 {
			return fmt.Errorf("max_requests 过小: %v (最小 1)", cfg.Concurrency.MaxRequests)
		}
		if cfg.Concurrency.MaxQueueSize > 100000 {
			return fmt.Errorf("max_queue_size 过大: %v (最大 100000)", cfg.Concurrency.MaxQueueSize)
		}
		if cfg.Concurrency.QueueTimeout < 1*time.Second || cfg.Concurrency.QueueTimeout > 5*time.Minute {
			return fmt.Errorf("queue_timeout 范围无效: %v (应在 1秒-5分钟之间)", cfg.Concurrency.QueueTimeout)
		}
	}
	return nil
}

func validateURLSchemes(cfg *Config) error {
	allowedSchemes := map[string]bool{"http": true, "https": true}
	for _, backend := range cfg.Backends {
		u, err := url.Parse(backend.URL)
		if err != nil {
			return fmt.Errorf("后端 %s URL 解析失败: %v", backend.Name, err)
		}
		if !allowedSchemes[u.Scheme] {
			return fmt.Errorf("后端 %s URL scheme 不允许: %s (仅允许 http/https)", backend.Name, u.Scheme)
		}
	}
	return nil
}

func validateRateLimitConfig(cfg *Config) error {
	if !cfg.RateLimit.Enabled {
		return nil
	}
	for model, rps := range cfg.RateLimit.PerModelRPS {
		if rps > 5000 {
			return fmt.Errorf("模型 %s 的 per_model_rps 过大: %v (最大 5000)", model, rps)
		}
		if rps < 0.1 {
			return fmt.Errorf("模型 %s 的 per_model_rps 过小: %v (最小 0.1)", model, rps)
		}
	}
	return nil
}

func validateConcurrencyConfig(cfg *Config) error {
	if !cfg.Concurrency.Enabled {
		return nil
	}
	if cfg.Concurrency.PerBackendLimit < 1 {
		return fmt.Errorf("per_backend_limit 过小: %v (最小 1)", cfg.Concurrency.PerBackendLimit)
	}
	if cfg.Concurrency.PerBackendLimit > 1000 {
		return fmt.Errorf("per_backend_limit 过大: %v (最大 1000)", cfg.Concurrency.PerBackendLimit)
	}
	return nil
}
