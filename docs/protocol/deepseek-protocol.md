# DeepSeek 协议规范

本文档描述 DeepSeek API 的协议格式和实现细节。

## 目录

- [概述](#概述)
- [请求格式](#请求格式)
- [响应格式](#响应格式)
- [错误响应](#错误响应)
- [与 OpenAI 的差异](#与-openai-的差异)
- [思考模式](#思考模式)
- [llm-proxy 实现](#llm-proxy-实现)

---

## 概述

DeepSeek 是一家中国 AI 公司，提供高性能的大语言模型服务。DeepSeek API 采用 OpenAI 兼容设计，支持标准的 Chat Completions 接口。

| 属性 | 值 |
|------|-----|
| **协议类型** | `deepseek` |
| **API 版本** | v1 |
| **基础 URL** | `https://api.deepseek.com` |
| **认证方式** | Bearer Token |
| **支持的模型** | deepseek-chat, deepseek-reasoner, deepseek-v3 |

---

## 请求格式

### 端点

```
POST https://api.deepseek.com/chat/completions
```

### 请求头

```
Content-Type: application/json
Authorization: Bearer {DEEPSEEK_API_KEY}
```

### 请求体结构

```json
{
  "model": "deepseek-chat",
  "messages": [
    {"role": "system", "content": "你是 helpful AI 助手"},
    {"role": "user", "content": "你好"}
  ],
  "stream": false,
  "temperature": 1.0,
  "top_p": 1.0,
  "max_tokens": 4096,
  "frequency_penalty": 0,
  "presence_penalty": 0,
  "response_format": {"type": "text"},
  "stop": null,
  "stream_options": {"include_usage": true},
  "tools": null,
  "tool_choice": "auto",
  "logprobs": false,
  "top_logprobs": 0,
  "thinking": {"type": "disabled"}
}
```

### 字段说明

#### 必填字段

| 字段名 | 类型 | 描述 |
|--------|------|------|
| **model** | string | 模型名称，如 `deepseek-chat`、`deepseek-reasoner` |
| **messages** | array | 对话历史列表 |

#### 可选字段

| 字段名 | 类型 | 默认值 | 描述 |
|--------|------|--------|------|
| **stream** | boolean | false | 是否启用流式响应 |
| **temperature** | number | 1.0 | 采样温度 (0-2)，值越高越随机 |
| **top_p** | number | 1.0 | 核采样参数 (0-1) |
| **max_tokens** | integer | 4096 | 最大生成 token 数 |
| **frequency_penalty** | number | 0 | 频率惩罚 (-2.0~2.0) |
| **presence_penalty** | number | 0 | 存在惩罚 (-2.0~2.0) |
| **response_format** | object | {type: "text"} | 输出格式 (`text` 或 `json_object`) |
| **stop** | string/string[] | null | 停止序列 |
| **stream_options** | object | null | 流式选项 |
| **tools** | array | null | 工具定义 |
| **tool_choice** | string/object | "auto" | 工具调用策略 |
| **logprobs** | boolean | false | 是否返回对数概率 |
| **top_logprobs** | integer | 0 | 返回概率最高的 N 个 token (0-20) |
| **thinking** | object | {type: "disabled"} | 思考模式控制 |

### 消息格式

```json
[
  {"role": "system", "content": "你是 helpful AI 助手"},
  {"role": "user", "content": "请解释什么是机器学习"},
  {"role": "assistant", "content": "机器学习是..."},
  {"role": "tool", "content": "工具结果", "tool_call_id": "call_123"}
]
```

---

## 响应格式

### 非流式响应

```json
{
  "id": "chatcmpl-123abc",
  "object": "chat.completion",
  "created": 1677652288,
  "model": "deepseek-chat",
  "system_fingerprint": "fp_xxx",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "这是助手回复",
        "reasoning_content": null,
        "tool_calls": []
      },
      "finish_reason": "stop",
      "logprobs": null
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 20,
    "total_tokens": 30,
    "prompt_cache_hit_tokens": 5,
    "prompt_cache_miss_tokens": 5,
    "completion_tokens_details": {
      "reasoning_tokens": 0
    }
  }
}
```

#### 响应字段说明

| 字段名 | 类型 | 描述 |
|--------|------|------|
| **id** | string | 完成 ID |
| **object** | string | 对象类型 (`chat.completion`) |
| **created** | integer | Unix 时间戳 |
| **model** | string | 模型名称 |
| **choices[].message.content** | string | 消息内容 |
| **choices[].message.reasoning_content** | string | 思考链内容（思考模式时） |
| **choices[].finish_reason** | string | 结束原因 |
| **usage.prompt_tokens** | integer | 提示词 token 数 |
| **usage.completion_tokens** | integer | 生成 token 数 |
| **usage.total_tokens** | integer | 总 token 数 |
| **usage.prompt_cache_hit_tokens** | integer | 缓存命中 token 数 |
| **usage.prompt_cache_miss_tokens** | integer | 缓存未命中 token 数 |

### 流式响应

启用 `stream: true` 后，响应为 SSE 格式：

```
data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"deepseek-chat","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"deepseek-chat","choices":[{"index":0,"delta":{"content":"你"},"finish_reason":null}]}

data: [DONE]
```

#### 结束原因 (finish_reason)

| 值 | 描述 |
|----|------|
| `stop` | 自然停止 |
| `length` | 达到 max_tokens 限制 |
| `tool_calls` | 模型决定调用工具 |
| `content_filter` | 内容被过滤 |
| `insufficient_system_resource` | 系统资源不足 |

---

## 错误响应

### HTTP 状态码

| 状态码 | 错误类型 | 原因 | 解决建议 |
|--------|----------|------|----------|
| 400 | Bad Request | 请求体格式错误 | 检查 JSON 格式 |
| 401 | Authentication Failed | API Key 错误 | 检查 API Key |
| 402 | Insufficient Funds | 账户余额不足 | 充值后重试 |
| 422 | Unprocessable Entity | 参数错误 | 检查参数值 |
| 429 | Rate Limit Exceeded | TPM/RPM 限制 | 降低请求频率 |
| 500 | Internal Server Error | 服务器内部错误 | 稍后重试 |
| 503 | Service Unavailable | 服务器过载 | 稍后重试 |

### 错误响应格式

```json
{
  "error": {
    "message": "错误描述信息",
    "type": "invalid_request_error" | "authentication_error" | "rate_limit_error" | "server_error",
    "code": "invalid_api_key" | "insufficient_quota" | ...
  }
}
```

### 常见错误类型

| 错误类型 | 描述 |
|----------|------|
| `invalid_request_error` | 请求参数无效 |
| `authentication_error` | 认证失败 |
| `insufficient_quota` | 配额不足 |
| `rate_limit_error` | 速率限制 |
| `server_error` | 服务器错误 |

---

## 与 OpenAI 的差异

### 兼容性

DeepSeek API **高度兼容** OpenAI API，可以直接替换使用。

### 主要差异

| 特性 | DeepSeek | OpenAI | 说明 |
|------|----------|--------|------|
| **reasoning_content** | ✅ 支持 | ❌ 不支持 | DeepSeek 专有的思考链输出 |
| **思考模式** | ✅ 支持 | ❌ 不支持 | 启用深度思考模式 |
| **prompt_cache** | ✅ 支持 | 部分 | 缓存命中/未命中 token 统计 |
| **流式格式** | `data:` 前缀 | 标准 SSE | 略有差异 |
| **模型名称** | deepseek-chat | gpt-4 | 不同的模型命名 |

### 不支持的参数

以下参数在思考模式下不会报错但不会生效：
- `temperature`
- `top_p`
- `presence_penalty`
- `frequency_penalty`

以下参数在思考模式下会报错：
- `logprobs`
- `top_logprobs`

---

## 思考模式

DeepSeek 支持特殊的思考模式（Reasoning Mode），适用于需要复杂推理的任务。

### 启用思考模式

```json
{
  "model": "deepseek-reasoner",
  "messages": [{"role": "user", "content": "证明哥德尔不完备定理"}],
  "thinking": {"type": "enabled"}
}
```

### 思考模式响应

启用后，响应中会包含 `reasoning_content` 字段：

```json
{
  "choices": [
    {
      "message": {
        "content": "最终答案",
        "reasoning_content": "详细的推理过程..."
      }
    }
  ],
  "usage": {
    "completion_tokens_details": {
      "reasoning_tokens": 5000
    }
  }
}
```

### 思考模式限制

| 特性 | 支持状态 |
|------|----------|
| JSON Output | ✅ 支持 |
| Tool Calls | ✅ 支持 |
| Chat Completions | ✅ 支持 |
| temperature/top_p | ⚠️ 不生效 |
| logprobs/top_logprobs | ❌ 报错 |

---

## llm-proxy 实现

### 协议配置

```yaml
backends:
- name: "deepseek"
  url: "https://api.deepseek.com/chat/completions"
  api_key: "{api-key}"
  protocol: "deepseek"
  timeout: 60s
  retry_times: 3
```

### 实现详情

DeepSeek 被标记为 **OpenAI 兼容协议**，共享以下策略：

- ✅ **请求转换**: 使用 `openai.NewRequestConverter`
- ✅ **响应转换**: 使用 `openai.NewResponseConverter`
- ✅ **流式处理**: 使用 `openai.NewStreamChunkConverter`
- ✅ **错误转换**: 使用 `openai.NewErrorConverter`

### 特殊处理

1. **流式响应**: DeepSeek 使用 `data:` 前缀转发时保持原格式
2. **思考模式**: 由应用层处理 `reasoning_content` 字段
3. **缓存统计**: 直接透传 `prompt_cache_hit_tokens` 等字段

### 错误码映射

| DeepSeek 错误码 | 内部错误代码 |
|-----------------|--------------|
| `invalid_request_error` | `INVALID_REQUEST` |
| `authentication_error` | `UNAUTHORIZED` |
| `rate_limit_error` | `RATE_LIMITED` |
| `server_error` | `BACKEND_ERROR` |

---

## 参考资料

- [DeepSeek API 文档](https://api.deepseek.com/docs)
- [DeepSeek 官方 GitHub](https://github.com/deepseek-ai)
- [OpenAI Chat Completions API](https://platform.openai.com/docs/api-reference/chat)
