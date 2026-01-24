package anthropic

import (
	"encoding/json"
	"strings"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

// ResponseConverter Anthropic 协议的响应转换策略。
// 负责将 Anthropic 响应格式转换为标准格式。
type ResponseConverter struct {
	logger port.Logger
}

// AnthropicResponse Anthropic API 响应格式。
type AnthropicResponse struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Content      []AnthropicContentBlock `json:"content"`
	Model        string                  `json:"model"`
	StopReason   string                  `json:"stop_reason"`
	StopSequence string                  `json:"stop_sequence"`
	Usage        AnthropicUsage          `json:"usage"`
}

// AnthropicContentBlock Anthropic 内容块。
type AnthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// AnthropicUsage Anthropic 使用统计。
type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// NewResponseConverter 创建 Anthropic 响应转换策略实例.
//
// 参数：
//   - logger: 日志记录器（可选）
//
// 返回：
//   - 初始化后的转换策略
func NewResponseConverter(logger port.Logger) *ResponseConverter {
	if logger == nil {
		logger = &port.NopLogger{}
	}
	return &ResponseConverter{
		logger: logger,
	}
}

// Convert 将 Anthropic 响应转换为标准格式。
//
// Anthropic 响应特点：
//   - content 字段是数组格式，包含多个 content block
//   - stop_reason 格式不同（stop_sequence vs stop）
//   - usage 字段使用 input_tokens/output_tokens
func (c *ResponseConverter) Convert(respBody []byte, model string) (*entity.Response, error) {
	if len(respBody) == 0 {
		return nil, nil
	}

	// 解析 Anthropic 响应
	var anthropicResp AnthropicResponse
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		c.logger.Debug("解析 Anthropic 响应失败",
			port.String("error", err.Error()),
		)
		return nil, nil
	}

	// 提取文本内容
	var content strings.Builder
	for _, block := range anthropicResp.Content {
		if block.Type == "text" {
			content.WriteString(block.Text)
		}
	}

	// 转换停止原因
	stopReason := c.convertStopReason(anthropicResp.StopReason)

	// 创建标准响应
	choice := entity.NewChoice(
		0,
		entity.NewMessage("assistant", content.String()),
		stopReason,
	)

	// 转换使用统计
	usage := entity.NewUsage(
		anthropicResp.Usage.InputTokens,
		anthropicResp.Usage.OutputTokens,
	)

	// 确定模型名称
	responseModel := model
	if responseModel == "" {
		responseModel = anthropicResp.Model
	}

	response := entity.NewResponse(
		anthropicResp.ID,
		responseModel,
		[]entity.Choice{choice},
		usage,
	)

	c.logger.Debug("Anthropic 响应转换完成",
		port.String("model", responseModel),
		port.Int("content_blocks", len(anthropicResp.Content)),
		port.String("stop_reason", stopReason),
	)

	return response, nil
}

// convertStopReason 转换 Anthropic 停止原因。
func (c *ResponseConverter) convertStopReason(anthropicReason string) string {
	switch anthropicReason {
	case "end_turn", "stop_sequence":
		return "stop"
	case "max_tokens":
		return "length"
	default:
		return anthropicReason
	}
}

// Supports 检查是否支持指定协议。
func (c *ResponseConverter) Supports(protocol types.Protocol) bool {
	return protocol == types.ProtocolAnthropic
}

// Protocol 返回支持的协议类型。
func (c *ResponseConverter) Protocol() types.Protocol {
	return types.ProtocolAnthropic
}

// Name 返回策略名称。
func (c *ResponseConverter) Name() string {
	return "AnthropicResponseConverter"
}
