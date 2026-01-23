# Streaming Response Fixes - Type Validation Issues

## Problems Description

### Problem 1: Empty Role Field

**Error:** `AI_TypeValidationError` from Cherry Studio (AI SDK)

**Root Cause:** The streaming response was sending empty `role` field (`"role": ""`) in the `delta` object, which violated the AI SDK's type validation rules. The SDK expects:
- `role` field to be `"assistant"` (or other valid role)
- OR the `role` field to be completely omitted (not present in JSON)

**Error Details:**
```json
{
  "code": "invalid_value",
  "path": ["choices", 0, "delta", "role"],
  "message": "Invalid input",
  "values": ["assistant"]
}
```

### Problem 2: Null Choices Array

**Error:** `AI_TypeValidationError` from Cherry Studio (AI SDK)

**Root Cause:** Some streaming chunks were sending `"choices": null` instead of an empty array, which violated the AI SDK's type validation. The SDK expects:
- `choices` field to always be an array (even if empty: `[]`)
- NOT `null`

**Error Details:**
```json
{
  "code": "invalid_type",
  "path": ["choices"],
  "message": "Expected array, received null"
}
```

## Solutions Implemented

### Solution 1: Fix Empty Role Field

#### 1.1. **Modified Message Struct JSON Tag** (`src/domain/entity/request.go`)

**Before:**
```go
type Message struct {
    Role       string     `json:"role"`
    Content    string     `json:"content"`
    ...
}
```

**After:**
```go
type Message struct {
    Role       string     `json:"role,omitempty"`  // Added omitempty
    Content    string     `json:"content"`
    ...
}
```

**Effect:** When `Role` is an empty string, it will be completely omitted from the JSON output.

#### 1.2. **Simplified Streaming Handler Logic** (`src/application/usecase/proxy_request.go`)

**Before:**
```go
if role == "" && content == "" {
    choice.Delta = &entity.Message{}
} else if role != "" {
    choice.Delta = &entity.Message{
        Role:    role,
        Content: content,
    }
} else {
    choice.Delta = &entity.Message{
        Content: content,
    }
}
```

**After:**
```go
if role != "" || content != "" {
    choice.Delta = &entity.Message{
        Role:    role,      // Will be omitted if empty due to omitempty
        Content: content,
    }
}
```

**Effect:** Cleaner code that relies on `omitempty` to handle empty role fields.

### Solution 2: Fix Null Choices Array

#### 2.1. **Modified ResponseBuilder** (`src/domain/entity/request.go`)

**Changes:**

1. **Initialize with empty array in constructor:**
```go
func NewResponseBuilder() *ResponseBuilder {
    return &ResponseBuilder{
        object:  "chat.completion",
        created: time.Now().Unix(),
        choices: []Choice{},  // Always initialize as empty array
    }
}
```

2. **Handle nil in Choices() setter:**
```go
func (rb *ResponseBuilder) Choices(choices []Choice) *ResponseBuilder {
    if choices == nil {
        rb.choices = []Choice{}  // Convert nil to empty array
    } else {
        rb.choices = choices
    }
    return rb
}
```

3. **Fix BuildUnsafe() to never return nil:**
```go
func (rb *ResponseBuilder) BuildUnsafe() *Response {
    if rb.choices == nil {
        rb.choices = []Choice{}
    }
    return &Response{
        ID:            rb.id,
        Object:        rb.object,
        Created:       rb.created,
        Model:         rb.model,
        Choices:       rb.choices,  // Always []Choice{} or populated array
        Usage:         rb.usage,
        StopReason:    rb.stopReason,
        StopSequences: rb.stopSequences,
        Headers:       rb.headers,
    }
}
```

**Effect:** `choices` field is guaranteed to be `[]` or `[...]`, never `null`.

#### 2.2. **Ensure Streaming Handler Initializes Array** (`src/application/usecase/proxy_request.go`)

```go
// Always initialize choices as empty array
choicesArray := []Choice{}

// ... populate choices ...

// Use builder that ensures non-nil
builder.Choices(choicesArray)
```

**Effect:** Every streaming chunk has valid `choices` array.

