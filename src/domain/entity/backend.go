package entity

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"llm-proxy/domain/port"
)

// BackendID is a value object for backend identifier.
type BackendID string

// NewBackendID creates a new backend ID.
func NewBackendID(name string) BackendID {
	return BackendID(name)
}

// String returns the string representation.
func (id BackendID) String() string {
	return string(id)
}

// BackendURL is a value object for backend URL validation.
type BackendURL string

// NewBackendURL creates and validates a backend URL.
func NewBackendURL(rawURL string) (BackendURL, error) {
	// Add scheme if not present
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	_, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return "", fmt.Errorf("无效的 URL 格式: %w", err)
	}
	return BackendURL(rawURL), nil
}

// String returns the string representation.
func (u BackendURL) String() string {
	return string(u)
}

// GetBaseURL returns the base URL without trailing path.
func (u BackendURL) GetBaseURL() string {
	s := string(u)
	parsed, err := url.Parse(s)
	if err != nil {
		return s
	}
	// Remove any path components for base URL
	return fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
}

// APIKey is a value object for API key.
type APIKey string

// Masked returns the masked API key.
func (k APIKey) Masked() string {
	s := string(k)
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}

// IsEmpty returns true if the key is empty.
func (k APIKey) IsEmpty() bool {
	return string(k) == ""
}

// Backend represents an LLM backend service.
type Backend struct {
	id       BackendID
	name     string
	url      BackendURL
	apiKey   APIKey
	enabled  bool
	protocol port.Protocol
}

// NewBackend creates a new backend entity.
func NewBackend(name, urlStr, apiKey string, enabled bool, protocol port.Protocol) (*Backend, error) {
	id := NewBackendID(name)
	validURL, err := NewBackendURL(urlStr)
	if err != nil {
		return nil, err
	}
	return &Backend{
		id:       id,
		name:     name,
		url:      validURL,
		apiKey:   APIKey(apiKey),
		enabled:  enabled,
		protocol: protocol,
	}, nil
}

// ID returns the backend ID.
func (b *Backend) ID() BackendID {
	return b.id
}

// Name returns the backend name.
func (b *Backend) Name() string {
	return b.name
}

// URL returns the backend URL.
func (b *Backend) URL() BackendURL {
	return b.url
}

// APIKey returns the API key.
func (b *Backend) APIKey() APIKey {
	return b.apiKey
}

// IsEnabled returns true if the backend is enabled.
func (b *Backend) IsEnabled() bool {
	return b.enabled
}

// Protocol returns the API protocol.
func (b *Backend) Protocol() port.Protocol {
	if b.protocol == "" {
		return port.ProtocolOpenAI
	}
	return b.protocol
}

// IsHealthy returns true if the backend appears healthy.
func (b *Backend) IsHealthy() bool {
	return b.enabled && b.url != ""
}

// String returns a string representation.
func (b *Backend) String() string {
	return fmt.Sprintf("Backend(%s, %s, enabled=%v)", b.name, b.url, b.enabled)
}

// BackendOption is a function type for configuring a backend.
type BackendOption func(*Backend)

// WithEnabled sets the enabled status.
func WithEnabled(enabled bool) BackendOption {
	return func(b *Backend) {
		b.enabled = enabled
	}
}

// WithProtocol sets the protocol.
func WithProtocol(protocol port.Protocol) BackendOption {
	return func(b *Backend) {
		b.protocol = protocol
	}
}

// BackendBuilder is a builder for creating Backend entities.
type BackendBuilder struct {
	name     string
	url      string
	apiKey   string
	enabled  bool
	protocol port.Protocol
}

// NewBackendBuilder creates a new backend builder.
func NewBackendBuilder() *BackendBuilder {
	return &BackendBuilder{
		enabled:  true,
		protocol: port.ProtocolOpenAI,
	}
}

// Name sets the backend name.
func (b *BackendBuilder) Name(name string) *BackendBuilder {
	b.name = name
	return b
}

// URL sets the backend URL.
func (b *BackendBuilder) URL(url string) *BackendBuilder {
	b.url = url
	return b
}

// APIKey sets the API key.
func (b *BackendBuilder) APIKey(apiKey string) *BackendBuilder {
	b.apiKey = apiKey
	return b
}

// Enabled sets the enabled status.
func (b *BackendBuilder) Enabled(enabled bool) *BackendBuilder {
	b.enabled = enabled
	return b
}

// Protocol sets the protocol.
func (b *BackendBuilder) Protocol(protocol port.Protocol) *BackendBuilder {
	b.protocol = protocol
	return b
}

// Build creates the backend entity.
func (b *BackendBuilder) Build() (*Backend, error) {
	return NewBackend(b.name, b.url, b.apiKey, b.enabled, b.protocol)
}

// BuildUnsafe creates the backend entity without validation.
func (b *BackendBuilder) BuildUnsafe() *Backend {
	backend, _ := NewBackend(b.name, b.url, b.apiKey, b.enabled, b.protocol)
	return backend
}

