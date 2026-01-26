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

type Detection struct {
	ErrorCodes    []string `yaml:"error_codes"`
	ErrorPatterns []string `yaml:"error_patterns"`
}

type Logging struct {
	Level             string `yaml:"level"`
	ConsoleLevel      string `yaml:"console_level"`
	BaseDir           string `yaml:"base_dir"`
	RequestDir        string `yaml:"request_dir"`
	ErrorDir          string `yaml:"error_dir"`
	GeneralFile       string `yaml:"general_file"`
	SeparateFiles     bool   `yaml:"separate_files"`
	MaskSensitive     *bool  `yaml:"mask_sensitive,omitempty"`
	EnableMetrics     bool   `yaml:"enable_metrics"`
	MaxFileSizeMB     int    `yaml:"max_file_size_mb"`
	MaxAgeDays        int    `yaml:"max_age_days,omitempty"`
	MaxBackups        int    `yaml:"max_backups,omitempty"`
	Compress          bool   `yaml:"compress,omitempty"`
	Format            string `yaml:"format,omitempty"`
	Colorize          *bool  `yaml:"colorize,omitempty"`
	ConsoleStyle      string `yaml:"console_style,omitempty"`
	ConsoleFormat     string `yaml:"console_format,omitempty"`
	DebugMode         bool   `yaml:"debug_mode,omitempty"`
	Async             bool   `yaml:"async"`
	BufferSize        int    `yaml:"buffer_size"`
	FlushInterval     int    `yaml:"flush_interval,omitempty"`
	DropOnFull        bool   `yaml:"drop_on_full"`
	RotateBySize      bool   `yaml:"rotate_by_size,omitempty"`
	RotateByTime      bool   `yaml:"rotate_by_time,omitempty"`
	TimeRotation      string `yaml:"time_rotation,omitempty"`
	DetailedMasking   *bool  `yaml:"detailed_masking,omitempty"`
	MaxLogContentSize int    `yaml:"max_log_content_size,omitempty"` // 最大日志内容大小(字节),0表示不限制

	// 新增：多目标日志配置
	EnableMultiTarget bool                      `yaml:"enable_multi_target"` // 启用多目标日志
	Console           ConsoleConfig             `yaml:"console"`             // 控制台配置
	File              FileConfig                `yaml:"file"`                // 文件配置
	Categories        map[string]CategoryConfig `yaml:"categories"`          // 分类配置
	RequestBody       RequestBodyConfig         `yaml:"request_body"`        // 请求体日志配置
}

// ConsoleConfig 控制台输出配置
type ConsoleConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Level    string `yaml:"level"`
	Style    string `yaml:"style"`  // compact/verbose
	Format   string `yaml:"format"` // markdown/plain
	Colorize bool   `yaml:"colorize"`
}

// FileConfig 文件输出配置
type FileConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Level      string `yaml:"level"`
	BaseDir    string `yaml:"base_dir"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxAgeDays int    `yaml:"max_age_days"`
	MaxBackups int    `yaml:"max_backups"`
	Compress   bool   `yaml:"compress"`
	Format     string `yaml:"format"` // json/text
}

// CategoryConfig 分类日志配置
type CategoryConfig struct {
	Target   string       `yaml:"target"`   // 输出目标: console/file/both/none
	Levels   LogLevelInfo `yaml:"levels"`   // 日志级别配置
	Path     string       `yaml:"path"`     // 文件路径
	MaxSize  int          `yaml:"max_size"` // 最大文件大小(MB)
	MaxAge   int          `yaml:"max_age"`  // 最大保留天数
	Compress bool         `yaml:"compress"` // 是否压缩
}

// RequestBodyConfig 请求体日志专用配置
type RequestBodyConfig struct {
	Enabled     bool   `yaml:"enabled"`      // 是否启用请求体日志
	BaseDir     string `yaml:"base_dir"`     // 基础目录
	MaxSizeMB   int    `yaml:"max_size_mb"`  // 单文件最大大小(MB)
	MaxAgeDays  int    `yaml:"max_age_days"` // 保留天数
	MaxBackups  int    `yaml:"max_backups"`  // 备份数量
	Compress    bool   `yaml:"compress"`     // 是否压缩
	IncludeBody *bool  `yaml:"include_body"`
}

// LogLevelInfo 日志级别信息
type LogLevelInfo struct {
	Console string `yaml:"console"` // 控制台日志级别
	File    string `yaml:"file"`    // 文件日志级别
}

func (l *Logging) ShouldMaskSensitive() bool {
	return l.MaskSensitive == nil || *l.MaskSensitive
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

func (l *Logging) GetMaxLogContentSize() int {
	if l.MaxLogContentSize <= 0 {
		return 0
	}
	return l.MaxLogContentSize
}

// RequestBodyConfig Getter 方法
func (r *RequestBodyConfig) IsEnabled() bool {
	// 如果所有配置字段都是零值（配置文件中未定义request_body段），则默认启用
	isEmptyConfig := r.BaseDir == "" &&
		r.MaxSizeMB == 0 &&
		r.MaxAgeDays == 0 &&
		r.MaxBackups == 0 &&
		!r.Compress &&
		r.IncludeBody == nil

	if isEmptyConfig {
		return true
	}
	return r.Enabled
}

func (r *RequestBodyConfig) GetBaseDir() string {
	if r.BaseDir == "" {
		return "./logs/request_body"
	}
	return r.BaseDir
}

func (r *RequestBodyConfig) GetMaxSizeMB() int {
	if r.MaxSizeMB <= 0 {
		return 200
	}
	return r.MaxSizeMB
}

func (r *RequestBodyConfig) GetMaxAgeDays() int {
	if r.MaxAgeDays <= 0 {
		return 14
	}
	return r.MaxAgeDays
}

func (r *RequestBodyConfig) GetMaxBackups() int {
	if r.MaxBackups <= 0 {
		return 10
	}
	return r.MaxBackups
}

func (r *RequestBodyConfig) ShouldCompress() bool {
	return r.Compress
}

func (r *RequestBodyConfig) ShouldIncludeBody() bool {
	if !r.IsEnabled() {
		return false
	}
	return r.IncludeBody == nil || *r.IncludeBody
}

type Config struct {
	Listen      string                 `yaml:"listen"`
	ProxyAPIKey string                 `yaml:"proxy_api_key"`
	Proxy       ProxyConfig            `yaml:"proxy"`
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

type ProxyConfig struct {
	EnableSystemPrompt bool  `yaml:"enable_system_prompt"`
	ForwardClientIP    *bool `yaml:"forward_client_ip"`
}

func (p *ProxyConfig) GetEnableSystemPrompt() bool {
	return p.EnableSystemPrompt
}

func (p *ProxyConfig) GetForwardClientIP() bool {
	return p.ForwardClientIP == nil || *p.ForwardClientIP
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
