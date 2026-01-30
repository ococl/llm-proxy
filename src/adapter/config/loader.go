package config

import (
	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
	"llm-proxy/infrastructure/config"
)

// ConfigAdapter adapts config.Config to port.ConfigProvider interface.
type ConfigAdapter struct {
	manager *config.Manager
}

// NewConfigAdapter creates a new config adapter.
func NewConfigAdapter(manager *config.Manager) *ConfigAdapter {
	return &ConfigAdapter{manager: manager}
}

// Get returns the current port.Config.
func (a *ConfigAdapter) Get() *port.Config {
	cfg := a.manager.Get()
	return a.convertConfig(cfg)
}

// GetBackend returns a backend by name.
func (a *ConfigAdapter) GetBackend(name string) *entity.Backend {
	cfg := a.manager.Get()
	for i := range cfg.Backends {
		if cfg.Backends[i].Name == name && cfg.Backends[i].IsEnabled() {
			backend, err := a.convertBackend(&cfg.Backends[i])
			if err != nil {
				continue
			}
			return backend
		}
	}
	return nil
}

// GetBackends returns all enabled backends.
func (a *ConfigAdapter) GetBackends() []*entity.Backend {
	cfg := a.manager.Get()
	backends := make([]*entity.Backend, 0, len(cfg.Backends))
	for i := range cfg.Backends {
		if cfg.Backends[i].IsEnabled() {
			backend, err := a.convertBackend(&cfg.Backends[i])
			if err != nil {
				continue
			}
			backends = append(backends, backend)
		}
	}
	return backends
}

// GetModelAlias returns a model alias by name.
func (a *ConfigAdapter) GetModelAlias(alias string) *port.ModelAlias {
	cfg := a.manager.Get()
	modelAlias, ok := cfg.Models[alias]
	if !ok || modelAlias == nil {
		return nil
	}
	backends := a.GetBackends()
	return a.convertModelAlias(modelAlias, backends)
}

// Watch returns a channel that signals configuration changes.
// The returned channel is closed when the adapter is no longer needed.
func (a *ConfigAdapter) Watch() <-chan struct{} {
	return a.manager.Watch()
}

// GetRateLimitConfig returns the rate limit configuration.
func (a *ConfigAdapter) GetRateLimitConfig() *port.RateLimitConfig {
	cfg := a.manager.Get()
	return &port.RateLimitConfig{
		Enabled:     cfg.RateLimit.Enabled,
		GlobalRPS:   cfg.RateLimit.GetGlobalRPS(),
		PerIPRPS:    cfg.RateLimit.GetPerIPRPS(),
		PerModelRPS: cfg.RateLimit.PerModelRPS,
		BurstFactor: cfg.RateLimit.GetBurstFactor(),
	}
}

// GetConcurrencyConfig returns the concurrency configuration.
func (a *ConfigAdapter) GetConcurrencyConfig() *port.ConcurrencyConfig {
	cfg := a.manager.Get()
	return &port.ConcurrencyConfig{
		Enabled:         cfg.Concurrency.Enabled,
		MaxRequests:     cfg.Concurrency.GetMaxRequests(),
		MaxQueueSize:    cfg.Concurrency.GetMaxQueueSize(),
		QueueTimeout:    cfg.Concurrency.GetQueueTimeout(),
		PerBackendLimit: cfg.Concurrency.GetPerBackendLimit(),
	}
}

// GetFallbackConfig returns the fallback configuration.
func (a *ConfigAdapter) GetFallbackConfig() *port.FallbackConfig {
	cfg := a.manager.Get()
	return &port.FallbackConfig{
		CooldownSeconds:       cfg.Fallback.CooldownSeconds,
		MaxRetries:            cfg.Fallback.MaxRetries,
		AliasFallback:         cfg.Fallback.AliasFallback,
		EnableBackoff:         cfg.Fallback.IsBackoffEnabled(),
		BackoffInitialDelay:   cfg.Fallback.GetBackoffInitialDelay(),
		BackoffMaxDelay:       cfg.Fallback.GetBackoffMaxDelay(),
		BackoffMultiplier:     cfg.Fallback.GetBackoffMultiplier(),
		BackoffJitter:         cfg.Fallback.GetBackoffJitter(),
		EnableCircuitBreaker:  cfg.Fallback.IsCircuitBreakerEnabled(),
		CircuitFailureThresh:  cfg.Fallback.GetCircuitFailureThreshold(),
		CircuitSuccessThresh:  cfg.Fallback.GetCircuitSuccessThreshold(),
		CircuitOpenTimeoutSec: cfg.Fallback.GetCircuitOpenTimeout(),
	}
}

