# 日志记录策略

## 概述

本文档描述 llm-proxy 项目的日志记录策略，包括日志级别分配、记录内容和使用场景。

## 日志级别分配

### DEBUG 级别 - 详细追踪信息

用于开发和调试时的详细信息记录，包含完整的请求/响应体。

**记录内容：**
- 客户端请求体（完整 JSON）
- 上游请求体（完整 JSON）
- 上游响应体（完整 JSON）
- 客户端响应体（完整 JSON）
- 流式数据块内容
- 协议转换详细信息（消息数量、系统提示注入等）
- 中间处理步骤（路由过滤、后端选择等）

**使用场景：**
- 开发环境调试
- 问题追踪和排查
- 性能分析

**示例：**
```
DEBUG 客户端请求体 req_id=abc123 request_body={"model":"gpt-4","messages":[...]}
DEBUG 上游请求体 req_id=abc123 backend=openai request_body={"model":"gpt-4","messages":[...]}
DEBUG 上游响应体 req_id=abc123 backend=openai response_body={"id":"chatcmpl-xxx","choices":[...]}
DEBUG 收到上游流式数据块 req_id=abc123 backend=openai chunk_index=1 chunk_data={"choices":[...]}
```

### INFO 级别 - 关键节点信息

用于记录请求处理的关键节点，不包含请求/响应体内容。

**记录内容：**
- 收到客户端请求（方法、路径、客户端地址）
- 发送上游请求（后端名称、模型）
- 上游请求完成（响应 ID、状态）
- 请求处理成功/失败
- 重试成功
- 降级触发

**使用场景：**
- 生产环境监控
- 请求流程跟踪
- 性能统计

**示例：**
```
INFO 收到客户端请求 req_id=abc123 method=POST path=/v1/chat/completions remote_addr=127.0.0.1:12345
INFO 发送上游请求 req_id=abc123 backend=openai backend_model=gpt-4
INFO 上游请求完成 req_id=abc123 backend=openai response_id=chatcmpl-xxx
INFO 非流式请求处理成功 req_id=abc123 model=gpt-4 response_id=chatcmpl-xxx
```

### WARN 级别 - 警告信息

用于记录非致命错误和异常情况。

**记录内容：**
- 验证失败
- 后端全部冷却
- 上游返回错误状态码
- 重试触发

**示例：**
```
WARN 非流式验证失败 req_id=abc123 model=invalid-model error=missing_model
WARN 后端全部冷却，尝试降级 req_id=abc123 model=gpt-4 cooldown_count=3
WARN 上游返回错误状态码 req_id=abc123 backend=openai status_code=429
```

### ERROR 级别 - 错误信息

用于记录导致请求失败的错误。

**记录内容：**
- 读取请求体失败
- 解析 JSON 失败
- 协议转换失败
- 上游请求发送失败
- 重试次数耗尽
- 其他致命错误

**示例：**
```
ERROR 读取请求体失败 req_id=abc123 error=io: read timeout
ERROR 解析上游响应JSON失败 req_id=abc123 backend=openai error=invalid character
ERROR 非流式请求失败 req_id=abc123 model=gpt-4 duration_ms=1500 error=no_backend
```

## 请求生命周期日志记录点

### 1. 收到客户端请求

```
INFO  收到客户端请求 req_id=xxx method=POST path=/v1/chat/completions remote_addr=...
DEBUG 检测客户端协议 req_id=xxx protocol=openai
DEBUG 请求体读取成功 req_id=xxx body_size=1024
DEBUG 请求体解析成功 req_id=xxx
DEBUG 客户端请求体 req_id=xxx request_body={"model":"gpt-4",...}
DEBUG 领域请求构建完成 req_id=xxx model=gpt-4 stream=false
```

### 2. 请求转换（协议转换）

```
DEBUG 开始协议转换（请求） req_id=xxx target_protocol=openai model=gpt-4 message_count=3
DEBUG 协议转换完成（请求） req_id=xxx target_protocol=openai result_message_count=4 system_prompt_injected=true
```

### 3. 上游请求发送

```
DEBUG 准备发送上游请求 req_id=xxx backend=openai backend_url=https://api.openai.com backend_model=gpt-4
DEBUG 上游请求体 req_id=xxx backend=openai request_body={"model":"gpt-4",...}
INFO  发送上游请求 req_id=xxx backend=openai backend_model=gpt-4
```

