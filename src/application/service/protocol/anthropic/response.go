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
// 增强支持多种内容块类型：
//   - text: 文本内容
//   - image: 图片内容
//   - document: 文档内容
//   - tool_use: 工具调用
//   - tool_result: 工具结果
//   - thinking: 思考块
//   - search_result: 搜索结果
type AnthropicContentBlock struct {
	Type         string                 `json:"type"`
	Text         string                 `json:"text,omitempty"`
	Citations    *AnthropicCitations    `json:"citations,omitempty"`
	CacheControl *AnthropicCacheControl `json:"cache_control,omitempty"`
	// 工具结果内容（可以是字符串或内容块数组）
	Content interface{} `json:"content,omitempty"`
	// 图片内容相关
	Source *AnthropicImageSource `json:"source,omitempty"`
	// 文档内容相关
	Document *AnthropicDocumentSource `json:"document,omitempty"`
	// 工具调用相关
	ID    string      `json:"id,omitempty"`
	Name  string      `json:"name,omitempty"`
	Input interface{} `json:"input,omitempty"`
	// 思考块
	Thinking  string `json:"thinking,omitempty"`
	Signature string `json:"signature,omitempty"`
	// 搜索结果
	SearchResult *AnthropicSearchResult `json:"search_result,omitempty"`
}

// AnthropicCitations 引用信息。
type AnthropicCitations struct {
	Enabled bool `json:"enabled"`
}

// AnthropicCacheControl 缓存控制。
type AnthropicCacheControl struct {
	Type string `json:"type"`
	TTL  string `json:"ttl,omitempty"`
}

// AnthropicImageSource 图片源。
type AnthropicImageSource struct {
	Type      string `json:"type"` // "base64" or "url"
	MediaType string `json:"media_type"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

// AnthropicDocumentSource 文档源。
type AnthropicDocumentSource struct {
	Type      string `json:"type"` // "base64", "text", or "url"
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
	Title     string `json:"title,omitempty"`
	Context   string `json:"context,omitempty"`
}

// AnthropicSearchResult 搜索结果。
type AnthropicSearchResult struct {
	Content   []AnthropicContentBlock `json:"content"`
	Source    string                  `json:"source"`
	Title     string                  `json:"title"`
	Citations *AnthropicCitations     `json:"citations,omitempty"`
}

// AnthropicUsage Anthropic 使用统计。
type AnthropicUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
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
//   - stop_reason 格式不同（end_turn, stop_sequence -> stop）
//   - usage 字段使用 input_tokens/output_tokens
//   - 支持多模态内容（图片、文档）
//   - 支持工具调用（tool_use, tool_result）
//   - 支持思考块（thinking）
//   - 支持搜索结果（search_result）
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
	textContent := c.extractTextContent(anthropicResp.Content)

	// 检查是否有特殊内容块
	hasToolUse := c.hasContentBlockType(anthropicResp.Content, "tool_use")
	hasThinking := c.hasContentBlockType(anthropicResp.Content, "thinking")
	hasSearchResult := c.hasContentBlockType(anthropicResp.Content, "search_result")
	hasImage := c.hasContentBlockType(anthropicResp.Content, "image")
	hasDocument := c.hasContentBlockType(anthropicResp.Content, "document")

	// 转换停止原因
	stopReason := c.convertStopReason(anthropicResp.StopReason)

	// 创建标准响应
	choice := entity.NewChoice(
		0,
		entity.NewMessage("assistant", textContent),
		stopReason,
	)

	// 转换使用统计（包含缓存信息）
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

	// 记录特殊内容块信息
	if hasToolUse || hasThinking || hasSearchResult || hasImage || hasDocument {
		c.logger.Debug("Anthropic 响应包含特殊内容块",
			port.String("model", responseModel),
			port.Int("content_blocks", len(anthropicResp.Content)),
			port.String("stop_reason", stopReason),
			port.Bool("has_tool_use", hasToolUse),
			port.Bool("has_thinking", hasThinking),
			port.Bool("has_search_result", hasSearchResult),
			port.Bool("has_image", hasImage),
			port.Bool("has_document", hasDocument),
		)
	}

	return response, nil
}

// extractTextContent 从内容块中提取纯文本内容。
func (c *ResponseConverter) extractTextContent(blocks []AnthropicContentBlock) string {
	var content strings.Builder

	for _, block := range blocks {
		switch block.Type {
		case "text":
			content.WriteString(block.Text)
		case "tool_use":
			// 工具调用作为特殊内容记录
			c.logger.Debug("Anthropic 工具调用",
				port.String("tool_id", block.ID),
				port.String("tool_name", block.Name),
			)
		case "tool_result":
			// 工具结果可以包含文本
			if resultText, ok := block.Content.(string); ok {
				content.WriteString(resultText)
			}
		case "thinking":
			// 思考块通常不显示给用户，但可以记录
			c.logger.Debug("Anthropic 思考块",
				port.Bool("has_signature", block.Signature != ""),
			)
		case "search_result":
			// 搜索结果可以包含文本
			for _, subBlock := range block.SearchResult.Content {
				if subBlock.Type == "text" {
					content.WriteString(subBlock.Text)
				}
			}
		case "image":
			c.logger.Debug("Anthropic 图片内容",
				port.String("media_type", block.Source.MediaType),
			)
		case "document":
			c.logger.Debug("Anthropic 文档内容",
				port.String("media_type", block.Document.MediaType),
				port.String("title", block.Document.Title),
			)
		}
	}

	return content.String()
}

// hasContentBlockType 检查是否存在指定类型的 content block。
func (c *ResponseConverter) hasContentBlockType(blocks []AnthropicContentBlock, blockType string) bool {
	for _, block := range blocks {
		if block.Type == blockType {
			return true
		}
	}
	return false
}

// convertStopReason 转换 Anthropic 停止原因。
func (c *ResponseConverter) convertStopReason(anthropicReason string) string {
	switch anthropicReason {
	case "end_turn", "stop_sequence":
		return "stop"
	case "max_tokens":
		return "length"
	case "content_filter", "refusal":
		return "content_filter"
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