// GetErrorFallbackConfig returns the error fallback configuration.
func (a *ConfigAdapter) GetErrorFallbackConfig() *port.ErrorFallbackConfig {
	cfg := a.manager.Get()
	return &port.ErrorFallbackConfig{
		ServerError: port.ServerErrorConfig{
			Enabled: cfg.ErrorFallback.ServerError.Enabled,
		},
		ClientError: port.ClientErrorConfig{
			Enabled:     cfg.ErrorFallback.ClientError.Enabled,
			StatusCodes: cfg.ErrorFallback.ClientError.StatusCodes,
			Patterns:    cfg.ErrorFallback.ClientError.Patterns,
		},
	}
}

// GetProxyConfig returns the proxy configuration.
func (a *ConfigAdapter) GetProxyConfig() *port.ProxyConfig {
	cfg := a.manager.Get()
	return &port.ProxyConfig{
		EnableSystemPrompt: cfg.Proxy.GetEnableSystemPrompt(),
		ForwardClientIP:    cfg.Proxy.GetForwardClientIP(),
	}
}

// GetTimeoutConfig returns the timeout configuration.
func (a *ConfigAdapter) GetTimeoutConfig() *port.TimeoutConfig {
	cfg := a.manager.Get()
	return &port.TimeoutConfig{
		ConnectTimeout: cfg.Timeout.ConnectTimeout,
		ReadTimeout:    cfg.Timeout.ReadTimeout,
		WriteTimeout:   cfg.Timeout.WriteTimeout,
		TotalTimeout:   cfg.Timeout.TotalTimeout,
	}
}

// GetListen returns the listen address.
func (a *ConfigAdapter) GetListen() string {
	return a.manager.Get().Listen
}

// GetProxyAPIKey returns the proxy API key.
func (a *ConfigAdapter) GetProxyAPIKey() string {
	return a.manager.Get().ProxyAPIKey
}