## JSON Output Examples

### First Chunk (with role)
```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion.chunk",
  "created": 1737634800,
  "model": "gpt-4",
  "choices": [{
    "index": 0,
    "delta": {
      "role": "assistant",
      "content": ""
    }
  }]
}
```

### Subsequent Chunks (content only)
```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion.chunk",
  "created": 1737634800,
  "model": "gpt-4",
  "choices": [{
    "index": 0,
    "delta": {
      "content": "Hello"
    }
  }]
}
```

### Empty Chunk (no content)
```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion.chunk",
  "created": 1737634800,
  "model": "gpt-4",
  "choices": []
}
```

### Final Chunk (with usage)
```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion.chunk",
  "created": 1737634800,
  "model": "gpt-4",
  "choices": [],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 25,
    "total_tokens": 35
  }
}
```

**Key Points:**
- ✅ The `role` field is **completely absent** when empty, not `"role": ""`
- ✅ The `choices` field is **always an array** (`[]` or `[...]`), never `null`

## Testing

### Unit Tests Added (`src/domain/entity/streaming_test.go`)

#### Tests for Problem 1 (Empty Role):
1. **TestMessage_JSONSerialization_EmptyRole**
   - Verifies empty role is omitted from JSON
   - Verifies non-empty role is included

2. **TestChoice_DeltaJSONSerialization**
   - Verifies delta with empty role omits role field
   - Verifies delta with role includes role field
   - Verifies nil delta is omitted

3. **TestResponse_StreamingChunkSerialization**
   - Full integration test for streaming chunk serialization
   - Ensures empty role in delta is omitted

#### Tests for Problem 2 (Null Choices):
4. **TestResponseBuilder_NeverReturnsNullChoices**
   - Verifies empty builder returns `[]` not `null`
   - Verifies builder with nil choices returns `[]`
   - Verifies builder with choices returns the array

5. **TestNewResponseBuilder_InitializesEmptyChoices**
   - Verifies constructor initializes choices as `[]`
   - Verifies JSON serialization produces `"choices":[]`

### Test Results

#### Streaming Tests (Problem 1)
```bash
$ cd src && go test ./domain/entity -v -run "TestMessage_JSON|TestChoice_Delta|TestResponse_Streaming"
=== RUN   TestMessage_JSONSerialization_EmptyRole
--- PASS: TestMessage_JSONSerialization_EmptyRole (0.00s)
=== RUN   TestChoice_DeltaJSONSerialization
--- PASS: TestChoice_DeltaJSONSerialization (0.00s)
=== RUN   TestResponse_StreamingChunkSerialization
--- PASS: TestResponse_StreamingChunkSerialization (0.00s)
PASS
ok  	llm-proxy/domain/entity	0.281s
```

#### ResponseBuilder Tests (Problem 2)
```bash
$ cd src && go test ./domain/entity -v -run "TestResponseBuilder_NeverReturnsNull|TestNewResponseBuilder_Initializes"
=== RUN   TestResponseBuilder_NeverReturnsNullChoices
=== RUN   TestResponseBuilder_NeverReturnsNullChoices/Empty_builder_should_return_empty_choices_array
=== RUN   TestResponseBuilder_NeverReturnsNullChoices/Builder_with_nil_choices_should_return_empty_array
=== RUN   TestResponseBuilder_NeverReturnsNullChoices/Builder_with_choices_should_return_those_choices
--- PASS: TestResponseBuilder_NeverReturnsNullChoices (0.00s)
=== RUN   TestNewResponseBuilder_InitializesEmptyChoices
--- PASS: TestNewResponseBuilder_InitializesEmptyChoices (0.00s)
PASS
ok  	llm-proxy/domain/entity	0.275s
```

### All Tests Passing
```bash
$ cd src && go test ./... -count=1
ok  	llm-proxy/adapter/backend	0.619s
ok  	llm-proxy/adapter/config	0.626s
ok  	llm-proxy/adapter/http	0.595s
ok  	llm-proxy/adapter/http/middleware	0.927s
ok  	llm-proxy/application/service	0.734s
ok  	llm-proxy/application/usecase	0.379s
ok  	llm-proxy/domain/entity	0.627s
ok  	llm-proxy/domain/service	0.680s
ok  	llm-proxy/infrastructure/config	0.601s
ok  	llm-proxy/infrastructure/logging	0.568s
```

