package google

import (
	"encoding/json"
	"fmt"
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
// 增强支持：
//   - Category: 安全类别（HARM_CATEGORY_HARASSMENT 等）
//   - Probability: 概率级别（UNSPECIFIED, NEGLIGIBLE, LOW, MEDIUM, HIGH）
//   - ProbabilityScore: 概率分数（0-1）
//   - Severity: 严重程度（UNSPECIFIED, NEGLIGIBLE, LOW, MEDIUM, HIGH）
//   - SeverityScore: 严重程度分数（0-1）
type GoogleSafetyRating struct {
	Category         string  `json:"category"`
	Probability      string  `json:"probability"`
	ProbabilityScore float64 `json:"probabilityScore,omitempty"`
	Severity         string  `json:"severity,omitempty"`
	SeverityScore    float64 `json:"severityScore,omitempty"`
}

// GoogleUsageMetadata 使用元数据。
type GoogleUsageMetadata struct {
	PromptTokenCount            int                      `json:"promptTokenCount"`
	CandidatesTokenCount        int                      `json:"candidatesTokenCount"`
	TotalTokenCount             int                      `json:"totalTokenCount"`
	PromptTokenCountDetails     *GoogleTokenCountDetails `json:"promptTokenCountDetails,omitempty"`
	CandidatesTokenCountDetails *GoogleTokenCountDetails `json:"candidatesTokenCountDetails,omitempty"`
}

// GoogleTokenCountDetails Token 计数详细信息。
type GoogleTokenCountDetails struct {
	CachedContentTokenCount int `json:"cachedContentTokenCount,omitempty"`
}

// GoogleResponseMetadata 响应元数据。
type GoogleResponseMetadata struct {
	ModelVersion string `json:"modelVersion,omitempty"`
}

// GoogleTurnMetrics 轮次指标。
type GoogleTurnMetrics struct {
	TurnTokenCount int `json:"turnTokenCount,omitempty"`
}

// Convert 将 Google Vertex AI 响应转换为标准格式。
//
// 增强支持：
//   - 多候选响应：支持返回多个候选并选择最佳的一个
//   - 安全评级提取：记录所有安全相关的信息
//   - 阻止原因处理：详细处理内容被阻止的情况
//   - 部分内容处理：即使被阻止也返回部分内容
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

	// 选择最佳候选（通常返回第一个，如果有多个候选）
	selectedCandidate := c.selectBestCandidate(googleResp.Candidates)

	// 提取文本内容（支持多候选）
	textContent := c.extractTextContent(selectedCandidate.Content.Parts)

	// 转换停止原因
	stopReason := c.convertStopReason(selectedCandidate.FinishReason)

	// 记录安全评级
	hasSafetyRatings := len(selectedCandidate.SafetyRatings) > 0
	if hasSafetyRatings {
		c.logSafetyRatings(selectedCandidate.SafetyRatings)
	}

	// 创建标准响应
	choice := entity.NewChoice(
		0,
		entity.NewMessage("assistant", textContent),
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

	// 记录响应信息
	c.logger.Debug("Google Vertex AI 响应转换完成",
		port.String("model", finalModel),
		port.Int("candidates", len(googleResp.Candidates)),
		port.String("stop_reason", stopReason),
		port.Bool("has_safety_ratings", hasSafetyRatings),
	)

	// 记录缓存使用信息
	if googleResp.UsageMetadata != nil && googleResp.UsageMetadata.PromptTokenCountDetails != nil {
		if cachedTokens := googleResp.UsageMetadata.PromptTokenCountDetails.CachedContentTokenCount; cachedTokens > 0 {
			c.logger.Debug("Google Vertex AI 响应包含缓存使用信息",
				port.Int("cached_content_tokens", cachedTokens),
			)
		}
	}

	return response, nil
}

// selectBestCandidate 选择最佳候选。
// 当前策略：选择第一个候选（通常是最佳/最新的）
// 未来可以扩展：基于 finish_reason、内容长度等选择
func (c *ResponseConverter) selectBestCandidate(candidates []GoogleCandidate) *GoogleCandidate {
	if len(candidates) == 0 {
		return nil
	}

	// 简单策略：返回第一个候选
	return &candidates[0]
}

// extractTextContent 从 parts 中提取文本内容。
func (c *ResponseConverter) extractTextContent(parts []GooglePart) string {
	var textContent strings.Builder

	for _, part := range parts {
		textContent.WriteString(part.Text)
	}

	return textContent.String()
}

// logSafetyRatings 记录安全评级信息。
func (c *ResponseConverter) logSafetyRatings(ratings []GoogleSafetyRating) {
	for _, rating := range ratings {
		c.logger.Debug("Google Vertex AI 安全评级",
			port.String("category", rating.Category),
			port.String("probability", rating.Probability),
			port.String("probability_score", formatFloat(rating.ProbabilityScore)),
			port.String("severity", rating.Severity),
			port.String("severity_score", formatFloat(rating.SeverityScore)),
		)
	}
}

// formatFloat 格式化浮点数为字符串。
func formatFloat(f float64) string {
	return string(fmt.Sprintf("%.4f", f))
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
	case "MEDIA_INPUT":
		return "content_filter"
	case "EMPTY":
		return "stop"
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
