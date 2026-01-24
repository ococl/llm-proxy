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
	logger port.Logger
}

type BackendError struct {
	StatusCode int
	Body       string
}

func (e *BackendError) Error() string {
	return fmt.Sprintf("backend error: status=%d, body=%s", e.StatusCode, e.Body)
}

// NewBackendClientAdapter creates a new backend client adapter.
func NewBackendClientAdapter(client *HTTPClient, logger port.Logger) *BackendClientAdapter {
	if logger == nil {
		logger = &port.NopLogger{}
	}
	return &BackendClientAdapter{
		client: client,
		logger: logger,
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
	reqID := req.ID().String()

	a.logger.Debug("准备发送上游请求",
		port.String("req_id", reqID),
		port.String("backend", backend.Name()),
		port.String("backend_url", backend.URL().String()),
		port.String("backend_model", backendModel),
	)

	body := map[string]interface{}{
		"model":    backendModel,
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
	body["stream"] = false

	if upstreamBodyJSON, err := json.Marshal(body); err == nil {
		a.logger.Debug("上游请求体",
			port.String("req_id", reqID),
			port.String("backend", backend.Name()),
			port.String("request_body", string(upstreamBodyJSON)),
		)
	}

	backendReq := &BackendRequest{
		Backend: backend,
		Body:    body,
		Headers: mergeHeadersWithDefaults(req.Headers()),
		Path:    "/chat/completions",
		Stream:  false,
	}

	a.logger.Info("发送上游请求",
		port.String("req_id", reqID),
		port.String("backend", backend.Name()),
		port.String("backend_model", backendModel),
	)

	httpResp, err := a.client.Send(ctx, backendReq)
	if err != nil {
		a.logger.Error("上游请求发送失败",
			port.String("req_id", reqID),
			port.String("backend", backend.Name()),
			port.Error(err),
		)
		return nil, err
	}
	defer httpResp.Body.Close()

	a.logger.Debug("收到上游响应",
		port.String("req_id", reqID),
		port.String("backend", backend.Name()),
		port.Int("status_code", httpResp.StatusCode),
	)

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		a.logger.Error("读取上游响应失败",
			port.String("req_id", reqID),
			port.String("backend", backend.Name()),
			port.Error(err),
		)
		return nil, err
	}

	a.logger.Debug("上游响应体读取完成",
		port.String("req_id", reqID),
		port.String("backend", backend.Name()),
		port.Int("body_size", len(respBody)),
	)

	a.logger.Debug("上游响应体",
		port.String("req_id", reqID),
		port.String("backend", backend.Name()),
		port.String("response_body", string(respBody)),
	)

	if httpResp.StatusCode >= 400 {
		a.logger.Warn("上游返回错误状态码",
			port.String("req_id", reqID),
			port.String("backend", backend.Name()),
			port.Int("status_code", httpResp.StatusCode),
		)
		return nil, &BackendError{
			StatusCode: httpResp.StatusCode,
			Body:       string(respBody),
		}
	}

	var respData map[string]interface{}
	if err := json.Unmarshal(respBody, &respData); err != nil {
		a.logger.Error("解析上游响应JSON失败",
			port.String("req_id", reqID),
			port.String("backend", backend.Name()),
			port.Error(err),
		)
		return nil, err
	}

	a.logger.Debug("上游响应解析成功",
		port.String("req_id", reqID),
		port.String("backend", backend.Name()),
	)

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
		a.logger.Warn("上游返回空choices字段",
			port.String("req_id", reqID),
			port.String("backend", backend.Name()),
		)
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

	response := builder.BuildUnsafe()

	a.logger.Info("上游请求完成",
		port.String("req_id", reqID),
		port.String("backend", backend.Name()),
		port.String("response_id", response.ID),
	)

	return response, nil
}

// SendStreaming sends a streaming request to the backend and calls handler for each chunk.
func (a *BackendClientAdapter) SendStreaming(
	ctx context.Context,
	req *entity.Request,
	backend *entity.Backend,
	backendModel string,
	handler func([]byte) error,
) error {
	reqID := req.ID().String()

	a.logger.Debug("准备发送上游流式请求",
		port.String("req_id", reqID),
		port.String("backend", backend.Name()),
		port.String("backend_url", backend.URL().String()),
		port.String("backend_model", backendModel),
	)

	body := map[string]interface{}{
		"model":    backendModel,
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
	body["stream"] = true

	if upstreamBodyJSON, err := json.Marshal(body); err == nil {
		a.logger.Debug("上游流式请求体",
			port.String("req_id", reqID),
			port.String("backend", backend.Name()),
			port.String("request_body", string(upstreamBodyJSON)),
		)
	}

	backendReq := &BackendRequest{
		Backend: backend,
		Body:    body,
		Headers: mergeHeadersWithDefaults(req.Headers()),
		Path:    "/chat/completions",
		Stream:  true,
	}

	a.logger.Info("发送上游流式请求",
		port.String("req_id", reqID),
		port.String("backend", backend.Name()),
		port.String("backend_model", backendModel),
	)

	httpResp, err := a.client.Send(ctx, backendReq)
	if err != nil {
		a.logger.Error("上游流式请求发送失败",
			port.String("req_id", reqID),
			port.String("backend", backend.Name()),
			port.Error(err),
		)
		return err
	}

	a.logger.Debug("收到上游流式响应",
		port.String("req_id", reqID),
		port.String("backend", backend.Name()),
		port.Int("status_code", httpResp.StatusCode),
	)

	if httpResp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()
		a.logger.Warn("上游流式请求返回错误状态码",
			port.String("req_id", reqID),
			port.String("backend", backend.Name()),
			port.Int("status_code", httpResp.StatusCode),
		)
		return &BackendError{
			StatusCode: httpResp.StatusCode,
			Body:       string(respBody),
		}
	}

	a.logger.Debug("开始读取上游流式响应",
		port.String("req_id", reqID),
		port.String("backend", backend.Name()),
	)

	reader := bufio.NewReader(httpResp.Body)
	chunkCount := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				a.logger.Debug("上游流式响应结束",
					port.String("req_id", reqID),
					port.String("backend", backend.Name()),
					port.Int("total_chunks", chunkCount),
				)
				break
			}
			a.logger.Error("读取上游流式响应失败",
				port.String("req_id", reqID),
				port.String("backend", backend.Name()),
				port.Error(err),
			)
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
			a.logger.Debug("收到上游[DONE]信号",
				port.String("req_id", reqID),
				port.String("backend", backend.Name()),
			)
			break
		}

		chunkCount++
		a.logger.Debug("收到上游流式数据块",
			port.String("req_id", reqID),
			port.String("backend", backend.Name()),
			port.Int("chunk_index", chunkCount),
			port.String("chunk_data", data),
		)
		if err := handler([]byte(data)); err != nil {
			a.logger.Error("处理上游流式数据块失败",
				port.String("req_id", reqID),
				port.String("backend", backend.Name()),
				port.Int("chunk_index", chunkCount),
				port.Error(err),
			)
			return err
		}
	}

	a.logger.Info("上游流式请求完成",
		port.String("req_id", reqID),
		port.String("backend", backend.Name()),
		port.Int("total_chunks", chunkCount),
	)

	return nil
}

