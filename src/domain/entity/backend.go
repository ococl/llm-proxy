package entity

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"llm-proxy/domain/types"
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
	protocol types.Protocol
	locale   string // 区域语言设置，用于设置 Accept-Language header（如 zh-CN, en-US）
}

// NewBackend creates a new backend entity.
func NewBackend(name, urlStr, apiKey string, enabled bool, protocol types.Protocol) (*Backend, error) {
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
func (b *Backend) Protocol() types.Protocol {
	if b.protocol == "" {
		return types.ProtocolOpenAI
	}
	return b.protocol
}

// Locale returns the locale setting for Accept-Language header.
func (b *Backend) Locale() string {
	return b.locale
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
func WithProtocol(protocol types.Protocol) BackendOption {
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
	protocol types.Protocol
	locale   string
}

// NewBackendBuilder creates a new backend builder.
func NewBackendBuilder() *BackendBuilder {
	return &BackendBuilder{
		enabled:  true,
		protocol: types.ProtocolOpenAI,
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
func (b *BackendBuilder) Protocol(protocol types.Protocol) *BackendBuilder {
	b.protocol = protocol
	return b
}

// Locale sets the locale for Accept-Language header.
func (b *BackendBuilder) Locale(locale string) *BackendBuilder {
	b.locale = locale
	return b
}

// Build creates the backend entity.
func (b *BackendBuilder) Build() (*Backend, error) {
	return NewBackendWithLocale(b.name, b.url, b.apiKey, b.enabled, b.protocol, b.locale)
}

// BuildUnsafe creates the backend entity without validation.
func (b *BackendBuilder) BuildUnsafe() *Backend {
	backend, _ := NewBackendWithLocale(b.name, b.url, b.apiKey, b.enabled, b.protocol, b.locale)
	return backend
}

// NewBackendWithLocale creates a new backend entity with the specified locale setting.
func NewBackendWithLocale(name, urlStr, apiKey string, enabled bool, protocol types.Protocol, locale string) (*Backend, error) {
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
		locale:   locale,
	}, nil
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
