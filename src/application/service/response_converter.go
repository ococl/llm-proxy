package service

import (
	"encoding/json"
	"strconv"
	"strings"

	"llm-proxy/domain/entity"
)

// ResponseConverter 负责响应格式的规范化、转换和流式响应合并。
// 支持多种 LLM 提供商响应格式的统一处理。
type ResponseConverter struct{}

// NewResponseConverter 创建一个新的响应转换器。
func NewResponseConverter() *ResponseConverter {
	return &ResponseConverter{}
}

// NormalizeResponse 规范化响应，确保一致的格式。
// 主要处理:
// 1. 空 choices 的默认填充
// 2. finish_reason 的标准化
// 3. usage 字段的完整性
func (rc *ResponseConverter) NormalizeResponse(resp *entity.Response) *entity.Response {
	if resp == nil {
		return nil
	}

	choices := resp.Choices
	if len(choices) == 0 {
		// 如果没有 choices，创建一个空的 assistant 选择
		choices = []entity.Choice{
			entity.NewChoice(0, entity.NewMessage("assistant", ""), "stop"),
		}
	}

	builder := entity.NewResponseBuilder().
		ID(resp.ID).
		Model(resp.Model).
		Created(resp.Created).
		Choices(choices).
		Usage(resp.Usage)

	if resp.StopReason != "" {
		builder = builder.StopReason(resp.StopReason)
	}
	if len(resp.StopSequences) > 0 {
		builder = builder.StopSequences(resp.StopSequences)
	}
	if resp.Headers != nil {
		builder = builder.Headers(resp.Headers)
	}

	normalized, err := builder.Build()
	if err != nil {
		// 构建失败时返回原始响应
		return resp
	}

	return normalized
}

// MergeStreamChunks 合并多个流式响应块。
// 这是实现流畅打字机效果的关键逻辑。
//
// 合并策略:
// 1. 逐块累积 delta 内容
// 2. 记录最后一个有效的 finish_reason
// 3. 累加 usage 统计
// 4. 处理多模态内容（转换为字符串）
func (rc *ResponseConverter) MergeStreamChunks(chunks []*entity.Response) *entity.Response {
	if len(chunks) == 0 {
		return nil
	}

	base := chunks[0]
	if len(chunks) == 1 {
		return base
	}

	var mergedContent strings.Builder
	var lastFinishReason string
	var totalUsage entity.Usage
	var toolCalls []entity.ToolCall

	for _, chunk := range chunks {
		if firstChoice := chunk.FirstChoice(); firstChoice != nil {
			// 合并 Delta 内容
			if firstChoice.Delta != nil {
				switch deltaContent := firstChoice.Delta.Content.(type) {
				case string:
					// 字符串内容直接追加
					mergedContent.WriteString(deltaContent)
				case []interface{}:
					// 多模态内容转换为字符串表示
					if contentStr := rc.convertMultimodalContent(deltaContent); contentStr != "" {
						mergedContent.WriteString(contentStr)
					}
				default:
					// 其他类型尝试 JSON 序列化
					if contentBytes, err := json.Marshal(deltaContent); err == nil {
						mergedContent.Write(contentBytes)
					}
				}

				// 收集工具调用
				if len(firstChoice.Delta.ToolCalls) > 0 {
					toolCalls = append(toolCalls, firstChoice.Delta.ToolCalls...)
				}
			}

			// 记录 finish_reason
			if firstChoice.FinishReason != "" {
				lastFinishReason = firstChoice.FinishReason
			}
		}

		// 累加 usage
		usage := chunk.Usage
		totalUsage = entity.NewUsage(
			totalUsage.PromptTokens+usage.PromptTokens,
			totalUsage.CompletionTokens+usage.CompletionTokens,
		)
	}

	// 构建合并后的消息
	finalMessage := entity.NewMessage("assistant", mergedContent.String())
	if len(toolCalls) > 0 {
		finalMessage.ToolCalls = toolCalls
	}

	mergedChoice := entity.NewChoice(
		0,
		finalMessage,
		lastFinishReason,
	)

	return entity.NewResponseBuilder().
		ID(base.ID).
		Model(base.Model).
		Created(base.Created).
		Choices([]entity.Choice{mergedChoice}).
		Usage(totalUsage).
		BuildUnsafe()
}

