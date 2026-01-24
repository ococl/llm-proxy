package openai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

// StreamChunkConverter OpenAI 协议的流式块转换策略。
// 处理 OpenAI 兼容的 SSE 格式流式响应。
type StreamChunkConverter struct {
	logger port.Logger
}

// NewStreamChunkConverter 创建 OpenAI 流式块转换策略实例。
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

// OpenAIStreamChunk 定义 OpenAI 流式响应的数据块结构。
type OpenAIStreamChunk struct {
	ID                string         `json:"id"`
	Object            string         `json:"object"`
	Created           int64          `json:"created"`
	Model             string         `json:"model"`
	SystemFingerprint string         `json:"system_fingerprint,omitempty"`
	Choices           []OpenAIChoice `json:"choices"`
}

// OpenAIChoice 定义 OpenAI 流式响应的选择结构。
type OpenAIChoice struct {
	Index        int         `json:"index"`
	Delta        OpenAIDelta `json:"delta"`
	LogProbs     interface{} `json:"logprobs,omitempty"`
	FinishReason string      `json:"finish_reason,omitempty"`
}

// OpenAIDelta 定义 OpenAI 流式响应的增量数据结构。
type OpenAIDelta struct {
	Role         string              `json:"role,omitempty"`
	Content      string              `json:"content,omitempty"`
	ToolCalls    []OpenAIToolCall    `json:"tool_calls,omitempty"`
	FunctionCall *OpenAIFunctionCall `json:"function_call,omitempty"`
}

// OpenAIToolCall 定义 OpenAI 工具调用结构。
type OpenAIToolCall struct {
	Index    int                `json:"index"`
	ID       string             `json:"id,omitempty"`
	Type     string             `json:"type,omitempty"`
	Function OpenAIFunctionCall `json:"function,omitempty"`
}

// OpenAIFunctionCall 定义 OpenAI 函数调用结构。
type OpenAIFunctionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// ParseChunk 解析 OpenAI 流式数据块。
//
// OpenAI SSE 格式：
//
//	data: {"id":"...","object":"chat.completion.chunk","choices":[...]}
//	data: [DONE]
func (c *StreamChunkConverter) ParseChunk(data []byte) (*entity.StreamChunk, error) {
	// 检查是否为 [DONE] 信号
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "[DONE]" {
		return &entity.StreamChunk{
			Finished: true,
			Content:  "",
		}, nil
	}

	// 移除 "data: " 前缀
	if strings.HasPrefix(trimmed, "data: ") {
		data = []byte(strings.TrimPrefix(trimmed, "data: "))
	}

	// 解析 JSON - 使用 map 以检测额外字段
	var rawChunk map[string]interface{}
	if err := json.Unmarshal(data, &rawChunk); err != nil {
		c.logger.Debug("解析 OpenAI 流式块失败",
			port.String("error", err.Error()),
			port.String("data", string(data)),
		)
		return nil, err
	}

	// 检测并记录特殊字段
	c.detectSpecialFields(rawChunk)

	// 解析为结构化数据
	var chunk OpenAIStreamChunk
	if err := json.Unmarshal(data, &chunk); err != nil {
		c.logger.Debug("解析 OpenAI 流式块失败",
			port.String("error", err.Error()),
			port.String("data", string(data)),
		)
		return nil, err
	}

	// 提取增量内容
	var content strings.Builder
	var stopReason string

	for _, choice := range chunk.Choices {
		content.WriteString(choice.Delta.Content)

		if choice.FinishReason != "" {
			stopReason = choice.FinishReason
		}

		// 检测 tool_calls
		if len(choice.Delta.ToolCalls) > 0 {
			c.logger.Debug("检测到工具调用",
				port.Int("tool_count", len(choice.Delta.ToolCalls)),
				port.String("first_tool_id", choice.Delta.ToolCalls[0].ID),
			)
		}
	}

	chunkResult := &entity.StreamChunk{
		Finished:   stopReason != "",
		Content:    content.String(),
		StopReason: stopReason,
	}

	// 如果有 system_fingerprint，记录到上下文
	if chunk.SystemFingerprint != "" {
		c.logger.Debug("OpenAI 流式块包含 system_fingerprint",
			port.String("system_fingerprint", chunk.SystemFingerprint),
		)
	}

	return chunkResult, nil
}

// BuildChunk 将标准流式块转换为 OpenAI 格式。
func (c *StreamChunkConverter) BuildChunk(chunk *entity.StreamChunk) ([]byte, error) {
	if chunk == nil {
		return nil, nil
	}

	openAIChunk := OpenAIStreamChunk{
		ID:      "",
		Object:  "chat.completion.chunk",
		Created: 0,
		Model:   "",
		Choices: []OpenAIChoice{
			{
				Index: 0,
				Delta: OpenAIDelta{
					Content: chunk.Content,
				},
				FinishReason: chunk.StopReason,
			},
		},
	}

	if chunk.Finished {
		openAIChunk.Choices[0].FinishReason = chunk.StopReason
	}

	return json.Marshal(openAIChunk)
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
	return protocol.IsOpenAICompatible()
}

// Protocol 返回支持的协议类型。
func (c *StreamChunkConverter) Protocol() types.Protocol {
	return types.ProtocolOpenAI
}

// Name 返回策略名称。
func (c *StreamChunkConverter) Name() string {
	return "OpenAIStreamChunkConverter"
}

// detectSpecialFields 检测并记录 OpenAI 流式块中的特殊字段。
// 用于调试和监控工具调用、logprobs、system_fingerprint 等。
func (c *StreamChunkConverter) detectSpecialFields(chunk map[string]interface{}) {
	// 检测 choices 中的 logprobs
	if choices, ok := chunk["choices"].([]interface{}); ok {
		for i, choice := range choices {
			if choiceMap, ok := choice.(map[string]interface{}); ok {
				if _, hasLogProbs := choiceMap["logprobs"]; hasLogProbs {
					c.logger.Debug("检测到 logprobs 字段",
						port.Int("choice_index", i),
					)
				}
			}
		}
	}

	// 检测 system_fingerprint
	if _, hasFingerprint := chunk["system_fingerprint"]; hasFingerprint {
		if fingerprint, ok := chunk["system_fingerprint"].(string); ok && fingerprint != "" {
			c.logger.Debug("检测到 system_fingerprint",
				port.String("system_fingerprint", fingerprint),
			)
		}
	}
}
