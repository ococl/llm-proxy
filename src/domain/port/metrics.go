package port

import (
	"context"
	"net/http"
	"time"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/types"
)

type Config struct {
	Listen        string
	ProxyAPIKey   string
	Proxy         ProxyConfig
	Backends      []*entity.Backend
	Models        map[string]*ModelAlias
	Fallback      FallbackConfig
	ErrorFallback ErrorFallbackConfig
	Logging       LoggingConfig
	Timeout       TimeoutConfig
	RateLimit     RateLimitConfig
	Concurrency   ConcurrencyConfig
}

type ProxyConfig struct {
	EnableSystemPrompt bool
	ForwardClientIP    bool
	CustomVariables    map[string]string
}

func (p *ProxyConfig) GetCustomVariables() map[string]string {
	if p.CustomVariables == nil {
		return make(map[string]string)
	}
	return p.CustomVariables
}

type FallbackConfig struct {
	CooldownSeconds       int
	MaxRetries            int
	AliasFallback         map[string][]string
	EnableBackoff         bool
	BackoffInitialDelay   int
	BackoffMaxDelay       int
	BackoffMultiplier     float64
	BackoffJitter         float64
	EnableCircuitBreaker  bool
	CircuitFailureThresh  int
	CircuitSuccessThresh  int
	CircuitOpenTimeoutSec int
}

// ErrorFallbackConfig 定义错误回退策略配置
type ErrorFallbackConfig struct {
	ServerError ServerErrorConfig
	ClientError ClientErrorConfig
}

// ServerErrorConfig 服务器错误回退配置
type ServerErrorConfig struct {
	Enabled bool
}

// ClientErrorConfig 客户端错误回退配置
type ClientErrorConfig struct {
	Enabled     bool
	StatusCodes []int
	Patterns    []string
}

type LoggingConfig struct {
	BaseDir       string
	MaskSensitive bool
	Async         AsyncConfig
	Rotation      RotationConfig
	Categories    map[string]CategoryConfig
}

func (l *LoggingConfig) GetBaseDir() string {
	if l.BaseDir == "" {
		return "./logs"
	}
	return l.BaseDir
}

type AsyncConfig struct {
	Enabled              bool
	BufferSize           int
	FlushIntervalSeconds int
	DropOnFull           bool
}

type RotationConfig struct {
	MaxSizeMB    int
	TimeStrategy string
	MaxAgeDays   int
	MaxBackups   int
	Compress     bool
}

type CategoryConfig struct {
	Level       string
	Target      string
	Path        string
	MaxSizeMB   int
	MaxAgeDays  int
	Compress    bool
	IncludeBody *bool
}

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

type TimeoutConfig struct {
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	TotalTimeout   time.Duration
}

type RateLimitConfig struct {
	Enabled     bool
	GlobalRPS   float64
	PerIPRPS    float64
	PerModelRPS map[string]float64
	BurstFactor float64
}

type ConcurrencyConfig struct {
	Enabled         bool
	MaxRequests     int
	MaxQueueSize    int
	QueueTimeout    time.Duration
	PerBackendLimit int
}

type BackendClient interface {
	Send(ctx context.Context, req *entity.Request, backend *entity.Backend, backendModel string) (*entity.Response, error)
	SendStreaming(ctx context.Context, req *entity.Request, backend *entity.Backend, backendModel string, handler func([]byte) error) error
	SendStreamingPassthrough(ctx context.Context, req *entity.Request, backend *entity.Backend, backendModel string) (*http.Response, error)
	GetHTTPClient() *http.Client
}

type ProtocolConverter interface {
	ToBackend(req *entity.Request, protocol types.Protocol) (*entity.Request, error)
	FromBackend(respBody []byte, model string, protocol types.Protocol) (*entity.Response, error)
}

type RequestValidator interface {
	Validate(req *entity.Request) error
}
