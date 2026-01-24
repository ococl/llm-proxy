# 日志系统完善方案

## 概述

本次日志系统完善为 llm-proxy Clean Architecture 重构项目添加了完整的请求追踪日志，覆盖从客户端请求接收到上游响应返回的完整生命周期。

## 日志架构

### 日志框架
- **使用框架**: Uber Zap (通过 `adapter/logging/ZapLoggerAdapter`)
- **日志接口**: `domain/port/Logger` (依赖倒置原则)
- **结构化日志**: 支持键值对字段，便于日志分析和追踪

### 日志级别分配原则

| 级别 | 使用场景 | 示例 |
|------|---------|------|
| **DEBUG** | 详细的诊断信息，开发调试用 | 协议检测、请求体大小、路由过滤结果 |
| **INFO** | 关键业务流程节点 | 收到请求、发送上游请求、请求完成 |
| **WARN** | 警告信息，不影响主流程 | 验证失败、空choices、冷却中的后端 |
| **ERROR** | 错误信息，影响当前请求 | 读取失败、解析失败、转换失败 |

## 日志覆盖的关键节点

### 1. HTTP Handler 层 (`adapter/http/handler.go`)

#### 1.1 请求接收
```
INFO  收到客户端请求
  req_id: 请求ID
  method: HTTP方法
  path: 请求路径
  remote_addr: 客户端地址

DEBUG 检测客户端协议
  req_id: 请求ID
  protocol: 协议类型 (openai/anthropic)
```

#### 1.2 请求体处理
```
DEBUG 请求体读取成功
  req_id: 请求ID
  body_size: 请求体字节数

DEBUG 请求体解析成功
  req_id: 请求ID

DEBUG 领域请求构建完成
  req_id: 请求ID
  model: 模型别名
  stream: 是否流式
```

#### 1.3 非流式请求处理
```
DEBUG 开始处理非流式请求
  req_id: 请求ID
  model: 模型别名

INFO  非流式请求处理成功
  req_id: 请求ID
  model: 模型别名
  response_id: 响应ID
```

#### 1.4 流式请求处理
```
DEBUG 开始处理流式请求
  req_id: 请求ID
  model: 模型别名

INFO  流式请求处理成功
  req_id: 请求ID
  model: 模型别名
```

#### 1.5 错误处理
```
ERROR 读取请求体失败
  req_id: 请求ID
  error: 错误信息

ERROR 解析请求体JSON失败
  req_id: 请求ID
  error: 错误信息

ERROR 构建领域请求失败
  req_id: 请求ID
  error: 错误信息
```

### 2. Protocol Converter 层 (`application/service/protocol_converter.go`)

#### 2.1 请求协议转换
```
DEBUG 开始协议转换（请求）
  req_id: 请求ID
  target_protocol: 目标协议
  model: 模型别名

DEBUG 协议转换完成（请求）
  req_id: 请求ID
  target_protocol: 目标协议
```

#### 2.2 响应协议转换
```
DEBUG 开始协议转换（响应）
  response_id: 响应ID
  source_protocol: 源协议

DEBUG 协议转换完成（响应）
  response_id: 响应ID
  source_protocol: 源协议
```

#### 2.3 转换错误
```
ERROR 协议转换失败（请求）
  req_id: 请求ID
  target_protocol: 目标协议
  error: 错误信息

ERROR 协议转换失败（响应）
  response_id: 响应ID
  source_protocol: 源协议
  error: 错误信息
```

### 3. Backend Client Adapter 层 (`adapter/backend/client_adapter.go`)

#### 3.1 非流式上游请求
```
DEBUG 准备发送上游请求
  req_id: 请求ID
  backend: 后端名称
  backend_url: 后端URL
  backend_model: 后端模型名

INFO  发送上游请求
  req_id: 请求ID
  backend: 后端名称
  backend_model: 后端模型名

DEBUG 收到上游响应
  req_id: 请求ID
  backend: 后端名称
  status_code: HTTP状态码

DEBUG 上游响应体读取完成
  req_id: 请求ID
  backend: 后端名称
  body_size: 响应体字节数

DEBUG 上游响应解析成功
  req_id: 请求ID
  backend: 后端名称

INFO  上游请求完成
  req_id: 请求ID
  backend: 后端名称
  response_id: 响应ID
```

