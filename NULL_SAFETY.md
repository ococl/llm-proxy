# Null 值安全机制说明

## 概述

为了防止上游 API 返回的 `null` 值导致客户端类型验证错误，本项目在以下关键位置实施了多层防御机制。

## 问题场景

### 场景 1: 上游返回 null choices
```json
{
  "id": "resp-123",
  "model": "gpt-4",
  "choices": null
}
```

### 场景 2: 内部逻辑错误导致 null
```go
var choices []entity.Choice
// ... 某处逻辑未初始化 choices ...
builder.Choices(choices)  // nil 切片
```

## 安全机制

### 1. ResponseBuilder 构造器初始化

**位置**: `src/domain/entity/request.go`

```go
func NewResponseBuilder() *ResponseBuilder {
    return &ResponseBuilder{
        object:  "chat.completion",
        created: time.Now().Unix(),
        choices: []Choice{},  // ✅ 始终初始化为空数组
    }
}
```

**保护**: 确保即使不调用 `Choices()` 方法，`choices` 字段也是空数组而非 nil。

### 2. Choices() 方法 nil 转换

**位置**: `src/domain/entity/request.go`

```go
func (rb *ResponseBuilder) Choices(choices []Choice) *ResponseBuilder {
    if choices == nil {
        rb.choices = []Choice{}  // ✅ 将 nil 转换为空数组
    } else {
        rb.choices = choices
    }
    return rb
}
```

**保护**: 外部传入的 nil 切片会被自动转换为空数组。

### 3. BuildUnsafe() 最后防线

**位置**: `src/domain/entity/request.go`

```go
func (rb *ResponseBuilder) BuildUnsafe() *Response {
    if rb.choices == nil {
        rb.choices = []Choice{}  // ✅ 双重检查，确保绝对安全
    }
    return &Response{
        // ...
        Choices: rb.choices,  // 保证永远是 []Choice{} 或有内容的数组
    }
}
```

**保护**: 即使前面所有机制都失效，这里也会进行最后的 nil 检查。

### 4. 流式处理 - 上游 null 检测与日志

**位置**: `src/application/usecase/proxy_request.go`

```go
choicesArray := []entity.Choice{}  // ✅ 默认空数组

choicesRaw, choicesExists := chunkData["choices"]
if choicesExists && choicesRaw == nil {
    uc.logger.Warn("streaming: upstream returned null choices, using empty array",
        port.String("req_id", reqID),
        port.String("backend", backend.Name()),
        port.String("response_id", responseID),
    )  // ✅ 记录异常情况
}

if choices, ok := chunkData["choices"].([]interface{}); ok && len(choices) > 0 {
    // 处理正常的 choices
}

builder.Choices(choicesArray)  // ✅ 保证传入的是空数组或有效数组
```

**保护**: 
- 检测上游返回的 null 值
- 记录警告日志，便于问题排查
- 使用空数组替代 null

### 5. 非流式处理 - 上游 null 检测

**位置**: `src/adapter/backend/client_adapter.go`

```go
choicesRaw, choicesExists := respData["choices"]
if choicesExists && choicesRaw == nil {
    // TODO: 未来可添加日志记录
    builder.Choices([]entity.Choice{})  // ✅ 显式设置空数组
} else if choices, ok := respData["choices"].([]interface{}); ok && len(choices) > 0 {
    // 处理正常的 choices
}
```

**保护**: 在解析上游响应时就检测并处理 null 值。

## JSON 序列化行为

### Go 切片的序列化规则

```go
var nilSlice []Choice        // nil 切片
emptySlice := []Choice{}     // 空切片

json.Marshal(nilSlice)   // 输出: null
json.Marshal(emptySlice) // 输出: []
```

### 我们的保证

所有 `Response.Choices` 字段序列化后**永远**是：
- `[]` (空数组) 或
- `[{"index": 0, ...}]` (有内容的数组)

**绝不会出现**: `"choices": null`

## 日志增强

### 1. 流式请求失败日志

**位置**: `src/adapter/http/handler.go`

```go
if err := h.proxyUseCase.ExecuteStreaming(ctx, req, streamHandler); err != nil {
    h.logger.Error("streaming request failed",
        port.String("req_id", req.ID().String()),  // ✅ 添加 req_id
        port.String("model", req.Model().String()),
        port.Error(err),
    )
    return
}
```

### 2. Panic 恢复日志

**位置**: `src/adapter/http/middleware.go`

```go
rm.logger.Error("Panic recovered",
    port.String("req_id", reqID),  // ✅ 统一使用 req_id
    port.String("error", fmt.Sprintf("%v", err)),
    port.String("stack", stackStr),
)
```

### 3. 上游 null 检测警告日志

**位置**: `src/application/usecase/proxy_request.go`

```go
uc.logger.Warn("streaming: upstream returned null choices, using empty array",
    port.String("req_id", reqID),
    port.String("backend", backend.Name()),
    port.String("response_id", responseID),
)
```

