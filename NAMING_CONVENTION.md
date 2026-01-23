# 命名规范 - Request ID

## 概述

本项目统一使用 `req_id` / `reqID` 作为请求标识符的命名规范。

## 统一规范

### 变量命名

| 场景 | 命名 | 示例 |
|------|------|------|
| Go 变量 | `reqID` (驼峰) | `reqID := req.ID().String()` |
| HTTP Header | `X-Request-ID` (Pascal-Kebab) | `r.Header.Get("X-Request-ID")` |
| JSON 字段 | `req_id` (蛇形) | `{"error": {"req_id": "req_abc123"}}` |
| 日志字段 | `req_id` (蛇形) | `port.String("req_id", reqID)` |
| Context Key | `req_id` (蛇形) | `ctx.Value("req_id")` |

### 类型定义

```go
// domain/entity/request.go
type RequestID string

func NewRequestID(id string) RequestID {
    return RequestID(id)
}
```

## 数据流

### 完整的请求 ID 流转

```
1. HTTP 请求到达
   ↓
2. Handler 生成 ID (handler.go:44)
   reqID := h.generateRequestID()  // "req_abc123def456"
   ↓
3. 转换为领域对象 (handler.go:205)
   entity.NewRequestID(reqID)
   ↓
4. UseCase 使用 (proxy_request.go:58)
   reqID := req.ID().String()
   ↓
5. 日志输出 (所有地方)
   port.String("req_id", reqID)
   ↓
6. HTTP 响应 (错误处理)
   {"error": {"req_id": "req_abc123def456"}}
```

### ID 格式

```go
// adapter/http/handler.go
func (h *ProxyHandler) generateRequestID() string {
    return "req_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:18]
}
```

**示例**: `req_abc123def456789012`

## 代码示例

### 1. 日志输出

```go
// ✅ 正确 - 使用 req_id
uc.logger.Info("request started",
    port.String("req_id", reqID),
    port.String("model", modelName),
)

// ❌ 错误 - 不要使用 trace_id
uc.logger.Info("request started",
    port.String("trace_id", reqID),  // 已废弃
)
```

### 2. 错误处理

```go
// ✅ 正确 - 使用 WithReqID
llmErr := domainerror.NewBadRequest("invalid input")
llmErr = llmErr.WithReqID(reqID)

// ❌ 错误 - 不要使用 WithTraceID
llmErr.WithTraceID(reqID)  // 已移除
```

### 3. HTTP 响应

```json
{
  "error": {
    "code": "BACKEND_ERROR",
    "message": "后端请求失败",
    "req_id": "req_abc123def456"
  }
}
```

### 4. HTTP Header 提取

```go
func extractReqID(r *http.Request) string {
    if reqID := r.Header.Get("X-Request-ID"); reqID != "" {
        return reqID
    }
    if reqID := r.Header.Get("X-Trace-ID"); reqID != "" {
        return reqID  // 兼容旧客户端
    }
    if reqID := r.Context().Value("req_id"); reqID != nil {
        if id, ok := reqID.(string); ok {
            return id
        }
    }
    return ""
}
```

## 领域层定义

### 常量 (domain/port/logging.go)

```go
const (
    FieldReqID = "req_id"
    // ... 其他字段
)

func ReqID(id string) Field {
    return String(FieldReqID, id)
}
```

### 错误类型 (domain/error/types.go)

```go
type LLMProxyError struct {
    Type        ErrorType
    Code        ErrorCode
    Message     string
    Cause       error
    ReqID       string  // 请求 ID
    BackendName string
    HTTPStatus  int
}

func (e *LLMProxyError) WithReqID(reqID string) *LLMProxyError {
    e.ReqID = reqID
    return e
}

func GetReqID(err error) string {
    var proxyErr *LLMProxyError
    if errors.As(err, &proxyErr) {
        return proxyErr.ReqID
    }
    return ""
}
```

## 向后兼容

### HTTP Header

为了兼容旧客户端，我们同时支持以下 Header：

1. **X-Request-ID** (推荐，优先级最高)
2. **X-Trace-ID** (兼容)

提取顺序：`X-Request-ID` > `X-Trace-ID` > `Context Value`

### 日志字段

**仅使用** `req_id`，不再使用 `trace_id`。

## 与分布式追踪的区别

### Request ID (请求 ID)

- **目的**: 业务层面的请求标识
- **范围**: 单次 API 请求
- **生命周期**: 从 HTTP 请求到响应
- **格式**: `req_abc123def456`
- **用途**: 日志关联、错误追踪、问题排查

### Trace ID (追踪 ID)

- **目的**: 观测性层面的调用链追踪
- **范围**: 跨服务的完整调用链
- **生命周期**: 整个分布式事务
- **格式**: OpenTelemetry 标准格式
- **用途**: 性能分析、链路追踪、依赖分析

**当前状态**: 本项目仅使用 Request ID，暂未集成分布式追踪系统。

**未来规划**: 如需集成 OpenTelemetry，将同时记录：

```go
logger.Info("request processed",
    port.String("req_id", reqID),           // 业务 ID
    port.String("trace_id", otelTraceID),   // 追踪 ID
)
```

## 重命名历史

### 2026-01-23 之前

使用混乱的命名：
- 变量: `traceID` / `reqID` 混用
- 日志: `trace_id`
- 错误: `TraceID`

### 2026-01-23 统一重命名

| 项目 | 旧名称 | 新名称 |
|------|--------|--------|
| Go 变量 | `traceID` | `reqID` |
| 日志字段 | `trace_id` | `req_id` |
| 常量 | `FieldTraceID` | `FieldReqID` |
| 函数 | `TraceID()` | `ReqID()` |
| 结构体字段 | `TraceID` | `ReqID` |
| 方法 | `WithTraceID()` | `WithReqID()` |
| 提取函数 | `extractTraceID()` | `extractReqID()` |
| HTTP 函数 | `WriteAPIErrorWithTrace()` | `WriteAPIErrorWithReqID()` |

### 影响的文件

1. `src/domain/port/logging.go` - 常量和辅助函数
2. `src/domain/error/types.go` - 错误结构体和方法
3. `src/domain/error/http.go` - HTTP 错误响应
4. `src/application/usecase/proxy_request.go` - 所有日志输出
5. `src/adapter/http/handler.go` - HTTP 处理器
6. `src/adapter/http/middleware.go` - 中间件
7. `src/adapter/http/error_presenter.go` - 错误展示
8. `src/adapter/logging/adapter.go` - 日志适配器
9. `src/adapter/http/http_test.go` - HTTP 测试

## 检查清单

在添加新代码时，请确保：

- [ ] 变量命名使用 `reqID`
- [ ] 日志字段使用 `"req_id"`
- [ ] HTTP Header 使用 `X-Request-ID`
- [ ] JSON 字段使用 `req_id`
- [ ] 不使用 `traceID` / `trace_id`
- [ ] 使用 `port.ReqID(id)` 辅助函数
- [ ] 错误处理使用 `WithReqID()`

## 相关文档

- [STREAMING_FIX.md](STREAMING_FIX.md) - 流式响应修复
- [NULL_SAFETY.md](NULL_SAFETY.md) - Null 值安全机制
- [README.md](README.md) - 项目总览
