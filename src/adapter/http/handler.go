package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"llm-proxy/application/usecase"
	"llm-proxy/domain/entity"
	domainerror "llm-proxy/domain/error"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

type ProxyHandler struct {
	proxyUseCase   *usecase.ProxyRequestUseCase
	config         port.ConfigProvider
	logger         port.Logger
	errorPresenter *ErrorPresenter
}

func NewProxyHandler(
	proxyUseCase *usecase.ProxyRequestUseCase,
	config port.ConfigProvider,
	logger port.Logger,
	errorPresenter *ErrorPresenter,
) *ProxyHandler {
	return &ProxyHandler{
		proxyUseCase:   proxyUseCase,
		config:         config,
		logger:         logger,
		errorPresenter: errorPresenter,
	}
}

func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	reqID := h.generateRequestID()

	h.logger.Info("收到客户端请求",
		port.String("req_id", reqID),
		port.String("method", r.Method),
		port.String("path", r.URL.Path),
		port.String("remote_addr", r.RemoteAddr),
	)

	cfg := h.config.Get()

	clientProtocol := h.detectProtocol(r)

	h.logger.Debug("检测客户端协议",
		port.String("req_id", reqID),
		port.String("protocol", string(clientProtocol)),
	)

	if cfg.ProxyAPIKey != "" {
		if !h.validateAPIKey(r, cfg.ProxyAPIKey, clientProtocol) {
			h.errorPresenter.WriteError(w, r, domainerror.NewUnauthorized("无效的 API Key"))
			return
		}
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("读取请求体失败",
			port.String("req_id", reqID),
			port.Error(err),
		)
		h.errorPresenter.WriteError(w, r, domainerror.NewBadRequest("无法读取请求体"))
		return
	}
	defer r.Body.Close()

	h.logger.Debug("请求体读取成功",
		port.String("req_id", reqID),
		port.Int("body_size", len(bodyBytes)),
	)

	var reqBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
		h.logger.Error("解析请求体JSON失败",
			port.String("req_id", reqID),
			port.Error(err),
		)
		h.errorPresenter.WriteError(w, r, domainerror.NewInvalidJSON(err))
		return
	}

	h.logger.Debug("请求体解析成功",
		port.String("req_id", reqID),
	)

	// 记录完整的客户端请求体（用于调试追踪）
	if reqBodyJSON, err := json.Marshal(reqBody); err == nil {
		h.logger.Debug("客户端请求体",
			port.String("req_id", reqID),
			port.String("request_body", string(reqBodyJSON)),
		)
	}

	if cfg.Proxy.EnableSystemPrompt {
		reqBody = h.injectSystemPrompt(reqBody)
	}

	req, err := h.buildDomainRequest(ctx, reqID, reqBody, clientProtocol, r.Header)
	if err != nil {
		h.logger.Error("构建领域请求失败",
			port.String("req_id", reqID),
			port.Error(err),
		)
		h.errorPresenter.WriteError(w, r, err)
		return
	}

	h.logger.Debug("领域请求构建完成",
		port.String("req_id", reqID),
		port.String("model", req.Model().String()),
		port.Bool("stream", req.IsStream()),
	)

	isStream := h.isStreamRequest(reqBody)

	if isStream {
		h.handleStreamingRequest(w, r, req)
	} else {
		h.handleNonStreamingRequest(w, r, req)
	}
}

func (h *ProxyHandler) handleNonStreamingRequest(w http.ResponseWriter, r *http.Request, req *entity.Request) {
	h.logger.Debug("开始处理非流式请求",
		port.String("req_id", req.ID().String()),
		port.String("model", req.Model().String()),
	)

	resp, err := h.proxyUseCase.Execute(r.Context(), req)
	if err != nil {
		h.logger.Error("非流式请求处理失败",
			port.String("req_id", req.ID().String()),
			port.String("model", req.Model().String()),
			port.Error(err),
		)
		h.errorPresenter.WriteError(w, r, err)
		return
	}

	h.logger.Info("非流式请求处理成功",
		port.String("req_id", req.ID().String()),
		port.String("model", req.Model().String()),
		port.String("response_id", resp.ID),
	)

	if respJSON, err := json.Marshal(resp); err == nil {
		h.logger.Debug("客户端响应体",
			port.String("req_id", req.ID().String()),
			port.String("response_body", string(respJSON)),
		)
	}

	h.writeResponse(w, resp)
}

