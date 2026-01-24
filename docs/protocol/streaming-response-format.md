# OpenAI 兼容的流式响应格式详解

## 概述

OpenAI Chat Completion API 的流式响应使用 Server-Sent Events (SSE) 协议传输增量数据块(chunk)。这种格式已成为 LLM API 的事实标准。

## 流式响应格式

### 请求参数

```json
{
  "model": "gpt-4",
  "messages": [
    {"role": "user", "content": "Hello"}
  ],
  "stream": true
}
```

### 响应格式

```http
HTTP/1.1 200 OK
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
X-Accel-Buffering: no

data: {"id":"chatcmpl-ABC123","object":"chat.completion.chunk","created":1699016000,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}

data: {"id":"chatcmpl-ABC123","object":"chat.completion.chunk","created":1699016000,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-ABC123","object":"chat.completion.chunk","created":1699016000,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":null}]}

data: [DONE]
```

## 数据块结构

### 完整响应块

```json
{
  "id": "chatcmpl-ABC123",
  "object": "chat.completion.chunk",
  "created": 1699016000,
  "model": "gpt-4",
  "system_fingerprint": "fp_abc123",
  "choices": [
    {
      "index": 0,
      "delta": {
        "role": "assistant",
        "content": "Hello"
      },
      "logprobs": null,
      "finish_reason": null
    }
  ]
}
```

### 字段说明

| 字段 | 类型 | 描述 |
|------|------|------|
| `id` | string | 完成 ID |
| `object` | string | 固定为 `chat.completion.chunk` |
| `created` | integer | Unix 时间戳 |
| `model` | string | 模型名称 |
| `system_fingerprint` | string | 后端指纹 |
| `choices[].index` | integer | 选项索引 |
| `choices[].delta` | object | 增量数据 |
| `choices[].logprobs` | object | 对数概率信息 |
| `choices[].finish_reason` | string | 结束原因 |

### Delta 字段详解

Delta 包含增量更新：

```json
// 第一个块：角色信息
{
  "delta": {
    "role": "assistant"
  },
  "finish_reason": null
}

// 内容块：文本增量
{
  "delta": {
    "content": "Hello"
  },
  "finish_reason": null
}

// 工具调用块
{
  "delta": {
    "tool_calls": [
      {
        "index": 0,
        "id": "call_abc",
        "type": "function",
        "function": {
          "name": "get_weather",
          "arguments": ""
        }
      }
    ]
  }
}

// 参数增量（多块组成完整 JSON）
{
  "delta": {
    "tool_calls": [
      {
        "index": 0,
        "function": {
          "arguments": "{\"location\""
        }
      }
    ]
  }
}
```

## 结束原因 (finish_reason)

| 值 | 描述 |
|----|------|
| `stop` | 自然停止（遇到 stop 序列或模型决定停止） |
| `length` | 达到 max_tokens 限制 |
| `tool_calls` | 模型决定调用工具 |
| `content_filter` | 内容被过滤 |
| `function_call` | 旧版函数调用（已弃用） |
| `refusal` | 模型拒绝回答 |

## Logprobs 格式

```json
{
  "choices": [
    {
      "index": 0,
      "delta": {
        "content": "H"
      },
      "logprobs": {
        "content": [
          {
            "token": "H",
            "logprob": -0.123,
            "bytes": [72],
            "top_logprobs": [
              {
                "token": "H",
                "logprob": -0.123,
                "bytes": [72]
              },
              {
                "token": "h",
                "logprob": -2.456,
                "bytes": [104]
              }
            ]
          }
        ]
      },
      "finish_reason": null
    }
  ]
}
```

## 工具调用流式处理

### 工具调用开始

```json
{
  "id": "chatcmpl-ABC123",
  "object": "chat.completion.chunk",
  "created": 1699016000,
  "model": "gpt-4",
  "choices": [
    {
      "index": 0,
      "delta": {
        "tool_calls": [
          {
            "index": 0,
            "id": "call_abc123",
            "type": "function",
            "function": {
              "name": "get_weather",
              "arguments": ""
            }
          }
        ]
      },
      "finish_reason": null
    }
  ]
}
```

### 参数分块传输

