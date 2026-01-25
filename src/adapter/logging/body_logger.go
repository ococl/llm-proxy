package logging

import (
	"llm-proxy/domain/port"
	"llm-proxy/infrastructure/logging"
)

// BodyLoggerAdapter 将 infrastructure/logging 的便捷函数适配为 port.BodyLogger 接口
type BodyLoggerAdapter struct{}

// NewBodyLoggerAdapter 创建一个新的 BodyLoggerAdapter
func NewBodyLoggerAdapter() *BodyLoggerAdapter {
	return &BodyLoggerAdapter{}
}

// LogRequestBody 记录请求体日志
func (a *BodyLoggerAdapter) LogRequestBody(
	reqID string,
	logType port.BodyLogType,
	method, path, protocol string,
	headers map[string][]string,
	body map[string]interface{},
) {
	logging.LogRequestBody(reqID, logging.BodyLogType(logType), method, path, protocol, headers, body)
}

// LogResponseBody 记录响应体日志
func (a *BodyLoggerAdapter) LogResponseBody(
	reqID string,
	logType port.BodyLogType,
	statusCode int,
	headers map[string][]string,
	body interface{},
) {
	logging.LogResponseBody(reqID, logging.BodyLogType(logType), statusCode, headers, body)
}
