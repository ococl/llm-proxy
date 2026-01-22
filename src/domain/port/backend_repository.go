package port

// Protocol represents the API protocol of a backend.
type Protocol string

const (
	ProtocolOpenAI    Protocol = "openai"
	ProtocolAnthropic Protocol = "anthropic"
)

// BackendRepository interface for backend data access.
// This abstracts the backend storage for better testability.
type BackendRepository interface {
	// GetAll returns all backends.
	GetAll() []Backend
	// GetByName returns a backend by name.
	GetByName(name string) *Backend
	// GetEnabled returns all enabled backends.
	GetEnabled() []Backend
	// GetByNames returns backends by their names.
	GetByNames(names []string) []Backend
}

// Backend represents an LLM backend configuration (simplified for port).
type Backend struct {
	Name     string
	URL      string
	APIKey   string
	Enabled  bool
	Protocol Protocol
}

// IsEnabled returns true if the backend is enabled.
func (b *Backend) IsEnabled() bool {
	return b.Enabled
}

// GetProtocol returns the protocol, defaulting to OpenAI.
func (b *Backend) GetProtocol() Protocol {
	if b.Protocol == "" {
		return ProtocolOpenAI
	}
	return b.Protocol
}

// ModelRoute represents a route from a model alias to a backend.
type ModelRoute struct {
	Backend  string
	Model    string
	Priority int
	Enabled  bool
}

// ModelAlias represents a model alias configuration.
type ModelAlias struct {
	Enabled bool
	Routes  []ModelRoute
}

// IsEnabled returns true if the alias is enabled.
func (m *ModelAlias) IsEnabled() bool {
	return m.Enabled
}

// String returns the string representation.
func (m *ModelAlias) String() string {
	return ""
}

// RouteResolver interface for resolving model routes.
type RouteResolver interface {
	// Resolve resolves a model alias to a list of routes.
	Resolve(alias string) ([]*Route, error)
}

// Route represents a resolved route with backend information.
type Route struct {
	Backend  *Backend
	Model    string
	Priority int
	Protocol Protocol
}

// IsEnabled returns true if the route is enabled.
func (r *Route) IsEnabled() bool {
	return r.Backend != nil && r.Backend.IsEnabled()
}

// LoadBalancer interface for selecting a backend from a list of routes.
type LoadBalancer interface {
	// Select selects a backend from the given routes.
	Select(routes []*Route) *Backend
}
