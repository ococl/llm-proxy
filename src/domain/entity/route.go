package entity

import (
	"fmt"
	"sort"

	"llm-proxy/domain/types"
)

// ModelAlias represents a model alias name.
type ModelAlias string

// NewModelAlias creates a new model alias.
func NewModelAlias(alias string) ModelAlias {
	return ModelAlias(alias)
}

// String returns the string representation.
func (m ModelAlias) String() string {
	return string(m)
}

// IsEmpty returns true if the alias is empty.
func (m ModelAlias) IsEmpty() bool {
	return string(m) == ""
}

// Route represents a routing rule from a model alias to a backend.
type Route struct {
	backend  *Backend
	model    string
	priority int
	enabled  bool
	protocol types.Protocol
}

// NewRoute creates a new route.
func NewRoute(backend *Backend, model string, priority int, enabled bool) *Route {
	return &Route{
		backend:  backend,
		model:    model,
		priority: priority,
		enabled:  enabled,
		protocol: backend.Protocol(),
	}
}

// Backend returns the backend.
func (r *Route) Backend() *Backend {
	return r.backend
}

// Model returns the model name.
func (r *Route) Model() string {
	return r.model
}

// Priority returns the priority (lower is higher priority).
func (r *Route) Priority() int {
	return r.priority
}

// IsEnabled returns true if the route is enabled.
func (r *Route) IsEnabled() bool {
	return r.enabled && r.backend.IsEnabled()
}

// Protocol returns the protocol.
func (r *Route) Protocol() types.Protocol {
	if r.protocol == "" {
		return r.backend.Protocol()
	}
	return r.protocol
}

// String returns a string representation.
func (r *Route) String() string {
	return fmt.Sprintf("Route(%s -> %s, priority=%d, enabled=%v)",
		r.model, r.backend.Name(), r.priority, r.enabled)
}

// WithProtocol creates a new route with a different protocol.
func (r *Route) WithProtocol(protocol types.Protocol) *Route {
	return &Route{
		backend:  r.backend,
		model:    r.model,
		priority: r.priority,
		enabled:  r.enabled,
		protocol: protocol,
	}
}

// RouteList is a collection of routes.
type RouteList []*Route

// FilterEnabled returns only enabled routes.
func (rl RouteList) FilterEnabled() RouteList {
	var result RouteList
	for _, r := range rl {
		if r.IsEnabled() {
			result = append(result, r)
		}
	}
	return result
}

// FilterByProtocol returns routes matching the protocol.
func (rl RouteList) FilterByProtocol(protocol types.Protocol) RouteList {
	var result RouteList
	for _, r := range rl {
		if r.Protocol() == protocol {
			result = append(result, r)
		}
	}
	return result
}

// SortByPriority sorts routes by priority (ascending).
func (rl RouteList) SortByPriority() RouteList {
	sorted := make(RouteList, len(rl))
	copy(sorted, rl)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority() < sorted[j].Priority()
	})
	return sorted
}

// GroupByPriority groups routes by priority level.
func (rl RouteList) GroupByPriority() map[int]RouteList {
	groups := make(map[int]RouteList)
	for _, r := range rl {
		priority := r.Priority()
		groups[priority] = append(groups[priority], r)
	}
	return groups
}

// First returns the first route or nil.
func (rl RouteList) First() *Route {
	if len(rl) == 0 {
		return nil
	}
	return rl[0]
}

// IsEmpty returns true if the list is empty.
func (rl RouteList) IsEmpty() bool {
	return len(rl) == 0
}

// Len returns the number of routes.
func (rl RouteList) Len() int {
	return len(rl)
}

// RouteConfig represents the configuration for a route.
type RouteConfig struct {
	BackendName string
	Model       string
	Priority    int
	Enabled     *bool
	Protocol    string
}

// IsEnabled returns true if the route is enabled.
func (rc *RouteConfig) IsEnabled() bool {
	return rc.Enabled == nil || *rc.Enabled
}

// GetProtocol returns the protocol, defaulting to the backend protocol.
func (rc *RouteConfig) GetProtocol(backendProtocol types.Protocol) types.Protocol {
	if rc.Protocol != "" {
		return types.Protocol(rc.Protocol)
	}
	return backendProtocol
}

// ModelAliasConfig represents the configuration for a model alias.
type ModelAliasConfig struct {
	Enabled *bool
	Routes  []RouteConfig
}

// IsEnabled returns true if the alias is enabled.
func (mac *ModelAliasConfig) IsEnabled() bool {
	return mac.Enabled == nil || *mac.Enabled
}

// FallbackConfig represents fallback configuration.
type FallbackConfig struct {
	AliasFallback map[string][]string
}

// GetFallbackAliases returns the fallback aliases for a given alias.
func (fc *FallbackConfig) GetFallbackAliases(alias string) []ModelAlias {
	if fc.AliasFallback == nil {
		return nil
	}
	fallbacks, ok := fc.AliasFallback[alias]
	if !ok {
		return nil
	}
	result := make([]ModelAlias, len(fallbacks))
	for i, fb := range fallbacks {
		result[i] = NewModelAlias(fb)
	}
	return result
}

// HasFallback returns true if the alias has fallback configuration.
func (fc *FallbackConfig) HasFallback(alias string) bool {
	if fc.AliasFallback == nil {
		return false
	}
	fallbacks, ok := fc.AliasFallback[alias]
	return ok && len(fallbacks) > 0
}

// RouteBuilder is a builder for creating Route entities.
type RouteBuilder struct {
	backend  *Backend
	model    string
	priority int
	enabled  bool
	protocol types.Protocol
}

// NewRouteBuilder creates a new route builder.
func NewRouteBuilder() *RouteBuilder {
	return &RouteBuilder{
		enabled:  true,
		priority: 1,
	}
}

// Backend sets the backend.
func (rb *RouteBuilder) Backend(backend *Backend) *RouteBuilder {
	rb.backend = backend
	return rb
}

// Model sets the model name.
func (rb *RouteBuilder) Model(model string) *RouteBuilder {
	rb.model = model
	return rb
}

// Priority sets the priority.
func (rb *RouteBuilder) Priority(priority int) *RouteBuilder {
	rb.priority = priority
	return rb
}

// Enabled sets the enabled status.
func (rb *RouteBuilder) Enabled(enabled bool) *RouteBuilder {
	rb.enabled = enabled
	return rb
}

// Protocol sets the protocol.
func (rb *RouteBuilder) Protocol(protocol types.Protocol) *RouteBuilder {
	rb.protocol = protocol
	return rb
}

// Build creates the route entity.
func (rb *RouteBuilder) Build() (*Route, error) {
	if rb.backend == nil {
		return nil, fmt.Errorf("backend is required")
	}
	if rb.model == "" {
		return nil, fmt.Errorf("model is required")
	}
	route := NewRoute(rb.backend, rb.model, rb.priority, rb.enabled)
	if rb.protocol != "" {
		route = route.WithProtocol(rb.protocol)
	}
	return route, nil
}

// BuildUnsafe creates the route entity without validation.
func (rb *RouteBuilder) BuildUnsafe() *Route {
	route, _ := rb.Build()
	return route
}
