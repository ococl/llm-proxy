package port

import (
	"context"
	"net/http"
	"time"
)

// Config represents the application configuration.
// This is a duplicate of config.Config for use in the domain layer.
// The actual implementation should map to this interface.
type Config struct {
	Listen      string
	ProxyAPIKey string
	Proxy       ProxyConfig
	Backends    []*Backend
	Models      map[string]*ModelAlias
	Fallback    FallbackConfig
	Detection   DetectionConfig
	Logging     LoggingConfig
	Timeout     TimeoutConfig
	RateLimit   RateLimitConfig
	Concurrency ConcurrencyConfig
}

// ProxyConfig represents proxy-specific configuration.
type ProxyConfig struct {
	EnableSystemPrompt bool
	ForwardClientIP    bool
}

// FallbackConfig represents fallback/retry configuration.
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

// DetectionConfig represents error detection configuration.
type DetectionConfig struct {
	ErrorCodes    []string
	ErrorPatterns []string
}

// LoggingConfig represents logging configuration.
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

// TimeoutConfig represents timeout configuration.
type TimeoutConfig struct {
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	TotalTimeout   time.Duration
}

// RateLimitConfig represents rate limit configuration.
type RateLimitConfig struct {
	Enabled     bool
	GlobalRPS   float64
	PerIPRPS    float64
	PerModelRPS map[string]float64
	BurstFactor float64
}

// ConcurrencyConfig represents concurrency configuration.
type ConcurrencyConfig struct {
	Enabled         bool
	MaxRequests     int
	MaxQueueSize    int
	QueueTimeout    time.Duration
	PerBackendLimit int
}

// BackendClient interface for making requests to backends.
type BackendClient interface {
	// Send sends a request to the given backend.
	Send(ctx context.Context, req *Request) (*Response, error)
	// GetHTTPClient returns the underlying HTTP client.
	GetHTTPClient() *http.Client
}

// Request represents a chat completion request.
type Request struct {
	Model         string
	Messages      []Message
	MaxTokens     int
	Temperature   float64
	TopP          float64
	Stream        bool
	Stop          []string
	Tools         []Tool
	ToolChoice    any
	User          string
	StreamHandler func(chunk []byte) error
}

// Message represents a chat message.
type Message struct {
	Role       string
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
}

// Tool represents a tool definition.
type Tool struct {
	Type     string
	Function ToolFunction
}

// ToolFunction represents a function tool.
type ToolFunction struct {
	Name        string
	Description string
	Parameters  map[string]any
}

// ToolCall represents a tool call from the model.
type ToolCall struct {
	ID       string
	Type     string
	Function ToolCallFunction
}

// ToolCallFunction represents a function tool call.
type ToolCallFunction struct {
	Name      string
	Arguments string
}

// Response represents a chat completion response.
type Response struct {
	ID            string
	Object        string
	Created       int64
	Model         string
	Choices       []Choice
	Usage         Usage
	StopReason    string
	StopSequences []string
}

// Choice represents a completion choice.
type Choice struct {
	Index        int
	Message      Message
	FinishReason string
	Delta        *Message
}

// Usage represents token usage.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// ResponseConverter interface for converting responses.
type ResponseConverter interface {
	// ToClient converts a backend response to client format.
	ToClient(resp *Response) (*Response, error)
}

// ProtocolConverter interface for converting requests/responses between protocols.
type ProtocolConverter interface {
	// ToBackend converts a client request to backend format.
	ToBackend(req *Request, protocol Protocol) (*Request, error)
	// FromBackend converts a backend response to client format.
	FromBackend(resp *Response, protocol Protocol) (*Response, error)
}

// RequestValidator interface for validating requests.
type RequestValidator interface {
	// Validate validates a request.
	Validate(req *Request) error
}

// RetryStrategy interface for retry logic.
type RetryStrategy interface {
	// ShouldRetry determines if a retry should be attempted.
	ShouldRetry(attempt int, lastErr error) bool
	// GetDelay returns the delay before the next retry.
	GetDelay(attempt int) time.Duration
	// GetMaxRetries returns the maximum number of retries.
	GetMaxRetries() int
}
