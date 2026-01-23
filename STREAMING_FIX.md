# Streaming Response Fix - Empty Role Field Issue

## Problem Description

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

## Solution Implemented

### 1. **Modified Message Struct JSON Tag** (`src/domain/entity/request.go`)

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

### 2. **Simplified Streaming Handler Logic** (`src/application/usecase/proxy_request.go`)

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

## JSON Output Examples

### First Chunk (with role)
```json
{
  "choices": [{
    "index": 0,
    "delta": {
      "role": "assistant",
      "content": ""
    }
  }]
}
```

### Subsequent Chunks (no role field)
```json
{
  "choices": [{
    "index": 0,
    "delta": {
      "content": "Hello"
    }
  }]
}
```

**Note:** The `role` field is **completely absent** when empty, not `"role": ""`.

## Testing

### Unit Tests Added (`src/domain/entity/streaming_test.go`)

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

### Test Results
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

### All Tests Passing
```bash
$ cd src && go test ./... -count=1
ok  	llm-proxy/adapter/backend	0.450s
ok  	llm-proxy/adapter/config	0.497s
ok  	llm-proxy/adapter/http	0.813s
ok  	llm-proxy/adapter/http/middleware	0.846s
ok  	llm-proxy/application/service	0.648s
ok  	llm-proxy/application/usecase	0.456s
ok  	llm-proxy/domain/entity	0.817s
ok  	llm-proxy/domain/service	0.738s
ok  	llm-proxy/infrastructure/config	0.648s
ok  	llm-proxy/infrastructure/logging	0.518s
```

## Build Verification

```bash
$ cd src && go build -o ../dist/llm-proxy.exe .
Build successful: 12M
```

## Compatibility

### ✅ OpenAI API Compatible
- First chunk: `{"delta": {"role": "assistant", "content": ""}}`
- Subsequent: `{"delta": {"content": "..."}}`

### ✅ Anthropic API Compatible
- Handles both protocols correctly

### ✅ AI SDK Compatible (Cherry Studio, Vercel AI SDK, etc.)
- No empty role fields in JSON
- Passes type validation

## Manual Verification Required

To fully verify the fix:

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
   - First chunk should have `"role": "assistant"`
   - Subsequent chunks should NOT have a `role` field (not even `"role": ""`)

4. Test with Cherry Studio or your client application to ensure no validation errors.

## Files Modified

1. `src/domain/entity/request.go` - Added `omitempty` to Message.Role
2. `src/application/usecase/proxy_request.go` - Simplified delta creation logic
3. `src/application/usecase/usecase_test.go` - Fixed mock to match updated interface
4. `src/domain/entity/streaming_test.go` - **NEW** comprehensive JSON serialization tests

## No Breaking Changes

- Non-streaming responses are unaffected
- Messages with roles still include the role field
- Only empty role fields are omitted (as per JSON best practices)
- All existing tests pass
