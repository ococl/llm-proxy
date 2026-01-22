package port

import (
	"context"
	"net/http"
	"time"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/types"
)

type Config struct {
	Listen      string
	ProxyAPIKey string
	Proxy       ProxyConfig
	Backends    []*entity.Backend
	Models      map[string]*ModelAlias
	Fallback    FallbackConfig
	Detection   DetectionConfig
	Logging     LoggingConfig
	Timeout     TimeoutConfig
	RateLimit   RateLimitConfig
	Concurrency ConcurrencyConfig
}

type ProxyConfig struct {
	EnableSystemPrompt bool
	ForwardClientIP    bool
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

type DetectionConfig struct {
	ErrorCodes    []string
	ErrorPatterns []string
}

type LoggingConfig struct {
	Level         string
	ConsoleLevel  string
	BaseDir       string
	EnableMetrics bool
	MaxFileSizeMB int
	MaxAgeDays    int
	MaxBackups    int
	Format        string
	Colorize      bool
	ConsoleFormat string
	DebugMode     bool
	SeparateFiles bool
	RequestDir    string
	ErrorDir      string
	MaskSensitive bool
	BufferSize    int
	FlushInterval int
	DropOnFull    bool
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
	Send(ctx context.Context, req *entity.Request, backend *entity.Backend) (*entity.Response, error)
	GetHTTPClient() *http.Client
}

type ProtocolConverter interface {
	ToBackend(req *entity.Request, protocol types.Protocol) (*entity.Request, error)
	FromBackend(resp *entity.Response, protocol types.Protocol) (*entity.Response, error)
}

type RequestValidator interface {
	Validate(req *entity.Request) error
}