### 4. 上游响应接收

```
DEBUG 收到上游响应 req_id=xxx backend=openai status_code=200
DEBUG 上游响应体读取完成 req_id=xxx backend=openai body_size=512
DEBUG 上游响应体 req_id=xxx backend=openai response_body={"id":"chatcmpl-xxx",...}
DEBUG 上游响应解析成功 req_id=xxx backend=openai
INFO  上游请求完成 req_id=xxx backend=openai response_id=chatcmpl-xxx
```

### 5. 流式响应处理

```
DEBUG 开始读取上游流式响应 req_id=xxx backend=openai
DEBUG 收到上游流式数据块 req_id=xxx backend=openai chunk_index=1 chunk_data={"choices":[...]}
DEBUG 处理流式数据块 req_id=xxx model=gpt-4 chunk_data={"choices":[...]}
DEBUG 收到上游[DONE]信号 req_id=xxx backend=openai
DEBUG 上游流式响应结束 req_id=xxx backend=openai total_chunks=25
INFO  上游流式请求完成 req_id=xxx backend=openai total_chunks=25
```

### 6. 响应转换（协议转换）

```
DEBUG 开始协议转换（响应） response_id=chatcmpl-xxx source_protocol=openai choice_count=1 prompt_tokens=10 completion_tokens=50
DEBUG 协议转换完成（响应） response_id=chatcmpl-xxx source_protocol=openai
```

### 7. 返回客户端响应

```
DEBUG 客户端响应体 req_id=xxx response_body={"id":"chatcmpl-xxx",...}
INFO  非流式请求处理成功 req_id=xxx model=gpt-4 response_id=chatcmpl-xxx
```

## 日志输出配置

### 生产环境推荐配置

```yaml
logging:
  level: info           # 只记录 INFO 及以上级别
  format: json          # 结构化日志便于分析
  mask_sensitive: true  # 启用敏感信息掩码
```

### 开发环境推荐配置

```yaml
logging:
  level: debug          # 记录所有级别日志
  format: console       # 控制台友好格式
  mask_sensitive: false # 禁用掩码便于调试
```

### 调试/追踪配置

```yaml
logging:
  level: debug
  format: json
  mask_sensitive: false
  separate_files: true  # 每个请求单独文件
  request_dir: ./logs/requests
  error_dir: ./logs/errors
```

## 日志字段说明

### 通用字段

- `req_id`: 请求唯一标识符（十六进制时间戳）
- `model`: 客户端请求的模型别名
- `backend`: 上游后端名称
- `backend_model`: 上游实际模型名称
- `response_id`: 上游响应 ID

### 性能字段

- `duration_ms`: 耗时（毫秒）
- `body_size`: 请求/响应体大小（字节）
- `chunk_index`: 流式数据块序号
- `total_chunks`: 流式数据块总数

### 状态字段

- `status_code`: HTTP 状态码
- `attempt`: 重试次数
- `max_retries`: 最大重试次数
- `cooldown_count`: 冷却中的后端数量

### 数据字段（DEBUG 级别）

- `request_body`: 请求体 JSON 字符串
- `response_body`: 响应体 JSON 字符串
- `chunk_data`: 流式数据块 JSON 字符串

## 注意事项

1. **敏感信息保护**：生产环境必须启用 `mask_sensitive: true`
2. **日志量控制**：DEBUG 级别会产生大量日志，仅在必要时启用
3. **性能影响**：完整请求/响应体序列化会影响性能，生产环境使用 INFO 级别
4. **磁盘空间**：启用 `separate_files: true` 时注意磁盘空间管理
5. **日志轮转**：配置日志轮转策略防止磁盘占满

## 日志分析示例

### 追踪单个请求的完整流程

```bash
# 按 req_id 过滤日志
grep 'req_id=abc123' llm-proxy.log
```

### 统计后端错误率

```bash
# 统计各后端的错误数
grep 'ERROR.*backend=' llm-proxy.log | awk -F'backend=' '{print $2}' | awk '{print $1}' | sort | uniq -c
```

### 分析请求延迟

```bash
# 提取请求耗时
grep 'duration_ms=' llm-proxy.log | grep -oP 'duration_ms=\K\d+' | sort -n
```