func (a *BackendClientAdapter) SendStreamingPassthrough(
	ctx context.Context,
	req *entity.Request,
	backend *entity.Backend,
	backendModel string,
) (*http.Response, error) {
	reqID := req.ID().String()

	a.logger.Debug("准备发送上游流式请求(透传模式)",
		port.String("req_id", reqID),
		port.String("backend", backend.Name()),
		port.String("backend_url", backend.URL().String()),
		port.String("backend_model", backendModel),
	)

	body := map[string]interface{}{
		"model":    backendModel,
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
	body["stream"] = true

	if upstreamBodyJSON, err := json.Marshal(body); err == nil {
		a.logger.Debug("上游流式请求体",
			port.String("req_id", reqID),
			port.String("backend", backend.Name()),
			port.String("request_body", string(upstreamBodyJSON)),
		)
	}

	backendReq := &BackendRequest{
		Backend: backend,
		Body:    body,
		Headers: mergeHeadersWithDefaults(req.Headers()),
		Path:    "/chat/completions",
		Stream:  true,
	}

	a.logger.Info("发送上游流式请求(透传模式)",
		port.String("req_id", reqID),
		port.String("backend", backend.Name()),
		port.String("backend_model", backendModel),
	)

	httpResp, err := a.client.Send(ctx, backendReq)
	if err != nil {
		a.logger.Error("上游流式请求发送失败(透传模式)",
			port.String("req_id", reqID),
			port.String("backend", backend.Name()),
			port.Error(err),
		)
		return nil, err
	}

	a.logger.Debug("收到上游流式响应(透传模式)",
		port.String("req_id", reqID),
		port.String("backend", backend.Name()),
		port.Int("status_code", httpResp.StatusCode),
	)

	if httpResp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()
		a.logger.Warn("上游流式请求返回错误状态码(透传模式)",
			port.String("req_id", reqID),
			port.String("backend", backend.Name()),
			port.Int("status_code", httpResp.StatusCode),
		)
		return nil, &BackendError{
			StatusCode: httpResp.StatusCode,
			Body:       string(respBody),
		}
	}

	return httpResp, nil
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
