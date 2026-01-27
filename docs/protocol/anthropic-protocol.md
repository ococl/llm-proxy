# Anthropic Claude 协议规范

本文档描述 Anthropic Claude API 的协议格式和实现细节。

## 目录

- [概述](#概述)
- [请求格式](#请求格式)
- [响应格式](#响应格式)
- [错误响应](#错误响应)
- [与 OpenAI 的差异](#与-openai-的差异)
- [llm-proxy 实现](#llm-proxy-实现)

---

## 概述

Anthropic 是 Claude 系列大语言模型的开发商，提供高质量的对话和推理能力。Anthropic API 使用独特的 Messages API 设计，与 OpenAI API **不兼容**。

| 属性 | 值 |
|------|-----|
| **协议类型** | `anthropic` |
| **API 版本** | 2023-06-01 (需在请求头中指定) |
| **基础 URL** | `https://api.anthropic.com` |
| **认证方式** | x-api-key Header |
| **支持的模型** | claude-3-5-sonnet-20241022, claude-3-5-haiku-20241022, claude-3-opus-20240229 |

---

## 请求格式

### 端点

```
POST https://api.anthropic.com/v1/messages
```

### 请求头

```
Content-Type: application/json
x-api-key: {ANTHROPIC_API_KEY}
anthropic-version: 2023-06-01
```

**重要**: `anthropic-version` 请求头是**必需的**，用于指定 API 版本。

### 请求体结构

```json
{
  "model": "claude-3-5-sonnet-20241022",
  "max_tokens": 1024,
  "system": "你是 helpful AI 助手",
  "messages": [
    {"role": "user", "content": "你好，请介绍一下你自己"}
  ],
  "temperature": 1.0,
  "top_p": 1.0,
  "top_k": 0,
  "stop_sequences": null,
  "stream": false,
  "metadata": null
}
```

### 字段说明

#### 必填字段

| 字段名 | 类型 | 描述 |
|--------|------|------|
| **model** | string | 模型 ID |
| **max_tokens** | integer | **必需**：最大生成 token 数 (1-4096) |
| **messages** | array | 对话历史数组 |

#### 可选字段

| 字段名 | 类型 | 默认值 | 描述 |
|--------|------|--------|------|
| **system** | string | null | 系统提示（独立字段，非 messages 数组） |
| **temperature** | number | 1.0 | 采样温度 (0-1) |
| **top_p** | number | 1.0 | 核采样参数 (0-1) |
| **top_k** | integer | 0 | Top-k 采样 |
| **stop_sequences** | array | null | 停止序列（最多 4 个） |
| **stream** | boolean | false | 是否流式响应 |
| **metadata** | object | null | 元数据（user_id 等） |

### 消息格式

```json
{
  "messages": [
    {
      "role": "user",
      "content": "用户消息"
    },
    {
      "role": "assistant",
      "content": "助手回复"
    }
  ]
}
```

**角色说明**:
- `user`: 用户消息
- `assistant`: 助手回复

**重要差异**:
- **不支持** `system` 角色在 messages 数组中
- `system` 提示必须使用独立的 `system` 字段
- messages 数组必须以 `user` 消息开始

### 多模态内容

Anthropic 支持文本和图像输入：

```json
{
  "messages": [
    {
      "role": "user",
      "content": [
        {
          "type": "text",
          "text": "这张图片里有什么？"
        },
        {
          "type": "image",
          "source": {
            "type": "base64",
            "media_type": "image/jpeg",
            "data": "/9j/4AAQSkZJRg..."
          }
        }
      ]
    }
  ]
}
```

---

## 响应格式

### 非流式响应

```json
{
  "id": "msg_01XFDUDYJgAACzvnptvVoYEL",
  "type": "message",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "你好！我是 Claude，一个由 Anthropic 开发的 AI 助手..."
    }
  ],
  "model": "claude-3-5-sonnet-20241022",
  "stop_reason": "end_turn",
  "stop_sequence": null,
  "usage": {
    "input_tokens": 15,
    "output_tokens": 50
  }
}
```

#### 响应字段说明

| 字段 | 类型 | 描述 |
|------|------|------|
| **id** | string | 消息 ID |
| **type** | string | 固定为 "message" |
| **role** | string | 固定为 "assistant" |
| **content** | array | 内容块数组 |
| **model** | string | 使用的模型 |
| **stop_reason** | string | 结束原因 |
| **usage.input_tokens** | integer | 输入 token 数 |
| **usage.output_tokens** | integer | 输出 token 数 |

### 流式响应

启用 `stream: true` 后，响应为 SSE 格式：

```
event: message_start
data: {"type":"message_start","message":{"id":"msg_01XFDUDYJgAACzvnptvVoYEL","type":"message","role":"assistant","content":[],"model":"claude-3-5-sonnet-20241022","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":15,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"你"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"好"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":50}}

event: message_stop
data: {"type":"message_stop"}
```

#### 流式事件类型

| 事件类型 | 描述 |
|---------|------|
| **message_start** | 消息开始，包含初始元数据 |
| **content_block_start** | 内容块开始 |
| **content_block_delta** | 内容块增量更新 |
| **content_block_stop** | 内容块结束 |
| **message_delta** | 消息元数据更新（stop_reason, usage） |
| **message_stop** | 消息结束 |

### 结束原因 (stop_reason)

| 值 | 描述 |
|----|------|
| **end_turn** | 自然完成 |
| **max_tokens** | 达到 max_tokens 限制 |
| **stop_sequence** | 遇到停止序列 |
| **tool_use** | 工具调用（需要工具结果） |

---

## 错误响应

### HTTP 状态码

| 状态码 | 说明 |
|--------|------|
| 400 | 请求参数错误 (invalid_request_error) |
| 401 | 认证失败 (authentication_error) |
| 403 | 权限不足 (permission_error) |
| 404 | 资源不存在 (not_found_error) |
| 429 | 速率限制 (rate_limit_error) |
| 500 | 内部服务器错误 (api_error) |
| 529 | 服务过载 (overloaded_error) |

### 错误响应格式

```json
{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "max_tokens: Field required"
  }
}
```

### 常见错误类型

| 错误类型 | 描述 |
|----------|------|
| **invalid_request_error** | 请求参数错误或缺失必填字段 |
| **authentication_error** | API Key 无效或缺失 |
| **permission_error** | 权限不足或模型访问受限 |
| **not_found_error** | 请求的资源不存在 |
| **rate_limit_error** | 超过速率限制 |
| **api_error** | Anthropic 服务器内部错误 |
| **overloaded_error** | 服务过载，建议重试 |

---

## 与 OpenAI 的差异

### 兼容性

Anthropic API 与 OpenAI API **完全不兼容**，需要进行协议转换。

### 主要差异

| 特性 | OpenAI | Anthropic | 说明 |
|------|--------|-----------|------|
| **端点路径** | `/chat/completions` | `/v1/messages` | 完全不同的端点 |
| **认证头** | `Authorization: Bearer` | `x-api-key` | 不同的认证方式 |
| **API 版本** | URL 中 | `anthropic-version` 请求头 | 版本指定方式不同 |
| **max_tokens** | 可选 | **必需** | Anthropic 强制要求 |
| **system 提示** | messages 数组中 | 独立的 `system` 字段 | **不兼容**：位置不同 |
| **响应格式** | choices 数组 | content 数组 | **不兼容**：结构不同 |
| **流式事件** | 单一 `data:` 格式 | 多种事件类型 | **不兼容**：事件结构不同 |
| **usage 字段** | 顶层 | 顶层 | 字段名相同但位置不同 |

### 字段映射表

| OpenAI 字段 | Anthropic 字段 | 说明 |
|-------------|----------------|------|
| messages[role=system] | system | 系统提示位置不同 |
| messages | messages | 数组格式相同，但不含 system |
| max_tokens | max_tokens | Anthropic 中为必填 |
| choices[0].message.content | content[0].text | 响应内容位置不同 |
| choices[0].finish_reason | stop_reason | 字段名不同 |
| usage | usage | 字段名相同 |

### 请求转换示例

#### OpenAI 请求

```json
{
  "model": "gpt-4",
  "messages": [
    {"role": "system", "content": "你是助手"},
    {"role": "user", "content": "你好"}
  ],
  "max_tokens": 1024
}
```

#### Anthropic 请求

```json
{
  "model": "claude-3-5-sonnet-20241022",
  "system": "你是助手",
  "messages": [
    {"role": "user", "content": "你好"}
  ],
  "max_tokens": 1024
}
```

---

## llm-proxy 实现

### 协议配置

```yaml
backends:
- name: "anthropic"
  url: "https://api.anthropic.com"  # 基础 URL，不包含路径
  api_key: "{api-key}"
  protocol: "anthropic"
  timeout: 60s
  retry_times: 3
```

### 实现详情

Anthropic 协议**不兼容** OpenAI，需要进行完整的协议转换：

#### 请求转换流程

1. **系统提示提取**
   - 从 `messages` 数组中提取 `role: system` 的消息
   - 将其 `content` 设置为独立的 `system` 字段
   - 从 messages 数组中移除 system 消息

2. **max_tokens 处理**
   - 如果客户端未提供 `max_tokens`，设置默认值（如 4096）
   - Anthropic 强制要求此字段

3. **端点路径**
   - 使用 `/v1/messages` 而非 `/chat/completions`
   - 在 `client_adapter.go` 中通过 `getPathForProtocol()` 处理

4. **请求头处理**
   - 将 `Authorization: Bearer {key}` 转换为 `x-api-key: {key}`
   - 添加 `anthropic-version: 2023-06-01` 请求头

#### 响应转换流程

1. **非流式响应**
   - 将 `content[0].text` 转换为 `choices[0].message.content`
   - 将 `stop_reason` 映射为 `finish_reason`
   - 保持 `usage` 字段结构（字段名相同）

2. **流式响应**
   - 将 Anthropic 的多种事件类型转换为 OpenAI 的统一格式
   - `content_block_delta` → `choices[0].delta.content`
   - `message_stop` → `data: [DONE]`

#### 代码实现位置

| 功能 | 文件路径 |
|------|---------|
| 请求转换 | `src/application/service/protocol/anthropic/request_converter.go` |
| 响应转换 | `src/application/service/protocol/anthropic/response_converter.go` |
| 流式处理 | `src/application/service/protocol/anthropic/stream_converter.go` |
| 错误转换 | `src/application/service/protocol/anthropic/error_converter.go` |
| 路径处理 | `src/adapter/backend/client_adapter.go` (getPathForProtocol) |

### 特殊处理

1. **system 消息提取**: 从 messages 数组中提取并移动到独立字段
2. **max_tokens 默认值**: 如果未提供，设置为 4096
3. **认证头转换**: Bearer Token → x-api-key
4. **API 版本注入**: 自动添加 anthropic-version 请求头
5. **流式事件转换**: 多种事件类型 → OpenAI 统一格式

### 错误码映射

| Anthropic 错误类型 | 内部错误代码 |
|-------------------|--------------|
| invalid_request_error | `INVALID_REQUEST` |
| authentication_error | `UNAUTHORIZED` |
| permission_error | `FORBIDDEN` |
| not_found_error | `NOT_FOUND` |
| rate_limit_error | `RATE_LIMITED` |
| api_error | `BACKEND_ERROR` |
| overloaded_error | `SERVICE_UNAVAILABLE` |

---

## 参考资料

- [Anthropic API 文档](https://docs.anthropic.com/en/api/messages)
- [Anthropic Messages API Reference](https://console.anthropic.com/docs/en/api/messages/create)
- [Claude Models Overview](https://docs.anthropic.com/en/docs/models-overview)
- [OpenAI Chat Completions API](https://platform.openai.com/docs/api-reference/chat)
