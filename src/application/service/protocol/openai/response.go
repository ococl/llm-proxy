package openai

import (
	"encoding/json"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

// ResponseConverter OpenAI 协议的响应转换策略。
// OpenAI 格式是标准格式，通常不需要转换。
type ResponseConverter struct {
	logger port.Logger
}

// NewResponseConverter 创建 OpenAI 响应转换策略实例。
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

// Convert 将 OpenAI 响应转换为标准格式。
//
// OpenAI 格式是标准格式，直接解析返回。
// 可以在此添加响应规范化逻辑。
func (c *ResponseConverter) Convert(respBody []byte, model string) (*entity.Response, error) {
	if len(respBody) == 0 {
		return nil, nil
	}

	// 解析 OpenAI 响应
	var openAIResp entity.Response
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		c.logger.Debug("解析 OpenAI 响应失败",
			port.String("error", err.Error()),
		)
		return nil, nil
	}

	// 如果没有提供 model，尝试从响应中获取
	if model == "" && openAIResp.Model != "" {
		model = openAIResp.Model
	}

	// OpenAI 格式是标准格式，无需转换
	// 后续可在此添加响应规范化逻辑
	return &openAIResp, nil
}

// Supports 检查是否支持指定协议。
func (c *ResponseConverter) Supports(protocol types.Protocol) bool {
	return protocol.IsOpenAICompatible()
}

// Protocol 返回支持的协议类型。
func (c *ResponseConverter) Protocol() types.Protocol {
	return types.ProtocolOpenAI
}

// Name 返回策略名称。
func (c *ResponseConverter) Name() string {
	return "OpenAIResponseConverter"
}
