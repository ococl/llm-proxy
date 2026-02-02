package config

import (
	"fmt"
	"net/url"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Backend struct {
	Name     string `yaml:"name"`
	URL      string `yaml:"url"`
	APIKey   string `yaml:"api_key,omitempty"`
	Enabled  *bool  `yaml:"enabled,omitempty"`
	Protocol string `yaml:"protocol,omitempty"` // "openai" or "anthropic", default: "openai"
	Locale   string `yaml:"locale,omitempty"`   // 接受语言设置，用于 Accept-Language header
}

func (b *Backend) IsEnabled() bool {
	return b.Enabled == nil || *b.Enabled
}

func (b *Backend) GetProtocol() string {
	if b.Protocol == "" {
		return "openai"
	}
	return b.Protocol
}

type ModelRoute struct {
	Backend  string `yaml:"backend"`
	Model    string `yaml:"model"`
	Priority int    `yaml:"priority"`
	Enabled  *bool  `yaml:"enabled,omitempty"`
	Protocol string `yaml:"protocol,omitempty"` // "openai" or "anthropic", overrides backend protocol
}

func (r *ModelRoute) IsEnabled() bool {
	return r.Enabled == nil || *r.Enabled
}

func (r *ModelRoute) GetProtocol(backendProtocol string) string {
	if r.Protocol != "" {
		return r.Protocol
	}
	return backendProtocol
}

type ModelAlias struct {
	Enabled *bool        `yaml:"enabled,omitempty"`
	Routes  []ModelRoute `yaml:"routes"`
}

func (m *ModelAlias) IsEnabled() bool {
	return m.Enabled == nil || *m.Enabled
}

type Fallback struct {
	CooldownSeconds       int                 `yaml:"cooldown_seconds"`
	MaxRetries            int                 `yaml:"max_retries"`
	AliasFallback         map[string][]string `yaml:"alias_fallback,omitempty"`
	EnableBackoff         *bool               `yaml:"enable_backoff,omitempty"`
	BackoffInitialDelay   int                 `yaml:"backoff_initial_delay,omitempty"`
	BackoffMaxDelay       int                 `yaml:"backoff_max_delay,omitempty"`
	BackoffMultiplier     float64             `yaml:"backoff_multiplier,omitempty"`
	BackoffJitter         float64             `yaml:"backoff_jitter,omitempty"`
	EnableCircuitBreaker  *bool               `yaml:"enable_circuit_breaker,omitempty"`
	CircuitFailureThresh  int                 `yaml:"circuit_failure_threshold,omitempty"`
	CircuitSuccessThresh  int                 `yaml:"circuit_success_threshold,omitempty"`
	CircuitOpenTimeoutSec int                 `yaml:"circuit_open_timeout,omitempty"`
}

func (f *Fallback) IsBackoffEnabled() bool {
	return f.EnableBackoff != nil && *f.EnableBackoff
}

func (f *Fallback) GetBackoffInitialDelay() int {
	if f.BackoffInitialDelay <= 0 {
		return 100
	}
	return f.BackoffInitialDelay
}

func (f *Fallback) GetBackoffMaxDelay() int {
	if f.BackoffMaxDelay <= 0 {
		return 5000
	}
	return f.BackoffMaxDelay
}

func (f *Fallback) GetBackoffMultiplier() float64 {
	if f.BackoffMultiplier <= 0 {
		return 2.0
	}
	return f.BackoffMultiplier
}

func (f *Fallback) GetBackoffJitter() float64 {
	if f.BackoffJitter < 0 || f.BackoffJitter > 1 {
		return 0.1
	}
	return f.BackoffJitter
}

func (f *Fallback) IsCircuitBreakerEnabled() bool {
	return f.EnableCircuitBreaker != nil && *f.EnableCircuitBreaker
}

func (f *Fallback) GetCircuitFailureThreshold() int {
	if f.CircuitFailureThresh <= 0 {
		return 5
	}
	return f.CircuitFailureThresh
}

func (f *Fallback) GetCircuitSuccessThreshold() int {
	if f.CircuitSuccessThresh <= 0 {
		return 2
	}
	return f.CircuitSuccessThresh
}

func (f *Fallback) GetCircuitOpenTimeout() int {
	if f.CircuitOpenTimeoutSec <= 0 {
		return 60
	}
	return f.CircuitOpenTimeoutSec
}

// ErrorFallbackConfig 错误回退策略配置
// 定义不同类型错误的回退规则，帮助系统在遇到特定错误时自动切换后端
type ErrorFallbackConfig struct {
	// ServerError 配置服务器错误（5xx）的回退策略
	ServerError ServerErrorConfig `yaml:"server_error"`
	// ClientError 配置客户端错误（4xx）的回退策略
	ClientError ClientErrorConfig `yaml:"client_error"`
}

// ServerErrorConfig 服务器错误回退配置
// 5xx 错误通常是上游服务暂时不可用，应立即回退到其他后端
type ServerErrorConfig struct {
	// Enabled 是否启用服务器错误回退
	// 启用后，5xx 错误会立即触发回退（不重试当前后端）
	Enabled bool `yaml:"enabled"`
}

// ClientErrorConfig 客户端错误回退配置
// 4xx 错误通常是请求问题，但某些状态码或错误消息需要回退
type ClientErrorConfig struct {
	// Enabled 是否启用客户端错误回退
	Enabled bool `yaml:"enabled"`
	// StatusCodes 需要回退的 HTTP 状态码列表
	// 例如 [401, 403, 429] 表示遇到这些状态码时立即回退
	StatusCodes []int `yaml:"codes"`
	// Patterns 错误消息中包含的关键词（不区分大小写）
	// 例如 ["insufficient_quota", "rate_limit", "billing"]
	// 匹配到这些关键词时会触发回退
	Patterns []string `yaml:"patterns"`
}

// Logging 精简版日志配置
type Logging struct {
	BaseDir       string                    `yaml:"base_dir"`
	MaskSensitive bool                      `yaml:"mask_sensitive"`
	Async         AsyncConfig               `yaml:"async"`
	Rotation      RotationConfig            `yaml:"rotation"`
	Categories    map[string]CategoryConfig `yaml:"categories"`
}

// AsyncConfig 异步日志配置
type AsyncConfig struct {
	Enabled              bool `yaml:"enabled"`
	BufferSize           int  `yaml:"buffer_size"`
	FlushIntervalSeconds int  `yaml:"flush_interval_seconds"`
	DropOnFull           bool `yaml:"drop_on_full"`
}

// RotationConfig 日志轮转配置
type RotationConfig struct {
	MaxSizeMB    int    `yaml:"max_size_mb"`
	TimeStrategy string `yaml:"time_strategy"` // daily/hourly/none
	MaxAgeDays   int    `yaml:"max_age_days"`
	MaxBackups   int    `yaml:"max_backups"`
	Compress     bool   `yaml:"compress"`
}

// CategoryConfig 分类日志配置
type CategoryConfig struct {
	Level       string `yaml:"level"`       // debug/info/warn/error/none
	Target      string `yaml:"target"`      // console/file/both/none
	Path        string `yaml:"path"`        // 文件路径
	MaxSizeMB   int    `yaml:"max_size_mb"` // 覆盖全局配置
	MaxAgeDays  int    `yaml:"max_age_days"`
	Compress    bool   `yaml:"compress"`
	IncludeBody *bool  `yaml:"include_body,omitempty"` // 仅 request_body 使用
}

// Logging 配置 getter 方法

func (l *Logging) GetBaseDir() string {
	if l.BaseDir == "" {
		return "./logs"
	}
	return l.BaseDir
}

func (l *Logging) IsAsyncEnabled() bool {
	return l.Async.Enabled
}

func (l *Logging) GetAsyncBufferSize() int {
	if l.Async.BufferSize <= 0 {
		return 10000
	}
	return l.Async.BufferSize
}

func (l *Logging) GetAsyncFlushInterval() int {
	if l.Async.FlushIntervalSeconds <= 0 {
		return 5
	}
	return l.Async.FlushIntervalSeconds
}

func (l *Logging) ShouldDropOnFull() bool {
	return l.Async.DropOnFull
}

func (l *Logging) GetRotationMaxSizeMB() int {
	if l.Rotation.MaxSizeMB <= 0 {
		return 100
	}
	return l.Rotation.MaxSizeMB
}

func (l *Logging) GetRotationTimeStrategy() string {
	if l.Rotation.TimeStrategy == "" {
		return "daily"
	}
	return l.Rotation.TimeStrategy
}

func (l *Logging) GetRotationMaxAgeDays() int {
	if l.Rotation.MaxAgeDays <= 0 {
		return 7
	}
	return l.Rotation.MaxAgeDays
}

func (l *Logging) GetRotationMaxBackups() int {
	if l.Rotation.MaxBackups <= 0 {
		return 21
	}
	return l.Rotation.MaxBackups
}

func (l *Logging) ShouldRotateCompress() bool {
	return l.Rotation.Compress
}

func (l *Logging) GetCategoryConfig(name string) (CategoryConfig, bool) {
	cfg, ok := l.Categories[name]
	return cfg, ok
}

// CategoryConfig 辅助方法

func (c *CategoryConfig) GetLevel() string {
	if c.Level == "" {
		return "info"
	}
	return c.Level
}

func (c *CategoryConfig) GetTarget() string {
	if c.Target == "" {
		return "both"
	}
	return c.Target
}

func (c *CategoryConfig) GetMaxSizeMB(defaultValue int) int {
	if c.MaxSizeMB <= 0 {
		return defaultValue
	}
	return c.MaxSizeMB
}

func (c *CategoryConfig) GetMaxAgeDays(defaultValue int) int {
	if c.MaxAgeDays <= 0 {
		return defaultValue
	}
	return c.MaxAgeDays
}

func (c *CategoryConfig) ShouldCompress(defaultValue bool) bool {
	// 零值表示使用默认值
	if !c.Compress && defaultValue {
		return defaultValue
	}
	return c.Compress
}

func (c *CategoryConfig) ShouldIncludeBody() bool {
	return c.IncludeBody == nil || *c.IncludeBody
}

type Config struct {
	Listen        string                 `yaml:"listen"`
	ProxyAPIKey   string                 `yaml:"proxy_api_key"`
	Proxy         ProxyConfig            `yaml:"proxy"`
	Backends      []Backend              `yaml:"backends"`
	Models        map[string]*ModelAlias `yaml:"models"`
	Fallback      Fallback               `yaml:"fallback"`
	ErrorFallback ErrorFallbackConfig    `yaml:"error_fallback"`
	Logging       Logging                `yaml:"logging"`
	Timeout       TimeoutConfig          `yaml:"timeout"`
	RateLimit     RateLimitConfig        `yaml:"rate_limit"`
	Concurrency   ConcurrencyConfig      `yaml:"concurrency"`
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

type ProxyConfig struct {
	EnableSystemPrompt bool                `yaml:"enable_system_prompt"`
	ForwardClientIP    *bool               `yaml:"forward_client_ip"`
	SystemPrompt       *SystemPromptConfig `yaml:"system_prompt,omitempty"`
}

// SystemPromptConfig 系统提示词配置
type SystemPromptConfig struct {
	// CustomVariables 自定义变量，用于覆盖内置变量的默认值
	// 例如：proxy 在 Linux 服务器，但客户端用户使用 Windows
	// 可以配置 { "_OS": "windows" } 来覆盖 ${_OS} 的值
	CustomVariables map[string]string `yaml:"custom_variables,omitempty"`
}

func (p *ProxyConfig) GetEnableSystemPrompt() bool {
	return p.EnableSystemPrompt
}

func (p *ProxyConfig) GetForwardClientIP() bool {
	return p.ForwardClientIP == nil || *p.ForwardClientIP
}

func (p *ProxyConfig) GetCustomVariables() map[string]string {
	if p.SystemPrompt == nil || p.SystemPrompt.CustomVariables == nil {
		return make(map[string]string)
	}
	return p.SystemPrompt.CustomVariables
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

// LoggingConfigChangedFunc is a callback for logging config changes
var LoggingConfigChangedFunc func(*Config) error

type Manager struct {
	config     *Config
	configPath string
	lastMod    time.Time
	mu         sync.RWMutex
	notifyChan chan struct{}
	stopChan   chan struct{}
}

func (cm *Manager) SetConfigForTest(cfg *Config) {
	cm.config = cfg
}

func NewManager(path string) (*Manager, error) {
	cm := &Manager{configPath: path}
	if err := cm.load(); err != nil {
		return nil, err
	}
	return cm, nil
}

func (cm *Manager) load() error {
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

func (cm *Manager) Get() *Config {
	cm.mu.RLock()
	cfg := cm.config
	cm.mu.RUnlock()

	stat, err := os.Stat(cm.configPath)
	if err != nil {
		return cfg
	}
	// 使用 After 而不是 Equal，兼容不同文件系统的时间精度
	if !stat.ModTime().After(cm.lastMod) {
		return cfg
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()
	if stat, err = os.Stat(cm.configPath); err != nil {
		return cm.config
	}
	if !stat.ModTime().After(cm.lastMod) {
		return cm.config
	}
	// 直接调用 tryReloadLocked，因为已经持有锁
	cm.tryReloadLocked()
	return cm.config
}

func (cm *Manager) tryReload() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.tryReloadLocked()
}

func (cm *Manager) tryReloadLocked() {
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return
	}
	stat, err := os.Stat(cm.configPath)
	if err != nil {
		return
	}

	oldCfg := cm.config
	loggingChanged := oldCfg == nil || loggingConfigChanged(&oldCfg.Logging, &cfg.Logging)

	cm.config = &cfg
	cm.lastMod = stat.ModTime()

	// 通知配置已变更
	if cm.notifyChan != nil {
		select {
		case cm.notifyChan <- struct{}{}:
		default:
			// 通道已满，跳过
		}
	}

	if loggingChanged && LoggingConfigChangedFunc != nil {
		LoggingConfigChangedFunc(&cfg)
	}
}

// Watch 启动配置文件的监控 goroutine。
// 返回一个通道,当配置文件发生变更时会发送信号。
// 调用 StopWatch() 来停止监控。
func (cm *Manager) Watch() <-chan struct{} {
	cm.notifyChan = make(chan struct{}, 1)
	cm.stopChan = make(chan struct{}, 1)

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-cm.stopChan:
				return
			case <-ticker.C:
				cm.mu.Lock()
				cm.tryReloadLocked()
				cm.mu.Unlock()
			}
		}
	}()

	return cm.notifyChan
}

// StopWatch 停止配置文件的监控。
func (cm *Manager) StopWatch() {
	if cm.stopChan != nil {
		cm.stopChan <- struct{}{}
	}
}

func loggingConfigChanged(old, new *Logging) bool {
	if old.BaseDir != new.BaseDir {
		return true
	}
	if old.MaskSensitive != new.MaskSensitive {
		return true
	}
	if old.Async != new.Async {
		return true
	}
	if old.Rotation != new.Rotation {
		return true
	}
	if len(old.Categories) != len(new.Categories) {
		return true
	}
	for name, oldCfg := range old.Categories {
		newCfg, ok := new.Categories[name]
		if !ok || oldCfg != newCfg {
			return true
		}
	}
	return false
}

func (cm *Manager) GetBackend(name string) *Backend {
	cfg := cm.Get()
	for i := range cfg.Backends {
		if cfg.Backends[i].Name == name {
			return &cfg.Backends[i]
		}
	}
	return nil
}

func Validate(cfg *Config) []error {
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

	return errors
}
