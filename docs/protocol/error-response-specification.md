# LLM API 错误响应格式规范

本文档汇总了主流 LLM API 的错误响应格式规范。

## 目录

- [OpenAI 错误响应](#openai-错误响应)
- [Anthropic 错误响应](#anthropic-错误响应)
- [Google AI 错误响应](#google-ai-错误响应)
- [通用错误码参考](#通用错误码参考)

---

## OpenAI 错误响应

### 错误响应结构

```json
{
  "error": {
    "message": "错误消息",
    "type": "错误类型",
    "param": "相关参数",
    "code": "错误码"
  }
}
```

### HTTP 状态码分类

| 状态码 | 错误类型 | 描述 |
|--------|----------|------|
| 400 | `invalid_request_error` | 请求参数错误 |
| 401 | `authentication_error` | 认证失败 |
| 403 | `permission_error` | 权限不足 |
| 404 | `not_found_error` | 资源不存在 |
| 429 | `rate_limit_error` | 速率限制 |
| 500 | `internal_server_error` | 服务器内部错误 |
| 503 | `service_unavailable_error` | 服务不可用 |

### 详细错误类型

#### 401 - 认证错误

```json
{
  "error": {
    "message": "Incorrect API key provided. You can find your API key at https://platform.openai.com/account/api-keys.",
    "type": "authentication_error",
    "param": null,
    "code": "invalid_api_key"
  }
}
```

#### 401 - 组织不存在

```json
{
  "error": {
    "message": "You must be a paid user to use the API. Please upgrade your plan at https://platform.openai.com/account/billing.",
    "type": "authentication_error",
    "param": null,
    "code": "organization_not_found"
  }
}
```

#### 400 - 请求参数错误

```json
{
  "error": {
    "message": "Invalid parameter: 'temperature' must be between 0 and 2.",
    "type": "invalid_request_error",
    "param": "temperature",
    "code": "param_invalid_value"
  }
}
```

#### 400 - 必填参数缺失

```json
{
  "error": {
    "message": "Missing required parameter: 'messages'.",
    "type": "invalid_request_error",
    "param": "messages",
    "code": "missing_required_parameter"
  }
}
```

#### 400 - 模型不支持

```json
{
  "error": {
    "message": "The model 'gpt-3.5-turbo' has been deprecated. Please use 'gpt-3.5-turbo-1106' instead.",
    "type": "invalid_request_error",
    "param": "model",
    "code": "model_deprecated"
  }
}
```

#### 429 - 速率限制

```json
{
  "error": {
    "message": "Rate limit reached. Please retry after 20 seconds.",
    "type": "rate_limit_error",
    "param": null,
    "code": "rate_limit_exceeded"
  }
}
```

#### 429 - Token 速率限制

```json
{
  "error": {
    "message": "This model's maximum context length is 128000 tokens. Please reduce your message length.",
    "type": "invalid_request_error",
    "param": "messages",
    "code": "context_length_exceeded"
  }
}
```

#### 500 - 服务器错误

```json
{
  "error": {
    "message": "An internal error occurred. Please retry your request.",
    "type": "internal_server_error",
    "param": null,
    "code": "internal_error"
  }
}
```

### 错误码完整列表

| 错误码 | 描述 |
|--------|------|
| `invalid_api_key` | 无效的 API Key |
| `invalid_auth_token` | 无效的认证令牌 |
| `invalid_api_key_org` | API Key 或组织 ID 无效 |
| `incorrect_api_key` | API Key 错误 |
| `model_deprecated` | 模型已弃用 |
| `rate_limit_exceeded` | 超过速率限制 |
| `context_length_exceeded` | 超出上下文长度限制 |
| `invalid_request_error` | 请求参数错误 |
| `authentication_error` | 认证错误 |
| `permission_error` | 权限错误 |
| `not_found_error` | 资源不存在 |
| `internal_server_error` | 服务器内部错误 |
| `service_unavailable_error` | 服务不可用 |

---

## Anthropic 错误响应

### 错误响应结构

```json
{
  "type": "error",
  "error": {
    "type": "错误类型",
    "message": "错误消息"
  }
}
```

### 错误类型

| 状态码 | 错误类型 | 描述 |
|--------|----------|------|
| 400 | `invalid_request` | 请求参数错误 |
| 401 | `authentication` | 认证失败 |
| 403 | `permission` | 权限不足 |
| 404 | `not_found` | 资源不存在 |
| 429 | `rate_limit` | 速率限制 |
| 500 | `overloaded_error` | 服务器过载（Anthropic 特有） |

### 详细错误示例

#### 401 - 认证失败

```json
{
  "type": "error",
  "error": {
    "type": "authentication",
    "message": "Invalid API Key"
  }
}
```

#### 400 - 请求错误

```json
{
  "type": "error",
  "error": {
    "type": "invalid_request",
    "message": "max_tokens must be at least 1"
  }
}
```

#### 400 - 超出上下文长度

```json
{
  "type": "error",
  "error": {
    "type": "invalid_request",
    "message": "Conversation length exceeds model context window"
  }
}
```

#### 429 - 速率限制

```json
{
  "type": "error",
  "error": {
    "type": "rate_limit",
    "message": "Rate limit exceeded. Please wait before retrying."
  }
}
```

#### 流式错误

```json
{
  "type": "message",
  "id": "msg_123",
  "role": "assistant",
  "content": null,
  "stop_reason": "error",
  "stop_sequence": null,
  "usage": {
    "input_tokens": 100,
    "output_tokens": 0
  }
}
```

### 流式错误事件

```json
{
  "type": "error",
  "error": {
    "type": "invalid_request",
    "message": "Connection terminated during request"
  }
}
```

---

## Google AI 错误响应

### 错误响应结构

```json
{
  "error": {
    "code": 状态码,
    "message": "错误消息",
    "status": "状态字符串",
    "details": []
  }
}
```

### 状态码

| gRPC 状态 | HTTP 状态 | 描述 |
|-----------|-----------|------|
| OK | 200 | 成功 |
| CANCELLED | 499 | 客户端取消 |
| INVALID_ARGUMENT | 400 | 请求参数错误 |
| DEADLINE_EXCEEDED | 504 | 超时 |
| NOT_FOUND | 404 | 资源不存在 |
| ALREADY_EXISTS | 409 | 资源已存在 |
| PERMISSION_DENIED | 403 | 权限不足 |
| RESOURCE_EXHAUSTED | 429 | 资源耗尽 |
| FAILED_PRECONDITION | 400 | 前置条件失败 |
| ABORTED | 409 | 中止 |
| OUT_OF_RANGE | 400 | 超出范围 |
| UNIMPLEMENTED | 501 | 未实现 |
| INTERNAL | 500 | 内部错误 |
| UNAVAILABLE | 503 | 不可用 |
| UNAUTHENTICATED | 401 | 未认证 |

### 错误示例

#### 400 - 请求参数错误

```json
{
  "error": {
    "code": 400,
    "message": "Invalid argument: temperature must be between 0.0 and 2.0",
    "status": "INVALID_ARGUMENT"
  }
}
```

#### 429 - 速率限制

```json
{
  "error": {
    "code": 429,
    "message": "Quota exceeded for Gemini API.",
    "status": "RESOURCE_EXHAUSTED",
    "details": [
      {
        "@type": "type.googleapis.com/google.rpc.ErrorInfo",
        "reason": "RATE_LIMIT_EXCEEDED",
        "domain": "googleapis.com"
      }
    ]
  }
}
```

---

## Azure OpenAI 错误响应

### 错误响应结构

```json
{
  "error": {
    "code": "错误码",
    "message": "错误消息"
  }
}
```

### 错误示例

#### 429 - 速率限制

```json
{
  "error": {
    "code": "429",
    "message": "Rate limit is exceeded. Try again in 1 seconds."
  }
}
```

#### 400 - 内容过滤

```json
{
  "error": {
    "code": "content_filter",
    "message": "The response was filtered due to the Azure OpenAI content filter policy."
  }
}
```

---

## 通用错误码参考

### HTTP 状态码速查

| 状态码 | 名称 | 建议处理 |
|--------|------|----------|
| 200 | OK | 成功处理 |
| 400 | Bad Request | 检查请求参数 |
| 401 | Unauthorized | 检查认证信息 |
| 403 | Forbidden | 检查权限 |
| 404 | Not Found | 检查资源是否存在 |
| 408 | Request Timeout | 重试请求 |
| 429 | Too Many Requests | 指数退避重试 |
| 500 | Internal Server Error | 重试请求 |
| 502 | Bad Gateway | 重试请求 |
| 503 | Service Unavailable | 稍后重试 |
| 504 | Gateway Timeout | 重试请求 |

### 错误处理策略

#### 指数退避重试

```python
import time

def retry_with_backoff(func, max_retries=5):
    for attempt in range(max_retries):
        try:
            return func()
        except RateLimitError:
            wait_time = 2 ** attempt
            time.sleep(wait_time)
        except (ServerError, ServiceUnavailableError):
            wait_time = 2 ** attempt
            time.sleep(wait_time)
    raise Exception(f"Failed after {max_retries} retries")
```

#### 错误分类处理

```go
func HandleAPIError(resp *http.Response) error {
    switch resp.StatusCode {
    case http.StatusBadRequest:
        return ParseValidationError(resp)
    case http.StatusUnauthorized:
        return ErrInvalidAPIKey
    case http.StatusForbidden:
        return ErrPermissionDenied
    case http.StatusNotFound:
        return ErrResourceNotFound
    case http.StatusTooManyRequests:
        return ParseRateLimitError(resp)
    case http.StatusInternalServerError:
        return ErrServerError
    case http.StatusServiceUnavailable:
        return ErrServiceUnavailable
    default:
        return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
    }
}
```

---

## LLM 代理中的错误处理

### 统一错误格式

```go
// LLMProxyError 定义统一的错误响应格式
type LLMProxyError struct {
    Code       string            `json:"code"`
    Message    string            `json:"message"`
    Type       string            `json:"type"`
    StatusCode int               `json:"status_code"`
    Details    map[string]string `json:"details,omitempty"`
    Source     string            `json:"source,omitempty"` // 错误来源: openai, anthropic, google
    RequestID  string            `json:"request_id,omitempty"`
    Timestamp  time.Time         `json:"timestamp"`
}

// NewLLMProxyError 创建新的代理错误
func NewLLMProxyError(source string, statusCode int, code string, message string) *LLMProxyError {
    return &LLMProxyError{
        Code:       code,
        Message:    message,
        Type:       GetErrorType(statusCode),
        StatusCode: statusCode,
        Source:     source,
        Timestamp:  time.Now(),
    }
}

// GetErrorType 根据状态码获取错误类型
func GetErrorType(statusCode int) string {
    switch statusCode {
    case 400:
        return "invalid_request_error"
    case 401:
        return "authentication_error"
    case 403:
        return "permission_error"
    case 404:
        return "not_found_error"
    case 429:
        return "rate_limit_error"
    case 500, 502, 503, 504:
        return "internal_server_error"
    default:
        return "unknown_error"
    }
}
```

### 错误转换示例

```go
// ConvertOpenAIError 将 OpenAI 错误转换为代理统一错误格式
func ConvertOpenAIError(openaiErr OpenAIError) *LLMProxyError {
    return NewLLMProxyError(
        "openai",
        GetHTTPCodeFromErrorType(openaiErr.Type),
        openaiErr.Code,
        openaiErr.Message,
    )
}

// GetHTTPCodeFromErrorType 将 OpenAI 错误类型映射到 HTTP 状态码
func GetHTTPCodeFromErrorType(errorType string) int {
    switch errorType {
    case "invalid_request_error":
        return http.StatusBadRequest
    case "authentication_error", "incorrect_api_key":
        return http.StatusUnauthorized
    case "permission_error":
        return http.StatusForbidden
    case "not_found_error":
        return http.StatusNotFound
    case "rate_limit_error":
        return http.StatusTooManyRequests
    case "internal_server_error", "service_unavailable_error":
        return http.StatusInternalServerError
    default:
        return http.StatusInternalServerError
    }
}
```

---

## 最佳实践

1. **始终检查状态码**: 不要假设请求总是成功
2. **实现重试机制**: 对 429、500、502、503、504 使用指数退避
3. **提供有意义的错误消息**: 将技术错误转换为用户友好的消息
4. **记录请求 ID**: 便于问题排查和客服支持
5. **实现降级策略**: 主要服务不可用时切换到备用服务
6. **设置合理的超时**: 避免长时间阻塞
7. **监控错误率**: 及时发现和处理系统问题

---

## 参考资料

- [OpenAI 错误码文档](https://platform.openai.com/docs/guides/error-codes)
- [Anthropic API 错误处理](https://docs.anthropic.com/en/api/errors)
- [Google AI 错误处理](https://ai.google.dev/docs/error-handling)
- [gRPC 状态码](https://grpc.io/docs/guides/status-codes/)
