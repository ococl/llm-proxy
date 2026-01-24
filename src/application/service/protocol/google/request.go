package google

import (
	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

// RequestConverter Google Vertex AI 协议的请求转换策略。
// 负责将标准请求格式转换为 Google Vertex AI 格式。
type RequestConverter struct {
	logger port.Logger
}

// NewRequestConverter 创建 Google Vertex AI 请求转换策略实例。
//
// Google Vertex AI 格式特点：
//   - 安全设置 (safetySettings) 用于内容过滤
//   - 实例 (instances) 和参数 (parameters) 结构
//   - 工具调用格式与 OpenAI 不同
//   - 支持 StopSequences 和最大输出 tokens
//
// 参数：
//   - logger: 日志记录器（可选）
//
// 返回：
//   - 初始化后的转换策略
func NewRequestConverter(logger port.Logger) *RequestConverter {
	if logger == nil {
		logger = &port.NopLogger{}
	}
	return &RequestConverter{
		logger: logger,
	}
}

// Convert 将请求转换为 Google Vertex AI 格式。
// Google Vertex AI 使用 content 结构中的 parts 数组存储消息内容。
func (c *RequestConverter) Convert(req *entity.Request, systemPrompt string) (*entity.Request, error) {
	if req == nil {
		return nil, nil
	}

	messages := req.Messages()
	if len(messages) == 0 {
		return req, nil
	}

	// 检查是否需要转换
	needsConversion := c.hasNonTextContent(messages) ||
		req.MaxTokens() == 0 ||
		len(req.Stop()) > 0 ||
		systemPrompt != ""

	if !needsConversion {
		return req, nil
	}

	// 构建 Google 格式的请求
	builder := entity.NewRequestBuilder().
		ID(req.ID()).
		Model(req.Model()).
		Messages(messages).
		MaxTokens(req.MaxTokens()).
		Temperature(req.Temperature()).
		TopP(req.TopP()).
		Stream(req.IsStream()).
		Stop(req.Stop()).
		// Google 的 tools 格式不同，需要在 HTTP 层转换
		Tools(nil).
		ToolChoice(req.ToolChoice()).
		User(req.User()).
		Context(req.Context()).
		StreamHandler(req.StreamHandler()).
		Headers(req.Headers()).
		ClientProtocol(string(types.ProtocolGoogle))

	// Google 不使用独立的 system 字段，系统提示应包含在第一个 user 消息中
	// 如果提供了 systemPrompt，将其添加到第一个 user 消息中
	if systemPrompt != "" {
		c.logger.Debug("Google Vertex AI: 系统提示需要合并到用户消息中",
			port.String("req_id", req.ID().String()),
		)
	}

	c.logger.Debug("Google Vertex AI 协议转换完成",
		port.String("req_id", req.ID().String()),
		port.Int("messages", len(messages)),
		port.Bool("needs_conversion", needsConversion),
	)

	return builder.BuildUnsafe(), nil
}

// hasNonTextContent 检查消息是否包含非文本内容（如多模态内容）。
func (c *RequestConverter) hasNonTextContent(messages []entity.Message) bool {
	for _, msg := range messages {
		switch msg.Content.(type) {
		case string:
			// 文本内容，不需要转换
			continue
		default:
			// 非字符串内容（如数组），需要转换
			return true
		}
	}
	return false
}

// Supports 检查是否支持指定协议。
func (c *RequestConverter) Supports(protocol types.Protocol) bool {
	return protocol == types.ProtocolGoogle
}

// Protocol 返回支持的协议类型。
func (c *RequestConverter) Protocol() types.Protocol {
	return types.ProtocolGoogle
}

// Name 返回策略名称。
func (c *RequestConverter) Name() string {
	return "GoogleVertexAIRequestConverter"
}
