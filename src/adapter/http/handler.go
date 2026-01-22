package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"llm-proxy/application/usecase"
	"llm-proxy/domain/entity"
	domainerror "llm-proxy/domain/error"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"

	"github.com/google/uuid"
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

	cfg := h.config.Get()

	clientProtocol := h.detectProtocol(r)

	if cfg.ProxyAPIKey != "" {
		if !h.validateAPIKey(r, cfg.ProxyAPIKey, clientProtocol) {
			h.errorPresenter.WriteError(w, r, domainerror.NewUnauthorized("无效的 API Key"))
			return
		}
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		h.errorPresenter.WriteError(w, r, domainerror.NewBadRequest("无法读取请求体"))
		return
	}
	defer r.Body.Close()

	var reqBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
		h.errorPresenter.WriteError(w, r, domainerror.NewInvalidJSON(err))
		return
	}

	if cfg.Proxy.EnableSystemPrompt {
		reqBody = h.injectSystemPrompt(reqBody)
	}

	req, err := h.buildDomainRequest(ctx, reqID, reqBody, clientProtocol)
	if err != nil {
		h.errorPresenter.WriteError(w, r, err)
		return
	}

	isStream := h.isStreamRequest(reqBody)

	if isStream {
		h.handleStreamingRequest(w, r, req)
	} else {
		h.handleNonStreamingRequest(w, r, req)
	}
}

func (h *ProxyHandler) handleNonStreamingRequest(w http.ResponseWriter, r *http.Request, req *entity.Request) {
	resp, err := h.proxyUseCase.Execute(r.Context(), req)
	if err != nil {
		h.errorPresenter.WriteError(w, r, err)
		return
	}

	h.writeResponse(w, resp)
}

func (h *ProxyHandler) handleStreamingRequest(w http.ResponseWriter, r *http.Request, req *entity.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.errorPresenter.WriteError(w, r, domainerror.NewInternalError("streaming not supported", nil))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	streamHandler := func(resp *entity.Response) error {
		respJSON, err := json.Marshal(resp)
		if err != nil {
			return err
		}
		if len(respJSON) > 0 {
			if _, err := w.Write(respJSON); err != nil {
				return err
			}
			flusher.Flush()
		}
		return nil
	}

	if err := h.proxyUseCase.ExecuteStreaming(r.Context(), req, streamHandler); err != nil {
		h.logger.Error("streaming request failed", port.Error(err))
		return
	}
}

func (h *ProxyHandler) writeResponse(w http.ResponseWriter, resp *entity.Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	respJSON, err := json.Marshal(resp)
	if err != nil {
		h.logger.Error("failed to marshal response", port.Error(err))
		return
	}

	if _, err := w.Write(respJSON); err != nil {
		h.logger.Error("failed to write response", port.Error(err))
	}
}

func (h *ProxyHandler) buildDomainRequest(
	ctx context.Context,
	reqID string,
	reqBody map[string]interface{},
	clientProtocol types.Protocol,
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
		Context(ctx)

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
	return "req_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:18]
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
