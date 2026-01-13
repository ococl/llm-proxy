# LLM Proxy

轻量级 LLM API 代理服务器，支持多提供商负载均衡、多级自动回退和异常检测。

[![CI/CD](https://github.com/ococl/llm-proxy/actions/workflows/ci.yml/badge.svg)](https://github.com/ococl/llm-proxy/actions/workflows/ci.yml)

## 功能特性

- **统一 API Key**：用户只需配置一个端点和密钥，代理自动处理后端认证
- **多对多模型别名**：统一不同提供商的模型命名（如 `anthropic/claude-opus-4-5`）
- **多级回退策略**：
  - L1：别名内后端优先级回退
  - L2：别名间跨模型回退
- **负载均衡**：同优先级后端自动随机分配
- **灵活启用控制**：后端、别名、路由三级 `enabled` 开关
- **冷却机制**：失败后端自动冷却，可配置时长
- **错误码通配符**：支持 `4xx`、`5xx` 等通配符匹配
- **完全透传**：Headers、Body 完全透传，支持 SSE 流式响应
- **配置热加载**：修改配置后下次请求自动生效
- **滚动日志**：按日期自动分割，支持敏感信息脱敏
- **性能指标**：可选记录请求耗时、后端耗时等指标
- **多平台支持**：Windows、Linux、macOS (amd64/arm64)

## 快速开始

### 下载

从 [Releases](https://github.com/ococl/llm-proxy/releases) 下载对应平台的二进制文件。

### 运行

```bash
# 1. 解压并进入目录
unzip llm-proxy-linux-amd64.zip
cd llm-proxy

# 2. 复制并编辑配置
cp config.example.yaml config.yaml
vim config.yaml

# 3. 启动代理
./llm-proxy-linux-amd64 -config config.yaml
```

### 客户端使用

```bash
# 使用统一 API Key 请求
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer sk-your-unified-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "anthropic/claude-opus-4-5",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": true
  }'

# 查看可用模型
curl http://localhost:8080/v1/models

# 健康检查
curl http://localhost:8080/health
```

## 配置说明

```yaml
listen: ":8080"

# 统一 API Key（用户使用此密钥访问代理）
proxy_api_key: "sk-your-unified-api-key"

# 后端定义
backends:
  - name: "provider-a"
    url: "https://api.provider-a.com/v1"
    api_key: "sk-real-api-key-a"        # 实际后端密钥
    enabled: true                        # 可选，默认 true

  - name: "provider-b"
    url: "https://api.provider-b.com/v1"
    api_key: "sk-real-api-key-b"
    enabled: false                       # 临时停用

# 模型别名（多对多映射）
models:
  "anthropic/claude-opus-4-5":
    enabled: true                        # 别名级开关，默认 true
    routes:
      - backend: "provider-a"
        model: "claude-opus-4-5"         # 后端实际模型名
        priority: 1                      # 优先级（数字越小越优先）
        enabled: true                    # 路由级开关，默认 true
      - backend: "provider-b"
        model: "claude-opus-4-5"
        priority: 2

  "anthropic/claude-sonnet-4-5":
    routes:
      - backend: "provider-a"
        model: "claude-sonnet-4-5"
        priority: 1

# 回退配置
fallback:
  cooldown_seconds: 300                  # 冷却时间（秒）
  max_retries: 3                         # 单次请求最大尝试次数（0=不限制）
  
  # L2 别名间回退（当主别名所有后端不可用时）
  alias_fallback:
    "anthropic/claude-opus-4-5":
      - "anthropic/claude-sonnet-4-5"    # 回退到 sonnet
      - "google/gemini-3-pro-preview"    # 再回退到 gemini
    "anthropic/claude-sonnet-4-5":
      - "google/gemini-3-pro-preview"

# 异常检测
detection:
  error_codes: ["4xx", "5xx"]            # 支持通配符
  error_patterns:
    - "insufficient_quota"
    - "rate_limit"
    - "exceeded"
    - "billing"
    - "quota"

# 日志配置
logging:
  level: "info"                          # debug/info/warn/error
  general_file: "./logs/proxy.log"       # 日志文件路径
  separate_files: false                  # 是否为每个请求创建独立文件
  request_dir: "./logs/requests"         # 独立请求日志目录
  error_dir: "./logs/errors"             # 独立错误日志目录
  mask_sensitive: true                   # 敏感信息脱敏（API Key 等）
  enable_metrics: false                  # 性能指标记录
  max_file_size_mb: 100                  # 单个日志文件最大大小（MB）
```

## 回退策略

### L1：别名内回退

```
请求 anthropic/claude-opus-4-5
  → provider-a (priority 1) → 失败 → 冷却
  → provider-b (priority 2) → 失败 → 冷却
  → 触发 L2 回退
```

### L2：别名间回退

```
anthropic/claude-opus-4-5 所有后端不可用
  → 回退到 anthropic/claude-sonnet-4-5
    → provider-a → 成功！
```

### 负载均衡

同优先级的多个后端会随机选择，实现负载均衡：

```yaml
routes:
  - backend: "provider-a"
    model: "model-x"
    priority: 1              # 同优先级
  - backend: "provider-b"
    model: "model-x"
    priority: 1              # 随机选择 a 或 b
  - backend: "provider-c"
    model: "model-x"
    priority: 2              # 仅当 priority 1 都不可用时使用
```

## 日志

### 日志级别

| 级别 | 内容 |
|------|------|
| ERROR | 严重错误（所有后端失败、配置加载失败） |
| WARN | 潜在问题（API Key 验证失败、后端返回错误） |
| INFO | 关键业务事件（请求开始/完成、后端切换） |
| DEBUG | 调试信息（路由解析、跳过原因） |

### 日志示例

```
[2026-01-13 09:41:00] [INFO] LLM Proxy 启动，监听地址: :8080
[2026-01-13 09:41:00] [INFO] 已加载 4 个后端，13 个模型别名
[2026-01-13 09:41:05] [INFO] [req_abc123] 收到请求: 模型=anthropic/claude-opus-4-5 客户端=127.0.0.1
[2026-01-13 09:41:06] [INFO] [req_abc123] 请求成功: 后端=provider-a 状态=200 耗时=1234ms
```

### 敏感信息脱敏

启用 `mask_sensitive: true` 后，API Key 会显示为：

```
sk-ab****cdef
```

## 构建

### 本地构建

```bash
# 单平台
cd src
go build -o ../dist/llm-proxy .

# 多平台（Windows）
build.bat all

# 多平台（Linux/macOS）
make build-all
```

### 构建产物

| 平台 | 文件 |
|------|------|
| Windows amd64 | `llm-proxy-windows-amd64.exe` |
| Windows arm64 | `llm-proxy-windows-arm64.exe` |
| Linux amd64 | `llm-proxy-linux-amd64` |
| Linux arm64 | `llm-proxy-linux-arm64` |
| macOS amd64 | `llm-proxy-darwin-amd64` |
| macOS arm64 | `llm-proxy-darwin-arm64` |

## 测试

```bash
cd src
go test -v ./...
```

## 目录结构

```
llm-proxy/
├── .github/workflows/      # CI/CD 配置
│   └── ci.yml
├── src/                    # 源代码
│   ├── main.go
│   ├── config.go
│   ├── proxy.go
│   ├── router.go
│   ├── backend.go
│   ├── detector.go
│   ├── logger.go
│   ├── *_test.go           # 单元测试
│   └── config.example.yaml
├── dist/                   # 构建产物
├── docs/                   # 设计文档
├── build.bat               # Windows 构建脚本
├── Makefile                # Linux/macOS 构建脚本
└── README.md
```

## API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/v1/chat/completions` | POST | 聊天补全（透传到后端） |
| `/v1/models` | GET | 获取可用模型列表 |
| `/models` | GET | 同上 |
| `/health` | GET | 健康检查 |
| `/healthz` | GET | 健康检查（K8s 兼容） |

## License

MIT
