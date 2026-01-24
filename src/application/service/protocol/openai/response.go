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
// 增强支持：
//   - tool_calls: 工具调用信息（通过 raw byte 提取）
//   - logprobs: 对数概率信息（通过 raw byte 提取）
//   - refusal: 拒绝回答（通过 raw byte 提取）
//   - annotations: 引用标注（通过 raw byte 提取）
//   - system_fingerprint: 系统指纹（通过 raw byte 提取）
func (c *ResponseConverter) Convert(respBody []byte, model string) (*entity.Response, error) {
	if len(respBody) == 0 {
		c.logger.Debug("OpenAI 响应为空")
		return nil, nil
	}

	// 解析 OpenAI 响应（使用 raw byte 以保留原始字段）
	var rawResp map[string]interface{}
	if err := json.Unmarshal(respBody, &rawResp); err != nil {
		c.logger.Debug("解析 OpenAI 响应失败",
			port.String("error", err.Error()),
		)
		return nil, nil
	}

	// 检测并记录响应中的特殊字段
	c.detectSpecialFields(rawResp)

	// 解析标准响应实体
	var openAIResp entity.Response
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		c.logger.Debug("解析 OpenAI 响应实体失败",
			port.String("error", err.Error()),
		)
		return nil, nil
	}

	// 如果没有提供 model，尝试从响应中获取
	if model == "" && openAIResp.Model != "" {
		model = openAIResp.Model
		c.logger.Debug("从响应中提取模型名称",
			port.String("model", model),
		)
	}

	// 提取额外的 OpenAI 特有字段用于日志记录
	hasToolCalls := c.hasToolCalls(rawResp)
	hasLogProbs := c.hasLogProbs(rawResp)
	systemFingerprint := c.extractSystemFingerprint(rawResp)
	hasRefusal := c.hasRefusal(rawResp)
	hasAnnotations := c.hasAnnotations(rawResp)

	// 记录额外字段信息
	if hasToolCalls || hasLogProbs || systemFingerprint != "" || hasRefusal || hasAnnotations {
		c.logger.Debug("OpenAI 响应包含额外字段",
			port.Bool("has_tool_calls", hasToolCalls),
			port.Bool("has_logprobs", hasLogProbs),
			port.String("system_fingerprint", systemFingerprint),
			port.Bool("has_refusal", hasRefusal),
			port.Bool("has_annotations", hasAnnotations),
		)
	}

	// 检查 choices 是否为空
	if len(openAIResp.Choices) == 0 {
		c.logger.Warn("OpenAI 响应中 choices 数组为空")
	}

	// 检查 finish_reason 是否为 null
	for i, choice := range openAIResp.Choices {
		if choice.FinishReason == "" && !choice.Delta.IsZero() {
			c.logger.Debug("OpenAI 响应 choice 的 finish_reason 为空",
				port.Int("choice_index", i),
			)
		}
	}

	// 检查 usage 是否为空（Usage 是值类型，比较零值）
	if openAIResp.Usage.IsEmpty() {
		c.logger.Debug("OpenAI 响应中 usage 信息为空")
	}

	// OpenAI 格式是标准格式，无需转换
	return &openAIResp, nil
}

// detectSpecialFields 检测并记录 OpenAI 响应中的特殊字段。
func (c *ResponseConverter) detectSpecialFields(rawResp map[string]interface{}) {
	// 检测 choices 中的特殊结构
	if choices, ok := rawResp["choices"].([]interface{}); ok {
		for i, choice := range choices {
			if choiceMap, ok := choice.(map[string]interface{}); ok {
				// 检测 content 过滤
				if finishReason, ok := choiceMap["finish_reason"].(string); ok {
					if finishReason == "content_filter" {
						c.logger.Debug("OpenAI 响应被内容过滤",
							port.Int("choice_index", i),
						)
					}
				}

				// 检测 logprobs
				if _, ok := choiceMap["logprobs"]; ok {
					c.logger.Debug("OpenAI 响应包含 logprobs",
						port.Int("choice_index", i),
					)
				}
			}
		}
	}

	// 检测 system_fingerprint
	if _, hasFP := rawResp["system_fingerprint"]; hasFP {
		if fp, ok := rawResp["system_fingerprint"].(string); ok && fp != "" {
			c.logger.Debug("OpenAI 响应包含 system_fingerprint",
				port.String("system_fingerprint", fp),
			)
		}
	}
}

// hasToolCalls 检查响应中是否包含工具调用。
func (c *ResponseConverter) hasToolCalls(rawResp map[string]interface{}) bool {
	choices, ok := rawResp["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return false
	}

	for _, choice := range choices {
		choiceMap, ok := choice.(map[string]interface{})
		if !ok {
			continue
		}

		msg, ok := choiceMap["message"].(map[string]interface{})
		if !ok {
			continue
		}

		if _, ok := msg["tool_calls"]; ok {
			return true
		}
	}
	return false
}

// hasLogProbs 检查响应中是否包含对数概率。
func (c *ResponseConverter) hasLogProbs(rawResp map[string]interface{}) bool {
	choices, ok := rawResp["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return false
	}

	choice := choices[0]
	choiceMap, ok := choice.(map[string]interface{})
	if !ok {
		return false
	}

	_, ok = choiceMap["logprobs"].(map[string]interface{})
	return ok
}

// extractSystemFingerprint 提取系统指纹。
func (c *ResponseConverter) extractSystemFingerprint(rawResp map[string]interface{}) string {
	if fp, ok := rawResp["system_fingerprint"].(string); ok {
		return fp
	}
	return ""
}

// hasRefusal 检查响应中是否包含拒绝回答。
func (c *ResponseConverter) hasRefusal(rawResp map[string]interface{}) bool {
	choices, ok := rawResp["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return false
	}

	for _, choice := range choices {
		choiceMap, ok := choice.(map[string]interface{})
		if !ok {
			continue
		}

		msg, ok := choiceMap["message"].(map[string]interface{})
		if !ok {
			continue
		}

		if _, ok := msg["refusal"]; ok {
			return true
		}
	}
	return false
}

// hasAnnotations 检查响应中是否包含引用标注。
func (c *ResponseConverter) hasAnnotations(rawResp map[string]interface{}) bool {
	choices, ok := rawResp["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return false
	}

	for _, choice := range choices {
		choiceMap, ok := choice.(map[string]interface{})
		if !ok {
			continue
		}

		msg, ok := choiceMap["message"].(map[string]interface{})
		if !ok {
			continue
		}

		content, ok := msg["content"].([]interface{})
		if !ok {
			continue
		}

		for _, block := range content {
			blockMap, ok := block.(map[string]interface{})
			if !ok {
				continue
			}

			if _, ok := blockMap["annotations"]; ok {
				return true
			}
		}
	}
	return false
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