func (h *ProxyHandler) handleStreamingRequest(w http.ResponseWriter, r *http.Request, req *entity.Request) {
	h.logger.Debug("开始处理流式请求",
		port.String("req_id", req.ID().String()),
		port.String("model", req.Model().String()),
	)

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.logger.Error("响应写入器不支持流式传输",
			port.String("req_id", req.ID().String()),
		)
		h.errorPresenter.WriteError(w, r, domainerror.NewInternalError("streaming not supported", nil))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	var headersWritten bool
	streamHandler := func(resp *entity.Response) error {
		if !headersWritten {
			for k, v := range resp.Headers {
				for _, val := range v {
					w.Header().Add(k, val)
				}
			}
			w.WriteHeader(http.StatusOK)
			headersWritten = true
		}

		respJSON, err := json.Marshal(resp)
		if err != nil {
			return err
		}
		if len(respJSON) > 0 {
			if _, err := w.Write([]byte("data: ")); err != nil {
				return err
			}
			if _, err := w.Write(respJSON); err != nil {
				return err
			}
			if _, err := w.Write([]byte("\n\n")); err != nil {
				return err
			}
			flusher.Flush()
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
	defer cancel()

	if err := h.proxyUseCase.ExecuteStreaming(ctx, req, streamHandler); err != nil {
		h.logger.Error("流式请求处理失败",
			port.String("req_id", req.ID().String()),
			port.String("model", req.Model().String()),
			port.Error(err),
		)
		return
	}

	h.logger.Info("流式请求处理成功",
		port.String("req_id", req.ID().String()),
		port.String("model", req.Model().String()),
	)

	w.Write([]byte("data: [DONE]\n\n"))
	flusher.Flush()
}

func (h *ProxyHandler) writeResponse(w http.ResponseWriter, resp *entity.Response) {
	for k, v := range resp.Headers {
		if k == "Content-Length" || k == "Transfer-Encoding" {
			continue
		}
		for _, val := range v {
			w.Header().Add(k, val)
		}
	}

	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}

	respJSON, err := json.Marshal(resp)
	if err != nil {
		h.logger.Error("序列化响应失败", port.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(respJSON)))
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(respJSON); err != nil {
		h.logger.Error("写入响应失败", port.Error(err))
	}
}

func (h *ProxyHandler) buildDomainRequest(
	ctx context.Context,
	reqID string,
	reqBody map[string]interface{},
	clientProtocol types.Protocol,
	clientHeaders http.Header,
) (*entity.Request, error) {
	modelAlias, ok := reqBody["model"].(string)
	if !ok || modelAlias == "" {
		return nil, domainerror.NewMissingModel()
	}

	messages, err := h.extractMessages(reqBody)
	if err != nil {
		return nil, err
	}

	builder := entity.NewRequestBuilder().
		ID(entity.NewRequestID(reqID)).
		Model(entity.NewModelAlias(modelAlias)).
		Messages(messages).
		Context(ctx).
		Headers(extractForwardHeaders(clientHeaders)).
		ClientProtocol(string(clientProtocol))

	if maxTokens, ok := reqBody["max_tokens"].(float64); ok {
		builder.MaxTokens(int(maxTokens))
	}

	if temperature, ok := reqBody["temperature"].(float64); ok {
		builder.Temperature(temperature)
	}

	if topP, ok := reqBody["top_p"].(float64); ok {
		builder.TopP(topP)
	}

	if stream, ok := reqBody["stream"].(bool); ok {
		builder.Stream(stream)
	}

	if stop, ok := reqBody["stop"].([]interface{}); ok {
		stopStrings := make([]string, 0, len(stop))
		for _, s := range stop {
			if str, ok := s.(string); ok {
				stopStrings = append(stopStrings, str)
			}
		}
		builder.Stop(stopStrings)
	}

	if tools, ok := reqBody["tools"].([]interface{}); ok {
		domainTools, err := h.extractTools(tools)
		if err != nil {
			return nil, err
		}
		builder.Tools(domainTools)
	}

	if user, ok := reqBody["user"].(string); ok {
		builder.User(user)
	}

	return builder.Build()
}

func (h *ProxyHandler) extractMessages(reqBody map[string]interface{}) ([]entity.Message, error) {
	messagesRaw, ok := reqBody["messages"].([]interface{})
	if !ok {
		return nil, domainerror.NewBadRequest("缺少 messages 字段")
	}

	messages := make([]entity.Message, 0, len(messagesRaw))
	for _, msgRaw := range messagesRaw {
		msgMap, ok := msgRaw.(map[string]interface{})
		if !ok {
			continue
		}

		role, _ := msgMap["role"].(string)
		content, _ := msgMap["content"].(string)

		msg := entity.NewMessage(role, content)

		if toolCalls, ok := msgMap["tool_calls"].([]interface{}); ok {
			domainToolCalls := make([]entity.ToolCall, 0, len(toolCalls))
			for _, tc := range toolCalls {
				if tcMap, ok := tc.(map[string]interface{}); ok {
					toolCall := entity.ToolCall{
						ID:   getString(tcMap, "id"),
						Type: getString(tcMap, "type"),
					}
					if fn, ok := tcMap["function"].(map[string]interface{}); ok {
						toolCall.Function = entity.ToolCallFunction{
							Name:      getString(fn, "name"),
							Arguments: getString(fn, "arguments"),
						}
					}
					domainToolCalls = append(domainToolCalls, toolCall)
				}
			}
			msg.ToolCalls = domainToolCalls
		}

		if toolCallID, ok := msgMap["tool_call_id"].(string); ok {
			msg.ToolCallID = toolCallID
		}

		messages = append(messages, msg)
	}

	return messages, nil
}

func (h *ProxyHandler) extractTools(toolsRaw []interface{}) ([]entity.Tool, error) {
	tools := make([]entity.Tool, 0, len(toolsRaw))
	for _, toolRaw := range toolsRaw {
		toolMap, ok := toolRaw.(map[string]interface{})
		if !ok {
			continue
		}

		tool := entity.Tool{
			Type: getString(toolMap, "type"),
		}

		if fn, ok := toolMap["function"].(map[string]interface{}); ok {
			tool.Function = entity.ToolFunction{
				Name:        getString(fn, "name"),
				Description: getString(fn, "description"),
			}
			if params, ok := fn["parameters"].(map[string]any); ok {
				tool.Function.Parameters = params
			}
		}

		tools = append(tools, tool)
	}

	return tools, nil
}

func (h *ProxyHandler) detectProtocol(r *http.Request) types.Protocol {
	if r.Header.Get("anthropic-version") != "" || r.Header.Get("x-api-key") != "" {
		return types.ProtocolAnthropic
	}
	return types.ProtocolOpenAI
}

func (h *ProxyHandler) validateAPIKey(r *http.Request, expectedKey string, protocol types.Protocol) bool {
	if protocol == types.ProtocolAnthropic {
		providedKey := r.Header.Get("x-api-key")
		return providedKey == expectedKey
	}

	auth := r.Header.Get("Authorization")
	expected := "Bearer " + expectedKey
	return auth == expected
}

func (h *ProxyHandler) injectSystemPrompt(reqBody map[string]interface{}) map[string]interface{} {
	return reqBody
}

func (h *ProxyHandler) isStreamRequest(reqBody map[string]interface{}) bool {
	stream, ok := reqBody["stream"].(bool)
	return ok && stream
}

func (h *ProxyHandler) generateRequestID() string {
	now := time.Now()
	timestamp := now.UnixMilli()
	reqIDStr := strconv.FormatInt(timestamp, 16)
	if len(reqIDStr) > 10 {
		reqIDStr = reqIDStr[len(reqIDStr)-10:]
	}
	return reqIDStr
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func extractForwardHeaders(clientHeaders http.Header) map[string][]string {
	headers := make(map[string][]string)
	forwardHeaders := []string{
		"X-Request-ID",
		"X-Forwarded-For",
		"X-Real-IP",
		"User-Agent",
		"Accept",
		"Accept-Language",
		"Accept-Encoding",
	}

	for _, key := range forwardHeaders {
		if values := clientHeaders.Values(key); len(values) > 0 {
			headers[key] = values
		}
	}

	if len(headers["User-Agent"]) == 0 {
		headers["User-Agent"] = []string{"opencode/1.1.34 ai-sdk/provider-utils/3.0.20 runtime/bun/1.3.5"}
	}

	return headers
}
