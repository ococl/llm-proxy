package service

import (
	"llm-proxy/domain/entity"
	domainerror "llm-proxy/domain/error"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

// ProtocolConverter converts requests and responses between protocols.
type ProtocolConverter struct {
	systemPrompts map[string]string
	logger        port.Logger
}

// NewProtocolConverter creates a new protocol converter.
func NewProtocolConverter(systemPrompts map[string]string, logger port.Logger) *ProtocolConverter {
	if systemPrompts == nil {
		systemPrompts = make(map[string]string)
	}
	if logger == nil {
		logger = &port.NopLogger{}
	}
	return &ProtocolConverter{
		systemPrompts: systemPrompts,
		logger:        logger,
	}
}

// ToBackend converts a request to the backend protocol format.
func (c *ProtocolConverter) ToBackend(req *entity.Request, protocol types.Protocol) (*entity.Request, error) {
	if req == nil {
		return nil, domainerror.NewInvalidRequest("request is nil")
	}

	c.logger.Debug("开始协议转换（请求）",
		port.String("req_id", req.ID().String()),
		port.String("target_protocol", string(protocol)),
		port.String("model", req.Model().String()),
		port.Int("message_count", len(req.Messages())),
	)

	var result *entity.Request
	var err error

	switch protocol {
	case types.ProtocolOpenAI:
		result, err = c.toOpenAIFormat(req)
	case types.ProtocolAnthropic:
		result, err = c.toAnthropicFormat(req)
	default:
		result, err = req, nil
	}

	if err != nil {
		c.logger.Error("协议转换失败（请求）",
			port.String("req_id", req.ID().String()),
			port.String("target_protocol", string(protocol)),
			port.Error(err),
		)
		return nil, err
	}

	c.logger.Debug("协议转换完成（请求）",
		port.String("req_id", req.ID().String()),
		port.String("target_protocol", string(protocol)),
		port.Int("result_message_count", len(result.Messages())),
		port.Bool("system_prompt_injected", len(result.Messages()) > len(req.Messages())),
	)

	return result, nil
}

// FromBackend converts a response from the backend protocol format.
func (c *ProtocolConverter) FromBackend(resp *entity.Response, protocol types.Protocol) (*entity.Response, error) {
	if resp == nil {
		return nil, domainerror.NewInvalidRequest("response is nil")
	}

	c.logger.Debug("开始协议转换（响应）",
		port.String("response_id", resp.ID),
		port.String("source_protocol", string(protocol)),
		port.Int("choice_count", len(resp.Choices)),
		port.Int("prompt_tokens", resp.Usage.PromptTokens),
		port.Int("completion_tokens", resp.Usage.CompletionTokens),
	)

	var result *entity.Response
	var err error

	switch protocol {
	case types.ProtocolOpenAI:
		result, err = c.fromOpenAIFormat(resp)
	case types.ProtocolAnthropic:
		result, err = c.fromAnthropicFormat(resp)
	default:
		result, err = resp, nil
	}

	if err != nil {
		c.logger.Error("协议转换失败（响应）",
			port.String("response_id", resp.ID),
			port.String("source_protocol", string(protocol)),
			port.Error(err),
		)
		return nil, err
	}

	c.logger.Debug("协议转换完成（响应）",
		port.String("response_id", resp.ID),
		port.String("source_protocol", string(protocol)),
	)

	return result, nil
}

// toOpenAIFormat converts a request to OpenAI format.
func (c *ProtocolConverter) toOpenAIFormat(req *entity.Request) (*entity.Request, error) {
	// Inject system prompt if configured for this model
	modelKey := req.Model().String()
	if systemPrompt, ok := c.systemPrompts[modelKey]; ok && systemPrompt != "" {
		// Prepend system message
		messages := make([]entity.Message, 0, len(req.Messages())+1)
		messages = append(messages, entity.NewMessage("system", systemPrompt))
		messages = append(messages, req.Messages()...)

		// Create new request with injected system prompt
		builder := entity.NewRequestBuilder().
			ID(req.ID()).
			Model(req.Model()).
			Messages(messages).
			MaxTokens(req.MaxTokens()).
			Temperature(req.Temperature()).
			TopP(req.TopP()).
			Stream(req.IsStream()).
			Stop(req.Stop()).
			Tools(req.Tools()).
			ToolChoice(req.ToolChoice()).
			User(req.User()).
			Context(req.Context()).
			StreamHandler(req.StreamHandler())

		return builder.BuildUnsafe(), nil
	}

	return req, nil
}

// toAnthropicFormat converts a request to Anthropic format.
// Anthropic 使用 system 字段而不是 role: system 消息,
// content 是数组格式而不是简单的字符串。
// TODO: 实现完整的 Anthropic 协议转换,包括:
// - 将 system 消息提取到单独的 system 字段
// - 将 content 转换为 Anthropic 的数组格式
// - 处理 max_tokens 参数(Anthropic 必须提供)
func (c *ProtocolConverter) toAnthropicFormat(req *entity.Request) (*entity.Request, error) {
	messages := req.Messages()
	if len(messages) == 0 {
		return req, nil
	}

	// 检查是否需要转换:有系统消息且有其他消息
	hasSystemPrompt := false
	for _, msg := range messages {
		if msg.Role == "system" {
			hasSystemPrompt = true
			break
		}
	}

	// 如果没有系统消息,直接返回
	if !hasSystemPrompt {
		return req, nil
	}

	// Anthropic 要求提供 max_tokens 参数
	maxTokens := req.MaxTokens()
	if maxTokens == 0 {
		maxTokens = 1024
	}

	// 构建新请求,设置 max_tokens
	builder := entity.NewRequestBuilder().
		ID(req.ID()).
		Model(req.Model()).
		Messages(req.Messages()).
		MaxTokens(maxTokens).
		Temperature(req.Temperature()).
		TopP(req.TopP()).
		Stream(req.IsStream()).
		Stop(req.Stop()).
		Tools(req.Tools()).
		ToolChoice(req.ToolChoice()).
		User(req.User()).
		Context(req.Context()).
		StreamHandler(req.StreamHandler())

	return builder.BuildUnsafe(), nil
}

// fromOpenAIFormat converts a response from OpenAI format.
func (c *ProtocolConverter) fromOpenAIFormat(resp *entity.Response) (*entity.Response, error) {
	// For now, pass through
	return resp, nil
}

// fromAnthropicFormat converts a response from Anthropic format.
func (c *ProtocolConverter) fromAnthropicFormat(resp *entity.Response) (*entity.Response, error) {
	// For now, pass through
	return resp, nil
}

// DefaultProtocolConverter returns a converter with no system prompts.
func DefaultProtocolConverter() *ProtocolConverter {
	return NewProtocolConverter(nil, &port.NopLogger{})
}