## Build Verification

```bash
$ cd src && go build -o ../dist/llm-proxy.exe .
Build successful
```

## 安全机制

### Null 值防护

为了防止上游 API 或内部逻辑错误导致 `choices` 为 null，项目实施了多层防御机制：

1. **ResponseBuilder 构造器初始化**: `choices: []Choice{}`
2. **Choices() 方法 nil 转换**: `if choices == nil { rb.choices = []Choice{} }`
3. **BuildUnsafe() 双重检查**: 最后防线确保绝对安全
4. **上游响应检测**: 检测并记录上游返回的 null 值

详细说明请参考: [NULL_SAFETY.md](NULL_SAFETY.md)

### 日志增强

- ✅ 所有关键日志包含 `req_id` 字段
- ✅ 流式请求失败日志包含模型和 req_id
- ✅ 上游 null 值触发警告日志
- ✅ Panic 恢复日志统一使用 req_id

## Compatibility

### ✅ OpenAI API Compatible
- First chunk: `{"delta": {"role": "assistant", "content": ""}}`
- Subsequent: `{"delta": {"content": "..."}}`

### ✅ Anthropic API Compatible
- Handles both protocols correctly

### ✅ AI SDK Compatible (Cherry Studio, Vercel AI SDK, etc.)
- No empty role fields in JSON
- No null choices arrays
- Passes type validation

## Manual Verification Required

To fully verify the fixes:

1. Start the proxy server:
   ```bash
   ./dist/llm-proxy.exe -config config.yaml
   ```

2. Send a streaming request:
   ```bash
   curl -N http://localhost:8080/v1/chat/completions \
     -H "Authorization: Bearer sk-your-unified-api-key" \
     -H "Content-Type: application/json" \
     -d '{
       "model": "anthropic/claude-sonnet-4",
       "messages": [{"role": "user", "content": "Hello"}],
       "stream": true
     }'
   ```

3. Verify the output:
   - ✅ First chunk should have `"role": "assistant"`
   - ✅ Subsequent chunks should NOT have a `role` field (not even `"role": ""`)
   - ✅ **Every chunk** should have `"choices": []` or `"choices": [{...}]`
   - ❌ **Never** `"choices": null`
   - ❌ **Never** `"delta": {"role": ""}`

4. Test with Cherry Studio or your client application to ensure no `AI_TypeValidationError`.

## Files Modified

### Problem 1 - Empty Role:
1. `src/domain/entity/request.go` - Added `omitempty` to Message.Role JSON tag
2. `src/application/usecase/proxy_request.go` - Simplified delta creation logic
3. `src/application/usecase/usecase_test.go` - Fixed mock to match updated interface

### Problem 2 - Null Choices:
1. `src/domain/entity/request.go` - Fixed ResponseBuilder methods (NewResponseBuilder, Choices, BuildUnsafe)
2. `src/application/usecase/proxy_request.go` - Added upstream null detection and logging
3. `src/adapter/backend/client_adapter.go` - Added null check in non-streaming response handling

### Logging Enhancements:
1. `src/adapter/http/handler.go` - Added req_id to streaming error logs
2. `src/adapter/http/middleware.go` - Unified field name to req_id in panic recovery
3. `src/application/usecase/proxy_request.go` - Added warning log for upstream null choices

### Tests Added:
1. `src/domain/entity/streaming_test.go` - JSON serialization tests for streaming
2. `src/domain/entity/null_safety_test.go` - Comprehensive null safety tests

### Documentation:
1. `NULL_SAFETY.md` - **NEW** Detailed null safety mechanism documentation

## No Breaking Changes

- ✅ Non-streaming responses are unaffected
- ✅ Messages with roles still include the role field
- ✅ Only empty role fields are omitted (as per JSON best practices)
- ✅ `choices` is always an array, preventing type errors
- ✅ All existing tests pass
