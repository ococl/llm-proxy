package types

// Protocol represents the API protocol of a backend.
// 支持多种 LLM 提供商的协议格式。
type Protocol string

const (
	// ProtocolOpenAI 表示 OpenAI API 协议格式
	ProtocolOpenAI Protocol = "openai"
	// ProtocolAnthropic 表示 Anthropic API 协议格式
	ProtocolAnthropic Protocol = "anthropic"
	// ProtocolGoogle 表示 Google Vertex AI API 协议格式
	ProtocolGoogle Protocol = "google"
	// ProtocolAzure 表示 Microsoft Azure OpenAI API 协议格式
	ProtocolAzure Protocol = "azure"
	// ProtocolDeepSeek 表示 DeepSeek API 协议格式
	ProtocolDeepSeek Protocol = "deepseek"
	// ProtocolGroq 表示 Groq API 协议格式
	ProtocolGroq Protocol = "groq"
	// ProtocolMistral 表示 Mistral AI API 协议格式
	ProtocolMistral Protocol = "mistral"
	// ProtocolCohere 表示 Cohere API 协议格式
	ProtocolCohere Protocol = "cohere"
	// ProtocolCustom 表示自定义协议（直通模式）
	ProtocolCustom Protocol = "custom"
)

// IsValid 检查协议值是否有效。
func (p Protocol) IsValid() bool {
	switch p {
	case ProtocolOpenAI, ProtocolAnthropic, ProtocolGoogle, ProtocolAzure,
		ProtocolDeepSeek, ProtocolGroq, ProtocolMistral, ProtocolCohere, ProtocolCustom:
		return true
	default:
		return false
	}
}

// IsAnthropicCompatible 检查协议是否与 Anthropic 兼容。
// 兼容 Anthropic 格式的提供商可以使用相同的转换逻辑。
func (p Protocol) IsAnthropicCompatible() bool {
	switch p {
	case ProtocolAnthropic:
		return true
	default:
		return false
	}
}

// IsOpenAICompatible 检查协议是否与 OpenAI 兼容。
// 兼容 OpenAI 格式的提供商可以使用相同的转换逻辑。
func (p Protocol) IsOpenAICompatible() bool {
	switch p {
	case ProtocolOpenAI, ProtocolAzure, ProtocolDeepSeek, ProtocolGroq,
		ProtocolMistral, ProtocolCohere:
		return true
	default:
		return false
	}
}

// RequiresSystemPromptField 检查协议是否需要独立的 system 字段。
// Anthropic 等协议要求 system prompt 作为独立字段，而不是 role: system 消息。
func (p Protocol) RequiresSystemPromptField() bool {
	return p == ProtocolAnthropic
}

// SupportsTools 检查协议是否支持工具调用功能。
func (p Protocol) SupportsTools() bool {
	switch p {
	case ProtocolOpenAI, ProtocolAnthropic, ProtocolAzure,
		ProtocolMistral, ProtocolCohere, ProtocolGoogle:
		return true
	default:
		return false
	}
}

// StreamingFormat 返回协议的流式响应格式类型。
type StreamingFormat string

const (
	// StreamingFormatSSE 表示 Server-Sent Events 格式
	StreamingFormatSSE StreamingFormat = "sse"
	// StreamingFormatJSONLines 表示 JSON Lines 格式
	StreamingFormatJSONLines StreamingFormat = "jsonlines"
	// StreamingFormatRaw 表示原始字节流
	StreamingFormatRaw StreamingFormat = "raw"
)

// GetStreamingFormat 返回协议的标准流式响应格式。
func (p Protocol) GetStreamingFormat() StreamingFormat {
	switch p {
	case ProtocolOpenAI, ProtocolAzure, ProtocolDeepSeek, ProtocolGroq, ProtocolMistral, ProtocolCohere:
		return StreamingFormatSSE
	case ProtocolAnthropic:
		// Anthropic 使用特定的 SSE 格式
		return StreamingFormatSSE
	case ProtocolGoogle:
		return StreamingFormatJSONLines
	default:
		return StreamingFormatSSE
	}
}
