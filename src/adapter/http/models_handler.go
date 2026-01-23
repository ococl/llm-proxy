package http

import (
	"encoding/json"
	"net/http"
	"time"

	"llm-proxy/domain/port"
)

// Model represents a model in the OpenAI API response format.
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ModelsResponse represents the response for the /v1/models endpoint.
type ModelsResponse struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

// ModelsHandler handles requests for the /v1/models endpoint.
type ModelsHandler struct {
	config port.ConfigProvider
	logger port.Logger
}

// NewModelsHandler creates a new models handler.
func NewModelsHandler(config port.ConfigProvider, logger port.Logger) *ModelsHandler {
	return &ModelsHandler{
		config: config,
		logger: logger,
	}
}

// ServeHTTP handles GET requests for the /v1/models endpoint.
func (h *ModelsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cfg := h.config.Get()

	var models []Model
	for alias := range cfg.Models {
		models = append(models, Model{
			ID:      alias,
			Object:  "model",
			Created: time.Now().Unix(),
			OwnedBy: "llm-proxy",
		})
	}

	resp := ModelsResponse{
		Object: "list",
		Data:   models,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("编码模型响应失败",
			port.Error(err),
		)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