// convertMultimodalContent 将多模态内容转换为字符串表示。
func (rc *ResponseConverter) convertMultimodalContent(content []interface{}) string {
	if len(content) == 0 {
		return ""
	}

	var result strings.Builder
	for _, item := range content {
		switch v := item.(type) {
		case string:
			result.WriteString(v)
		case map[string]interface{}:
			if text, ok := v["text"].(string); ok {
				result.WriteString(text)
			} else if typeStr, ok := v["type"].(string); ok {
				switch typeStr {
				case "text":
					if text, ok := v["text"].(string); ok {
						result.WriteString(text)
					}
				case "image_url":
					result.WriteString("[image]")
				default:
					if itemBytes, err := json.Marshal(v); err == nil {
						result.Write(itemBytes)
					}
				}
			}
		default:
			if itemBytes, err := json.Marshal(v); err == nil {
				result.Write(itemBytes)
			}
		}
	}
	return result.String()
}

// ConvertStopReason 标准化 stop_reason。
// 不同提供商可能使用不同的停止原因标识符。
func (rc *ResponseConverter) ConvertStopReason(reason string, fromProtocol string) string {
	// 定义停止原因的标准化映射
	stopReasonMappings := map[string][]string{
		"stop":           {"stop", "STOP", "stop_sequence"},
		"length":         {"length", "LENGTH", "max_tokens", "max_output_tokens"},
		"tool_use":       {"tool_use", "TOOL_USE", "function_call", "tool_calls"},
		"content_filter": {"content_filter", "CONTENT_FILTER", "blocked"},
	}

	reason = strings.ToLower(reason)
	for standard, variants := range stopReasonMappings {
		for _, variant := range variants {
			if reason == variant {
				return standard
			}
		}
	}

	// 未知原因，返回原始值
	return reason
}

// ExtractStreamEvent 从 SSE 事件行中提取数据。
// SSE 格式: "data: {...}\n\n"
func (rc *ResponseConverter) ExtractStreamEvent(line string) (json.RawMessage, bool) {
	line = strings.TrimSpace(line)

	// 检查是否为 data 行
	if !strings.HasPrefix(line, "data:") {
		return nil, false
	}

	// 提取 data 部分
	data := strings.TrimSpace(strings.TrimPrefix(line, "data"))

	// 检查是否为 [DONE] 标记
	if data == "[DONE]" {
		return nil, true
	}

	// 解析 JSON
	var eventData json.RawMessage
	if err := json.Unmarshal([]byte(data), &eventData); err != nil {
		return nil, false
	}

	return eventData, true
}

// BuildStreamEvent 构建 SSE 格式的事件行。
func (rc *ResponseConverter) BuildStreamEvent(data interface{}) string {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return "data: " + string(jsonBytes) + "\n\n"
}

// BuildStreamDone 构建 SSE 的 [DONE] 事件。
func (rc *ResponseConverter) BuildStreamDone() string {
	return "data: [DONE]\n\n"
}

// ParseUsage 解析不同格式的 usage 字段。
// 不同提供商的 usage 格式可能不同。
func (rc *ResponseConverter) ParseUsage(usageData map[string]interface{}) entity.Usage {
	if usageData == nil {
		return entity.Usage{}
	}

	promptTokens := rc.parseInt(usageData["prompt_tokens"])
	completionTokens := rc.parseInt(usageData["completion_tokens"])

	// 某些提供商使用 total_tokens，某些使用其他字段
	if total, ok := usageData["total_tokens"].(float64); ok {
		// 如果同时提供了 total，优先使用精确值
		if promptTokens > 0 && completionTokens > 0 {
			return entity.NewUsage(promptTokens, completionTokens)
		}
		return entity.Usage{
			PromptTokens:     int(total),
			CompletionTokens: 0,
			TotalTokens:      int(total),
		}
	}

	return entity.NewUsage(promptTokens, completionTokens)
}

func (rc *ResponseConverter) parseInt(value interface{}) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return 0
}
