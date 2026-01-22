package backend

import (
	"context"
	"net/http"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
)

// BackendClientAdapter adapts HTTPClient to port.BackendClient interface.
type BackendClientAdapter struct {
	client *HTTPClient
}

// NewBackendClientAdapter creates a new backend client adapter.
func NewBackendClientAdapter(client *HTTPClient) *BackendClientAdapter {
	return &BackendClientAdapter{
		client: client,
	}
}

// Send sends a request to the backend and returns a response.
func (a *BackendClientAdapter) Send(ctx context.Context, req *entity.Request, backend *entity.Backend) (*entity.Response, error) {
	body := map[string]interface{}{
		"model":    req.Model().String(),
		"messages": req.Messages(),
	}

	if req.MaxTokens() > 0 {
		body["max_tokens"] = req.MaxTokens()
	}
	if req.Temperature() != 1.0 {
		body["temperature"] = req.Temperature()
	}
	if req.TopP() != 1.0 {
		body["top_p"] = req.TopP()
	}
	if len(req.Stop()) > 0 {
		body["stop"] = req.Stop()
	}
	if len(req.Tools()) > 0 {
		body["tools"] = req.Tools()
	}
	if req.ToolChoice() != nil {
		body["tool_choice"] = req.ToolChoice()
	}
	if req.User() != "" {
		body["user"] = req.User()
	}
	body["stream"] = req.IsStream()

	backendReq := &BackendRequest{
		Backend: backend,
		Body:    body,
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
		},
		Path:   "/chat/completions",
		Stream: req.IsStream(),
	}

	httpResp, err := a.client.Send(ctx, backendReq)
	if err != nil {
		return nil, err
	}

	resp := entity.NewResponseBuilder().
		ID(httpResp.Header.Get("x-request-id")).
		Model(req.Model().String()).
		BuildUnsafe()

	return resp, nil
}

// GetHTTPClient returns the underlying HTTP client.
func (a *BackendClientAdapter) GetHTTPClient() *http.Client {
	return a.client.GetHTTPClient()
}

// Ensure BackendClientAdapter implements port.BackendClient.
var _ port.BackendClient = (*BackendClientAdapter)(nil)
