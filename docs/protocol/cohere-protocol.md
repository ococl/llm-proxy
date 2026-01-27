# Cohere 協議規範

本文檔描述 Cohere API 的協議格式和實現細節。

## 目錄

- [概述](#概述)
- [請求格式](#請求格式)
- [響應格式](#響應格式)
- [錯誤響應](#錯誤響應)
- [與 OpenAI 的差異](#與-openai-的差異)
- [llm-proxy 實現](#llm-proxy-實現)

---

## 概述

Cohere 是一家專注於企業級語言模型的公司，提供 Command 系列模型。Cohere 的 API 設計與 OpenAI 高度兼容，同時提供一些企業級功能。

| 屬性 | 值 |
|------|-----|
| **協議類型** | `cohere` |
| **API 版本** | v1 |
| **基礎 URL** | `https://api.cohere.com/v1` |
| **認證方式** | Bearer Token |
| **支持的模型** | command-r-plus, command-r, command, command-nightly |

---

## 請求格式

### 端點

```
POST https://api.cohere.com/v1/chat
```

### 請求頭

```
Content-Type: application/json
Authorization: Bearer {COHERE_API_KEY}
```

### 請求體結構

```json
{
  "model": "command-r-plus",
  "message": "你好，請介紹一下你自己",
  "chat_history": [
    {"role": "USER", "message": "之前問過的問題"},
    {"role": "CHATBOT", "message": "之前的回答"}
  ],
  "connectors": null,
  "preamble": "你是 helpful AI 助手",
  "temperature": 1.0,
  "max_tokens": 1000,
  "k": null,
  "p": null,
  "logit_bias": null,
  "stop_sequences": null,
  "frequency_penalty": 0,
  "presence_penalty": 0,
  "tools": null,
  "tool_results": null,
  "force_single_step": false,
  "stream": false,
  "citation_quality": "accurate"
}
```

### 字段說明

#### 必填字段

| 字段名 | 類型 | 描述 |
|--------|------|------|
| **message** | string | 用戶消息（單輪對話）或 |
| **chat_history** | array | 對話歷史（多輪對話） |

#### 可選字段

| 字段名 | 類型 | 默認值 | 描述 |
|--------|------|--------|------|
| **model** | string | "command" | 模型名稱 |
| **preamble** | string | null | 系統提示 |
| **chat_history** | array | null | 對話歷史 |
| **connectors** | array | null | 連接器配置（RAG） |
| **temperature** | number | 1.0 | 采樣溫度 |
| **max_tokens** | integer | 1000 | 最大生成 token 數 |
| **k** | integer | null | Top-k 采樣 |
| **p** | number | null | Top-p 采樣 |
| **frequency_penalty** | number | 0 | 頻率懲罰 |
| **presence_penalty** | number | 0 | 存在懲罰 |
| **stop_sequences** | array | null | 停止序列 |
| **force_single_step** | boolean | false | 禁用工具調用 |
| **stream** | boolean | false | 是否流式響應 |
| **citation_quality** | string | "accurate" | 引文質量控制 |

### 消息格式

#### 單輪對話

```json
{
  "message": "用戶消息"
}
```

#### 多輪對話

```json
{
  "message": "新消息",
  "chat_history": [
    {"role": "USER", "message": "第一輪用戶消息"},
    {"role": "CHATBOT", "message": "第一輪助手回複"}
  ]
}
```

| 角色 | 說明 |
|------|------|
| **USER** | 用戶消息 |
| **CHATBOT** | 助手回複 |

---

## 響應格式

### 非流式響應

```json
{
  "response_id": "resp-123abc",
  "message": "助手回複內容",
  "chat_history": [
    {"role": "USER", "message": "用戶消息"},
    {"role": "CHATBOT", "message": "助手回複"}
  ],
  "token_count": {
    "prompt_tokens": 50,
    "response_tokens": 100,
    "total_tokens": 150
  },
  "finish_reason": "COMPLETE",
  "tool_calls": [],
  "citations": []
}
```

#### 響應字段說明

| 字段 | 類型 | 描述 |
|------|------|------|
| **response_id** | string | 響應 ID |
| **message** | string | 助手回複內容 |
| **chat_history** | array | 更新後的對話歷史 |
| **token_count.prompt_tokens** | integer | 輸入 token 數 |
| **token_count.response_tokens** | integer | 輸出 token 數 |
| **token_count.total_tokens** | integer | 總 token 數 |
| **finish_reason** | string | 結束原因 |
| **citations** | array | 引文列表（RAG） |

### 流式響應

啟用 `stream: true` 後，響應為 SSE 格式：

```
data: {"event_type":"stream-start","response_id":"resp-123"}

data: {"event_type":"text-generation","text":"你"}

data: {"event_type":"text-generation","text":"好"}

data: {"event_type":"stream-end","response_id":"resp-123","finish_reason":"COMPLETE"}
```

### 結束原因 (finish_reason)

| 值 | 描述 |
|----|------|
| **COMPLETE** | 自然完成 |
| **MAX_TOKENS** | 達到最大 token 限制 |
| **ERROR** | 發生錯誤 |
| **TOOL_USE** | 工具調用 |

---

## 錯誤響應

### HTTP 狀態碼

| 狀態碼 | 說明 |
|--------|------|
| 400 | 請求參數錯誤 |
| 401 | 認證失敗 |
| 403 | 權限不足 |
| 429 | 速率限制 |
| 500 | 內部服務器錯誤 |
| 503 | 服務不可用 |

### 錯誤響應格式

```json
{
  "message": "錯誤描述信息",
  "error_type": "authentication_error"
}
```

### 常見錯誤類型

| 錯誤類型 | 描述 |
|----------|------|
| **authentication_error** | 認證失敗 |
| **invalid_request_error** | 請求參數錯誤 |
| **rate_limit_error** | 速率限制 |
| **server_error** | 服務器錯誤 |
| **permission_error** | 權限不足 |

---

## 與 OpenAI 的差異

### 兼容性

**重要**: Cohere API 與 OpenAI API **不兼容**。Cohere 使用完全不同的請求/響應格式，需要進行協議轉換才能與 OpenAI 客戶端互操作。

### 主要差異

| 特性 | OpenAI | Cohere | 說明 |
|------|--------|--------|------|
| **端點路徑** | `/chat/completions` | `/v1/chat` | 完全不同的端點 |
| **消息格式** | messages 數組 | message + chat_history | **不兼容**：需要轉換 |
| **系統提示** | system 消息 | 獨立的 preamble 字段 | **不兼容**：位置不同 |
| **角色命名** | user/assistant | USER/CHATBOT | 大小寫和命名不同 |
| **模型名稱** | gpt-4 | command-r-plus | 不同的命名規範 |
| **引文** | 無 | 支持 | Cohere 獨有的 RAG 功能 |
| **連接器** | 無 | 支持 | Cohere 獨有的 RAG 連接器 |
| **響應格式** | choices 數組 | 單個 message 字段 | **不兼容**：結構不同 |

### 字段映射表

| OpenAI 字段 | Cohere 字段 | 說明 |
|-------------|-------------|------|
| messages[].content | message | 消息內容 |
| messages[].role | chat_history[].role | 角色（命名不同） |
| system 消息 | preamble | 系統提示 |
| tools | tools | 工具定義 |
| response_format | 不支持 | Cohere 不支持 |

### 請求轉換示例

#### OpenAI 請求

```json
{
  "model": "gpt-4",
  "messages": [
    {"role": "system", "content": "你是助手"},
    {"role": "user", "content": "你好"}
  ]
}
```

#### Cohere 請求

```json
{
  "model": "command-r-plus",
  "preamble": "你是助手",
  "message": "你好",
  "chat_history": []
}
```

---

## RAG 支持

Cohere 提供強大的 RAG（檢索增強生成）支持。

### 連接器配置

```json
{
  "connectors": [
    {
      "id": "web-search",
      "fields": {}
    },
    {
      "id": "database-connector",
      "fields": {
        "query_type": "semantic"
      }
    }
  ]
}
```

### 引文響應

```json
{
  "message": "根據文檔，答案是...",
  "citations": [
    {
      "start": 0,
      "end": 10,
      "text_snippet": "相關文檔片段",
      "document_id": "doc-123"
    }
  ]
}
```

---

## llm-proxy 實現

### 協議配置

```yaml
backends:
- name: "cohere"
  url: "https://api.cohere.com"  # 基礎 URL，不包含路徑
  api_key: "{api-key}"
  protocol: "cohere"
  timeout: 60s
  retry_times: 3
```

### 實現詳情

Cohere 協議**不兼容** OpenAI，需要進行完整的協議轉換：

#### 請求轉換流程

1. **消息格式轉換**
   - 提取最後一條 `user` 消息作為 `message` 字段
   - 將之前的消息轉換為 `chat_history` 數組
   - 角色映射：`user` → `USER`，`assistant` → `CHATBOT`

2. **系統提示提取**
   - 從 `messages` 數組中提取 `role: system` 的消息
   - 將其 `content` 設置為 `preamble` 字段

3. **端點路徑**
   - 使用 `/v1/chat` 而非 `/chat/completions`
   - 在 `client_adapter.go` 中通過 `getPathForProtocol()` 處理

#### 響應轉換流程

1. **非流式響應**
   - 將 Cohere 的 `message` 字段轉換為 OpenAI 的 `choices[0].message.content`
   - 將 `token_count` 轉換為 `usage` 對象
   - 將 `finish_reason` 映射為 OpenAI 格式

2. **流式響應**
   - 將 Cohere 的 `text-generation` 事件轉換為 OpenAI 的 `data: [DONE]` 格式
   - 保持 SSE 格式兼容性

#### 代碼實現位置

| 功��� | 文件路徑 |
|------|---------|
| 請求轉換 | `src/application/service/protocol/cohere/request_converter.go` |
| 響應轉換 | `src/application/service/protocol/cohere/response_converter.go` |
| 流式處理 | `src/application/service/protocol/cohere/stream_converter.go` |
| 錯誤轉換 | `src/application/service/protocol/cohere/error_converter.go` |
| 路徑處理 | `src/adapter/backend/client_adapter.go` (getPathForProtocol) |

### 特殊處理

1. **消息格式轉換**: 將 OpenAI 的 messages 數組轉換為 Cohere 的 message + chat_history 格式
2. **preamble 提取**: 從 system 消息中提取 preamble
3. **引文處理**: 透傳 citation 字段給客戶端（如果客戶端支持）
4. **RAG 支持**: 直接透傳 connectors 配置（如果客戶端提供）

### 錯誤碼映射

| Cohere 錯誤類型 | 內部錯誤代碼 |
|-----------------|--------------|
| invalid_request_error | `INVALID_REQUEST` |
| authentication_error | `UNAUTHORIZED` |
| permission_error | `BAD_REQUEST` |
| rate_limit_error | `RATE_LIMITED` |
| server_error | `BACKEND_ERROR` |

---

## 參考資料

- [Cohere API 文檔](https://docs.cohere.com/reference)
- [Cohere Chat API](https://docs.cohere.com/reference/chat)
- [Cohere RAG 指南](https://docs.cohere.com/docs/retrieval-augmented-generation)
- [OpenAI Chat Completions API](https://platform.openai.com/docs/api-reference/chat)
