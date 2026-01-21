package proxy

import (
	"encoding/json"
	"fmt"
	"strings"

	"llm-proxy/logging"
)

// RequestBodyPreparer 请求体准备器
type RequestBodyPreparer struct {
	converter       *ProtocolConverter
	requestDetector *RequestDetector
}

// NewRequestBodyPreparer 创建请求体准备器
func NewRequestBodyPreparer(converter *ProtocolConverter, detector *RequestDetector) *RequestBodyPreparer {
	return &RequestBodyPreparer{
		converter:       converter,
		requestDetector: detector,
	}
}

// PrepareResult 准备结果
type PrepareResult struct {
	Body             []byte
	ConversionMeta   *ConversionMetadata
	IsPassthrough    bool
	OriginalBodySize int
	ConvertedSize    int
}

// PrepareRequestBody 准备请求体
func (p *RequestBodyPreparer) PrepareRequestBody(
	reqBody map[string]interface{},
	originalBody []byte,
	route *ResolvedRoute,
	protocol string,
	clientProtocol RequestProtocol,
	reqID string,
	logBuilder *strings.Builder,
) (*PrepareResult, error) {
	// 检测是否为直通场景
	isPassthrough := (clientProtocol == ProtocolAnthropic && protocol == "anthropic") ||
		(clientProtocol == ProtocolOpenAI && protocol == "openai")

	if isPassthrough {
		return p.handlePassthrough(originalBody, route, protocol, reqID, logBuilder)
	}

	if protocol == "anthropic" && clientProtocol == ProtocolOpenAI {
		return p.handleOpenAIToAnthropic(reqBody, route, reqID, logBuilder, len(originalBody))
	}

	if protocol == "openai" && clientProtocol == ProtocolAnthropic {
		return p.handleAnthropicToOpenAI(reqBody, route, reqID, logBuilder, originalBody)
	}

	// OpenAI → OpenAI（非直通场景）
	return p.handleOpenAIToOpenAI(reqBody, route, reqID, logBuilder, originalBody)
}

// handlePassthrough 处理协议直通
func (p *RequestBodyPreparer) handlePassthrough(
	originalBody []byte,
	route *ResolvedRoute,
	protocol string,
	reqID string,
	logBuilder *strings.Builder,
) (*PrepareResult, error) {
	var tempBody map[string]interface{}
	if err := json.Unmarshal(originalBody, &tempBody); err != nil {
		logBuilder.WriteString(fmt.Sprintf("解析原始请求体失败: %v\n", err))
		logging.ProxySugar.Errorw("解析原始请求体失败", "reqID", reqID, "error", err)
		return nil, err
	}

	originalModel, _ := tempBody["model"].(string)
	if originalModel == route.Model {
		logBuilder.WriteString(fmt.Sprintf("✓ 协议直通 (%s)，model字段无需替换\n", protocol))
		logging.ProxySugar.Infow("协议直通处理完成(无修改)",
			"reqID", reqID,
			"protocol", protocol,
			"model", route.Model,
			"body_size", len(originalBody))

		return &PrepareResult{
			Body:             originalBody,
			IsPassthrough:    true,
			OriginalBodySize: len(originalBody),
			ConvertedSize:    len(originalBody),
		}, nil
	}

	tempBody["model"] = route.Model
	newBody, err := json.Marshal(tempBody)
	if err != nil {
		logBuilder.WriteString(fmt.Sprintf("序列化请求体失败: %v\n", err))
		logging.ProxySugar.Errorw("序列化请求体失败", "reqID", reqID, "error", err)
		return nil, err
	}

	logBuilder.WriteString(fmt.Sprintf("✓ 协议直通 (%s)，仅替换model字段: %s → %s\n", protocol, originalModel, route.Model))
	logging.ProxySugar.Infow("协议直通处理完成",
		"reqID", reqID,
		"protocol", protocol,
		"original_model", originalModel,
		"target_model", route.Model,
		"original_size", len(originalBody),
		"modified_size", len(newBody))

	return &PrepareResult{
		Body:             newBody,
		IsPassthrough:    true,
		OriginalBodySize: len(originalBody),
		ConvertedSize:    len(newBody),
	}, nil
}

