package anthropic

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

// StreamChunkConverter Anthropic 协议的流式块转换策略。
// 处理 Anthropic 特定的 SSE 格式流式响应。
type StreamChunkConverter struct {
	logger port.Logger
}

// AnthropicStreamEvent Anthropic 流式事件类型。
const (
	AnthropicEventMessageStart      = "message_start"
	AnthropicEventContentBlockStart = "content_block_start"
	AnthropicEventContentBlockDelta = "content_block_delta"
	AnthropicEventContentBlockStop  = "content_block_stop"
	AnthropicEventMessageDelta      = "message_delta"
	AnthropicEventMessageStop       = "message_stop"
	AnthropicEventPing              = "ping"
	AnthropicEventUnknown           = "unknown"
)

// AnthropicContentBlockType Anthropic 内容块类型。
type AnthropicContentBlockType string

// Anthropic 内容块类型常量。
const (
	AnthropicBlockText         AnthropicContentBlockType = "text"
	AnthropicBlockToolUse      AnthropicContentBlockType = "tool_use"
	AnthropicBlockToolResult   AnthropicContentBlockType = "tool_result"
	AnthropicBlockImage        AnthropicContentBlockType = "image"
	AnthropicBlockDocument     AnthropicContentBlockType = "document"
	AnthropicBlockThinking     AnthropicContentBlockType = "thinking"
	AnthropicBlockSearchResult AnthropicContentBlockType = "search_result"
)

// AnthropicStreamChunk Anthropic 流式响应数据块。
type AnthropicStreamChunk struct {
	Type       string                 `json:"type"`
	Index      int                    `json:"index,omitempty"`
	Content    interface{}            `json:"content,omitempty"` // content_block_start 使用对象
	Delta      map[string]interface{} `json:"delta,omitempty"`
	StopReason string                 `json:"stop_reason,omitempty"`
	Usage      map[string]int         `json:"usage,omitempty"`
}

// NewStreamChunkConverter 创建 Anthropic 流式块转换策略实例.
//
// 参数：
//   - logger: 日志记录器（可选）
//
// 返回：
//   - 初始化后的转换策略
func NewStreamChunkConverter(logger port.Logger) *StreamChunkConverter {
	if logger == nil {
		logger = &port.NopLogger{}
	}
	return &StreamChunkConverter{
		logger: logger,
	}
}

// ParseChunk 解析 Anthropic 流式数据块。
//
// Anthropic SSE 格式：
//
//	event: message_start
//	data: {"type":"message_start",...}
//
//	event: content_block_delta
//	data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"..."}}
//
//	event: content_block_start
//	data: {"type":"content_block_start","content":{"type":"tool_use","id":"...","name":"..."}}
//
//	event: message_stop
//	data: {"type":"message_stop","stop_reason":"stop_sequence"}
func (c *StreamChunkConverter) ParseChunk(data []byte) (*entity.StreamChunk, error) {
	// 检查是否为 [DONE] 信号
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "[DONE]" {
		return &entity.StreamChunk{
			Finished: true,
			Content:  "",
		}, nil
	}

	// 移除 "event: " 和 "data: " 前缀
	lines := strings.Split(trimmed, "\n")
	var eventData string
	var eventType string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			eventData = strings.TrimPrefix(line, "data: ")
		}
	}

	if eventData == "" {
		eventData = trimmed
	}

	// 解析为 map 以检测特殊内容块类型
	var rawChunk map[string]interface{}
	if err := json.Unmarshal([]byte(eventData), &rawChunk); err != nil {
		c.logger.Debug("解析 Anthropic 流式块失败",
			port.String("error", err.Error()),
			port.String("data", eventData),
		)
		return nil, err
	}

	// 记录事件类型
	if eventType != "" {
		c.logger.Debug("Anthropic 流式事件",
			port.String("event_type", eventType),
		)
	}

	// 检测特殊内容块类型
	c.detectContentBlockTypes(rawChunk)

	// 尝试解析为 Anthropic 事件格式
	var chunk AnthropicStreamChunk
	if err := json.Unmarshal([]byte(eventData), &chunk); err != nil {
		c.logger.Debug("解析 Anthropic 流式块失败",
			port.String("error", err.Error()),
			port.String("data", eventData),
		)
		return nil, err
	}

	// 提取内容并检测特殊块
	var content strings.Builder
	var stopReason string

	switch chunk.Type {
	case AnthropicEventContentBlockDelta:
		// 处理各种 delta 类型
		c.handleContentDelta(chunk.Delta, &content)
	case AnthropicEventContentBlockStart:
		// 处理 content_block_start 事件
		c.handleContentBlockStart(chunk.Content, &content)
	case AnthropicEventMessageStop:
		stopReason = chunk.StopReason
		if stopReason == "" {
			stopReason = "stop"
		}
	}

	// 检查是否为结束事件
	isFinished := chunk.Type == AnthropicEventMessageStop

	return &entity.StreamChunk{
		Finished:   isFinished,
		Content:    content.String(),
		StopReason: stopReason,
	}, nil
}