// CooldownDuration is a value object for cooldown duration.
type CooldownDuration time.Duration

// DefaultCooldown is the default cooldown duration.
const DefaultCooldown = 300 * time.Second

// NewCooldownDuration creates a new cooldown duration.
func NewCooldownDuration(seconds int) CooldownDuration {
	if seconds <= 0 {
		return CooldownDuration(DefaultCooldown)
	}
	return CooldownDuration(time.Duration(seconds) * time.Second)
}

// Duration returns the duration.
func (d CooldownDuration) Duration() time.Duration {
	return time.Duration(d)
}

// Int returns the duration in seconds.
func (d CooldownDuration) Int() int {
	return int(time.Duration(d).Seconds())
}

// TimeoutConfig represents timeout configuration.
type TimeoutConfig struct {
	Connect time.Duration
	Read    time.Duration
	Write   time.Duration
	Total   time.Duration
}

// DefaultTimeoutConfig returns the default timeout configuration.
func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		Connect: 10 * time.Second,
		Read:    60 * time.Second,
		Write:   60 * time.Second,
		Total:   10 * time.Minute,
	}
}

// GetConnectTimeout returns the connect timeout.
func (t TimeoutConfig) GetConnectTimeout() time.Duration {
	if t.Connect <= 0 {
		return 10 * time.Second
	}
	return t.Connect
}

// GetReadTimeout returns the read timeout.
func (t TimeoutConfig) GetReadTimeout() time.Duration {
	if t.Read <= 0 {
		return 60 * time.Second
	}
	return t.Read
}

// GetWriteTimeout returns the write timeout.
func (t TimeoutConfig) GetWriteTimeout() time.Duration {
	if t.Write <= 0 {
		return 60 * time.Second
	}
	return t.Write
}

// GetTotalTimeout returns the total timeout.
func (t TimeoutConfig) GetTotalTimeout() time.Duration {
	if t.Total <= 0 {
		return 10 * time.Minute
	}
	return t.Total
}

// BackendFilter is a value object for filtering backends.
type BackendFilter struct {
	Enabled   *bool
	Protocols []port.Protocol
	Names     []string
}

// Match checks if the backend matches the filter.
func (f *BackendFilter) Match(backend *Backend) bool {
	if f == nil {
		return true
	}
	if f.Enabled != nil && backend.IsEnabled() != *f.Enabled {
		return false
	}
	if len(f.Protocols) > 0 {
		protocolMatches := false
		for _, p := range f.Protocols {
			if backend.Protocol() == p {
				protocolMatches = true
				break
			}
		}
		if !protocolMatches {
			return false
		}
	}
	if len(f.Names) > 0 {
		nameMatches := false
		for _, name := range f.Names {
			if backend.Name() == name {
				nameMatches = true
				break
			}
		}
		if !nameMatches {
			return false
		}
	}
	return true
}

// RetryConfig represents retry configuration.
type RetryConfig struct {
	MaxRetries          int
	EnableBackoff       bool
	BackoffInitialDelay time.Duration
	BackoffMaxDelay     time.Duration
	BackoffMultiplier   float64
	BackoffJitter       float64
}

// DefaultRetryConfig returns the default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:          3,
		EnableBackoff:       true,
		BackoffInitialDelay: 100 * time.Millisecond,
		BackoffMaxDelay:     5 * time.Second,
		BackoffMultiplier:   2.0,
		BackoffJitter:       0.1,
	}
}

// GetMaxRetries returns the maximum number of retries.
func (r RetryConfig) GetMaxRetries() int {
	if r.MaxRetries <= 0 {
		return 3
	}
	return r.MaxRetries
}

// GetBackoffInitialDelay returns the initial backoff delay.
func (r RetryConfig) GetBackoffInitialDelay() time.Duration {
	if r.BackoffInitialDelay <= 0 {
		return 100 * time.Millisecond
	}
	return r.BackoffInitialDelay
}

// GetBackoffMaxDelay returns the maximum backoff delay.
func (r RetryConfig) GetBackoffMaxDelay() time.Duration {
	if r.BackoffMaxDelay <= 0 {
		return 5 * time.Second
	}
	return r.BackoffMaxDelay
}

// GetBackoffMultiplier returns the backoff multiplier.
func (r RetryConfig) GetBackoffMultiplier() float64 {
	if r.BackoffMultiplier <= 0 {
		return 2.0
	}
	return r.BackoffMultiplier
}

// GetBackoffJitter returns the backoff jitter (0-1).
func (r RetryConfig) GetBackoffJitter() float64 {
	if r.BackoffJitter < 0 || r.BackoffJitter > 1 {
		return 0.1
	}
	return r.BackoffJitter
}

