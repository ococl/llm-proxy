package google

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

// StreamChunkConverter Google Vertex AI 协议的流式响应转换策略。
// 负责解析 Google Vertex AI 的 JSON Lines 流式格式并转换为标准格式。
type StreamChunkConverter struct {
	logger port.Logger
}

// NewStreamChunkConverter 创建 Google Vertex AI 流式响应转换策略实例。
func NewStreamChunkConverter(logger port.Logger) *StreamChunkConverter {
	if logger == nil {
		logger = &port.NopLogger{}
	}
	return &StreamChunkConverter{
		logger: logger,
	}
}

// GoogleStreamChunk Google Vertex AI 流式响应块。
type GoogleStreamChunk struct {
	ID             string                `json:"id"`
	Object         string                `json:"object"`
	Created        int64                 `json:"created"`
	Model          string                `json:"model"`
	Chunk          string                `json:"chunk,omitempty"`
	PromptFeedback *GooglePromptFeedback `json:"promptFeedback,omitempty"`
	Candidates     []GoogleCandidate     `json:"candidates,omitempty"`
	UsageMetadata  *GoogleUsageMetadata  `json:"usageMetadata,omitempty"`
}

// ParseChunk 解析 Google Vertex AI 流式响应块。
// Google Vertex AI 使用 JSON Lines 格式（每行一个 JSON 对象）。
func (c *StreamChunkConverter) ParseChunk(data []byte) (*entity.StreamChunk, error) {
	if len(data) == 0 {
		return nil, nil
	}

	// Google Vertex AI 使用 JSON Lines 格式，每行一个 JSON 对象
	// 尝试解析整行数据
	var chunk GoogleStreamChunk
	if err := json.Unmarshal(data, &chunk); err != nil {
		// 如果解析失败，尝试查找 JSON 对象
		c.logger.Debug("解析 Google Vertex AI 流式块失败",
			port.String("error", err.Error()),
			port.String("data", string(data[:min(100, len(data))])),
		)
		return nil, nil
	}

	// 检查是否是最后一个块
	if chunk.UsageMetadata != nil {
		// 流结束，返回 usage 信息
		return &entity.StreamChunk{
			Finished:   true,
			Content:    "",
			StopReason: "stop",
		}, nil
	}

	// 提取文本内容
	var content strings.Builder
	if chunk.Chunk != "" {
		content.WriteString(chunk.Chunk)
	}

	// 检查候选内容
	if len(chunk.Candidates) > 0 {
		for _, candidate := range chunk.Candidates {
			for _, part := range candidate.Content.Parts {
				content.WriteString(part.Text)
			}
		}
	}

	// 检查是否被阻止
	if chunk.PromptFeedback != nil && chunk.PromptFeedback.BlockReason != "" {
		return &entity.StreamChunk{
			Finished:   true,
			Content:    "",
			StopReason: "content_filter",
			Error:      chunk.PromptFeedback.BlockReasonMessage,
		}, nil
	}

	return &entity.StreamChunk{
		Finished: false,
		Content:  content.String(),
	}, nil
}

// BuildChunk 将标准流式块转换为 Google Vertex AI 格式。
// 此方法主要用于测试，实际场景中通常不需要从客户端发送流式块。
func (c *StreamChunkConverter) BuildChunk(chunk *entity.StreamChunk) ([]byte, error) {
	if chunk == nil {
		return nil, nil
	}

	// 构建 Google 格式的流式响应
	googleChunk := map[string]any{
		"chunk": chunk.Content,
	}

	if chunk.Finished {
		googleChunk["usageMetadata"] = map[string]int{
			"promptTokenCount":     0,
			"candidatesTokenCount": len(chunk.Content),
			"totalTokenCount":      len(chunk.Content),
		}
		if chunk.StopReason != "" {
			googleChunk["candidates"] = []map[string]any{
				{
					"finishReason": c.googleStopReason(chunk.StopReason),
				},
			}
		}
	}

	return json.Marshal(googleChunk)
}

// googleStopReason 将标准停止原因转换为 Google 格式。
func (c *StreamChunkConverter) googleStopReason(reason string) string {
	switch reason {
	case "stop":
		return "STOP"
	case "length":
		return "MAX_TOKENS"
	case "content_filter":
		return "SAFETY"
	default:
		return reason
	}
}

// ParseStream 解析整个流式响应。
// 返回解析器通道，每次解析出一个块时发送。
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
				if len(line) > 0 {
					// 尝试解析最后一行
					chunk, parseErr := c.ParseChunk(bytes.TrimSpace(line))
					if parseErr != nil {
						errs <- parseErr
					} else if chunk != nil {
						chunks <- chunk
					}
				}
				break
			}

			// 跳过空行
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}

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
	return protocol == types.ProtocolGoogle
}

// Protocol 返回支持的协议类型。
func (c *StreamChunkConverter) Protocol() types.Protocol {
	return types.ProtocolGoogle
}

// Name 返回策略名称。
func (c *StreamChunkConverter) Name() string {
	return "GoogleVertexAIStreamChunkConverter"
}
