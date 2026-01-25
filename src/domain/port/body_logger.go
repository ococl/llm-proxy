package port

// BodyLogType 日志类型常量
type BodyLogType string

const (
	BodyLogTypeClientRequest    BodyLogType = "client_request"
	BodyLogTypeClientResponse   BodyLogType = "client_response"
	BodyLogTypeUpstreamRequest  BodyLogType = "upstream_request"
	BodyLogTypeUpstreamResponse BodyLogType = "upstream_response"
)

// BodyLogger 接口提供请求/响应体日志记录功能
// 用于解耦 adapter 层与 infrastructure/logging 的直接依赖
type BodyLogger interface {
	// LogRequestBody 记录请求体日志
	LogRequestBody(reqID string, logType BodyLogType, method, path, protocol string, headers map[string][]string, body map[string]interface{})

	// LogResponseBody 记录响应体日志
	LogResponseBody(reqID string, logType BodyLogType, statusCode int, headers map[string][]string, body interface{})
}