// CalculateDelay calculates the backoff delay for a given attempt.
func (r RetryConfig) CalculateDelay(attempt int) time.Duration {
	initial := r.GetBackoffInitialDelay()
	maxDelay := r.GetBackoffMaxDelay()
	multiplier := r.GetBackoffMultiplier()
	jitter := r.GetBackoffJitter()

	delay := float64(initial) * pow(multiplier, float64(attempt))
	if delay > float64(maxDelay) {
		delay = float64(maxDelay)
	}

	// Add jitter
	jitterAmount := delay * jitter
	delay = delay - jitterAmount + (jitterAmount*2*float64(time.Now().UnixNano()%1000)/1000)

	return time.Duration(delay)
}

func pow(base, exp float64) float64 {
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}

// CircuitBreakerConfig represents circuit breaker configuration.
type CircuitBreakerConfig struct {
	Enabled           bool
	FailureThreshold  int
	SuccessThreshold  int
	OpenTimeout       time.Duration
}

// DefaultCircuitBreakerConfig returns the default circuit breaker configuration.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		Enabled:          false,
		FailureThreshold: 5,
		SuccessThreshold: 2,
		OpenTimeout:      60 * time.Second,
	}
}

// GetFailureThreshold returns the failure threshold.
func (c CircuitBreakerConfig) GetFailureThreshold() int {
	if c.FailureThreshold <= 0 {
		return 5
	}
	return c.FailureThreshold
}

// GetSuccessThreshold returns the success threshold.
func (c CircuitBreakerConfig) GetSuccessThreshold() int {
	if c.SuccessThreshold <= 0 {
		return 2
	}
	return c.SuccessThreshold
}

// GetOpenTimeout returns the open timeout.
func (c CircuitBreakerConfig) GetOpenTimeout() time.Duration {
	if c.OpenTimeout <= 0 {
		return 60 * time.Second
	}
	return c.OpenTimeout
}

// RateLimitConfig represents rate limit configuration.
type RateLimitConfig struct {
	Enabled     bool
	GlobalRPS   float64
	PerIPRPS    float64
	PerModelRPS map[string]float64
	BurstFactor float64
}

// DefaultRateLimitConfig returns the default rate limit configuration.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Enabled:     true,
		GlobalRPS:   1000,
		PerIPRPS:    100,
		BurstFactor: 1.5,
	}
}

// GetGlobalRPS returns the global RPS.
func (r RateLimitConfig) GetGlobalRPS() float64 {
	if r.GlobalRPS <= 0 {
		return 1000
	}
	return r.GlobalRPS
}

// GetPerIPRPS returns the per-IP RPS.
func (r RateLimitConfig) GetPerIPRPS() float64 {
	if r.PerIPRPS <= 0 {
		return 100
	}
	return r.PerIPRPS
}

// GetBurstFactor returns the burst factor.
func (r RateLimitConfig) GetBurstFactor() float64 {
	if r.BurstFactor <= 0 {
		return 1.5
	}
	return r.BurstFactor
}

// GetModelRPS returns the RPS for a specific model.
func (r RateLimitConfig) GetModelRPS(model string) float64 {
	if r.PerModelRPS == nil {
		return 0
	}
	if rps, ok := r.PerModelRPS[model]; ok {
		return rps
	}
	return 0
}

// ConcurrencyConfig represents concurrency configuration.
type ConcurrencyConfig struct {
	Enabled         bool
	MaxRequests     int
	MaxQueueSize    int
	QueueTimeout    time.Duration
	PerBackendLimit int
}

// DefaultConcurrencyConfig returns the default concurrency configuration.
func DefaultConcurrencyConfig() ConcurrencyConfig {
	return ConcurrencyConfig{
		Enabled:         true,
		MaxRequests:     500,
		MaxQueueSize:    1000,
		QueueTimeout:    30 * time.Second,
		PerBackendLimit: 100,
	}
}

// GetMaxRequests returns the maximum concurrent requests.
func (c ConcurrencyConfig) GetMaxRequests() int {
	if c.MaxRequests <= 0 {
		return 500
	}
	return c.MaxRequests
}

// GetMaxQueueSize returns the maximum queue size.
func (c ConcurrencyConfig) GetMaxQueueSize() int {
	if c.MaxQueueSize <= 0 {
		return 1000
	}
	return c.MaxQueueSize
}

// GetQueueTimeout returns the queue timeout.
func (c ConcurrencyConfig) GetQueueTimeout() time.Duration {
	if c.QueueTimeout <= 0 {
		return 30 * time.Second
	}
	return c.QueueTimeout
}

// GetPerBackendLimit returns the per-backend limit.
func (c ConcurrencyConfig) GetPerBackendLimit() int {
	if c.PerBackendLimit <= 0 {
		return 100
	}
	return c.PerBackendLimit
}

// ParseBool parses a string to bool.
func ParseBool(s string) bool {
	lower := strings.ToLower(s)
	return lower == "true" || lower == "1" || lower == "yes"
}

// FormatBool formats a bool to string.
func FormatBool(b bool) string {
	return strconv.FormatBool(b)
}
