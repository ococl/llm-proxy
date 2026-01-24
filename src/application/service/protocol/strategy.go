package protocol

import (
	"llm-proxy/domain/entity"
	"llm-proxy/domain/types"

	domainerror "llm-proxy/domain/error"
)

// RequestConverterStrategy 定义请求协议转换策略接口。
// 每个 LLM 提供商实现此接口以处理请求格式转换。
//
// 职责：
// - 将标准请求格式转换为特定提供商的请求格式
// - 处理提供商特定的字段（如 Anthropic 的 system 字段）
// - 处理工具调用格式差异
type RequestConverterStrategy interface {
	// Convert 将请求转换为目标协议格式。
	//
	// 参数：
	//   - req: 原始请求（客户端请求或标准化请求）
	//   - systemPrompt: 可选的全局系统提示
	//
	// 返回：
	//   - 转换后的请求
	//   - 错误信息（如果转换失败）
	Convert(req *entity.Request, systemPrompt string) (*entity.Request, error)

	// Supports 检查策略是否支持指定协议。
	Supports(protocol types.Protocol) bool

	// Protocol 返回策略支持的协议类型。
	Protocol() types.Protocol

	// Name 返回策略名称，用于日志和调试。
	Name() string
}

// ResponseConverterStrategy 定义响应协议转换策略接口。
// 每个 LLM 提供商实现此接口以处理响应格式转换。
//
// 职责：
// - 将提供商的响应格式转换为标准响应格式
// - 处理内容类型差异（如 Anthropic 的 content 数组）
// - 处理停止原因标准化
type ResponseConverterStrategy interface {
	// Convert 将响应从目标协议格式转换为标准格式。
	//
	// 参数：
	//   - respBody: 原始响应体字节
	//   - model: 模型名称
	//
	// 返回：
	//   - 标准化后的响应
	//   - 错误信息（如果转换失败）
	Convert(respBody []byte, model string) (*entity.Response, error)

	// Supports 检查策略是否支持指定协议。
	Supports(protocol types.Protocol) bool

	// Protocol 返回策略支持的协议类型。
	Protocol() types.Protocol

	// Name 返回策略名称，用于日志和调试。
	Name() string
}

// StreamChunkConverterStrategy 定义流式响应块转换策略接口。
// 处理流式响应中的单个数据块。
//
// 职责：
// - 解析提供商特定的流式格式
// - 提取增量数据（delta）
// - 处理流式结束信号
type StreamChunkConverterStrategy interface {
	// ParseChunk 解析流式数据块。
	//
	// 参数：
	//   - data: 原始数据块字节
	//
	// 返回：
	//   - 解析后的流式块结构
	//   - 错误信息
	ParseChunk(data []byte) (*entity.StreamChunk, error)

	// BuildChunk 将标准流式块转换为目标协议格式。
	//
	// 参数：
	//   - chunk: 标准流式块
	//
	// 返回：
	//   - 格式化的数据块字节
	//   - 错误信息
	BuildChunk(chunk *entity.StreamChunk) ([]byte, error)

	// ParseStream 解析整个流式响应。
	//
	// 参数：
	//   - stream: 流式响应字节
	//
	// 返回：
	//   - 解析器通道
	//   - 错误通道
	ParseStream(stream []byte) (<-chan *entity.StreamChunk, <-chan error)

	// Supports 检查策略是否支持指定协议。
	Supports(protocol types.Protocol) bool

	// Protocol 返回策略支持的协议类型。
	Protocol() types.Protocol

	// Name 返回策略名称，用于日志和调试。
	Name() string
}

// ErrorConverterStrategy 定义错误响应转换策略接口。
// 处理提供商特定的错误格式转换为标准错误格式。
type ErrorConverterStrategy interface {
	// Convert 将提供商错误响应转换为标准错误格式。
	//
	// 参数：
	//   - statusCode: HTTP 状态码
	//   - respBody: 响应体字节
	//
	// 返回：
	//   - 标准化错误信息
	Convert(statusCode int, respBody []byte) *domainerror.LLMProxyError

	// Supports 检查策略是否支持指定协议。
	Supports(protocol types.Protocol) bool

	// Protocol 返回策略支持的协议类型。
	Protocol() types.Protocol

	// Name 返回策略名称，用于日志和调试。
	Name() string
}
