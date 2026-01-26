package port

import "time"

// === 日志字段常量定义 ===
// 采用 snake_case 命名规范，保持一致性和可维护性
const (
	// === 请求相关字段 ===
	FieldReqID         = "req_id"          // 请求ID
	FieldResponseID    = "response_id"     // 响应ID
	FieldRequestBodyID = "request_body_id" // 请求体日志ID

	// === 后端相关字段 ===
	FieldBackend      = "backend"       // 后端名称
	FieldBackendURL   = "backend_url"   // 后端地址
	FieldBackendModel = "backend_model" // 后端模型名称

	// === 模型相关字段 ===
	FieldModel         = "model"          // 模型名称
	FieldOriginalModel = "original_model" // 原始模型名称

	// === HTTP 相关字段 ===
	FieldMethod     = "method"      // HTTP方法
	FieldPath       = "path"        // 请求路径
	FieldProtocol   = "protocol"    // 客户端协议
	FieldRemoteAddr = "remote_addr" // 客户端远程地址
	FieldStatusCode = "status_code" // HTTP状态码

	// === 请求体/响应体相关字段 ===
	FieldBodySize = "body_size" // 请求体/响应体大小

	// === 路由相关字段 ===
	FieldTotalRoutes    = "total_routes"    // 总路由数
	FieldCooldownCount  = "cooldown_count"  // 冷却中的后端数量
	FieldAvailableCount = "available_count" // 可用的后端数量
	FieldFallbackRoutes = "fallback_routes" // 降级路由数

	// === 性能相关字段 ===
	FieldDurationMS = "duration_ms" // 持续时间（毫秒）
	FieldDelayMS    = "delay_ms"    // 延迟时间（毫秒）
	FieldTotalBytes = "total_bytes" // 总字节数

	// === 重试相关字段 ===
	FieldAttempt       = "attempt"        // 当前尝试次数
	FieldMaxRetries    = "max_retries"    // 最大重试次数
	FieldTotalAttempts = "total_attempts" // 总尝试次数
	FieldNextAttempt   = "next_attempt"   // 下次尝试序号

	// === 流式相关字段 ===
	FieldStreaming      = "streaming"       // 是否流式请求
	FieldClientProtocol = "client_protocol" // 客户端协议类型

	// === 错误相关字段 ===
	FieldErrorType = "error_type" // 错误类型
	FieldErrorCode = "error_code" // 错误代码
	FieldMessage   = "message"    // 消息内容
	FieldPriority  = "priority"   // 优先级
	FieldError     = "error"      // 错误信息

	// === SSE 流式相关字段 ===
	FieldChunkIndex = "chunk_index" // 数据块索引
	FieldChunkCount = "chunk_count" // 数据块数量
)

// === 字段辅助函数 ===

// ReqID 创建请求ID字段
func ReqID(id string) Field {
	return String(FieldReqID, id)
}

// ResponseID 创建响应ID字段
func ResponseID(id string) Field {
	return String(FieldResponseID, id)
}

// Backend 创建后端名称字段
func Backend(name string) Field {
	return String(FieldBackend, name)
}

// BackendURL 创建后端地址字段
func BackendURL(url string) Field {
	return String(FieldBackendURL, url)
}

// BackendModel 创建后端模型字段
func BackendModel(model string) Field {
	return String(FieldBackendModel, model)
}

// Model 创建模型字段
func Model(name string) Field {
	return String(FieldModel, name)
}

// OriginalModel 创建原始模型字段
func OriginalModel(name string) Field {
	return String(FieldOriginalModel, name)
}

// StatusCode 创建状态码字段
func StatusCode(code int) Field {
	return Int(FieldStatusCode, code)
}

// BodySize 创建请求体大小字段
func BodySize(size int) Field {
	return Int(FieldBodySize, size)
}

// DurationMS 创建持续时间字段
func DurationMS(d time.Duration) Field {
	return Int64(FieldDurationMS, d.Milliseconds())
}

// DurationMSInt 创建持续时间字段（使用毫秒值）
func DurationMSInt(ms int64) Field {
	return Int64(FieldDurationMS, ms)
}

// DelayMS 创建延迟时间字段
func DelayMS(d time.Duration) Field {
	return Int64(FieldDelayMS, d.Milliseconds())
}

// DelayMSInt 创建延迟时间字段（使用毫秒值）
func DelayMSInt(ms int64) Field {
	return Int64(FieldDelayMS, ms)
}

// TotalBytes 创建总字节数字段
func TotalBytes(n int) Field {
	return Int(FieldTotalBytes, n)
}

// Attempt 创建尝试次数字段
func Attempt(n int) Field {
	return Int(FieldAttempt, n)
}

// MaxRetries 创建最大重试次数字段
func MaxRetries(n int) Field {
	return Int(FieldMaxRetries, n)
}

// TotalAttempts 创建总尝试次数字段
func TotalAttempts(n int) Field {
	return Int(FieldTotalAttempts, n)
}

// NextAttempt 创建下次尝试字段
func NextAttempt(n int) Field {
	return Int(FieldNextAttempt, n)
}

// Streaming 创建流式字段
func Streaming(b bool) Field {
	return Bool(FieldStreaming, b)
}

// ClientProtocol 创建客户端协议字段
func ClientProtocol(p string) Field {
	return String(FieldClientProtocol, p)
}

// ChunkIndex 创建数据块索引字段
func ChunkIndex(n int) Field {
	return Int(FieldChunkIndex, n)
}

// ChunkCount 创建数据块数字段
func ChunkCount(n int) Field {
	return Int(FieldChunkCount, n)
}

// Method 创建方法字段
func Method(m string) Field {
	return String(FieldMethod, m)
}

// Path 创建路径字段
func Path(p string) Field {
	return String(FieldPath, p)
}

// RemoteAddr 创建远程地址字段
func RemoteAddr(a string) Field {
	return String(FieldRemoteAddr, a)
}

// TotalRoutes 创建总路由数字段
func TotalRoutes(n int) Field {
	return Int(FieldTotalRoutes, n)
}

// CooldownCount 创建冷却数量字段
func CooldownCount(n int) Field {
	return Int(FieldCooldownCount, n)
}

// AvailableCount 创建可用数量字段
func AvailableCount(n int) Field {
	return Int(FieldAvailableCount, n)
}

// FallbackRoutes 创建降级路由数字段
func FallbackRoutes(n int) Field {
	return Int(FieldFallbackRoutes, n)
}

// Priority 创建优先级字段
func Priority(p int) Field {
	return Int(FieldPriority, p)
}

// Content 创建内容字段（用于请求/错误日志）
func Content(c string) Field {
	return String("content", c)
}

// Status 创建状态字段
func Status(s string) Field {
	return String("status", s)
}

// BackendDetails 创建后端详情字段
func BackendDetails(d string) Field {
	return String("backend_details", d)
}
