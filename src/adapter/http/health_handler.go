package http

import (
	"encoding/json"
	"net/http"

	"llm-proxy/domain/port"
)

type HealthHandler struct {
	configProvider port.ConfigProvider
	logger         port.Logger
}

func NewHealthHandler(configProvider port.ConfigProvider, logger port.Logger) *HealthHandler {
	return &HealthHandler{
		configProvider: configProvider,
		logger:         logger,
	}
}

type HealthStatus struct {
	Status   string `json:"status"`
	Backends int    `json:"backends"`
	Models   int    `json:"models"`
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cfg := h.configProvider.Get()

	status := HealthStatus{
		Status:   "healthy",
		Backends: len(cfg.Backends),
		Models:   len(cfg.Models),
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(status); err != nil {
		h.logger.Error("failed to encode health status", port.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func (h *HealthHandler) IsHealthy() bool {
	cfg := h.configProvider.Get()

	if len(cfg.Backends) == 0 {
		return false
	}

	for _, b := range cfg.Backends {
		if b.IsEnabled() {
			return true
		}
	}

	return false
}
