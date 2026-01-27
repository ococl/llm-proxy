# Google Vertex AI Gemini 协议规范

本文档描述 Google Vertex AI Gemini API 的协议格式和实现细节。

## 目录

- [概述](#概述)
- [请求格式](#请求格式)
- [响应格式](#响应格式)
- [错误响应](#错误响应)
- [与 OpenAI 的差异](#与-openai-的差异)
- [llm-proxy 实现](#llm-proxy-实现)

---

## 概述

Google Vertex AI 提供 Gemini 系列模型的 API 访问，支持多模态输入（文本、图像、视频、音频）。Vertex AI API 使用独特的格式，与 OpenAI API **不兼容**。

| 属性 | 值 |
|------|-----|
| **协议类型** | `google` |
| **API 版本** | v1 |
| **基础 URL** | `https://aiplatform.googleapis.com/v1` |
| **认证方式** | Google Cloud OAuth 2.0 或 API Key |
| **支持的模型** | gemini-2.0-flash-exp, gemini-1.5-pro, gemini-1.5-flash |

---

## 请求格式

### 端点

```
POST https://aiplatform.googleapis.com/v1/projects/{PROJECT_ID}/locations/{LOCATION}/publishers/google/models/{MODEL}:generateContent
```

**端点组成部分**:
- `{PROJECT_ID}`: Google Cloud 项目 ID
- `{LOCATION}`: 区域（如 `us-central1`, `asia-northeast1`）
- `{MODEL}`: 模型名称（如 `gemini-1.5-pro`, `gemini-2.0-flash-exp`）

### 请求头

```
Content-Type: application/json
Authorization: Bearer {ACCESS_TOKEN}
```

**认证方式**:
- **OAuth 2.0**: 使用 Google Cloud 服务账号生成的访问令牌
- **API Key**: 在 URL 中添加 `?key={API_KEY}` 参数

### 请求体结构

```json
{
  "contents": [
    {
      "role": "user",
      "parts": [
        {"text": "你好，请介绍一下你自己"}
      ]
    }
  ],
  "systemInstruction": {
    "parts": [
      {"text": "你是 helpful AI 助手"}
    ]
  },
  "generationConfig": {
    "temperature": 1.0,
    "topP": 0.95,
    "topK": 40,
    "maxOutputTokens": 8192,
    "stopSequences": [],
    "candidateCount": 1,
    "responseMimeType": "text/plain"
  },
  "safetySettings": [
    {
      "category": "HARM_CATEGORY_HARASSMENT",
      "threshold": "BLOCK_MEDIUM_AND_ABOVE"
    }
  ],
  "tools": []
}
```

### 字段说明

#### 必填字段

| 字段名 | 类型 | 描述 |
|--------|------|------|
| **contents** | array | 对话内容数组，包含 role 和 parts |

#### 可选字段

| 字段名 | 类型 | 默认值 | 描述 |
|--------|------|--------|------|
| **systemInstruction** | object | null | 系统指令（独立字段） |
| **generationConfig** | object | {} | 生成配置 |
| **generationConfig.temperature** | number | 1.0 | 采样温度 (0-2) |
| **generationConfig.topP** | number | 0.95 | 核采样参数 (0-1) |
| **generationConfig.topK** | integer | 40 | Top-k 采样 |
| **generationConfig.maxOutputTokens** | integer | 8192 | 最大输出 token 数 |
| **generationConfig.stopSequences** | array | [] | 停止序列 |
| **generationConfig.candidateCount** | integer | 1 | 候选响应数量 |
| **generationConfig.responseMimeType** | string | "text/plain" | 响应 MIME 类型 |
| **safetySettings** | array | [] | 安全设置 |
| **tools** | array | [] | 工具定义 |

### 消息格式

```json
{
  "contents": [
    {
      "role": "user",
      "parts": [
        {"text": "用户消息"}
      ]
    },
    {
      "role": "model",
      "parts": [
        {"text": "模型回复"}
      ]
    }
  ]
}
```

**角色说明**:
- `user`: 用户消息
- `model`: 模型回复（对应 OpenAI 的 `assistant`）

**重要差异**:
- 使用 `parts` 数组而非单一 `content` 字段
- 角色名为 `model` 而非 `assistant`
- `systemInstruction` 是独立字段，不在 `contents` 数组中

### 多模态内容

Gemini 支持文本、图像、视频、音频输入：

```json
{
  "contents": [
    {
      "role": "user",
      "parts": [
        {"text": "这张图片里有什么？"},
        {
          "inlineData": {
            "mimeType": "image/jpeg",
            "data": "/9j/4AAQSkZJRg..."
          }
        }
      ]
    }
  ]
}
```

**支持的 MIME 类型**:
- 图像: `image/jpeg`, `image/png`, `image/webp`, `image/heic`
- 视频: `video/mp4`, `video/mpeg`, `video/mov`, `video/avi`
- 音频: `audio/wav`, `audio/mp3`, `audio/aiff`, `audio/aac`

### 安全设置

```json
{
  "safetySettings": [
    {
      "category": "HARM_CATEGORY_HARASSMENT",
      "threshold": "BLOCK_MEDIUM_AND_ABOVE"
    },
    {
      "category": "HARM_CATEGORY_HATE_SPEECH",
      "threshold": "BLOCK_MEDIUM_AND_ABOVE"
    },
    {
      "category": "HARM_CATEGORY_SEXUALLY_EXPLICIT",
      "threshold": "BLOCK_MEDIUM_AND_ABOVE"
    },
    {
      "category": "HARM_CATEGORY_DANGEROUS_CONTENT",
      "threshold": "BLOCK_MEDIUM_AND_ABOVE"
    }
  ]
}
```

**安全类别**:
- `HARM_CATEGORY_HARASSMENT`: 骚扰
- `HARM_CATEGORY_HATE_SPEECH`: 仇恨言论
- `HARM_CATEGORY_SEXUALLY_EXPLICIT`: 性暗示内容
- `HARM_CATEGORY_DANGEROUS_CONTENT`: 危险内容

**阈值**:
- `BLOCK_NONE`: 不阻止
- `BLOCK_ONLY_HIGH`: 仅阻止高风险
- `BLOCK_MEDIUM_AND_ABOVE`: 阻止中等及以上风险
- `BLOCK_LOW_AND_ABOVE`: 阻止低等及以上风险

---

## 响应格式

### 非流式响应

```json
{
  "candidates": [
    {
      "content": {
        "role": "model",
        "parts": [
          {"text": "你好！我是 Gemini，一个由 Google 开发的大型语言模型..."}
        ]
      },
      "finishReason": "STOP",
      "safetyRatings": [
        {
          "category": "HARM_CATEGORY_HARASSMENT",
          "probability": "NEGLIGIBLE",
          "probabilityScore": 0.1,
          "severity": "HARM_SEVERITY_NEGLIGIBLE",
          "severityScore": 0.05
        }
      ],
      "index": 0
    }
  ],
  "usageMetadata": {
    "promptTokenCount": 15,
    "candidatesTokenCount": 50,
    "totalTokenCount": 65
  },
  "modelVersion": "gemini-1.5-pro-002"
}
```

#### 响应字段说明

| 字段 | 类型 | 描述 |
|------|------|------|
| **candidates** | array | 候选响应数组 |
| **candidates[].content** | object | 内容对象 |
| **candidates[].content.role** | string | 固定为 "model" |
| **candidates[].content.parts** | array | 内容部分数组 |
| **candidates[].finishReason** | string | 结束原因 |
| **candidates[].safetyRatings** | array | 安全评级 |
| **usageMetadata.promptTokenCount** | integer | 输入 token 数 |
| **usageMetadata.candidatesTokenCount** | integer | 输出 token 数 |
| **usageMetadata.totalTokenCount** | integer | 总 token 数 |

### 流式响应

启用流式响应时，端点变为 `:streamGenerateContent`：

```
POST https://aiplatform.googleapis.com/v1/projects/{PROJECT_ID}/locations/{LOCATION}/publishers/google/models/{MODEL}:streamGenerateContent
```

**流式响应格式**: JSON Lines（每行一个 JSON 对象）

```json
{"candidates":[{"content":{"role":"model","parts":[{"text":"你"}]},"index":0}],"usageMetadata":{"promptTokenCount":15,"candidatesTokenCount":1,"totalTokenCount":16}}
{"candidates":[{"content":{"role":"model","parts":[{"text":"好"}]},"index":0}],"usageMetadata":{"promptTokenCount":15,"candidatesTokenCount":2,"totalTokenCount":17}}
{"candidates":[{"content":{"role":"model","parts":[{"text":"！"}]},"finishReason":"STOP","index":0}],"usageMetadata":{"promptTokenCount":15,"candidatesTokenCount":3,"totalTokenCount":18}}
```

**重要**: Vertex AI 使用 **JSON Lines** 格式，而非 SSE (Server-Sent Events)。

### 结束原因 (finishReason)

| 值 | 描述 |
|----|------|
| **STOP** | 自然完成 |
| **MAX_TOKENS** | 达到最大 token 限制 |
| **SAFETY** | 内容被安全过滤器阻止 |
| **RECITATION** | 内容被引用检测阻止 |
| **OTHER** | 其他原因 |

---

## 错误响应

### HTTP 状态码

| 状态码 | 说明 |
|--------|------|
| 400 | 请求参数错误 |
| 401 | 认证失败 |
| 403 | 权限不足或配额超限 |
| 404 | 资源不存在 |
| 429 | 速率限制 |
| 500 | 内部服务器错误 |
| 503 | 服务不可用 |

### 错误响应格式

```json
{
  "error": {
    "code": 400,
    "message": "Invalid request: missing required field 'contents'",
    "status": "INVALID_ARGUMENT",
    "details": [
      {
        "@type": "type.googleapis.com/google.rpc.BadRequest",
        "fieldViolations": [
          {
            "field": "contents",
            "description": "Field is required"
          }
        ]
      }
    ]
  }
}
```

### 常见错误状态

| 状态 | 描述 |
|------|------|
| **INVALID_ARGUMENT** | 请求参数无效 |
| **UNAUTHENTICATED** | 认证失败 |
| **PERMISSION_DENIED** | 权限不足 |
| **NOT_FOUND** | 资源不存在 |
| **RESOURCE_EXHAUSTED** | 配额超限 |
| **INTERNAL** | 内部错误 |
| **UNAVAILABLE** | 服务不可用 |

### 内容被阻止响应

```json
{
  "candidates": [
    {
      "finishReason": "SAFETY",
      "safetyRatings": [
        {
          "category": "HARM_CATEGORY_HARASSMENT",
          "probability": "HIGH",
          "blocked": true
        }
      ]
    }
  ],
  "promptFeedback": {
    "blockReason": "SAFETY",
    "safetyRatings": [
      {
        "category": "HARM_CATEGORY_HARASSMENT",
        "probability": "HIGH"
      }
    ]
  }
}
```

---

## 与 OpenAI 的差异

### 兼容性

Google Vertex AI API 与 OpenAI API **完全不兼容**，需要进行完整的协议转换。

### 主要差异

| 特性 | OpenAI | Google Vertex AI | 说明 |
|------|--------|------------------|------|
| **端点格式** | `/chat/completions` | `/{project}/.../{model}:generateContent` | 完全不同的 URL 结构 |
| **认证方式** | `Authorization: Bearer` | Google Cloud OAuth 2.0 | 不同的认证机制 |
| **消息字段** | `messages[].content` | `contents[].parts[]` | **不兼容**：结构不同 |
| **角色命名** | `assistant` | `model` | 角色名称不同 |
| **系统提示** | `messages[role=system]` | `systemInstruction` | **不兼容**：独立字段 |
| **响应结构** | `choices[]` | `candidates[]` | **不兼容**：字段名不同 |
| **流式格式** | SSE (Server-Sent Events) | JSON Lines | **不兼容**：完全不同的格式 |
| **安全设置** | 无 | `safetySettings` | Vertex AI 独有 |
| **多模态** | 支持（base64） | 支持（inlineData） | 格式不同 |

### 字段映射表

| OpenAI 字段 | Vertex AI 字段 | 说明 |
|-------------|----------------|------|
| messages | contents | 消息数组 |
| messages[].role: assistant | contents[].role: model | 角色名不同 |
| messages[].content | contents[].parts[].text | 内容结构不同 |
| messages[role=system] | systemInstruction | 系统提示位置不同 |
| max_tokens | generationConfig.maxOutputTokens | 字段名不同 |
| temperature | generationConfig.temperature | 嵌套层级不同 |
| top_p | generationConfig.topP | 嵌套层级不同 |
| stop | generationConfig.stopSequences | 字段名不同 |
| choices | candidates | 字段名不同 |
| choices[].message.content | candidates[].content.parts[].text | 结构不同 |
| choices[].finish_reason | candidates[].finishReason | 字段名不同 |
| usage.prompt_tokens | usageMetadata.promptTokenCount | 字段名不同 |
| usage.completion_tokens | usageMetadata.candidatesTokenCount | 字段名不同 |

### 请求转换示例

#### OpenAI 请求

```json
{
  "model": "gpt-4",
  "messages": [
    {"role": "system", "content": "你是助手"},
    {"role": "user", "content": "你好"}
  ],
  "max_tokens": 1024,
  "temperature": 0.7
}
```

#### Vertex AI 请求

```json
{
  "contents": [
    {
      "role": "user",
      "parts": [{"text": "你好"}]
    }
  ],
  "systemInstruction": {
    "parts": [{"text": "你是助手"}]
  },
  "generationConfig": {
    "maxOutputTokens": 1024,
    "temperature": 0.7
  }
}
```

---

## llm-proxy 实现

### 协议配置

在 llm-proxy 中配置 Google Vertex AI 后端时，URL 应包含完整的模型路径（不包括 RPC 方法名）：

```yaml
backends:
- name: "vertex-ai"
  url: "https://aiplatform.googleapis.com/v1/projects/{PROJECT_ID}/locations/us-central1/publishers/google/models/gemini-1.5-pro"
  api_key: "{access-token}"  # OAuth 2.0 访问令牌
  protocol: "google"
  timeout: 60s
  retry_times: 3
```

**重要说明**: 
- **URL 配置**: 必须包含完整的项目路径，但**不要**包含 `:generateContent` 或 `:streamGenerateContent` 后缀
- **自动添加后缀**: llm-proxy 会根据请求类型自动添加：
  - 非流式请求 → 添加 `:generateContent`
  - 流式请求 → 添加 `:streamGenerateContent`
- **认证方式**: 
  - `api_key` 应为 Google Cloud OAuth 2.0 访问令牌
  - 或者在 URL 中添加 `?key={API_KEY}` 参数（适用于 Gemini API）

**Gemini API 配置示例**（开发者版本，更简单）：

```yaml
backends:
- name: "gemini-api"
  url: "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-pro"
  api_key: "{your-api-key}"  # 直接使用 API Key
  protocol: "google"
```

### 实现详情

Google Vertex AI 协议**不兼容** OpenAI，需要进行完整的协议转换：

#### 请求转换流程

1. **消息格式转换**
   - 将 `messages[]` 转换为 `contents[]`
   - 将 `content` 字符串转换为 `parts[{text}]` 数组
   - 角色映射：`assistant` → `model`

2. **系统提示提取**
   - 从 `messages` 数组中提取 `role: system` 的消息
   - 将其转换为 `systemInstruction.parts[{text}]` 格式
   - 从 contents 数组中移除 system 消息

3. **生成配置转换**
   - 将顶层参数移动到 `generationConfig` 对象
   - `max_tokens` → `maxOutputTokens`
   - `stop` → `stopSequences`

4. **端点路径**
   - 非流式: `:generateContent`
   - 流式: `:streamGenerateContent`

#### 响应转换流程

1. **非流式响应**
   - 将 `candidates[0].content.parts[0].text` 转换为 `choices[0].message.content`
   - 将 `finishReason` 映射为 `finish_reason`
   - 将 `usageMetadata` 转换为 `usage` 对象

2. **流式响应**
   - 解析 JSON Lines 格式（每行一个 JSON 对象）
   - 转换为 SSE 格式：`data: {...}\n\n`
   - 将 `candidates[].content.parts[].text` 转换为 `choices[].delta.content`
   - 最后发送 `data: [DONE]`

#### 代码实现位置

| 功能 | 文件路径 |
|------|---------|
| 请求转换 | `src/application/service/protocol/google/request.go` |
| 响应转换 | `src/application/service/protocol/google/response.go` |
| 流式处理 | `src/application/service/protocol/google/stream.go` |
| 错误转换 | `src/application/service/protocol/google/error.go` |

### 特殊处理

1. **系统提示提取**: 从 messages 数组中提取并移动到 `systemInstruction` 字段
2. **角色名称映射**: `assistant` ↔ `model`
3. **Parts 数组转换**: 字符串内容 ↔ `parts[{text}]` 数组
4. **流式格式转换**: JSON Lines ↔ SSE
5. **安全评级处理**: 记录 `safetyRatings` 信息（可选）
6. **多候选处理**: 如果返回多个候选，选择第一个

### 错误码映射

| Vertex AI 状态 | 内部错误代码 |
|----------------|--------------|
| INVALID_ARGUMENT | `INVALID_REQUEST` |
| UNAUTHENTICATED | `UNAUTHORIZED` |
| PERMISSION_DENIED | `FORBIDDEN` |
| NOT_FOUND | `NOT_FOUND` |
| RESOURCE_EXHAUSTED | `RATE_LIMITED` |
| INTERNAL | `BACKEND_ERROR` |
| UNAVAILABLE | `SERVICE_UNAVAILABLE` |

### 认证处理

Google Cloud 认证有两种方式：

1. **OAuth 2.0 访问令牌**（推荐）
   ```yaml
   api_key: "{access-token}"
   ```
   - 使用服务账号生成访问令牌
   - 令牌有效期通常为 1 小时
   - 需要定期刷新

2. **API Key**
   ```yaml
   url: "https://aiplatform.googleapis.com/v1/projects/{PROJECT_ID}/locations/us-central1/publishers/google/models/gemini-1.5-pro?key={API_KEY}"
   ```
   - 在 URL 中添加 `?key={API_KEY}` 参数
   - 适用于简单场景

---

## 参考资料

- [Vertex AI Gemini API 文档](https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/inference)
- [generateContent API 参考](https://cloud.google.com/vertex-ai/generative-ai/docs/reference/rest/v1/projects.locations.publishers.models/generateContent)
- [streamGenerateContent API 参考](https://cloud.google.com/vertex-ai/generative-ai/docs/reference/rest/v1/projects.locations.publishers.models/streamGenerateContent)
- [Gemini 模型概述](https://cloud.google.com/vertex-ai/generative-ai/docs/learn/models)
- [Google Cloud 认证](https://cloud.google.com/docs/authentication)
- [OpenAI Chat Completions API](https://platform.openai.com/docs/api-reference/chat)
