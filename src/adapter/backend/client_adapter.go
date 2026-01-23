package backend

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
)

type BackendClientAdapter struct {
	client *HTTPClient
}

type BackendError struct {
	StatusCode int
	Body       string
}

func (e *BackendError) Error() string {
	return fmt.Sprintf("backend error: status=%d, body=%s", e.StatusCode, e.Body)
}

// NewBackendClientAdapter creates a new backend client adapter.
func NewBackendClientAdapter(client *HTTPClient) *BackendClientAdapter {
	return &BackendClientAdapter{
		client: client,
	}
}

func getKeys(m map[string]interface{}) string {
	keys := ""
	for k := range m {
		if keys != "" {
			keys += ", "
		}
		keys += k
	}
	return keys
}

// Send sends a non-streaming request to the backend and returns a response.
func (a *BackendClientAdapter) Send(ctx context.Context, req *entity.Request, backend *entity.Backend, backendModel string) (*entity.Response, error) {
	body := map[string]interface{}{
		"model":    backendModel, // Use backend's model name instead of client's alias
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
	body["stream"] = false // Non-streaming request

	backendReq := &BackendRequest{
		Backend: backend,
		Body:    body,
		Headers: mergeHeadersWithDefaults(req.Headers()),
		Path:    "/chat/completions",
		Stream:  false,
	}

	httpResp, err := a.client.Send(ctx, backendReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	if httpResp.StatusCode >= 400 {
		return nil, &BackendError{
			StatusCode: httpResp.StatusCode,
			Body:       string(respBody),
		}
	}

	var respData map[string]interface{}
	if err := json.Unmarshal(respBody, &respData); err != nil {
		return nil, err
	}

	responseID := httpResp.Header.Get("x-request-id")
	if responseID == "" {
		responseID = "resp-" + req.ID().String()
	}

	headers := make(map[string][]string)
	for k, v := range httpResp.Header {
		if !isHopByHopHeader(k) {
			headers[k] = v
		}
	}

	builder := entity.NewResponseBuilder().
		ID(responseID).
		Model(req.Model().String()).
		Headers(headers)

	if usage, ok := respData["usage"].(map[string]interface{}); ok {
		promptTokens, _ := usage["prompt_tokens"].(float64)
		completionTokens, _ := usage["completion_tokens"].(float64)
		builder.Usage(entity.NewUsage(int(promptTokens), int(completionTokens)))
	}

	choicesRaw, choicesExists := respData["choices"]
	if choicesExists && choicesRaw == nil {
		// TODO: Add logger to client_adapter for better diagnostics
		builder.Choices([]entity.Choice{})
	} else if choices, ok := respData["choices"].([]interface{}); ok && len(choices) > 0 {
		if choiceMap, ok := choices[0].(map[string]interface{}); ok {
			finishReason, _ := choiceMap["finish_reason"].(string)

			if messageMap, ok := choiceMap["message"].(map[string]interface{}); ok {
				content, _ := messageMap["content"].(string)
				role, _ := messageMap["role"].(string)

				choice := entity.Choice{
					Index: 0,
					Message: entity.Message{
						Role:    role,
						Content: content,
					},
					FinishReason: finishReason,
				}

				builder.Choices([]entity.Choice{choice})
			}
		}
	}

	return builder.BuildUnsafe(), nil
}

// SendStreaming sends a streaming request to the backend and calls handler for each chunk.
func (a *BackendClientAdapter) SendStreaming(
	ctx context.Context,
	req *entity.Request,
	backend *entity.Backend,
	backendModel string,
	handler func([]byte) error,
) error {
	body := map[string]interface{}{
		"model":    backendModel, // Use backend's model name instead of client's alias
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
	body["stream"] = true // Streaming request

	backendReq := &BackendRequest{
		Backend: backend,
		Body:    body,
		Headers: mergeHeadersWithDefaults(req.Headers()),
		Path:    "/chat/completions",
		Stream:  true,
	}

	httpResp, err := a.client.Send(ctx, backendReq)
	if err != nil {
		return err
	}

	if httpResp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()
		return &BackendError{
			StatusCode: httpResp.StatusCode,
			Body:       string(respBody),
		}
	}

	reader := bufio.NewReader(httpResp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		// SSE format: "data: <json>"
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// Check for [DONE] message
		if data == "[DONE]" {
			break
		}

		if err := handler([]byte(data)); err != nil {
			return err
		}
	}

	return nil
}

// GetHTTPClient returns the underlying HTTP client.
func (a *BackendClientAdapter) GetHTTPClient() *http.Client {
	return a.client.GetHTTPClient()
}

// Ensure BackendClientAdapter implements port.BackendClient.
var _ port.BackendClient = (*BackendClientAdapter)(nil)

func mergeHeadersWithDefaults(clientHeaders map[string][]string) map[string][]string {
	headers := make(map[string][]string)
	headers["Content-Type"] = []string{"application/json"}

	for k, v := range clientHeaders {
		if k != "Content-Type" && k != "Authorization" && k != "X-Api-Key" {
			headers[k] = v
		}
	}

	return headers
}
