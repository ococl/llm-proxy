package service

import (
	"encoding/json"
	"strings"

	"llm-proxy/domain/entity"
	domainerror "llm-proxy/domain/error"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

// ProtocolConverter 负责在不同 LLM API 协议之间转换请求和响应。
// 支持 OpenAI、Anthropic 等主流格式的互转。
type ProtocolConverter struct {
	systemPrompts map[string]string
	logger        port.Logger
}

// NewProtocolConverter 创建一个新的协议转换器。
// systemPrompts 是可选的模型系统提示映射，用于自动注入系统提示。
// logger 用于记录协议转换过程中的调试和错误信息。
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

// ToBackend 将请求转换为后端协议格式。
func (c *ProtocolConverter) ToBackend(req *entity.Request, protocol types.Protocol) (*entity.Request, error) {
	if req == nil {
		return nil, domainerror.NewInvalidRequest("请求不能为空")
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
	case types.ProtocolOpenAI, types.ProtocolAzure, types.ProtocolDeepSeek, types.ProtocolGroq:
		result, err = c.toOpenAIFormat(req)
	case types.ProtocolAnthropic:
		result, err = c.toAnthropicFormat(req)
	case types.ProtocolMistral, types.ProtocolCohere, types.ProtocolGoogle:
		// 这些协议目前使用 OpenAI 兼容格式
		result, err = c.toOpenAIFormat(req)
	default:
		result, err = c.toCustomFormat(req)
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

// FromBackend 将后端响应转换为标准格式。
func (c *ProtocolConverter) FromBackend(resp *entity.Response, protocol types.Protocol) (*entity.Response, error) {
	if resp == nil {
		return nil, domainerror.NewInvalidRequest("响应不能为空")
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
	case types.ProtocolOpenAI, types.ProtocolAzure, types.ProtocolDeepSeek, types.ProtocolGroq:
		result, err = c.fromOpenAIFormat(resp)
	case types.ProtocolAnthropic:
		result, err = c.fromAnthropicFormat(resp)
	case types.ProtocolMistral, types.ProtocolCohere, types.ProtocolGoogle:
		result, err = c.fromOpenAIFormat(resp)
	default:
		result, err = c.fromCustomFormat(resp)
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

// toOpenAIFormat 将请求转换为 OpenAI 兼容格式。
// 主要处理系统提示注入和多模态内容适配。
func (c *ProtocolConverter) toOpenAIFormat(req *entity.Request) (*entity.Request, error) {
	messages := req.Messages()

	// 检查是否需要注入系统提示
	modelKey := req.Model().String()
	if systemPrompt, ok := c.systemPrompts[modelKey]; ok && systemPrompt != "" {
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
			newMessages = append(newMessages, entity.NewMessage("system", systemPrompt))
			newMessages = append(newMessages, messages...)

			// 构建新请求
			builder := entity.NewRequestBuilder().
				ID(req.ID()).
				Model(req.Model()).
				Messages(newMessages).
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
				ClientProtocol(string(types.ProtocolOpenAI))

			return builder.BuildUnsafe(), nil
		}
	}

	// 无需转换，返回原始请求
	return req, nil
}

// toAnthropicFormat 将请求转换为 Anthropic 格式。
// Anthropic 协议的主要特点:
// 1. 使用独立的 system 字段而不是 role: system 消息
// 2. content 支持数组格式（多模态内容）
// 3. max_tokens 参数是必需的
// 4. 工具调用格式与 OpenAI 不同
func (c *ProtocolConverter) toAnthropicFormat(req *entity.Request) (*entity.Request, error) {
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

	maxTokens := req.MaxTokens()
	needsConversion := false

	if len(systemPrompts) > 0 {
		needsConversion = true
	}

	if len(nonSystemMessages) != len(messages) {
		needsConversion = true
	}

	if maxTokens == 0 {
		needsConversion = true
		maxTokens = 1024
	}

	if !needsConversion {
		return req, nil
	}

	// 构建 Anthropic 格式的请求
	// 注意: system 字段和工具调用转换在 HTTP 层处理
	// 这里我们只确保消息格式正确
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

	c.logger.Debug("Anthropic 协议转换完成",
		port.String("req_id", req.ID().String()),
		port.Int("original_messages", len(messages)),
		port.Int("system_prompts", len(systemPrompts)),
		port.Int("filtered_messages", len(nonSystemMessages)),
	)

	return builder.BuildUnsafe(), nil
}

// toCustomFormat 处理自定义协议（直通模式）
func (c *ProtocolConverter) toCustomFormat(req *entity.Request) (*entity.Request, error) {
	// 直通模式，不进行转换
	return req, nil
}

// fromOpenAIFormat 从 OpenAI 格式响应转换。
func (c *ProtocolConverter) fromOpenAIFormat(resp *entity.Response) (*entity.Response, error) {
	// OpenAI 格式是标准格式，直接返回
	// 后续可以在此添加响应规范化逻辑
	return resp, nil
}

// fromAnthropicFormat 从 Anthropic 格式响应转换。
// Anthropic 响应的主要特点:
// 1. 使用 content 数组格式而非单个文本
// 2. stop_reason 格式不同
// 3. usage 字段结构略有差异
func (c *ProtocolConverter) fromAnthropicFormat(resp *entity.Response) (*entity.Response, error) {
	// 检查是否需要转换 Anthropic 特定格式
	if len(resp.Choices) == 0 {
		return resp, nil
	}

	firstChoice := resp.FirstChoice()
	if firstChoice == nil {
		return resp, nil
	}

	// Anthropic 的 content 可能是数组，需要转换为字符串
	switch content := firstChoice.Message.Content.(type) {
	case []interface{}:
		// 将数组内容转换为字符串
		var result strings.Builder
		for _, item := range content {
			if itemStr, ok := item.(string); ok {
				result.WriteString(itemStr)
			} else if itemMap, ok := item.(map[string]interface{}); ok {
				if text, ok := itemMap["text"].(string); ok {
					result.WriteString(text)
				} else if itemBytes, err := json.Marshal(itemMap); err == nil {
					result.Write(itemBytes)
				}
			}
		}
		// 创建新的响应，使用转换后的内容
		normalizedContent := result.String()
		newChoice := entity.NewChoice(
			firstChoice.Index,
			entity.NewMessage(firstChoice.Message.Role, normalizedContent),
			firstChoice.FinishReason,
		)
		return entity.NewResponseBuilder().
			ID(resp.ID).
			Model(resp.Model).
			Created(resp.Created).
			Choices([]entity.Choice{newChoice}).
			Usage(resp.Usage).
			BuildUnsafe(), nil
	case string:
		// 已经是字符串，无需转换
		return resp, nil
	default:
		// 其他类型，尝试转换为字符串
		if contentBytes, err := json.Marshal(content); err == nil {
			newContent := string(contentBytes)
			newChoice := entity.NewChoice(
				firstChoice.Index,
				entity.NewMessage(firstChoice.Message.Role, newContent),
				firstChoice.FinishReason,
			)
			return entity.NewResponseBuilder().
				ID(resp.ID).
				Model(resp.Model).
				Created(resp.Created).
				Choices([]entity.Choice{newChoice}).
				Usage(resp.Usage).
				BuildUnsafe(), nil
		}
	}

	return resp, nil
}

// fromCustomFormat 处理自定义协议响应（直通模式）
func (c *ProtocolConverter) fromCustomFormat(resp *entity.Response) (*entity.Response, error) {
	return resp, nil
}

// ConvertToolCall 转换工具调用格式。
// 不同协议的工具调用格式略有不同，需要进行适配。
func (c *ProtocolConverter) ConvertToolCall(toolCall *entity.ToolCall, toProtocol types.Protocol) (*entity.ToolCall, error) {
	if toolCall == nil {
		return nil, nil
	}

	switch toProtocol {
	case types.ProtocolAnthropic:
		// Anthropic 工具调用格式与 OpenAI 不同
		// 需要在 HTTP 层进行完整转换，这里仅做基本适配
		return toolCall, nil
	default:
		return toolCall, nil
	}
}

// ConvertToolResult 转换工具结果格式。
func (c *ProtocolConverter) ConvertToolResult(toolResult any, toProtocol types.Protocol) (any, error) {
	switch toProtocol {
	case types.ProtocolAnthropic:
		// Anthropic 期望工具结果格式不同
		// 返回结构化的工具结果
		return toolResult, nil
	default:
		return toolResult, nil
	}
}

// MergeSystemPrompts 合并多个系统提示。
// 用于处理从多个来源获取的系统提示。
func (c *ProtocolConverter) MergeSystemPrompts(prompts []string) string {
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

// DefaultProtocolConverter 返回一个使用空系统提示的默认转换器。
func DefaultProtocolConverter() *ProtocolConverter {
	return NewProtocolConverter(nil, &port.NopLogger{})
}
