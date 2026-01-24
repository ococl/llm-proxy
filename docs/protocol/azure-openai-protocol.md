# Azure OpenAI 协议规范

本文档描述 Azure OpenAI Service API 的协议格式和实现细节。

## 目录

- [概述](#概述)
- [请求格式](#请求格式)
- [响应格式](#响应格式)
- [错误响应](#错误响应)
- [与 OpenAI 的差异](#与-openai-的差异)
- [llm-proxy 实现](#llm-proxy-实现)

---

## 概述

Azure OpenAI Service 提供与 OpenAI 兼容的 API，允许开发者通过 Microsoft Azure 平台访问 GPT-4、GPT-3.5 等模型。

| 属性 | 值 |
|------|-----|
| **协议类型** | `azure` |
| **API 版本** | `2024-02-15-preview` 及更新版本 |
| **认证方式** | API Key 或 Azure Active Directory |
| **支持的模型** | gpt-4, gpt-4 Turbo, gpt-35-turbo, text-embedding-3 |

---

## 请求格式

### 端点

```
POST https://{resource-name}.openai.azure.com/openai/deployments/{deployment-id}/chat/completions?api-version=2024-02-15-preview
```

### 请求头

```
Content-Type: application/json
Authorization: Bearer {api-key}
```

### 请求体结构

```json
{
  "model": "gpt-4",
  "messages": [
    {"role": "system", "content": "你是 helpful AI 助手"},
    {"role": "user", "content": "你好"}
  ],
  "temperature": 1.0,
  "top_p": 1.0,
  "max_tokens": null,
  "frequency_penalty": 0,
  "presence_penalty": 0,
  "stream": false,
  "stop": null,
  "tools": null,
  "tool_choice": "auto",
  "presence_penalty": 0
}
```

### 字段说明

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| **model** | string | 是 | - | 模型名称或部署 ID |
| **messages** | array | 是 | - | 对话消息数组 |
| **temperature** | number | 否 | 1.0 | 采样温度 (0-2) |
| **top_p** | number | 否 | 1.0 | 核采样参数 (0-1) |
| **max_tokens** | integer | 否 | 16 | 生成的最大 token 数 |
| **stream** | boolean | 否 | false | 是否启用流式响应 |
| **stop** | string/array | 否 | null | 停止序列 |
| **frequency_penalty** | number | 否 | 0 | 频率惩罚 (-2~2) |
| **presence_penalty** | number | 否 | 0 | 存在惩罚 (-2~2) |
| **tools** | array | 否 | null | 可用工具列表 |
| **tool_choice** | string | 否 | "auto" | 工具选择策略 |

### 消息格式

```json
[
  {"role": "system", "content": "系统提示"},
  {"role": "user", "content": "用户消息"},
  {"role": "assistant", "content": "助手回复"},
  {"role": "tool", "content": "工具结果", "tool_call_id": "call_123"}
]
```

| 角色 | 说明 |
|------|------|
| **system** | 定义助手行为和约束 |
| **user** | 用户输入 |
| **assistant** | 助手历史回复 |
| **tool** | 工具执行结果 |

---

## 响应格式

### 非流式响应

```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1699016000,
  "model": "gpt-4",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "你好！有什么可以帮助你的吗？",
        "tool_calls": null
      },
      "finish_reason": "stop",
      "logprobs": null
    }
  ],
  "usage": {
    "prompt_tokens": 15,
    "completion_tokens": 12,
    "total_tokens": 27
  }
}
```

### 流式响应

启用 `stream: true` 后，响应为 SSE 格式：

```
data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1699016000,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1699016000,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"你"},"finish_reason":null}]}

data: [DONE]
```

### 响应字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| **id** | string | 完成 ID |
| **object** | string | 对象类型 (`chat.completion` 或 `chat.completion.chunk`) |
| **created** | integer | Unix 时间戳 |
| **model** | string | 模型名称 |
| **choices[].message.content** | string | 生成的文本内容 |
| **choices[].finish_reason** | string | 结束原因 (`stop`, `length`, `tool_calls`) |
| **usage.prompt_tokens** | integer | 输入 token 数 |
| **usage.completion_tokens** | integer | 输出 token 数 |
| **usage.total_tokens** | integer | 总 token 数 |

---

## 错误响应

### HTTP 状态码

| 状态码 | 说明 |
|--------|------|
| 400 | 请求参数错误 |
| 401 | 认证失败 |
| 403 | 权限不足 |
| 404 | 资源不存在 |
| 429 | 速率限制 |
| 500 | 内部服务器错误 |
| 503 | 服务不可用 |

### 错误响应格式

Azure OpenAI 使用与 OpenAI 不同的错误格式：

```json
{
  "error": {
    "code": "content_filter",
    "message": "The response was filtered due to the Azure OpenAI content filter policy."
  }
}
```

### 常见错误码

| 错误码 | HTTP 状态码 | 说明 |
|--------|-------------|------|
| `content_filter` | 400 | 内容被过滤 |
| `429` | 429 | 速率限制 |
| `invalid_api_key` | 401 | 无效的 API Key |
| `invalid_request_error` | 400 | 请求参数错误 |
| `rate_limit_exceeded` | 429 | 超过速率限制 |

### 错误处理示例

```json
// 内容过滤错误
{
  "error": {
    "code": "content_filter",
    "message": "The response was filtered due to the Azure OpenAI content filter policy."
  }
}

// 速率限制错误
{
  "error": {
    "code": "429",
    "message": "Rate limit is exceeded. Try again in 1 seconds."
  }
}

// 认证错误
{
  "error": {
    "code": "invalid_api_key",
    "message": "Invalid API key. Please provide a valid API key."
  }
}
```

---

## 与 OpenAI 的差异

### 主要差异

| 特性 | OpenAI | Azure OpenAI | 说明 |
|------|--------|--------------|------|
| **端点格式** | `api.openai.com/v1/...` | `{resource}.openai.azure.com/...` | Azure 使用资源名称 |
| **部署 ID** | 模型名称即部署 ID | 需单独配置部署 ID | Azure 架构差异 |
| **认证** | Bearer Token | Bearer Token 或 AAD | Azure 支持更多认证方式 |
| **错误格式** | `type` 字段 | `code` 字段 | 字段命名不同 |
| **内容过滤** | 可配置 | 内置内容过滤 | Azure 更严格 |

### 字段映射

| OpenAI 字段 | Azure 字段 | 说明 |
|-------------|------------|------|
| `model` | `model` 或 `deployment-id` | Azure 支持两种方式 |
| `type` (错误) | `code` (错误) | 错误格式差异 |
| `organization` | 不支持 | Azure 无组织概念 |

### 兼容性说明

Azure OpenAI API 与 OpenAI API 高度兼容，但存在以下差异：

1. **端点 URL 不同** - 需要根据 Azure 资源配置
2. **错误格式略有不同** - Azure 使用 `code` 而非 `type`
3. **内容过滤更严格** - Azure 可能有额外的安全检查
4. **部署概念** - Azure 需要预先配置模型部署

---

## llm-proxy 实现

### 协议配置

```yaml
backends:
- name: "azure-openai"
  url: "https://{resource-name}.openai.azure.com/openai/deployments/{deployment-id}/chat/completions"
  api_key: "{api-key}"
  protocol: "azure"
  timeout: 60s
  retry_times: 3
```

### 错误转换器

Azure 错误转换器位于：`src/application/service/protocol/azure/error.go`

**实现的错误码映射**：

| Azure 错误码 | 内部错误代码 | 可重试 |
|--------------|--------------|--------|
| `content_filter` | `INVALID_REQUEST` | 否 |
| `429`, `rate_limit_exceeded` | `RATE_LIMITED` | 是 |
| `invalid_api_key`, `authentication_error` | `UNAUTHORIZED` | 否 |
| `invalid_request_error` | `INVALID_REQUEST` | 否 |
| `not_found_error` | `BAD_REQUEST` | 否 |
| 其他数字状态码 | 根据 HTTP 状态码映射 | 根据状态码 |

### 请求转换

Azure 使用与 OpenAI 相同的请求格式，共享 `openai.NewRequestConverter`。

### 响应转换

Azure 使用与 OpenAI 相同的响应格式，共享 `openai.NewResponseConverter`。

### 流式处理

Azure 使用标准的 SSE 格式，与 OpenAI 兼容，共享 `openai.NewStreamChunkConverter`。

---

## 参考资料

- [Azure OpenAI Service 文档](https://learn.microsoft.com/azure/cognitive-services/openai/)
- [Azure OpenAI REST API 参考](https://learn.microsoft.com/azure/cognitive-services/openai/reference)
- [OpenAI Chat Completions API](https://platform.openai.com/docs/api-reference/chat)
