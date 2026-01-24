# Groq 协议规范

本文档描述 Groq API 的协议格式和实现细节。

## 目录

- [概述](#概述)
- [请求格式](#请求格式)
- [响应格式](#响应格式)
- [错误响应](#错误响应)
- [与 OpenAI 的差异](#与-openai-的差异)
- [llm-proxy 实现](#llm-proxy-实现)

---

## 概述

Groq 是一家专注于高性能 AI 推理的公司，提供极低延迟的 LLM API 服务。Groq API **完全兼容** OpenAI API规范。

| 属性 | 值 |
|------|-----|
| **协议类型** | `groq` |
| **API 版本** | v1 |
| **基础 URL** | `https://api.groq.com/openai/v1` |
| **认证方式** | Bearer Token |
| **支持的模型** | llama-3.3-70b-versatile, llama-3.1-8b-instant, gemma2-9b-it |

---

## 请求格式

### 端点

```
POST https://api.groq.com/openai/v1/chat/completions
```

### 请求头

```
Content-Type: application/json
Authorization: Bearer {GROQ_API_KEY}
```

### 请求体结构

```json
{
  "model": "llama-3.3-70b-versatile",
  "messages": [
    {"role": "system", "content": "你是 helpful AI 助手"},
    {"role": "user", "content": "你好"}
  ],
  "temperature": 1.0,
  "top_p": 1.0,
  "max_completion_tokens": 4096,
  "frequency_penalty": 0,
  "presence_penalty": 0,
  "stream": false,
  "stop": null,
  "tools": null,
  "tool_choice": "auto",
  "parallel_tool_calls": true,
  "stream_options": null,
  "documents": null,
  "exclude_domains": null,
  "exclude_instance_ids": null,
  "compound_custom": null
}
```

### 字段说明

| 字段名 | 类型 | 必填 | 默认值 | 描述 |
|--------|------|------|--------|------|
| **model** | string | 是 | - | 模型 ID |
| **messages** | array | 是 | - | 对话历史数组 |
| **temperature** | number | 否 | 1.0 | 采样温度 (0-2) |
| **top_p** | number | 否 | 1.0 | 核采样参数 (0-1) |
| **max_completion_tokens** | integer | 否 | - | 生成 token 上限 |
| **frequency_penalty** | number | 否 | 0 | 频率惩罚 (-2.0~2.0) |
| **presence_penalty** | number | 否 | 0 | 存在惩罚 (-2.0~2.0) |
| **stream** | boolean | 否 | false | 是否启用流式响应 |
| **stop** | string/array | 否 | null | 停止序列 |
| **tools** | array | 否 | null | 可用工具定义 |
| **tool_choice** | string/object | 否 | "auto" | 工具选择策略 |
| **parallel_tool_calls** | boolean | 否 | true | 是否并行执行工具调用 |
| **stream_options** | object | 否 | null | 流式响应选项 |
| **documents** | array | 否 | null | RAG 场景文档数组 |
| **exclude_domains** | array | 否 | null | 排除的域名 |
| **exclude_instance_ids** | array | 否 | null | 排除的实例 ID |
| **compound_custom** | object | 否 | null | 自定义配置 |

### 消息格式

```json
[
  {"role": "system", "content": "你是 helpful AI 助手"},
  {"role": "user", "content": "用户消息"},
  {"role": "assistant", "content": "助手回复"},
  {"role": "tool", "content": "工具结果", "tool_call_id": "call_123"}
]
```

---

## 响应格式

### 非流式响应

```json
{
  "id": "chatcmpl-f51b2cd2-bef7-417e-964e-a08f0b513c22",
  "object": "chat.completion",
  "created": 1730241104,
  "model": "llama3-8b-8192",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "生成的文本内容..."
      },
      "logprobs": null,
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "queue_time": 0.037493756,
    "prompt_tokens": 18,
    "prompt_time": 0.000680594,
    "completion_tokens": 556,
    "completion_time": 0.463333333,
    "total_tokens": 574,
    "total_time": 0.464013927
  },
  "system_fingerprint": "fp_179b0f92c9",
  "x_groq": {
    "id": "req_01jbd6g2qdfw2adyrt2az8hz4w"
  }
}
```

#### Groq 扩展字段

| 字段 | 类型 | 描述 |
|------|------|------|
| **usage.queue_time** | number | 排队时间（秒） |
| **usage.prompt_time** | number | 处理提示的时间（秒） |
| **usage.completion_time** | number | 生成时间（秒） |
| **usage.total_time** | number | 总耗时（秒） |
| **x_groq.id** | string | 请求追踪 ID |

### 流式响应

启用 `stream: true` 后，响应为 SSE 格式：

```
data: {"id":"chatcmpl-...","object":"chat.completion.chunk","created":1730241104,"model":"llama3-8b-8192","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}

data: {"id":"chatcmpl-...","object":"chat.completion.chunk","created":1730241104,"model":"llama3-8b-8192","choices":[{"index":0,"delta":{"content":"第"},"finish_reason":null}]}

data: {"id":"chatcmpl-...","object":"chat.completion.chunk","created":1730241104,"model":"llama3-8b-8192","choices":[{"index":0,"delta":{"一"},"finish_reason":null}]}

data: [DONE]
```

### 结束原因 (finish_reason)

| 值 | 描述 |
|----|------|
| `stop` | 自然停止 |
| `length` | 达到 max_tokens 限制 |
| `tool_calls` | 模型决定调用工具 |
| `content_filter` | 内容被过滤 |

---

## 错误响应

### HTTP 状态码

| 状态码 | 错误类型 | 说明 |
|--------|----------|------|
| 400 | Bad Request | 请求语法无效 |
| 401 | Unauthorized | 认证凭据无效 |
| 404 | Not Found | 资源不存在 |
| 413 | Request Entity Too Large | 请求体过大 |
| 422 | Unprocessable Entity | 语义错误 |
| 429 | Too Many Requests | 超过速率限制 |
| **498** | Flex Tier Capacity Exceeded | **Groq 特有** - Flex 套餐容量已满 |
| **499** | Request Cancelled | **Groq 特有** - 请求被取消 |
| 500 | Internal Server Error | 服务器内部错误 |
| 502 | Bad Gateway | 上游服务器无效响应 |
| 503 | Service Unavailable | 服务不可用 |

### 错误响应格式

```json
{
  "error": {
    "message": "错误描述信息",
    "type": "invalid_request_error" | "authentication_error" | "rate_limit_error" | "internal_server_error"
  }
}
```

### Python SDK 异常类型

| 异常类型 | 说明 |
|----------|------|
| `groq.APIConnectionError` | 连接错误 |
| `groq.RateLimitError` | 速率限制 |
| `groq.APIStatusError` | HTTP 状态错误 |

### Groq 特有错误码

| 状态码 | 说明 | 是否可重试 |
|--------|------|------------|
| 498 | Flex Tier 容量超限 | 否（需要升级套餐） |
| 499 | 请求取消 | 否（客户端取消） |

---

## 与 OpenAI 的差异

### 兼容性

Groq API **完全兼容** OpenAI API 规范，可以直接替换使用：

```python
import openai

client = openai.OpenAI(
    base_url="https://api.groq.com/openai/v1",
    api_key=os.environ.get("GROQ_API_KEY")
)
```

### 主要差异

| 差异点 | 说明 |
|--------|------|
| **响应头 `x_groq`** | 包含请求追踪 ID |
| **状态码 498** | Flex Tier 容量超限（OpenAI 无此状态码） |
| **状态码 499** | 请求取消（OpenAI 无此状态码） |
| **usage 字段** | 包含 `queue_time`、`prompt_time` 等额外指标 |
| **模型名称** | 使用 Llama 等模型而非 GPT |
| **max_completion_tokens** | Groq 使用此字段名（与 OpenAI 一致） |

### 响应扩展字段

| 字段 | OpenAI | Groq | 说明 |
|------|--------|------|------|
| **usage.queue_time** | ❌ | ✅ | Groq 特有 |
| **usage.prompt_time** | ❌ | ✅ | Groq 特有 |
| **usage.completion_time** | ❌ | ✅ | Groq 特有 |
| **usage.total_time** | ❌ | ✅ | Groq 特有 |
| **x_groq.id** | ❌ | ✅ | Groq 特有请求 ID |

---

## llm-proxy 实现

### 协议配置

```yaml
backends:
- name: "groq"
  url: "https://api.groq.com/openai/v1/chat/completions"
  api_key: "{api-key}"
  protocol: "groq"
  timeout: 60s
  retry_times: 3
```

### 实现详情

Groq 被标记为 **OpenAI 兼容协议**，共享以下策略：

- ✅ **请求转换**: 使用 `openai.NewRequestConverter`
- ✅ **响应转换**: 使用 `openai.NewResponseConverter`
- ✅ **流式处理**: 使用 `openai.NewStreamChunkConverter`
- ✅ **错误转换**: 使用 `openai.NewErrorConverter`

### 特殊处理

1. **x_groq 扩展**: 响应头中的 `x_groq.id` 可用于请求追踪
2. **扩展使用指标**: `queue_time`、`prompt_time`、`completion_time` 透传给客户端
3. **特有状态码**: 498/499 由通用错误转换器处理（回退到 HTTP 状态码）

### 错误码映射

| Groq 状态码 | 内部错误代码 | 可重试 |
|-------------|--------------|--------|
| 400 | `INVALID_REQUEST` | 否 |
| 401 | `UNAUTHORIZED` | 否 |
| 429 | `RATE_LIMITED` | 是 |
| 498 | `CAPACITY_EXCEEDED` | 否 |
| 499 | `REQUEST_CANCELLED` | 否 |
| 500-503 | `BACKEND_ERROR` | 是 |

---

## 参考资料

- [Groq Console 文档](https://console.groq.com/docs)
- [Groq API 参考](https://console.groq.com/docs/api-reference)
- [OpenAI Chat Completions API](https://platform.openai.com/docs/api-reference/chat)
