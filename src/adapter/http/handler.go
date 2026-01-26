package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
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
	bodyLogger     port.BodyLogger
	errorPresenter *ErrorPresenter
}

func NewProxyHandler(
	proxyUseCase *usecase.ProxyRequestUseCase,
	config port.ConfigProvider,
	logger port.Logger,
	bodyLogger port.BodyLogger,
	errorPresenter *ErrorPresenter,
) *ProxyHandler {
	return &ProxyHandler{
		proxyUseCase:   proxyUseCase,
		config:         config,
		logger:         logger,
		bodyLogger:     bodyLogger,
		errorPresenter: errorPresenter,
	}
}

// getRealIP 从请求头中获取真实客户端 IP。
// 优先使用 X-Real-Ip，其次使用 X-Forwarded-For，最后回退到 RemoteAddr。
func getRealIP(r *http.Request) string {
	// 优先检查 X-Real-Ip 头
	realIP := r.Header.Get("X-Real-Ip")
	if realIP != "" {
		return realIP
	}

	// 检查 X-Forwarded-For 头（可能包含多个 IP，取第一个）
	forwardedFor := r.Header.Get("X-Forwarded-For")
	if forwardedFor != "" {
		// X-Forwarded-For 格式: "client, proxy1, proxy2"
		// 取第一个逗号之前的部分作为真实客户端 IP
		if idx := strings.Index(forwardedFor, ","); idx != -1 {
			return strings.TrimSpace(forwardedFor[:idx])
		}
		return forwardedFor
	}

	// 回退到 RemoteAddr
	return r.RemoteAddr
}

func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	reqID := h.generateRequestID()

	h.logger.Info("收到客户端请求",
		port.ReqID(reqID),
		port.Method(r.Method),
		port.Path(r.URL.Path),
		port.RemoteAddr(getRealIP(r)),
	)

	cfg := h.config.Get()

	clientProtocol := h.detectProtocol(r)

	h.logger.Debug("检测客户端协议",
		port.ReqID(reqID),
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
			port.ReqID(reqID),
			port.Error(err),
		)
		h.errorPresenter.WriteError(w, r, domainerror.NewBadRequest("无法读取请求体"))
		return
	}
	defer r.Body.Close()

	h.logger.Debug("请求体读取成功",
		port.ReqID(reqID),
		port.BodySize(len(bodyBytes)),
	)

	var reqBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
		h.logger.Error("解析请求体JSON失败",
			port.ReqID(reqID),
			port.Error(err),
		)
		h.errorPresenter.WriteError(w, r, domainerror.NewInvalidJSON(err))
		return
	}

	h.logger.Debug("请求体解析成功",
		port.ReqID(reqID),
	)

	h.bodyLogger.LogRequestBody(reqID, port.BodyLogTypeClientRequest, r.Method, r.URL.Path, r.Proto, map[string][]string(r.Header), reqBody)

	if cfg.Proxy.EnableSystemPrompt {
		reqBody = h.injectSystemPrompt(reqBody)
	}

	req, err := h.buildDomainRequest(ctx, reqID, reqBody, clientProtocol, r.Header)
	if err != nil {
		h.logger.Error("构建领域请求失败",
			port.ReqID(reqID),
			port.Error(err),
		)
		h.errorPresenter.WriteError(w, r, err)
		return
	}

	h.logger.Debug("领域请求构建完成",
		port.ReqID(reqID),
		port.Model(req.Model().String()),
		port.Streaming(req.IsStream()),
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
		port.ReqID(req.ID().String()),
		port.Model(req.Model().String()),
	)

	resp, err := h.proxyUseCase.Execute(r.Context(), req)
	if err != nil {
		h.logger.Error("非流式请求处理失败",
			port.ReqID(req.ID().String()),
			port.Model(req.Model().String()),
			port.Error(err),
		)
		h.errorPresenter.WriteError(w, r, err)
		return
	}

	h.logger.Info("非流式请求处理成功",
		port.ReqID(req.ID().String()),
		port.Model(req.Model().String()),
		port.ResponseID(resp.ID),
	)

	h.bodyLogger.LogResponseBody(req.ID().String(), port.BodyLogTypeClientResponse, http.StatusOK, resp.Headers, resp)

	h.writeResponse(w, resp)
}