```json
// 第一块参数
{
  "choices": [
    {
      "delta": {
        "tool_calls": [
          {
            "index": 0,
            "function": {
              "arguments": "{\"loc"
            }
          }
        ]
      }
    }
  ]
}

// 第二块参数
{
  "choices": [
    {
      "delta": {
        "tool_calls": [
          {
            "index": 0,
            "function": {
              "arguments": "ation\": \"Bei"
            }
          }
        ]
      }
    }
  ]
}

// 最后一块
{
  "choices": [
    {
      "delta": {
        "tool_calls": [
          {
            "index": 0,
            "function": {
              "arguments": "jing\"}"
            }
          }
        ]
      },
      "finish_reason": "tool_calls"
    }
  ]
}
```

## 特殊处理

### 空内容块

某些模型可能发送空内容块用于心跳：

```json
{
  "id": "chatcmpl-ABC123",
  "object": "chat.completion.chunk",
  "created": 1699016000,
  "model": "gpt-4",
  "choices": [
    {
      "index": 0,
      "delta": {},
      "logprobs": null,
      "finish_reason": null
    }
  ]
}
```

### 客户端实现示例

```python
import requests
import json

def stream_chat_completion(api_key, messages, model="gpt-4"):
    headers = {
        "Authorization": f"Bearer {api_key}",
        "Content-Type": "application/json"
    }
    
    data = {
        "model": model,
        "messages": messages,
        "stream": True
    }
    
    response = requests.post(
        "https://api.openai.com/v1/chat/completions",
        headers=headers,
        json=data,
        stream=True
    )
    
    content = ""
    
    for line in response.iter_lines():
        if line:
            line = line.decode('utf-8')
            
            if line.startswith('data: '):
                data_str = line[6:]
                
                if data_str == '[DONE]':
                    break
                
                try:
                    chunk = json.loads(data_str)
                    
                    if chunk.get("choices"):
                        delta = chunk["choices"][0].get("delta", {})
                        chunk_content = delta.get("content", "")
                        
                        if chunk_content:
                            content += chunk_content
                            print(chunk_content, end="", flush=True)
                            
                except json.JSONDecodeError:
                    continue
    
    return content
```

## LLM 代理中的实现

### Go 语言实现

```go
package sse

import (
    "encoding/json"
    "fmt"
    "net/http"
)

type ChatCompletionChunk struct {
    ID                string   `json:"id"`
    Object            string   `json:"object"`
    Created           int64    `json:"created"`
    Model             string   `json:"model"`
    SystemFingerprint string   `json:"system_fingerprint,omitempty"`
    Choices           []Choice `json:"choices"`
}

type Choice struct {
    Index        int          `json:"index"`
    Delta        Delta        `json:"delta"`
    LogProbs     interface{}  `json:"logprobs,omitempty"`
    FinishReason string       `json:"finish_reason,omitempty"`
}

type Delta struct {
    Role         string         `json:"role,omitempty"`
    Content      string         `json:"content,omitempty"`
    ToolCalls    []ToolCall     `json:"tool_calls,omitempty"`
    FunctionCall *FunctionCall  `json:"function_call,omitempty"`
}

type ToolCall struct {
    Index       int           `json:"index"`
    ID          string        `json:"id"`
    Type        string        `json:"type"`
    Function    FunctionCall  `json:"function"`
}

type FunctionCall struct {
    Name      string `json:"name"`
    Arguments string `json:"arguments"`
}

// WriteChunk 写入 SSE 数据块
func WriteChunk(w http.ResponseWriter, chunk *ChatCompletionChunk) error {
    data, err := json.Marshal(chunk)
    if err != nil {
        return err
    }
    
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    
    _, err = fmt.Fprintf(w, "data: %s\n\n", data)
    if err != nil {
        return err
    }
    
    if f, ok := w.(http.Flusher); ok {
        f.Flush()
    }
    
    return nil
}

// WriteDone 写入结束标记
func WriteDone(w http.ResponseWriter) error {
    _, err := w.Write([]byte("data: [DONE]\n\n"))
    if err != nil {
        return err
    }
    
    if f, ok := w.(http.Flusher); ok {
        f.Flush()
    }
    
    return nil
}
```

## 最佳实践

1. **及时刷新**: 使用 `http.Flusher` 立即发送数据
2. **处理空块**: 忽略内容为空的 delta
3. **合并参数**: 工具调用的 arguments 需要合并
4. **错误处理**: 优雅处理连接中断
5. **超时设置**: 设置合理的读取超时
6. **资源清理**: 确保正确关闭连接