#### 3.2 流式上游请求
```
DEBUG 准备发送上游流式请求
  req_id: 请求ID
  backend: 后端名称
  backend_url: 后端URL
  backend_model: 后端模型名

INFO  发送上游流式请求
  req_id: 请求ID
  backend: 后端名称
  backend_model: 后端模型名

DEBUG 收到上游流式响应
  req_id: 请求ID
  backend: 后端名称
  status_code: HTTP状态码

DEBUG 开始读取上游流式响应
  req_id: 请求ID
  backend: 后端名称

DEBUG 收到上游[DONE]信号
  req_id: 请求ID
  backend: 后端名称

DEBUG 上游流式响应结束
  req_id: 请求ID
  backend: 后端名称
  total_chunks: 总数据块数

INFO  上游流式请求完成
  req_id: 请求ID
  backend: 后端名称
  total_chunks: 总数据块数
```

#### 3.3 上游错误处理
```
ERROR 上游请求发送失败
  req_id: 请求ID
  backend: 后端名称
  error: 错误信息

WARN  上游返回错误状态码
  req_id: 请求ID
  backend: 后端名称
  status_code: HTTP状态码

ERROR 读取上游响应失败
  req_id: 请求ID
  backend: 后端名称
  error: 错误信息

ERROR 解析上游响应JSON失败
  req_id: 请求ID
  backend: 后端名称
  error: 错误信息

WARN  上游返回空choices字段
  req_id: 请求ID
  backend: 后端名称

ERROR 处理上游流式数据块失败
  req_id: 请求ID
  backend: 后端名称
  chunk_index: 数据块索引
  error: 错误信息
```

### 4. Proxy Request UseCase 层 (`application/usecase/proxy_request.go`)

此层已有完善的日志记录（原有实现），包括：
- 请求开始/完成
- 路由解析/过滤
- 后端选择
- 降级策略
- 重试逻辑

## 完整请求追踪示例

### 成功的非流式请求日志链路

```
[INFO ] 收到客户端请求 req_id=abc123 method=POST path=/v1/chat/completions remote_addr=127.0.0.1:50001
[DEBUG] 检测客户端协议 req_id=abc123 protocol=openai
[DEBUG] 请求体读取成功 req_id=abc123 body_size=256
[DEBUG] 请求体解析成功 req_id=abc123
[DEBUG] 领域请求构建完成 req_id=abc123 model=gpt-4 stream=false
[DEBUG] 开始处理非流式请求 req_id=abc123 model=gpt-4
[INFO ] 非流式请求开始 req_id=abc123 model=gpt-4 client_protocol=openai stream=false
[DEBUG] 路由解析完成 req_id=abc123 model=gpt-4 total_routes=2
[DEBUG] 路由过滤完成 req_id=abc123 model=gpt-4 available_count=2
[DEBUG] 后端选择 req_id=abc123 model=gpt-4 backend=openai-1 backend_model=gpt-4-turbo
[DEBUG] 开始协议转换（请求） req_id=abc123 target_protocol=openai model=gpt-4
[DEBUG] 协议转换完成（请求） req_id=abc123 target_protocol=openai
[DEBUG] 准备发送上游请求 req_id=abc123 backend=openai-1 backend_url=https://api.openai.com/v1 backend_model=gpt-4-turbo
[INFO ] 发送上游请求 req_id=abc123 backend=openai-1 backend_model=gpt-4-turbo
[DEBUG] 收到上游响应 req_id=abc123 backend=openai-1 status_code=200
[DEBUG] 上游响应体读取完成 req_id=abc123 backend=openai-1 body_size=512
[DEBUG] 上游响应解析成功 req_id=abc123 backend=openai-1
[INFO ] 上游请求完成 req_id=abc123 backend=openai-1 response_id=chatcmpl-xyz789
[DEBUG] 开始协议转换（响应） response_id=chatcmpl-xyz789 source_protocol=openai
[DEBUG] 协议转换完成（响应） response_id=chatcmpl-xyz789 source_protocol=openai
[INFO ] 非流式请求完成 req_id=abc123 duration_ms=1234
[INFO ] 非流式请求处理成功 req_id=abc123 model=gpt-4 response_id=chatcmpl-xyz789
```

### 流式请求日志链路