func (h *ProxyHandler) handleStreamingRequest(w http.ResponseWriter, r *http.Request, req *entity.Request) {
	h.logger.Debug("开始处理流式请求",
		port.ReqID(req.ID().String()),
		port.Model(req.Model().String()),
	)

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.logger.Error("响应写入器不支持streaming传输",
			port.ReqID(req.ID().String()),
		)
		h.errorPresenter.WriteError(w, r, domainerror.NewInternalError("streaming not supported", nil))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	var sseChunks []byte
	passthroughHandler := func(chunk []byte) error {
		if _, err := w.Write(chunk); err != nil {
			h.logger.Error("写入streaming数据块失败",
				port.ReqID(req.ID().String()),
				port.Error(err),
			)
			return err
		}
		sseChunks = append(sseChunks, chunk...)
		flusher.Flush()
		return nil
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
	defer cancel()

	if err := h.proxyUseCase.ExecuteStreamingPassthrough(ctx, req, passthroughHandler); err != nil {
		h.logger.Error("流式请求处理失败",
			port.ReqID(req.ID().String()),
			port.Model(req.Model().String()),
			port.Error(err),
		)
		return
	}

	lines := strings.Split(string(sseChunks), "\n")
	validChunks := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "data:") || line == "" || strings.HasPrefix(line, ":") {
			validChunks++
		}
	}

	h.logger.Info("流式请求处理成功",
		port.ReqID(req.ID().String()),
		port.Model(req.Model().String()),
		port.Field{Key: "chunks", Value: validChunks},
		port.Field{Key: "bytes", Value: len(sseChunks)},
	)

	h.bodyLogger.LogResponseBody(req.ID().String(), port.BodyLogTypeClientResponse, http.StatusOK, w.Header(), string(sseChunks))
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

	// 处理 tool_choice 字段
	if toolChoice, ok := reqBody["tool_choice"]; ok && toolChoice != nil {
		builder.ToolChoice(toolChoice)
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

	if len(messagesRaw) == 0 {
		return nil, domainerror.NewBadRequest("messages 数组不能为空")
	}

	messages := make([]entity.Message, 0, len(messagesRaw))
	for i, msgRaw := range messagesRaw {
		msgMap, ok := msgRaw.(map[string]interface{})
		if !ok {
			return nil, domainerror.NewBadRequest(fmt.Sprintf("messages[%d] 必须是一个对象", i))
		}

		role, ok := msgMap["role"].(string)
		if !ok || role == "" {
			return nil, domainerror.NewBadRequest(fmt.Sprintf("messages[%d] 缺少有效的 role 字段", i))
		}

		// 支持任意类型的 content（字符串、数组多模态内容、对象等）
		content := msgMap["content"]

		msg := entity.NewMessageWithContent(role, content)

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

		// 处理 cache_control 字段（Anthropic API 缓存控制）
		if cacheControl, ok := msgMap["cache_control"]; ok && cacheControl != nil {
			msg.CacheControl = cacheControl
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
		if tool.Type == "" {
			tool.Type = "function"
		}

		// 处理函数类型的工具
		if tool.Type == "function" {
			fn, ok := toolMap["function"].(map[string]interface{})
			if !ok {
				// function 类型工具必须有 function 字段
				continue
			}

			name := getString(fn, "name")
			if name == "" {
				// function.name 是必填字段，为空则跳过此工具
				continue
			}

			tool.Function = entity.ToolFunction{
				Name:        name,
				Description: getString(fn, "description"),
			}
			if params, ok := fn["parameters"].(map[string]any); ok {
				tool.Function.Parameters = params
			}

			tools = append(tools, tool)
		} else {
			tools = append(tools, tool)
		}
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
