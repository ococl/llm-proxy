# LLM 协议规范文档索引

本文档索引整理了 `docs/protocol/` 目录下所有的协议规范文档和 JSON Schema 文件，为 llm-proxy 项目提供完整的 API 协议参考。

## 📁 目录结构

```
docs/protocol/
├── 协议规范文档 (.md)
│   ├── sse-specification.md           # SSE 协议规范
│   ├── json-schema-specification.md   # JSON Schema 规范
│   ├── error-response-specification.md # 错误响应格式
│   ├── openapi-specification.md       # OpenAPI 规范
│   └── streaming-response-format.md   # 流式响应格式
│
└── JSON Schema 文件 (.schema.json)
    ├── openai-chat-completion.schema.json   # OpenAI Chat Completion API
    └── anthropic-messages.schema.json       # Anthropic Messages API
```

## 📄 协议规范文档说明

### 1. SSE 协议规范 (`sse-specification.md`)

**内容概述**:
- Server-Sent Events 核心概念和特性
- 事件流格式详解 (ABNF 语法)
- 数据字段、事件类型、事件 ID、重连间隔
- OpenAI 兼容的流式响应格式
- 浏览器端和 Go 语言实现示例

**适用场景**: 实现 LLM 流式响应代理

### 2. JSON Schema 规范 (`json-schema-specification.md`)

**内容概述**:
- JSON Schema 核心概念和版本说明
- 基本类型验证 (字符串、数值、数组、对象)
- 复杂验证规则 (枚举、条件逻辑、引用)
- 字符串格式验证
- LLM API 请求/响应验证示例
- 推荐工具和最佳实践

**适用场景**: API 请求验证、数据结构定义

### 3. 错误响应格式规范 (`error-response-specification.md`)

**内容概述**:
- OpenAI 错误响应格式和错误码详解
- Anthropic 错误响应格式
- Google AI 错误响应格式
- Azure OpenAI 错误响应格式
- 通用 HTTP 状态码和处理策略
- Go 语言错误处理实现示例

**适用场景**: 错误处理、降级策略、监控告警

### 4. OpenAPI 规范 (`openapi-specification.md`)

**内容概述**:
- OpenAPI 3.1.0 核心概念
- OpenAI 官方 OpenAPI 规范参考
- LLM API 工具调用规范
- 请求/响应定义示例

**适用场景**: API 文档生成、SDK 集成

### 5. 流式响应格式详解 (`streaming-response-format.md`)

**内容概述**:
- OpenAI 流式响应完整格式
- 数据块结构详解
- Delta 字段处理
- 结束原因 (finish_reason)
- 工具调用流式处理
- 多语言实现示例 (Python、Go)

**适用场景**: 实现流式 API 代理、客户端开发

## 📑 JSON Schema 文件说明

### OpenAI Chat Completion Schema (`openai-chat-completion.schema.json`)

**包含内容**:
- 请求格式: model, messages, temperature, tools 等
- 响应格式: id, choices, usage 等
- 流式响应格式: ChatCompletionChunk
- 消息格式: role, content, tool_calls 等
- 工具调用格式: function, parameters

**使用场景**: 验证请求/响应格式、生成文档、代码生成

### Anthropic Messages Schema (`anthropic-messages.schema.json`)

**包含内容**:
- 请求格式: model, messages, max_tokens 等
- 响应格式: content blocks, stop_reason, usage
- 多模态内容块: text, image, document
- 流式事件: message_start, content_block_delta 等
- 工具调用格式: tool_use, tool_result

**使用场景**: Anthropic Claude API 集成、验证

## 🚀 快速开始

### 1. 实现流式代理

```go
// 参考 sse-specification.md 中的实现示例
func ProxyStreamHandler(w http.ResponseWriter, r *http.Request) {
    // 1. 解析请求 (参考 openai-chat-completion.schema.json)
    var req ChatCompletionRequest
    json.NewDecoder(r.Body).Decode(&req)
    
    // 2. 验证请求 (使用 JSON Schema)
    if err := ValidateJSON(req, openaiSchema); err != nil {
        WriteError(w, err)
        return
    }
    
    // 3. 调用上游 API (流式模式)
    upstreamResp, err := CallUpstreamStream(req)
    if err != nil {
        // 4. 处理错误 (参考 error-response-specification.md)
        WriteError(w, err)
        return
    }
    
    // 5. 转发流式响应 (参考 streaming-response-format.md)
    for chunk := range upstreamResp.Chunks() {
        sse.WriteChunk(w, chunk)
    }
    sse.WriteDone(w)
}
```

### 2. 验证请求

```go
import "github.com/xeipuuv/gojsonschema"

// 加载 OpenAI Schema
openaiSchema := gojsonschema.NewReferenceLoader(
    "docs/protocol/openai-chat-completion.schema.json",
)

// 验证请求
documentLoader := gojsonschema.NewGoLoader(request)
result, err := gojsonschema.Validate(openaiSchema, documentLoader)

if !result.Valid() {
    for _, err := range result.Errors() {
        log.Printf("验证错误: %s", err.String())
    }
}
```

### 3. 处理错误

```go
// 参考 error-response-specification.md
func HandleAPIError(err error) LLMProxyError {
    switch e := err.(type) {
    case *OpenAIError:
        return ConvertOpenAIError(e)
    case *AnthropicError:
        return ConvertAnthropicError(e)
    default:
        return NewLLMProxyError("unknown", 500, "internal_error", err.Error())
    }
}
```

## 📚 文档链接

### 官方文档

- **OpenAI API**: https://platform.openai.com/docs/api-reference
- **Anthropic API**: https://docs.anthropic.com/en/api/messages
- **JSON Schema**: https://json-schema.org/
- **W3C SSE**: https://www.w3.org/TR/2015/REC-eventsource-20150203/
- **OpenAPI**: https://spec.openapis.org/oas/v3.1.0

### 相关项目

- **OpenAI OpenAPI Spec**: https://github.com/openai/openai-openapi
- **JSON Schema Validator**: https://www.jsonschemavalidator.net/

## 🔧 工具推荐

### JSON Schema 验证

```bash
# Node.js
npm install ajv

# Go
go get github.com/xeipuuv/gojsonschema

# Python
pip install jsonschema
```

### API 测试

```bash
# curl 测试流式 API
curl -X POST https://api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}],"stream":true}'
```

## 📝 版本信息

- **创建时间**: 2026-01-24
- **项目版本**: llm-proxy v1.0
- **维护建议**: 定期检查官方文档更新

## 🤝 贡献指南

如需添加新的协议规范：

1. 在 `docs/protocol/` 目录下创建新文件
2. 更新本索引文件
3. 遵循现有文档的格式和风格
4. 包含官方文档链接和示例代码

## 许可证

本协议文档仅供学习和参考使用。