**用途**: 
- 监控上游 API 异常行为
- 快速定位是哪个后端返回了 null
- 便于后续与上游服务商沟通

## 测试覆盖

### 新增测试文件

**位置**: `src/domain/entity/null_safety_test.go`

测试用例：

1. **TestResponseBuilder_NullSafety_UpstreamNullChoices**
   - 模拟上游返回 `"choices": null`
   - 验证反序列化后的处理逻辑
   - 确保最终输出为空数组

2. **TestResponseBuilder_NullSafety_DirectNilAssignment**
   - 直接传入 `nil` 给 `Choices()` 方法
   - 验证 nil 转换为空数组
   - 验证 JSON 序列化结果

3. **TestResponseBuilder_NullSafety_BuildUnsafeWithoutChoices**
   - 不调用 `Choices()` 方法
   - 验证构造器默认初始化的空数组
   - 验证 JSON 输出

4. **TestChoice_NullSafety_DeltaField**
   - 测试 `Delta` 为 nil 时的序列化
   - 测试 `Delta` 为空对象时的序列化
   - 验证 `omitempty` 标签行为

### 测试结果

```bash
$ cd src && go test ./domain/entity -v -run "NullSafety"
=== RUN   TestResponseBuilder_NullSafety_UpstreamNullChoices
--- PASS: TestResponseBuilder_NullSafety_UpstreamNullChoices (0.00s)
=== RUN   TestResponseBuilder_NullSafety_DirectNilAssignment
--- PASS: TestResponseBuilder_NullSafety_DirectNilAssignment (0.00s)
=== RUN   TestResponseBuilder_NullSafety_BuildUnsafeWithoutChoices
--- PASS: TestResponseBuilder_NullSafety_BuildUnsafeWithoutChoices (0.00s)
=== RUN   TestChoice_NullSafety_DeltaField
--- PASS: TestChoice_NullSafety_DeltaField (0.00s)
PASS
ok      llm-proxy/domain/entity 0.371s
```

## 调试指南

### 如何排查 null 相关问题

1. **检查日志中的 req_id**
   - 所有关键日志现在都包含 `req_id`
   - 可以快速关联请求链路

2. **搜索警告日志**
   ```bash
   grep "upstream returned null choices" logs/*.log
   ```

3. **定位具体后端**
   ```bash
   grep "backend.*null choices" logs/*.log
   ```

4. **查看完整请求上下文**
   ```bash
   grep "req_id.*YOUR_REQ_ID" logs/*.log
   ```

### 添加更多调试日志 (可选)

如果需要更详细的调试信息，可以在以下位置添加 Debug 日志：

```go
// src/adapter/backend/client_adapter.go
if choicesExists && choicesRaw == nil {
    // TODO: 添加 logger 字段到 BackendClientAdapter
    // a.logger.Debug("upstream null choices detected",
    //     port.String("backend", backend.Name()),
    //     port.String("response_id", responseID),
    // )
    builder.Choices([]entity.Choice{})
}
```

## 最佳实践

### 1. 永远使用 Builder 模式

```go
// ✅ 好的做法
resp := entity.NewResponseBuilder().
    ID("resp-123").
    Model("gpt-4").
    Choices(processedChoices).
    BuildUnsafe()

// ❌ 避免直接构造
resp := &entity.Response{
    Choices: choices,  // 可能为 nil
}
```

### 2. 处理上游响应时检查 nil

```go
// ✅ 好的做法
choicesRaw, exists := data["choices"]
if exists && choicesRaw == nil {
    logger.Warn("upstream null detected")
    builder.Choices([]entity.Choice{})
} else if choices, ok := data["choices"].([]interface{}); ok {
    // 处理正常情况
}

// ❌ 避免直接断言
choices := data["choices"].([]interface{})  // panic if nil
```

### 3. 记录异常情况

```go
// ✅ 好的做法
if unexpectedCondition {
    logger.Warn("unexpected null value",
        port.String("req_id", reqID),
        port.String("field", "choices"),
    )
    // 使用默认值
}
```

## 总结

### 多层防御体系

```
上游 API 返回
    ↓
[1] 反序列化检测 (adapter/backend)
    ↓
[2] Builder.Choices() nil 转换 (domain/entity)
    ↓
[3] BuildUnsafe() 双重检查 (domain/entity)
    ↓
[4] JSON 序列化输出
    ↓
永远是 [] 或 [...]，绝不会是 null
```

### 日志监控

- ✅ 所有关键路径都有 `req_id`
- ✅ 上游 null 值会触发警告日志
- ✅ Panic 恢复包含完整上下文
- ✅ 便于后续问题排查和分析

### 测试保障

- ✅ 4 个专门的 null 安全测试
- ✅ 覆盖上游 null、直接 nil、未初始化等场景
- ✅ 验证 JSON 序列化输出符合预期
- ✅ 所有现有测试保持通过

## 相关文档

- [STREAMING_FIX.md](STREAMING_FIX.md) - 流式响应修复说明
- [README.md](README.md) - 项目总体说明
