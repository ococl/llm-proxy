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
	Level            string
	ConsoleLevel     string
	BaseDir          string
	EnableMetrics    bool
	MaxFileSizeMB    int
	MaxAgeDays       int
	MaxBackups       int
	Format           string
	Colorize         bool
	ConsoleFormat    string
	DebugMode        bool
	SeparateFiles    bool
	RequestDir       string
	ErrorDir         string
	MaskSensitive    bool
	BufferSize       int
	FlushInterval    int
	DropOnFull       bool
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

type Request struct {
	Model            string
	Messages         []Message
	MaxTokens        int
	Temperature      float64
	TopP             float64
	Stream           bool
	Stop             []string
	Tools            []Tool
	ToolChoice       any
	User             string
	StreamHandler    func(chunk []byte) error
}

type Message struct {
	Role       string
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
}

type Tool struct {
	Type     string
	Function ToolFunction
}

type ToolFunction struct {
	Name        string
	Description string
	Parameters  map[string]any
}

type ToolCall struct {
	ID       string
	Type     string
	Function ToolCallFunction
}

type ToolCallFunction struct {
	Name      string
	Arguments string
}

type Response struct {
	ID      string
	Object  string
	Created int64
	Model   string
	Choices []Choice
	Usage   Usage
}

type Choice struct {
	Index        int
	Message      Message
	FinishReason string
	Delta        *Message
}

type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type BackendClient interface {
	Send(ctx context.Context, req *Request) (*Response, error)
	GetHTTPClient() *http.Client
}

type ProtocolConverter interface {
	ToBackend(req *Request, protocol types.Protocol) (*Request, error)
	FromBackend(resp *Response, protocol types.Protocol) (*Response, error)
}

type RequestValidator interface {
	Validate(req *Request) error
}