// BuildChunk 将标准流式块转换为 Anthropic 格式。
func (c *StreamChunkConverter) BuildChunk(chunk *entity.StreamChunk) ([]byte, error) {
	if chunk == nil {
		return nil, nil
	}

	// 构建 Anthropic 格式的流式块
	var anthropicChunk AnthropicStreamChunk

	if chunk.Finished {
		anthropicChunk = AnthropicStreamChunk{
			Type:       AnthropicEventMessageStop,
			StopReason: chunk.StopReason,
		}
	} else {
		anthropicChunk = AnthropicStreamChunk{
			Type:  AnthropicEventContentBlockDelta,
			Delta: map[string]interface{}{"text": chunk.Content},
		}
	}

	return json.Marshal(anthropicChunk)
}

// ParseStream 解析整个流式响应。
func (c *StreamChunkConverter) ParseStream(stream []byte) (<-chan *entity.StreamChunk, <-chan error) {
	chunks := make(chan *entity.StreamChunk)
	errs := make(chan error, 1)

	go func() {
		defer close(chunks)
		defer close(errs)

		reader := bufio.NewReader(bytes.NewReader(stream))

		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				break
			}

			// 跳过空行
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}

			// 解析块
			chunk, parseErr := c.ParseChunk(line)
			if parseErr != nil {
				errs <- parseErr
				continue
			}

			if chunk != nil {
				chunks <- chunk
				if chunk.Finished {
					break
				}
			}
		}
	}()

	return chunks, errs
}

// Supports 检查是否支持指定协议。
func (c *StreamChunkConverter) Supports(protocol types.Protocol) bool {
	return protocol == types.ProtocolAnthropic
}

// Protocol 返回支持的协议类型。
func (c *StreamChunkConverter) Protocol() types.Protocol {
	return types.ProtocolAnthropic
}

// Name 返回策略名称。
func (c *StreamChunkConverter) Name() string {
	return "AnthropicStreamChunkConverter"
}

