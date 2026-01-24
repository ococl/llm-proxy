# Server-Sent Events (SSE) 协议规范

**版本**: W3C Recommendation (2015年2月3日)  
**文档地址**: https://www.w3.org/TR/2015/REC-eventsource-20150203/  
**MIME 类型**: `text/event-stream`  
**字符编码**: UTF-8

## 概述

Server-Sent Events (SSE) 是一种服务器推送技术，允许服务器通过 HTTP 连接向客户端自动推送数据。与 WebSocket 不同，SSE 是单向的（服务器→客户端），更适合只需要服务器推送的场景，如实时新闻、股票价格、LLM 流式响应等。

## 核心特性

- **单向通信**: 只能由服务器发送数据到客户端
- **基于 HTTP**: 使用标准 HTTP 协议，不需要特殊协议
- **自动重连**: 连接断开后浏览器自动尝试重连
- **事件类型**: 支持自定义事件类型
- **Last-Event-ID**: 支持断点续传，客户端可告知最后收到的消息 ID

## 连接状态

| 状态 | 值 | 描述 |
|------|-----|------|
| CONNECTING | 0 | 连接尚未建立或正在重连 |
| OPEN | 1 | 连接已打开，正在接收事件 |
| CLOSED | 2 | 连接已关闭，不自动重连 |

## 事件流格式

### ABNF 语法

```
stream        = [ bom ] *event
event         = *( comment / field ) end-of-line
comment       = ":" *any-char end-of-line
field         = 1*name-char [ ":" [ space ] *any-char ] end-of-line
end-of-line   = ( cr lf / cr / lf )
```

### 字段说明

| 字段名 | 描述 | 示例 |
|--------|------|------|
| `data` | 事件数据，可多行 | `data: Hello\ndata: World` |
| `event` | 事件类型名称 | `event: message` |
| `id` | 事件 ID，用于断点续传 | `id: 123` |
| `retry` | 重连间隔（毫秒） | `retry: 5000` |
| `:` | 注释行，以冒号开头 | `: This is a comment` |

### 数据字段 (data)

- 多行数据会在行尾添加换行符 `\n`
- 空行表示事件结束，触发事件派发
- 数据以 `data:` 开头

```sse
# 单行数据
data: Hello World

# 多行数据（实际内容为 "line1\nline2"）
data: line1
data: line2
```

### 事件类型字段 (event)

- 默认为 `message` 事件
- 可自定义事件类型供客户端监听

```sse
event: add
data: {"user": "Alice"}

event: remove  
data: {"user": "Bob"}
```

### 事件 ID 字段 (id)

- 设置最后事件 ID
- 客户端重连时会发送 `Last-Event-ID` 头
- 用于断点续传

```sse
data: Message 1
id: 1

data: Message 2
id: 2
```

### 重连间隔字段 (retry)

- 服务器建议客户端重连间隔（毫秒）
- 只在注释行后有效

```sse
: Recommended reconnection interval is 5 seconds
retry: 5000
```

## HTTP 请求头

### 请求头

```http
Accept: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
Last-Event-ID: <id>  # 重连时携带
```

### 响应头

```http
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

## 连接处理

### 成功响应

- HTTP 200 OK
- Content-Type: `text/event-stream`
- 客户端触发 `open` 事件

### 断开重连

| 状态码 | 行为 |
|--------|------|
| 204 No Content | 不自动重连 |
| 500, 502, 503, 504 | 自动重连 |
| 301, 302, 307, 303 | 自动重连，跟随重定向 |

### 失败处理

- 非 200 状态码（除重定向外）不自动重连
- Content-Type 不匹配不自动重连

## OpenAI 兼容的流式响应格式

OpenAI 的 Chat Completion API 使用 SSE 格式传输流式数据，但有以下特殊约定：

### 标准 SSE 响应

```sse
data: {"id": "chatcmpl-abc", "object": "chat.completion.chunk", "created": 1234567890, "model": "gpt-4", "choices": [...]}