// handleOpenAIToAnthropic 处理 OpenAI → Anthropic 转换
func (p *RequestBodyPreparer) handleOpenAIToAnthropic(
	reqBody map[string]interface{},
	route *ResolvedRoute,
	reqID string,
	logBuilder *strings.Builder,
	originalBodySize int,
) (*PrepareResult, error) {
	modifiedBody := make(map[string]interface{})
	for k, v := range reqBody {
		modifiedBody[k] = v
	}
	modifiedBody["model"] = route.Model

	newBody, err := p.converter.ConvertToAnthropic(modifiedBody)
	if err != nil {
		logBuilder.WriteString(fmt.Sprintf("转换为Anthropic格式失败: %v\n", err))
		logging.ProxySugar.Errorw("协议转换失败",
			"reqID", reqID,
			"protocol", "anthropic",
			"error", err)
		return nil, err
	}

	// 获取转换元数据
	convMeta := p.converter.GetLastConversion()
	convertedSize := len(newBody)

	// 记录转换详情
	if convMeta != nil {
		logging.ProxySugar.Infow("参数转换详情",
			"reqID", reqID,
			"backend", route.BackendName,
			"input_max_tokens", convMeta.InputMaxTokens,
			"output_max_tokens", convMeta.OutputMaxTokens,
			"max_tokens_source", convMeta.MaxTokensSource,
			"input_temperature", convMeta.InputTemperature,
			"output_temperature", convMeta.OutputTemperature,
			"input_top_p", convMeta.InputTopP,
			"output_top_p", convMeta.OutputTopP,
			"input_stream", convMeta.InputStream,
			"output_stream", convMeta.OutputStream,
			"input_stop", convMeta.InputStop,
			"output_stop", convMeta.OutputStop,
			"input_tools_count", convMeta.InputTools,
			"output_tools_count", convMeta.OutputTools,
			"system_prompt_length", convMeta.SystemPromptLen)

		logBuilder.WriteString("✓ 已转换为Anthropic协议格式\n")
		logBuilder.WriteString(fmt.Sprintf("  max_tokens: %v → %d (%s)\n",
			convMeta.InputMaxTokens, convMeta.OutputMaxTokens, convMeta.MaxTokensSource))
		if convMeta.SystemPromptLen > 0 {
			logBuilder.WriteString(fmt.Sprintf("  system prompt: %d 字符\n", convMeta.SystemPromptLen))
		}
	} else {
		logging.ProxySugar.Warnw("转换元数据为空", "reqID", reqID, "backend", route.BackendName)
		logBuilder.WriteString("✓ 已转换为Anthropic协议格式（无元数据）\n")
	}

	logging.ProxySugar.Infow("Anthropic 请求体转换完成",
		"reqID", reqID,
		"original_size_bytes", originalBodySize,
		"converted_size_bytes", convertedSize,
		"backend", route.BackendName,
		"model", route.Model)

	return &PrepareResult{
		Body:             newBody,
		ConversionMeta:   convMeta,
		IsPassthrough:    false,
		OriginalBodySize: originalBodySize,
		ConvertedSize:    convertedSize,
	}, nil
}

// handleAnthropicToOpenAI 处理 Anthropic → OpenAI 转换
func (p *RequestBodyPreparer) handleAnthropicToOpenAI(
	reqBody map[string]interface{},
	route *ResolvedRoute,
	reqID string,
	logBuilder *strings.Builder,
	originalBody []byte,
) (*PrepareResult, error) {
	convertedBody, err := p.requestDetector.ConvertAnthropicToOpenAI(reqBody)
	if err != nil {
		logBuilder.WriteString(fmt.Sprintf("转换Anthropic请求失败: %v\n", err))
		logging.NetworkSugar.Errorw("转换Anthropic请求失败", "reqID", reqID, "error", err)
		return nil, err
	}

	convertedBody["model"] = route.Model
	newBody, err := json.Marshal(convertedBody)
	if err != nil {
		logBuilder.WriteString(fmt.Sprintf("序列化请求体失败: %v\n", err))
		logging.ProxySugar.Errorw("序列化请求体失败", "reqID", reqID, "error", err)
		return nil, err
	}

	logBuilder.WriteString("✓ 已转换为OpenAI协议格式\n")
	logging.ProxySugar.Infow("协议转换完成",
		"reqID", reqID,
		"from", "anthropic",
		"to", "openai",
		"backend", route.BackendName)

	return &PrepareResult{
		Body:             newBody,
		IsPassthrough:    false,
		OriginalBodySize: len(originalBody),
		ConvertedSize:    len(newBody),
	}, nil
}

// handleOpenAIToOpenAI 处理 OpenAI → OpenAI（非直通）
func (p *RequestBodyPreparer) handleOpenAIToOpenAI(
	reqBody map[string]interface{},
	route *ResolvedRoute,
	reqID string,
	logBuilder *strings.Builder,
	originalBody []byte,
) (*PrepareResult, error) {
	modifiedBody := make(map[string]interface{})
	for k, v := range reqBody {
		modifiedBody[k] = v
	}
	modifiedBody["model"] = route.Model

	newBody, err := json.Marshal(modifiedBody)
	if err != nil {
		logBuilder.WriteString(fmt.Sprintf("序列化请求体失败: %v\n", err))
		logging.ProxySugar.Errorw("序列化请求体失败", "reqID", reqID, "error", err)
		return nil, err
	}

	logBuilder.WriteString("✓ 使用OpenAI协议格式\n")

	return &PrepareResult{
		Body:             newBody,
		IsPassthrough:    false,
		OriginalBodySize: len(originalBody),
		ConvertedSize:    len(newBody),
	}, nil
}
