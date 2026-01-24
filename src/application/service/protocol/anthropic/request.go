package anthropic

import (
	"strings"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

// RequestConverter Anthropic 协议的请求转换策略。
// 负责将标准请求格式转换为 Anthropic 格式。
type RequestConverter struct {
	logger        port.Logger
	systemPrompts map[string]string
}

// NewRequestConverter 创建 Anthropic 请求转换策略实例。
//
// Anthropic 格式特点：
//   - system prompt 作为独立字段，而非 role: system 消息
//   - content 支持数组格式（多模态内容）
//   - max_tokens 参数是必需的
//   - 工具调用格式与 OpenAI 不同
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

// Convert 将请求转换为 Anthropic 格式。
func (c *RequestConverter) Convert(req *entity.Request, systemPrompt string) (*entity.Request, error) {
	if req == nil {
		return nil, nil
	}

	messages := req.Messages()
	if len(messages) == 0 {
		return req, nil
	}

	// 提取并合并所有 system 消息
	var systemPrompts []string
	var nonSystemMessages []entity.Message

	for _, msg := range messages {
		if msg.Role == "system" {
			// 提取 system 消息内容
			if content, ok := msg.Content.(string); ok && content != "" {
				systemPrompts = append(systemPrompts, content)
			}
		} else {
			// 保留非 system 消息
			nonSystemMessages = append(nonSystemMessages, msg)
		}
	}

	// 确定 max_tokens 值
	maxTokens := req.MaxTokens()
	if maxTokens == 0 {
		maxTokens = 1024 // Anthropic 要求必须设置 max_tokens
	}

	// 检查是否需要转换
	needsConversion := len(systemPrompts) > 0 ||
		len(nonSystemMessages) != len(messages) ||
		req.MaxTokens() == 0

	if !needsConversion {
		return req, nil
	}

	// 合并系统提示
	var finalSystemPrompt string
	if systemPrompt != "" {
		finalSystemPrompt = systemPrompt
	} else if len(systemPrompts) > 0 {
		finalSystemPrompt = c.mergeSystemPrompts(systemPrompts)
	} else if sp, ok := c.systemPrompts[req.Model().String()]; ok {
		finalSystemPrompt = sp
	}

	// 构建 Anthropic 格式的请求
	// 注意：tools 字段在 HTTP 层进行完整转换
	builder := entity.NewRequestBuilder().
		ID(req.ID()).
		Model(req.Model()).
		Messages(nonSystemMessages).
		MaxTokens(maxTokens).
		Temperature(req.Temperature()).
		TopP(req.TopP()).
		Stream(req.IsStream()).
		Stop(req.Stop()).
		// Anthropic 的 tools 格式不同，需要在 HTTP 层转换
		Tools(nil).
		ToolChoice(req.ToolChoice()).
		User(req.User()).
		Context(req.Context()).
		StreamHandler(req.StreamHandler()).
		Headers(req.Headers()).
		ClientProtocol(string(types.ProtocolAnthropic))

	// 如果有系统提示，添加到请求中
	if finalSystemPrompt != "" {
		// Anthropic 使用 system 字段存储系统提示
		builder.System(finalSystemPrompt)
	}

	c.logger.Debug("Anthropic 协议转换完成",
		port.String("req_id", req.ID().String()),
		port.Int("original_messages", len(messages)),
		port.Int("system_prompts", len(systemPrompts)),
		port.Int("filtered_messages", len(nonSystemMessages)),
	)

	return builder.BuildUnsafe(), nil
}

// mergeSystemPrompts 合并多个系统提示。
func (c *RequestConverter) mergeSystemPrompts(prompts []string) string {
	if len(prompts) == 0 {
		return ""
	}
	if len(prompts) == 1 {
		return prompts[0]
	}

	// 使用双换行符合并多个系统提示
	var result strings.Builder
	for i, prompt := range prompts {
		if i > 0 {
			result.WriteString("\n\n")
		}
		result.WriteString(prompt)
	}
	return result.String()
}

// Supports 检查是否支持指定协议。
func (c *RequestConverter) Supports(protocol types.Protocol) bool {
	return protocol == types.ProtocolAnthropic
}

// Protocol 返回支持的协议类型。
func (c *RequestConverter) Protocol() types.Protocol {
	return types.ProtocolAnthropic
}

// Name 返回策略名称。
func (c *RequestConverter) Name() string {
	return "AnthropicRequestConverter"
}