data: [DONE]
```

### 特殊约定

1. **Content-Type**: `text/event-stream`
2. **编码**: UTF-8
3. **结束标记**: `data: [DONE]` 表示流结束
4. **JSON 数据**: 每行 `data:` 后是完整的 JSON 对象
5. **错误处理**: 错误信息可能在 `error` 字段中

### 示例响应

```http
HTTP/1.1 200 OK
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive

data: {"id":"chatcmpl-ABC123","object":"chat.completion.chunk","created":1699016000,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}

data: {"id":"chatcmpl-ABC123","object":"chat.completion.chunk","created":1699016000,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-ABC123","object":"chat.completion.chunk","created":1699016000,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":null}]}

data: [DONE]
```

## LLM 代理中的 SSE 实现

### 核心接口设计

```go
// SSEWriter 定义 SSE 写入接口
type SSEWriter interface {
    // WriteEvent 写入单个事件
    WriteEvent(event string, data interface{}) error
    // WriteChunk 写入原始数据块
    WriteChunk(data []byte) error
    // Flush 刷新缓冲区
    Flush() error
    // Close 关闭流
    Close() error
}

// ChatCompletionWriter 定义聊天完成流写入接口
type ChatCompletionWriter interface {
    // WriteChunk 写入完成块
    WriteChunk(chunk *ChatCompletionChunk) error
    // WriteDone 写入结束标记
    WriteDone() error
}
```

### 推荐的实现方式

```go
// 使用标准库实现 SSE 响应
func WriteSSE(w http.ResponseWriter, data interface{}) error {
    jsonData, err := json.Marshal(data)
    if err != nil {
        return err
    }
    
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("Transfer-Encoding", "chunked")
    
    fmt.Fprintf(w, "data: %s\n\n", jsonData)
    w.(http.Flusher).Flush()
    
    return nil
}

// 写入结束标记
func WriteDone(w http.ResponseWriter) error {
    _, err := w.Write([]byte("data: [DONE]\n\n"))
    if err != nil {
        return err
    }
    w.(http.Flusher).Flush()
    return nil
}
```

### 最佳实践

1. **设置正确的 Headers**:
   - `Content-Type: text/event-stream`
   - `Cache-Control: no-cache`
   - `Connection: keep-alive`

2. **禁用压缩**:
   - SSE 不应压缩，因为数据是流式的

3. **使用 Flusher**:
   - 及时刷新缓冲区确保实时性

4. **处理客户端断开**:
   - 检测 `w.Write()` 错误
   - 适当超时设置

5. **重连支持**:
   - 支持 `Last-Event-ID` 头
   - 维护消息队列支持断点续传

## 浏览器端使用

```javascript
// 创建 EventSource
const source = new EventSource('/stream-endpoint');

// 连接打开
source.onopen = () => {
    console.log('连接已建立');
};

// 接收消息
source.onmessage = (event) => {
    const data = JSON.parse(event.data);
    console.log('收到数据:', data);
};

// 错误处理
source.onerror = () => {
    console.log('连接错误，尝试重连...');
};

// 自定义事件
source.addEventListener('custom', (event) => {
    const data = JSON.parse(event.data);
    console.log('自定义事件:', data);
});

// 关闭连接
source.close();
```

## 跨域考虑

SSE 支持 CORS：

```http
Access-Control-Allow-Origin: *
Access-Control-Allow-Headers: Cache-Control, Last-Event-ID
```

## 参考资料

- [W3C SSE 规范](https://www.w3.org/TR/2015/REC-eventsource-20150203/)
- [MDN Server-Sent Events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events)
- [WHATWG HTML Living Standard](https://html.spec.whatwg.org/dev/server-sent-events.html)
- [OpenAI Chat Completion Streaming](https://platform.openai.com/docs/api-reference/chat/streaming)