// convertConfig converts config.Config to port.Config.
func (a *ConfigAdapter) convertConfig(cfg *config.Config) *port.Config {
	backends := make([]*entity.Backend, 0, len(cfg.Backends))
	for i := range cfg.Backends {
		if cfg.Backends[i].IsEnabled() {
			backend, err := a.convertBackend(&cfg.Backends[i])
			if err != nil {
				continue
			}
			backends = append(backends, backend)
		}
	}

	models := make(map[string]*port.ModelAlias, len(cfg.Models))
	for name, modelAlias := range cfg.Models {
		if modelAlias.IsEnabled() {
			models[name] = a.convertModelAlias(modelAlias, backends)
		}
	}

	return &port.Config{
		Listen:      cfg.Listen,
		ProxyAPIKey: cfg.ProxyAPIKey,
		Proxy: port.ProxyConfig{
			EnableSystemPrompt: cfg.Proxy.GetEnableSystemPrompt(),
			ForwardClientIP:    cfg.Proxy.GetForwardClientIP(),
			CustomVariables:    cfg.Proxy.GetCustomVariables(),
		},
		Backends: backends,
		Models:   models,
		Fallback: port.FallbackConfig{
			CooldownSeconds:       cfg.Fallback.CooldownSeconds,
			MaxRetries:            cfg.Fallback.MaxRetries,
			AliasFallback:         cfg.Fallback.AliasFallback,
			EnableBackoff:         cfg.Fallback.IsBackoffEnabled(),
			BackoffInitialDelay:   cfg.Fallback.GetBackoffInitialDelay(),
			BackoffMaxDelay:       cfg.Fallback.GetBackoffMaxDelay(),
			BackoffMultiplier:     cfg.Fallback.GetBackoffMultiplier(),
			BackoffJitter:         cfg.Fallback.GetBackoffJitter(),
			EnableCircuitBreaker:  cfg.Fallback.IsCircuitBreakerEnabled(),
			CircuitFailureThresh:  cfg.Fallback.GetCircuitFailureThreshold(),
			CircuitSuccessThresh:  cfg.Fallback.GetCircuitSuccessThreshold(),
			CircuitOpenTimeoutSec: cfg.Fallback.GetCircuitOpenTimeout(),
		},
		ErrorFallback: port.ErrorFallbackConfig{
			ServerError: port.ServerErrorConfig{
				Enabled: cfg.ErrorFallback.ServerError.Enabled,
			},
			ClientError: port.ClientErrorConfig{
				Enabled:     cfg.ErrorFallback.ClientError.Enabled,
				StatusCodes: cfg.ErrorFallback.ClientError.StatusCodes,
				Patterns:    cfg.ErrorFallback.ClientError.Patterns,
			},
		},
		Logging: port.LoggingConfig{
			Level:             cfg.Logging.GetLevel(),
			ConsoleLevel:      cfg.Logging.GetConsoleLevel(),
			BaseDir:           cfg.Logging.GetBaseDir(),
			EnableMetrics:     cfg.Logging.EnableMetrics,
			MaxFileSizeMB:     cfg.Logging.GetMaxFileSizeMB(),
			MaxAgeDays:        cfg.Logging.GetMaxAgeDays(),
			MaxBackups:        cfg.Logging.GetMaxBackups(),
			Format:            cfg.Logging.GetFormat(),
			Colorize:          cfg.Logging.GetColorize(),
			ConsoleFormat:     cfg.Logging.GetConsoleFormat(),
			DebugMode:         cfg.Logging.DebugMode,
			SeparateFiles:     cfg.Logging.SeparateFiles,
			RequestDir:        cfg.Logging.RequestDir,
			ErrorDir:          cfg.Logging.ErrorDir,
			MaskSensitive:     cfg.Logging.ShouldMaskSensitive(),
			BufferSize:        cfg.Logging.GetBufferSize(),
			FlushInterval:     cfg.Logging.GetFlushInterval(),
			DropOnFull:        cfg.Logging.ShouldDropOnFull(),
			MaxLogContentSize: cfg.Logging.GetMaxLogContentSize(),
		},
		Timeout: port.TimeoutConfig{
			ConnectTimeout: cfg.Timeout.ConnectTimeout,
			ReadTimeout:    cfg.Timeout.ReadTimeout,
			WriteTimeout:   cfg.Timeout.WriteTimeout,
			TotalTimeout:   cfg.Timeout.TotalTimeout,
		},
		RateLimit: port.RateLimitConfig{
			Enabled:     cfg.RateLimit.Enabled,
			GlobalRPS:   cfg.RateLimit.GetGlobalRPS(),
			PerIPRPS:    cfg.RateLimit.GetPerIPRPS(),
			PerModelRPS: cfg.RateLimit.PerModelRPS,
			BurstFactor: cfg.RateLimit.GetBurstFactor(),
		},
		Concurrency: port.ConcurrencyConfig{
			Enabled:         cfg.Concurrency.Enabled,
			MaxRequests:     cfg.Concurrency.GetMaxRequests(),
			MaxQueueSize:    cfg.Concurrency.GetMaxQueueSize(),
			QueueTimeout:    cfg.Concurrency.GetQueueTimeout(),
			PerBackendLimit: cfg.Concurrency.GetPerBackendLimit(),
		},
	}
}

// convertBackend converts config.Backend to entity.Backend.
func (a *ConfigAdapter) convertBackend(cfg *config.Backend) (*entity.Backend, error) {
	protocol := types.ProtocolOpenAI
	if cfg.GetProtocol() == "anthropic" {
		protocol = types.ProtocolAnthropic
	}

	// 使用 BackendBuilder 来创建后端实体
	return entity.NewBackendBuilder().
		Name(cfg.Name).
		URL(cfg.URL).
		APIKey(cfg.APIKey).
		Enabled(cfg.IsEnabled()).
		Protocol(protocol).
		Locale(cfg.Locale).
		Build()
}

// convertModelAlias converts config.ModelAlias to port.ModelAlias.
func (a *ConfigAdapter) convertModelAlias(cfg *config.ModelAlias, backends []*entity.Backend) *port.ModelAlias {
	routes := make([]port.ModelRoute, 0, len(cfg.Routes))
	for _, route := range cfg.Routes {
		if !route.IsEnabled() {
			continue
		}

		backendNames := make(map[string]*entity.Backend)
		for _, be := range backends {
			backendNames[be.Name()] = be
		}

		var routeBackends []*entity.Backend
		if be, ok := backendNames[route.Backend]; ok {
			routeBackends = append(routeBackends, be)
		}

		routes = append(routes, port.ModelRoute{
			Backend:  route.Backend,
			Model:    route.Model,
			Priority: route.Priority,
			Enabled:  route.IsEnabled(),
		})
	}

	return &port.ModelAlias{
		Enabled: cfg.IsEnabled(),
		Routes:  routes,
	}
}

// Ensure ConfigAdapter implements port.ConfigProvider interface.
// var _ port.ConfigProvider = (*ConfigAdapter)(nil)
