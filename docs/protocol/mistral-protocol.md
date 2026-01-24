# Mistral AI 协议规范

本文檔描述 Mistral AI API 的協議格式和實現細節。

## 目錄

- [概述](#概述)
- [請求格式](#請求格式)
- [響應格式](#響應格式)
- [錯誤響應](#錯誤響應)
- [與 OpenAI 的差異](#與-openai-的差異)
- [llm-proxy 實現](#llm-proxy-實現)

---

## 概述

Mistral AI 是一家法國 AI 公司，提供高性能的大語言模型服務。Mistral API 採用 OpenAI 兼容設計，並提供一些獨特的功能擴展。

| 屬性 | 值 |
|------|-----|
| **協議類型** | `mistral` |
| **API 版本** | v1 |
| **基礎 URL** | `https://api.mistral.ai/v1` |
| **認證方式** | Bearer Token |
| **支持的模型** | mistral-small-latest, mistral-medium-latest, mistral-large-latest, open-mistral-7b |

---

## 請求格式

### 端點

```
POST https://api.mistral.ai/v1/chat/completions
```

### 請求頭

```
Content-Type: application/json
Authorization: Bearer {MISTRAL_API_KEY}
```

### 請求體結構

```json
{
  "model": "mistral-large-latest",
  "messages": [
    {"role": "system", "content": "你是 helpful AI 助手"},
    {"role": "user", "content": "你好"}
  ],
  "max_tokens": null,
  "temperature": null,
  "top_p": 1,
  "frequency_penalty": 0,
  "presence_penalty": 0,
  "n": null,
  "stream": false,
  "stop": null,
  "random_seed": null,
  "response_format": null,
  "tools": null,
  "tool_choice": "auto",
  "parallel_tool_calls": true,
  "safe_prompt": false,
  "metadata": null,
  "prediction": null,
  "prompt_mode": "reasoning"
}
```

### 字段說明

#### 必填字段

| 字段名 | 類型 | 描述 |
|--------|------|------|
| **model** | string | 模型 ID，如 `mistral-small-latest`、`mistral-large-latest` |
| **messages** | array | 對話消息數組 |

#### 可選字段

| 字段名 | 類型 | 默認值 | 描述 |
|--------|------|--------|------|
| **max_tokens** | integer | null | 生成的最大 token 數量 |
| **temperature** | number | null | 采樣溫度，建議 0.0-0.7 |
| **top_p** | number | 1 | 核采樣參數 |
| **frequency_penalty** | number | 0 | 頻率懲罰 |
| **presence_penalty** | number | 0 | 存在懲罰 |
| **n** | integer | null | 返回的補全數量 |
| **stream** | boolean | false | 是否流式返回 |
| **stop** | string/array | null | 停止生成的 token |
| **random_seed** | integer | null | 隨機采樣種子 |
| **response_format** | object | null | 輸出格式控制 |
| **tools** | array | null | 可用工具列表 |
| **tool_choice** | string/object | "auto" | 工具選擇策略 |
| **parallel_tool_calls** | boolean | true | 是否啟用並行工具調用 |
| **safe_prompt** | boolean | false | 是否注入安全提示 |
| **metadata** | object | null | 元數據映射 |
| **prediction** | object | null | 預期完成內容 |
| **prompt_mode** | string | "reasoning" | 推理模式開關 |

### Mistral 特有字段

#### prediction 字段

```json
"prediction": {
  "content": "預期的完成內容"
}
```

此功能允許指定預期內容以優化響應延遲。

#### response_format 字段

```json
// 文本模式
"response_format": {"type": "text"}

// JSON 對象模式
"response_format": {"type": "json_object"}

// JSON Schema 模式（Mistral 特有）
"response_format": {"type": "json_schema", "json_schema": {...}}
```

#### prompt_mode 字段

```json
"prompt_mode": "reasoning"
```

用於推理模型的提示模式開關。

### 消息格式

```json
[
  {"role": "system", "content": "你是 helpful AI 助手"},
  {"role": "user", "content": "用戶消息"},
  {"role": "assistant", "content": "助手回復"},
  {"role": "tool", "content": "工具結果", "tool_call_id": "call_123"}
]
```

---

## 響應格式

### 非流式響應

```json
{
  "id": "cmpl-e5cc70bb28c444948073e77776eb30ef",
  "object": "chat.completion",
  "created": 1702256327,
  "model": "mistral-small-latest",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "回答內容",
        "tool_calls": null
      },
      "finish_reason": "stop",
      "logprobs": null
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 20,
    "total_tokens": 30
  }
}
```

### 流式響應

啟用 `stream: true` 後，響應為 SSE 格式：

```
data: {"id":"cmpl-e5cc70bb28c444948073e77776eb30ef","object":"chat.completion.chunk","created":1702256327,"model":"mistral-small-latest","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}

data: {"id":"cmpl-e5cc70bb28c444948073e77776eb30ef","object":"chat.completion.chunk","created":1702256327,"model":"mistral-small-latest","choices":[{"index":0,"delta":{"content":"首"},"finish_reason":null}]}

data: [DONE]
```

### 結束原因 (finish_reason)

| 值 | 描述 |
|----|------|
| **stop** | 正常停止（遇到 stop 詞或自然結束） |
| **tool_calls** | 觸發工具調用 |
| **length** | 達到 max_tokens 限制 |
| **content_filter** | 內容被過濾 |
| **function_call** | 函數調用（遺留字段） |

---

## 錯誤響應

### HTTP 狀態碼

| 狀態碼 | 說明 | 處理建議 |
|--------|------|----------|
| 400 | Bad Request | 檢查請求參數格式和必填字段 |
| 401 | Unauthorized | 檢查 API Key 是否有效 |
| 403 | Forbidden | 檢查訪問權限 |
| 404 | Not Found | 檢查端點 URL |
| 429 | Rate Limit | 請求頻率超限，等待後重試 |
| 500 | Internal Server Error | 服務器內部錯誤 |
| 503 | Service Unavailable | 服務暫時不可用 |

### 錯誤響應格式

```json
{
  "object": "error",
  "type": "invalid_request_error",
  "message": "錯誤描述信息",
  "param": "參數名",
  "code": "錯誤代碼"
}
```

### 錯誤字段說明

| 字段 | 類型 | 說明 |
|------|------|------|
| **object** | string | 對象類型，固定為 "error" |
| **type** | string | 錯誤類型 |
| **message** | string | 人類可讀的錯誤描述 |
| **param** | string | 引發錯誤的參數名 |
| **code** | string | 錯誤代碼 |

### 常見錯誤類型

| 錯誤類型 | 描述 |
|----------|------|
| **invalid_request_error** | 參數缺失、格式錯誤、無效值 |
| **authentication_error** | API Key 無效或過期 |
| **rate_limit_error** | 請求超出頻率限制 |
| **service_unavailable_error** | 服務暫時不可用 |

---

## 與 OpenAI 的差異

### 兼容性

Mistral API 與 OpenAI API **高度兼容**，可以無縫遷移。

### 主要差異

| 特性 | OpenAI | Mistral | 說明 |
|------|--------|---------|------|
| **json_schema 支持** | ❌ | ✅ | Mistral 原生支持 JSON Schema |
| **parallel_tool_calls** | 默認 false | 默認 true | Mistral 默認啟用並行工具調用 |
| **prediction 字段** | ❌ | ✅ | Mistral 特有功能 |
| **prompt_mode** | ❌ | ✅ | Mistral 推理模式控制 |
| **random_seed** | ❌ | ✅ | Mistral 隨機種子控制 |
| **safe_prompt** | ❌ | ✅ | Mistral 安全提示注入 |

### 字段映射表

| OpenAI 字段 | Mistral 字段 | 說明 |
|-------------|--------------|------|
| model | model | 相同 |
| messages | messages | 相同 |
| max_tokens | max_tokens | 相同 |
| temperature | temperature | 相同 |
| top_p | top_p | 相同 |
| tools | tools | 兼容但參數結構略有不同 |
| response_format | response_format | Mistral 額外支持 json_schema |
| - | parallel_tool_calls | Mistral 特有 |
| - | prompt_mode | Mistral 特有 |
| - | prediction | Mistral 特有 |
| - | safe_prompt | Mistral 特有 |
| - | metadata | Mistral 特有 |
| - | random_seed | Mistral 特有 |

### 遷移建議

1. **API 調用**: 基本無縫遷移，端點和認證方式相同
2. **工具調用**: Mistral 的 `parallel_tool_calls` 默認開啟，需注意行為差異
3. **新功能**: 可利用 Mistral 的 `json_schema`、`prediction`、`prompt_mode` 等特性

---

## llm-proxy 實現

### 協議配置

```yaml
backends:
- name: "mistral"
  url: "https://api.mistral.ai/v1/chat/completions"
  api_key: "{api-key}"
  protocol: "mistral"
  timeout: 60s
  retry_times: 3
```

### 實現詳情

Mistral 被標記為 **OpenAI 兼容協議**，共享以下策略：

- ✅ **請求轉換**: 使用 `openai.NewRequestConverter`
- ✅ **響應轉換**: 使用 `openai.NewResponseConverter`
- ✅ **流式處理**: 使用 `openai.NewStreamChunkConverter`
- ✅ **錯誤轉換**: 使用 `openai.NewErrorConverter`

### 特殊處理

1. **parallel_tool_calls**: 默認為 true，與 OpenAI 不同
2. **Mistral 特有字段**: 直接透傳 `prediction`、`prompt_mode` 等字段
3. **JSON Schema**: response_format 支持 `json_schema` 類型

### 錯誤碼映射

| Mistral 錯誤類型 | 內部錯誤代碼 |
|-----------------|--------------|
| invalid_request_error | `INVALID_REQUEST` |
| authentication_error | `UNAUTHORIZED` |
| rate_limit_error | `RATE_LIMITED` |
| service_unavailable_error | `BACKEND_ERROR` |

---

## 參考資料

- [Mistral AI 官方文檔](https://docs.mistral.ai/)
- [Mistral Chat API 端點](https://docs.mistral.ai/api/endpoint/chat)
- [OpenAI Chat Completions API](https://platform.openai.com/docs/api-reference/chat)
