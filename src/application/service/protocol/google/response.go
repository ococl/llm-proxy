package google

import (
	"encoding/json"
	"strings"

	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

// ResponseConverter Google Vertex AI 协议的响应转换策略。
// 负责将 Google Vertex AI 格式转换为标准响应格式。
type ResponseConverter struct {
	logger port.Logger
}

// NewResponseConverter 创建 Google Vertex AI 响应转换策略实例。
func NewResponseConverter(logger port.Logger) *ResponseConverter {
	if logger == nil {
		logger = &port.NopLogger{}
	}
	return &ResponseConverter{
		logger: logger,
	}
}

// GoogleVertexAIResponse Google Vertex AI 响应格式。
type GoogleVertexAIResponse struct {
	ID             string                `json:"id"`
	Object         string                `json:"object"`
	Created        int64                 `json:"created"`
	Model          string                `json:"model"`
	PromptFeedback *GooglePromptFeedback `json:"promptFeedback,omitempty"`
	Candidates     []GoogleCandidate     `json:"candidates"`
	UsageMetadata  *GoogleUsageMetadata  `json:"usageMetadata,omitempty"`
}

// GooglePromptFeedback 提示反馈信息。
type GooglePromptFeedback struct {
	BlockReason        string `json:"blockReason,omitempty"`
	BlockReasonMessage string `json:"blockReasonMessage,omitempty"`
}

// GoogleCandidate 候选响应。
type GoogleCandidate struct {
	Index         int                  `json:"index"`
	Content       GoogleContent        `json:"content"`
	FinishReason  string               `json:"finishReason,omitempty"`
	SafetyRatings []GoogleSafetyRating `json:"safetyRatings,omitempty"`
}

// GoogleContent 内容结构。
type GoogleContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GooglePart `json:"parts"`
}

// GooglePart 内容部分。
type GooglePart struct {
	Text string `json:"text,omitempty"`
}

// GoogleSafetyRating 安全评级。
type GoogleSafetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
}

// GoogleUsageMetadata 使用元数据。
type GoogleUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// Convert 将 Google Vertex AI 响应转换为标准格式。
func (c *ResponseConverter) Convert(respBody []byte, model string) (*entity.Response, error) {
	if len(respBody) == 0 {
		return nil, nil
	}

	// 解析 Google Vertex AI 响应
	var googleResp GoogleVertexAIResponse
	if err := json.Unmarshal(respBody, &googleResp); err != nil {
		c.logger.Debug("解析 Google Vertex AI 响应失败",
			port.String("error", err.Error()),
		)
		return nil, nil
	}

	// 检查是否有内容块
	if len(googleResp.Candidates) == 0 {
		// 如果没有候选内容，检查是否被阻止
		if googleResp.PromptFeedback != nil && googleResp.PromptFeedback.BlockReason != "" {
			c.logger.Debug("Google Vertex AI 请求被阻止",
				port.String("block_reason", googleResp.PromptFeedback.BlockReason),
				port.String("block_message", googleResp.PromptFeedback.BlockReasonMessage),
			)
		}
		return nil, nil
	}

	// 提取文本内容（只取第一个候选）
	var textContent strings.Builder
	if len(googleResp.Candidates) > 0 {
		for _, part := range googleResp.Candidates[0].Content.Parts {
			textContent.WriteString(part.Text)
		}
	}

	// 转换停止原因
	stopReason := c.convertStopReason(googleResp.Candidates[0].FinishReason)

	// 创建标准响应
	choice := entity.NewChoice(
		0,
		entity.NewMessage("assistant", textContent.String()),
		stopReason,
	)

	// 使用元数据
	var usage entity.Usage
	if googleResp.UsageMetadata != nil {
		usage = entity.NewUsage(
			googleResp.UsageMetadata.PromptTokenCount,
			googleResp.UsageMetadata.CandidatesTokenCount,
		)
	}

	// 确定使用的模型名称
	finalModel := model
	if finalModel == "" {
		finalModel = googleResp.Model
	}

	// 构建响应
	response := entity.NewResponse(
		googleResp.ID,
		finalModel,
		[]entity.Choice{choice},
		usage,
	)

	c.logger.Debug("Google Vertex AI 响应转换完成",
		port.String("model", finalModel),
		port.Int("candidates", len(googleResp.Candidates)),
		port.String("stop_reason", stopReason),
	)

	return response, nil
}

// convertStopReason 转换 Google 的停止原因。
func (c *ResponseConverter) convertStopReason(googleReason string) string {
	switch googleReason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY":
		return "content_filter"
	case "RECITATION":
		return "content_filter"
	default:
		return googleReason
	}
}

// Supports 检查是否支持指定协议。
func (c *ResponseConverter) Supports(protocol types.Protocol) bool {
	return protocol == types.ProtocolGoogle
}

// Protocol 返回支持的协议类型。
func (c *ResponseConverter) Protocol() types.Protocol {
	return types.ProtocolGoogle
}

// Name 返回策略名称。
func (c *ResponseConverter) Name() string {
	return "GoogleVertexAIResponseConverter"
}