// detectContentBlockTypes 检测 Anthropic 流式块中的特殊内容块类型。
// 用于调试和监控工具调用、思考块、搜索结果等多模态内容。
func (c *StreamChunkConverter) detectContentBlockTypes(chunk map[string]interface{}) {
	// 检测 content_block_start 事件中的特殊块类型
	if content, ok := chunk["content"].(map[string]interface{}); ok {
		if blockType, ok := content["type"].(string); ok {
			switch AnthropicContentBlockType(blockType) {
			case AnthropicBlockToolUse:
				c.logger.Debug("检测到工具调用块 (tool_use)",
					port.String("tool_id", getStringFromMap(content, "id")),
					port.String("tool_name", getStringFromMap(content, "name")),
				)
			case AnthropicBlockToolResult:
				c.logger.Debug("检测到工具结果块 (tool_result)",
					port.String("tool_use_id", getStringFromMap(content, "tool_use_id")),
				)
			case AnthropicBlockThinking:
				c.logger.Debug("检测到思考块 (thinking)")
			case AnthropicBlockSearchResult:
				c.logger.Debug("检测到搜索结果块 (search_result)")
			case AnthropicBlockImage:
				c.logger.Debug("检测到图像块 (image)")
			case AnthropicBlockDocument:
				c.logger.Debug("检测到文档块 (document)")
			}
		}
	}

	// 检测 delta 中的特殊类型
	if delta, ok := chunk["delta"].(map[string]interface{}); ok {
		if deltaType, ok := delta["type"].(string); ok {
			switch AnthropicContentBlockType(deltaType) {
			case AnthropicBlockToolUse:
				c.logger.Debug("检测到工具调用增量 (tool_use)",
					port.String("tool_use_id", getStringFromMap(delta, "tool_use_id")),
					port.String("tool_name", getStringFromMap(delta, "name")),
				)
			case AnthropicBlockToolResult:
				c.logger.Debug("检测到工具结果增量 (tool_result)",
					port.String("tool_use_id", getStringFromMap(delta, "tool_use_id")),
				)
			case AnthropicBlockThinking:
				c.logger.Debug("检测到思考增量 (thinking)")
			}
		}
	}
}

// handleContentDelta 处理各种 delta 类型的增量数据。
func (c *StreamChunkConverter) handleContentDelta(delta map[string]interface{}, content *strings.Builder) {
	if delta == nil {
		return
	}

	deltaType, ok := delta["type"].(string)
	if !ok {
		return
	}

	switch AnthropicContentBlockType(deltaType) {
	case AnthropicBlockText, "text_delta":
		// 文本增量 - Anthropic 使用 "text_delta" 类型
		if text, ok := delta["text"].(string); ok {
			content.WriteString(text)
		}
	case AnthropicBlockToolUse:
		// 工具调用增量 - 记录但不写入文本内容
		c.logger.Debug("处理工具调用增量",
			port.String("tool_use_id", getStringFromMap(delta, "tool_use_id")),
			port.String("tool_name", getStringFromMap(delta, "name")),
		)
	case AnthropicBlockToolResult:
		// 工具结果增量
		c.logger.Debug("处理工具结果增量",
			port.String("tool_use_id", getStringFromMap(delta, "tool_use_id")),
		)
	case AnthropicBlockThinking:
		// 思考增量 - 记录但不写入主文本内容
		c.logger.Debug("处理思考增量")
	case AnthropicBlockSearchResult:
		// 搜索结果增量
		c.logger.Debug("处理搜索结果增量")
	}
}

// handleContentBlockStart 处理 content_block_start 事件。
func (c *StreamChunkConverter) handleContentBlockStart(contentData interface{}, content *strings.Builder) {
	if contentData == nil {
		return
	}

	contentMap, ok := contentData.(map[string]interface{})
	if !ok {
		return
	}

	blockType, ok := contentMap["type"].(string)
	if !ok {
		return
	}

	switch AnthropicContentBlockType(blockType) {
	case AnthropicBlockToolUse:
		// 工具调用开始 - 记录工具信息
		c.logger.Debug("工具调用开始",
			port.String("tool_id", getStringFromMap(contentMap, "id")),
			port.String("tool_name", getStringFromMap(contentMap, "name")),
		)
	case AnthropicBlockThinking:
		// 思考块开始
		c.logger.Debug("思考块开始")
	case AnthropicBlockSearchResult:
		// 搜索结果块开始
		c.logger.Debug("搜索结果块开始")
	case AnthropicBlockImage, AnthropicBlockDocument:
		// 多模态内容块开始
		c.logger.Debug("多模态内容块开始",
			port.String("block_type", blockType),
		)
	}
}

// getStringFromMap 从 map 中安全获取字符串值。
func getStringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
