package config

import (
	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
)

// BackendRepository implements port.BackendRepository using ConfigAdapter.
type BackendRepository struct {
	adapter *ConfigAdapter
}

// NewBackendRepository creates a new backend repository.
func NewBackendRepository(adapter *ConfigAdapter) *BackendRepository {
	return &BackendRepository{
		adapter: adapter,
	}
}

// GetAll returns all backends.
func (r *BackendRepository) GetAll() []*entity.Backend {
	cfg := r.adapter.Get()
	return cfg.Backends
}

// GetByName returns a backend by name.
func (r *BackendRepository) GetByName(name string) *entity.Backend {
	return r.adapter.GetBackend(name)
}

// GetEnabled returns all enabled backends.
func (r *BackendRepository) GetEnabled() []*entity.Backend {
	all := r.GetAll()
	var enabled []*entity.Backend
	for _, b := range all {
		if b.IsEnabled() {
			enabled = append(enabled, b)
		}
	}
	return enabled
}

// GetByNames returns backends by names.
func (r *BackendRepository) GetByNames(names []string) []*entity.Backend {
	var backends []*entity.Backend
	for _, name := range names {
		if backend := r.GetByName(name); backend != nil {
			backends = append(backends, backend)
		}
	}
	return backends
}

// Ensure BackendRepository implements port.BackendRepository.
var _ port.BackendRepository = (*BackendRepository)(nil)
