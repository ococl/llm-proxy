# Anthropic 协议配置示例

## 场景说明
本配置展示如何配置 llm-proxy 使用 Anthropic 原生协议访问上游服务（如 oocc），
同时对下游客户端仍然提供 OpenAI 兼容的 API 接口。

## 配置方式

### 方式 1: 后端级别配置（推荐用于整个后端都使用同一协议）

```yaml
backends:
  - name: "oocc-anthropic"
    url: "https://your-oocc-endpoint.com/v1"
    api_key: "sk-ant-xxx"
    protocol: "anthropic"  # 指定该后端使用 Anthropic 协议
    enabled: true

  - name: "openai-backend"
    url: "https://api.openai.com/v1"
    api_key: "sk-xxx"
    protocol: "openai"  # 或者省略，默认就是 openai
    enabled: true

models:
  "claude-sonnet-4":
    routes:
      - backend: "oocc-anthropic"
        model: "claude-sonnet-4-20250514"
        priority: 1
```

### 方式 2: 模型级别配置（推荐用于同一后端混合使用不同协议）

```yaml
backends:
  - name: "mixed-backend"
    url: "https://your-endpoint.com/v1"
    api_key: "sk-xxx"
    protocol: "openai"  # 后端默认协议
    enabled: true

models:
  "claude-sonnet-4":
    routes:
      - backend: "mixed-backend"
        model: "claude-sonnet-4-20250514"
        protocol: "anthropic"  # 模型级别覆盖后端协议
        priority: 1
  
  "gpt-4":
    routes:
      - backend: "mixed-backend"
        model: "gpt-4-turbo"
        protocol: "openai"  # 或省略，使用后端默认协议
        priority: 1
```

## 完整配置示例

```yaml
listen: ":8080"
proxy_api_key: "sk-your-unified-api-key"

proxy:
  enable_system_prompt: false
  forward_client_ip: true

# 后端配置
backends:
  # OOCC 后端 - 使用 Anthropic 协议
  - name: "oocc"
    url: "https://your-oocc.com/v1"
    api_key: "sk-ant-oocc-xxx"
    protocol: "anthropic"
    enabled: true

  # OpenAI 官方后端
  - name: "openai"
    url: "https://api.openai.com/v1"
    api_key: "sk-openai-xxx"
    protocol: "openai"  # 可省略
    enabled: true

# 模型路由配置
models:
  # Claude 模型 - 路由到 OOCC (Anthropic 协议)
  "anthropic/claude-sonnet-4":
    routes:
      - backend: "oocc"
        model: "claude-sonnet-4-20250514"
        priority: 1

  "anthropic/claude-opus-4":
    routes:
      - backend: "oocc"
        model: "claude-opus-4-20250514"
        priority: 1

  # GPT 模型 - 路由到 OpenAI
  "openai/gpt-4o":
    routes:
      - backend: "openai"
        model: "gpt-4o"
        priority: 1

# 故障转移配置
fallback:
  cooldown_seconds: 300
  max_retries: 3

# 错误检测
detection:
  error_codes: ["4xx", "5xx"]
  error_patterns:
    - "insufficient_quota"
    - "rate_limit"
    - "exceeded"

# 日志配置
logging:
  level: "info"
  console_level: "info"
  base_dir: "./logs"
  mask_sensitive: true
  debug_mode: false
```

## 客户端使用方式

客户端仍然使用 OpenAI SDK 格式调用：

```python
from openai import OpenAI

client = OpenAI(
    api_key="sk-your-unified-api-key",
    base_url="http://localhost:8080/v1"
)

# 调用 Claude 模型（代理会自动转换为 Anthropic 协议）
response = client.chat.completions.create(
    model="anthropic/claude-sonnet-4",
    messages=[
        {"role": "user", "content": "Hello!"}
    ]
)

print(response.choices[0].message.content)
```

## 协议转换说明

### OpenAI → Anthropic 转换

代理会自动处理以下转换：

1. **消息格式**：
   - 提取 `system` 消息到 Anthropic 的 `system` 字段
   - 转换 `messages` 数组格式

2. **参数映射**：
   - `max_tokens` / `max_completion_tokens` → `max_tokens`
   - `stop` → `stop_sequences`
   - 其他参数：`temperature`, `top_p`, `stream` 直接传递

3. **请求头**：
   - 添加 `anthropic-version: 2023-06-01`
   - 使用 `x-api-key` 替代 `Authorization: Bearer`
   - 移除 OpenAI 特定头部

4. **工具调用**：
   - OpenAI `tools` 格式 → Anthropic `tools` 格式
   - `tool_choice` 格式转换

### Anthropic → OpenAI 转换

响应会自动转换回 OpenAI 格式，客户端无感知。

## 注意事项

1. **协议优先级**：模型级别 `protocol` > 后端级别 `protocol` > 默认 `openai`

2. **必需参数**：Anthropic 要求 `max_tokens`，如果客户端未提供，代理会自动设置为 4096

3. **流式响应**：两种协议的流式格式不同，代理会自动处理转换

4. **错误处理**：协议转换失败会记录详细日志并回退到下一个路由