```
[INFO ] 收到客户端请求 req_id=def456 method=POST path=/v1/chat/completions remote_addr=127.0.0.1:50002
[DEBUG] 检测客户端协议 req_id=def456 protocol=openai
[DEBUG] 请求体读取成功 req_id=def456 body_size=280
[DEBUG] 请求体解析成功 req_id=def456
[DEBUG] 领域请求构建完成 req_id=def456 model=gpt-4 stream=true
[DEBUG] 开始处理流式请求 req_id=def456 model=gpt-4
[INFO ] 请求开始 req_id=def456 model=gpt-4 client_protocol=openai
[DEBUG] 路由解析完成 req_id=def456 model=gpt-4 total_routes=2
[DEBUG] 后端选择 req_id=def456 model=gpt-4 backend=openai-1 backend_model=gpt-4-turbo
[DEBUG] 开始协议转换（请求） req_id=def456 target_protocol=openai model=gpt-4
[DEBUG] 协议转换完成（请求） req_id=def456 target_protocol=openai
[DEBUG] 准备发送上游流式请求 req_id=def456 backend=openai-1 backend_url=https://api.openai.com/v1 backend_model=gpt-4-turbo
[INFO ] 发送上游流式请求 req_id=def456 backend=openai-1 backend_model=gpt-4-turbo
[DEBUG] 收到上游流式响应 req_id=def456 backend=openai-1 status_code=200
[DEBUG] 开始读取上游流式响应 req_id=def456 backend=openai-1
[DEBUG] 收到上游[DONE]信号 req_id=def456 backend=openai-1
[DEBUG] 上游流式响应结束 req_id=def456 backend=openai-1 total_chunks=42
[INFO ] 上游流式请求完成 req_id=def456 backend=openai-1 total_chunks=42
[INFO ] 请求完成 req_id=def456 duration_ms=3456
[INFO ] 流式请求处理成功 req_id=def456 model=gpt-4
```

## 代码变更总结

### 修改的文件

1. **src/adapter/http/handler.go**
   - 添加请求接收日志
   - 添加请求体处理日志
   - 添加请求处理开始/完成日志
   - 添加错误处理日志

2. **src/application/service/protocol_converter.go**
   - 添加 logger 字段
   - 更新构造函数签名 `NewProtocolConverter(systemPrompts, logger)`
   - 在 `ToBackend()` 和 `FromBackend()` 中添加转换日志

3. **src/adapter/backend/client_adapter.go**
   - 添加 logger 字段
   - 更新构造函数签名 `NewBackendClientAdapter(client, logger)`
   - 在 `Send()` 中添加完整的请求/响应日志
   - 在 `SendStreaming()` 中添加流式处理日志

4. **src/main.go**
   - 更新 `NewProtocolConverter` 调用，传入 logger
   - 更新 `NewBackendClientAdapter` 调用，传入 logger

5. **测试文件**
   - `src/application/service/protocol_converter_test.go`: 更新所有测试用例，传入 `&port.NopLogger{}`
   - `src/adapter/backend/client_adapter_test.go`: 更新所有测试用例，传入 `&port.NopLogger{}`

### 依赖注入优化

所有新增的 logger 参数都遵循以下原则：
- 接受 `port.Logger` 接口（依赖倒置）
- 提供 nil 检查，默认使用 `&port.NopLogger{}`（防御性编程）
- 测试中使用 `&port.NopLogger{}` 保持测试简洁

## 日志配置建议

### 开发环境
```yaml
logging:
  level: debug        # 显示所有DEBUG级别日志
  colorize: true      # 启用彩色输出
  format: console     # 人类可读格式
```

### 生产环境
```yaml
logging:
  level: info         # 只记录INFO及以上级别
  colorize: false     # 禁用彩色输出
  format: json        # 结构化JSON格式，便于日志采集
```

### 故障排查
```yaml
logging:
  level: debug        # 临时开启DEBUG级别
  format: json        # 保持JSON格式便于分析
```

## 监控和分析建议

### 关键指标追踪

通过日志可以提取以下指标：
1. **请求成功率**: `INFO 非流式请求处理成功` / `INFO 收到客户端请求`
2. **平均响应时间**: 分析 `duration_ms` 字段
3. **上游后端健康度**: `ERROR 上游请求发送失败` 按 backend 分组
4. **协议转换错误率**: `ERROR 协议转换失败` 计数

### 日志聚合查询示例

使用 ELK/Loki 等日志系统查询：

```
# 查找请求ID为abc123的完整链路
req_id:abc123

# 查找所有上游错误
level:ERROR AND (message:"上游请求" OR message:"上游响应")

# 统计各后端的请求量
message:"发送上游请求" | stats count by backend

# 查找慢请求（>5秒）
duration_ms:>5000 AND message:"请求完成"
```

## 总结

本次日志完善实现了：
✅ **完整覆盖**: 从请求接收到响应返回的所有关键节点
✅ **结构化日志**: 统一的键值对格式，便于分析
✅ **合理分级**: DEBUG/INFO/WARN/ERROR 清晰划分
✅ **追踪能力**: 通过 req_id 可完整追踪请求生命周期
✅ **依赖倒置**: 遵循 Clean Architecture 原则
✅ **测试完备**: 所有修改都通过了测试

这套日志系统可以满足：
- **开发调试**: DEBUG 级别提供详细诊断信息
- **运维监控**: INFO 级别记录关键业务流程
- **故障排查**: 完整的错误上下文和请求链路
- **性能分析**: duration_ms 和 body_size 等性能指标
