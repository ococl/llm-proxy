package service

import (
	"encoding/json"

	"llm-proxy/domain/entity"
	domainerror "llm-proxy/domain/error"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"

	"llm-proxy/application/service/protocol"
)

// ProtocolConverter 负责在不同 LLM API 协议之间转换请求和响应。
// 支持 OpenAI、Anthropic、Google Vertex AI 等主流格式的互转。
// 使用 Strategy Pattern 实现，通过 StrategyRegistry 管理各种协议的转换策略。
type ProtocolConverter struct {
	registry      *protocol.StrategyRegistry
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

	factory := protocol.NewStrategyFactory(logger)
	registry := factory.CreateDefaultRegistry()

	return &ProtocolConverter{
		registry:      registry,
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

	// 获取请求转换策略
	strategy := c.registry.GetRequestStrategy(protocol)
	if strategy == nil {
		c.logger.Warn("未找到请求转换策略，使用直通模式",
			port.String("protocol", string(protocol)),
		)
		return req, nil
	}

	// 获取系统提示
	systemPrompt := c.systemPrompts[req.Model().String()]

	// 执行转换
	result, err := strategy.Convert(req, systemPrompt)
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
func (c *ProtocolConverter) FromBackend(respBody []byte, model string, protocol types.Protocol) (*entity.Response, error) {
	if len(respBody) == 0 {
		return nil, domainerror.NewInvalidRequest("响应不能为空")
	}

	c.logger.Debug("开始协议转换（响应）",
		port.String("source_protocol", string(protocol)),
		port.String("model", model),
	)

	// 获取响应转换策略
	strategy := c.registry.GetResponseStrategy(protocol)
	if strategy == nil {
		c.logger.Warn("未找到响应转换策略，尝试直接解析",
			port.String("protocol", string(protocol)),
		)
		// 尝试直接解析为标准格式
		var resp entity.Response
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return nil, err
		}
		return &resp, nil
	}

	// 执行转换
	result, err := strategy.Convert(respBody, model)
	if err != nil {
		c.logger.Error("协议转换失败（响应）",
			port.String("source_protocol", string(protocol)),
			port.Error(err),
		)
		return nil, err
	}

	c.logger.Debug("协议转换完成（响应）",
		port.String("source_protocol", string(protocol)),
		port.String("response_id", result.ID),
	)

	return result, nil
}

// ConvertToolCall 转换工具调用格式。
// 不同协议的工具调用格式略有不同，需要进行适配。
func (c *ProtocolConverter) ConvertToolCall(toolCall *entity.ToolCall, toProtocol types.Protocol) (*entity.ToolCall, error) {
	if toolCall == nil {
		return nil, nil
	}

	switch toProtocol {
	case types.ProtocolAnthropic:
		return toolCall, nil
	default:
		return toolCall, nil
	}
}

// ConvertToolResult 转换工具结果格式。
func (c *ProtocolConverter) ConvertToolResult(toolResult any, toProtocol types.Protocol) (any, error) {
	switch toProtocol {
	case types.ProtocolAnthropic:
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
	var result string
	for i, prompt := range prompts {
		if i > 0 {
			result += "\n\n"
		}
		result += prompt
	}
	return result
}

// DefaultProtocolConverter 返回一个使用空系统提示的默认转换器。
func DefaultProtocolConverter() *ProtocolConverter {
	return NewProtocolConverter(nil, &port.NopLogger{})
}
