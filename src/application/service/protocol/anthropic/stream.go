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
//	event: message_stop
//	data: {"type":"message_stop","stop_reason":"stop_sequence"}
func (c *StreamChunkConverter) ParseChunk(data []byte) (*entity.StreamChunk, error) {
	// Anthropic 使用特定的事件格式
	// 首先检查是否为纯 [DONE]
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

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "event: ") {
			// 事件类型，用于日志记录
			_ = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			eventData = strings.TrimPrefix(line, "data: ")
		}
	}

	if eventData == "" {
		eventData = trimmed
	}

	// 尝试解析为 Anthropic 事件格式
	var chunk AnthropicStreamChunk
	if err := json.Unmarshal([]byte(eventData), &chunk); err != nil {
		c.logger.Debug("解析 Anthropic 流式块失败",
			port.String("error", err.Error()),
			port.String("data", eventData),
		)
		return nil, err
	}

	// 提取文本内容
	var content strings.Builder
	var stopReason string

	switch chunk.Type {
	case AnthropicEventContentBlockDelta:
		if delta, ok := chunk.Delta["text"].(string); ok {
			content.WriteString(delta)
		}
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
