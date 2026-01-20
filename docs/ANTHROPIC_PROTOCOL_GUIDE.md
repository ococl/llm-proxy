# 配置 OOCC 上游使用 Anthropic 协议指南

## 问题背景

您需要将 llm-proxy 配置为：
- **上游（OOCC）**：使用 Anthropic 原生 API 协议
- **下游（客户端）**：继续使用 OpenAI 兼容格式

## 解决方案

已为项目添加协议转换功能，支持在后端级别或模型级别指定 API 协议。

## 代码变更

### 1. 配置结构扩展 (`src/config/config.go`)

```go
type Backend struct {
    Name     string `yaml:"name"`
    URL      string `yaml:"url"`
    APIKey   string `yaml:"api_key,omitempty"`
    Enabled  *bool  `yaml:"enabled,omitempty"`
    Protocol string `yaml:"protocol,omitempty"` // 新增：支持 "openai" 或 "anthropic"
}

type ModelRoute struct {
    Backend  string `yaml:"backend"`
    Model    string `yaml:"model"`
    Priority int    `yaml:"priority"`
    Enabled  *bool  `yaml:"enabled,omitempty"`
    Protocol string `yaml:"protocol,omitempty"` // 新增：模型级别协议覆盖
}
```

### 2. 协议转换器 (`src/proxy/protocol.go`)

新增 `ProtocolConverter` 类，负责：
- OpenAI → Anthropic 请求格式转换
- Anthropic → OpenAI 响应格式转换
- 消息格式、工具调用、参数映射等

### 3. 代理逻辑更新 (`src/proxy/proxy.go`)

- 根据配置自动选择协议
- 转换请求体格式
- 设置正确的 HTTP 头部（`x-api-key` vs `Authorization`）
- 添加 Anthropic 必需的头部（`anthropic-version`）

## 配置方法

### 方式 1：后端级别配置（推荐）

适用于整个后端统一使用一种协议：

```yaml
backends:
  - name: "oocc"
    url: "https://your-oocc.com/v1"
    api_key: "sk-ant-xxx"
    protocol: "anthropic"  # 👈 关键配置
    enabled: true

models:
  "anthropic/claude-sonnet-4":
    routes:
      - backend: "oocc"
        model: "claude-sonnet-4-20250514"
        priority: 1
```

### 方式 2：模型级别配置

适用于同一后端需要混合使用不同协议：

```yaml
backends:
  - name: "mixed-backend"
    url: "https://your-endpoint.com/v1"
    api_key: "sk-xxx"
    protocol: "openai"  # 后端默认协议

models:
  "claude-sonnet-4":
    routes:
      - backend: "mixed-backend"
        model: "claude-sonnet-4"
        protocol: "anthropic"  # 👈 模型级别覆盖
        priority: 1
  
  "gpt-4":
    routes:
      - backend: "mixed-backend"
        model: "gpt-4"
        # 使用后端默认的 openai 协议
        priority: 1
```

## 协议优先级

```
模型级别 protocol > 后端级别 protocol > 默认 "openai"
```

## 协议转换细节

### OpenAI → Anthropic

1. **消息格式**：
   - 提取 `system` 角色消息 → Anthropic `system` 字段
   - 保留 `user` 和 `assistant` 消息

2. **参数映射**：
   ```
   max_tokens / max_completion_tokens → max_tokens
   stop → stop_sequences
   temperature, top_p, stream → 直接传递
   ```

3. **HTTP 头部**：
   ```
   Authorization: Bearer xxx → x-api-key: xxx
   添加: anthropic-version: 2023-06-01
   移除: OpenAI-Organization, OpenAI-Project
   ```

4. **工具调用**：
   ```json
   // OpenAI 格式
   {"type": "function", "function": {"name": "...", "parameters": {...}}}
   
   // 转换为 Anthropic 格式
   {"name": "...", "description": "...", "input_schema": {...}}
   ```

### Anthropic → OpenAI

响应自动转换回 OpenAI 格式，客户端无感知。

## 客户端使用

客户端代码**无需修改**，继续使用 OpenAI SDK：

```python
from openai import OpenAI

client = OpenAI(
    api_key="sk-your-unified-api-key",
    base_url="http://localhost:8080/v1"
)

# 代理会自动转换为 Anthropic 协议发送到 OOCC
response = client.chat.completions.create(
    model="anthropic/claude-sonnet-4",
    messages=[
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": "Hello!"}
    ]
)
```

## 配置示例文件

- **完整示例**：`src/config.anthropic.example.yaml`
- **详细文档**：`docs/anthropic_protocol_config.md`

## 测试步骤

1. **更新配置文件**：
   ```bash
   cp src/config.anthropic.example.yaml config.yaml
   # 修改 OOCC 的 URL 和 API Key
   ```

2. **编译运行**：
   ```bash
   cd src
   go build -o ../llm-proxy
   cd ..
   ./llm-proxy -config config.yaml
   ```

3. **测试请求**：
   ```bash
   curl http://localhost:8080/v1/chat/completions \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer sk-your-unified-api-key" \
     -d '{
       "model": "anthropic/claude-sonnet-4",
       "messages": [{"role": "user", "content": "Hello!"}]
     }'
   ```

4. **查看日志**：
   ```bash
   tail -f logs/general.log
   ```
   
   应该能看到类似日志：
   ```
   协议: anthropic
   已转换为Anthropic协议格式
   ```

## 注意事项

1. **max_tokens 参数**：Anthropic 要求必须提供，代理会自动设置默认值 4096

2. **流式响应**：两种协议的 SSE 格式不同，代理会自动处理

3. **错误处理**：协议转换失败会记录详细日志并触发回退机制

4. **调试模式**：设置 `logging.debug_mode: true` 可查看详细的协议转换日志

## 故障排查

### 问题：请求失败，提示格式错误

**检查**：
- 确认 `protocol: "anthropic"` 配置正确
- 查看日志中是否有 "已转换为Anthropic协议格式"
- 检查 OOCC 端点是否支持 Anthropic 原生格式

### 问题：认证失败

**检查**：
- Anthropic 使用 `x-api-key` 头部，不是 `Authorization`
- 确认 API Key 格式正确（通常以 `sk-ant-` 开头）

### 问题：响应格式错误

**检查**：
- 确认 OOCC 返回的是标准 Anthropic 响应格式
- 查看错误日志中的响应内容

## 后续优化建议

1. **流式响应转换**：当前实现可能需要进一步优化 SSE 格式转换
2. **更多参数支持**：如 Anthropic 的 `thinking` 参数等
3. **响应缓存**：考虑添加 Anthropic Prompt Caching 支持
4. **测试覆盖**：添加协议转换的单元测试

## 相关文件

- `src/config/config.go` - 配置结构定义
- `src/proxy/protocol.go` - 协议转换逻辑
- `src/proxy/proxy.go` - 代理主逻辑
- `src/config.anthropic.example.yaml` - 配置示例
- `docs/anthropic_protocol_config.md` - 详细文档

