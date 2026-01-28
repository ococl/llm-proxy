package port

import (
	"llm-proxy/domain/entity"
	"llm-proxy/domain/types"
)

// BackendRepository interface for backend data access.
type BackendRepository interface {
	GetAll() []*entity.Backend
	GetByName(name string) *entity.Backend
	GetEnabled() []*entity.Backend
	GetByNames(names []string) []*entity.Backend
}

// Route represents a resolved route with backend information.
type Route struct {
	Backend   *entity.Backend
	Model     string
	Priority  int
	Protocol  types.Protocol
	Enabled   bool
	Reasoning bool // 该模型是否需要处理 reasoning_content 字段
}

func (r *Route) IsEnabled() bool {
	return r.Enabled && r.Backend != nil && r.Backend.IsEnabled()
}

// LoadBalancer interface for selecting a backend from a list of routes.
type LoadBalancer interface {
	Select(routes []*Route) *entity.Backend
}

// RouteResolver interface for resolving model routes.
type RouteResolver interface {
	Resolve(alias string) ([]*Route, error)
}

// ModelAlias represents a model alias configuration.
type ModelAlias struct {
	Enabled bool
	Routes  []ModelRoute
}

func (m *ModelAlias) IsEnabled() bool {
	return m.Enabled
}

func (m *ModelAlias) String() string {
	return ""
}

// ModelRoute represents a route from a model alias to a backend.
type ModelRoute struct {
	Backend   string
	Model     string
	Priority  int
	Enabled   bool
	Reasoning bool // 该模型是否需要处理 reasoning_content 字段
}
