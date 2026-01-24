package openai

import (
	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

// RequestConverter OpenAI 协议的请求转换策略。
// 负责将标准请求格式转换为 OpenAI 兼容格式。
type RequestConverter struct {
	logger        port.Logger
	systemPrompts map[string]string
}

// NewRequestConverter 创建 OpenAI 请求转换策略实例。
//
// 参数：
//   - systemPrompts: 模型名称到系统提示的映射（可选）
//   - logger: 日志记录器（可选）
//
// 返回：
//   - 初始化后的转换策略
func NewRequestConverter(systemPrompts map[string]string, logger port.Logger) *RequestConverter {
	if systemPrompts == nil {
		systemPrompts = make(map[string]string)
	}
	if logger == nil {
		logger = &port.NopLogger{}
	}
	return &RequestConverter{
		logger:        logger,
		systemPrompts: systemPrompts,
	}
}

// Convert 将请求转换为 OpenAI 格式。
//
// OpenAI 格式特点：
//   - 系统提示作为 role: system 的消息
//   - messages 为消息数组
//   - stream 为布尔值
//   - 工具调用使用 tool_calls 字段
func (c *RequestConverter) Convert(req *entity.Request, systemPrompt string) (*entity.Request, error) {
	if req == nil {
		return nil, nil
	}

	messages := req.Messages()

	// 检查是否需要注入系统提示
	var effectiveSystemPrompt string
	if systemPrompt != "" {
		effectiveSystemPrompt = systemPrompt
	} else if sp, ok := c.systemPrompts[req.Model().String()]; ok {
		effectiveSystemPrompt = sp
	}

	if effectiveSystemPrompt != "" {
		// 检查是否已有系统消息
		hasSystemMessage := false
		for _, msg := range messages {
			if msg.Role == "system" {
				hasSystemMessage = true
				break
			}
		}

		if !hasSystemMessage {
			// 前置系统消息
			newMessages := make([]entity.Message, 0, len(messages)+1)
			newMessages = append(newMessages, entity.NewMessage("system", effectiveSystemPrompt))
			newMessages = append(newMessages, messages...)

			return c.buildRequest(req, newMessages), nil
		}
	}

	// 无需转换，返回原始请求
	return req, nil
}

// buildRequest 构建 OpenAI 格式的请求。
func (c *RequestConverter) buildRequest(req *entity.Request, messages []entity.Message) *entity.Request {
	return entity.NewRequestBuilder().
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
		StreamHandler(req.StreamHandler()).
		Headers(req.Headers()).
		ClientProtocol(string(types.ProtocolOpenAI)).
		BuildUnsafe()
}

// Supports 检查是否支持指定协议。
func (c *RequestConverter) Supports(protocol types.Protocol) bool {
	return protocol.IsOpenAICompatible()
}

// Protocol 返回支持的协议类型。
func (c *RequestConverter) Protocol() types.Protocol {
	return types.ProtocolOpenAI
}

// Name 返回策略名称。
func (c *RequestConverter) Name() string {
	return "OpenAIRequestConverter"
}
