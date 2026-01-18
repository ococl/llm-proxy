# LLM-Proxy - 企业级 LLM API 代理服务

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.25.5-blue.svg)](https://golang.org/dl/)

**LLM-Proxy** 是一个高性能、可扩展的大语言模型 API 代理服务,为企业提供统一的 LLM 访问入口,支持负载均衡、自动故障转移、限流控制、并发管理等生产级特性。

---

## ✨ 核心特性

### 🚀 高可用架构
- **多后端负载均衡**: 支持多个 LLM 服务商并发调用,智能分发请求
- **自动故障转移**: 双层回退机制 (L1/L2),检测失败自动切换备用后端
- **冷却机制**: 失败后端自动进入冷却期,避免雪崩效应
- **健康检查**: 实时监控后端状态,动态调整路由策略

### 🎯 流量控制
- **多级限流**: 全局/IP/模型三层限流,基于 Token Bucket 算法
- **并发控制**: 请求队列管理,支持队列超时和溢出策略
- **优先级路由**: 基于优先级的后端选择,同级随机负载均衡

### 🔧 灵活配置
- **热重载配置**: 修改配置文件自动生效,无需重启服务
- **模型别名**: 统一模型命名,屏蔽底层供应商差异
- **系统提示词注入**: 自动注入系统级提示词,实现统一的行为控制
- **错误检测规则**: 可自定义 HTTP 状态码和响应体模式触发回退

### 📊 可观测性
- **结构化日志**: 基于 Zap 的高性能日志,支持多种输出格式
- **请求追踪**: 全链路 Trace ID,快速定位问题
- **敏感信息脱敏**: 自动脱敏 API Key、Token 等敏感字段
- **性能指标**: 请求时长、重试次数、后端状态等关键指标

---

## 📦 快速开始

### 环境要求
- Go 1.25.5 或更高版本
- 支持平台: Windows, Linux, macOS (AMD64/ARM64)

### 安装

#### 从源码构建
```bash
# 克隆项目
git clone https://github.com/ococl/llm-proxy.git
cd llm-proxy

# 快速构建 (当前平台)
make dev

# 或构建所有平台版本
make build-all
```

#### 使用预编译二进制
从 [Releases](https://github.com/ococl/llm-proxy/releases) 下载对应平台的二进制文件。

---

## 🔧 配置说明

### 基础配置

创建 `config.yaml`:

```yaml
# 监听地址
listen: ":8765"

# 代理全局 API Key (可选,用于访问控制)
proxy_api_key: "your-secret-key"

# 代理配置
proxy:
  enable_system_prompt: true   # 启用系统提示词注入
  forward_client_ip: true      # 转发客户端真实 IP

# 后端服务商配置
backends:
  - name: openai
    url: https://api.openai.com/v1
    api_key: sk-xxx
    enabled: true
    
  - name: anthropic
    url: https://api.anthropic.com/v1
    api_key: sk-ant-xxx
    enabled: true
    
  - name: local-llm
    url: http://localhost:8080/v1
    enabled: true

# 模型别名映射
models:
  gpt-4:
    routes:
      - backend: openai
        model: gpt-4-turbo-preview
        priority: 1
      - backend: local-llm      # 备用后端
        model: mixtral-8x7b
        priority: 2
        
  claude:
    routes:
      - backend: anthropic
        model: claude-3-opus-20240229
        priority: 1

# 故障转移配置
fallback:
  cooldown_seconds: 300   # 后端冷却时长 (秒)
  max_retries: 3          # 最大重试次数
  
  # L2 跨模型回退 (当所有 gpt-4 后端失败时,自动尝试 claude)
  alias_fallback:
    gpt-4: [claude]

# 错误检测规则
detection:
  error_codes: [4xx, 5xx]  # 触发回退的 HTTP 状态码 (支持通配符)
  error_patterns:           # 触发回退的响应体关键词
    - insufficient_quota
    - rate_limit
    - overloaded

# 限流配置
rate_limit:
  enabled: true
  global_rps: 1000      # 全局每秒请求数
  per_ip_rps: 100       # 每 IP 每秒请求数
  burst_factor: 1.5     # 突发流量倍数
  per_model_rps:        # 每模型限流
    gpt-4: 50

# 并发控制
concurrency:
  enabled: true
  max_requests: 500         # 最大并发请求数
  max_queue_size: 1000      # 最大队列长度
  queue_timeout: 30s        # 队列超时时间
  per_backend_limit: 100    # 每后端并发限制

# 日志配置
logging:
  level: info                 # 日志级别: debug, info, warn, error
  console_level: info         # 控制台日志级别
  base_dir: ./logs            # 日志目录
  separate_files: true        # 请求/错误日志分离
  mask_sensitive: true        # 脱敏敏感信息
  max_file_size_mb: 100       # 单文件大小限制
  max_age_days: 7             # 日志保留天数
  format: json                # 日志格式: json, text
  console_format: markdown    # 控制台格式: json, markdown
```

### 启动服务

```bash
# 使用默认配置 (config.yaml)
./llm-proxy

# 指定配置文件
./llm-proxy -config /path/to/config.yaml
```

---

## 🔌 API 使用

### 调用示例

```bash
# 使用 OpenAI SDK
export OPENAI_API_KEY="your-secret-key"  # 对应 config.yaml 中的 proxy_api_key
export OPENAI_BASE_URL="http://localhost:8765/v1"

python your_script.py
```

```python
# Python 示例
from openai import OpenAI

client = OpenAI(
    api_key="your-secret-key",
    base_url="http://localhost:8765/v1"
)

response = client.chat.completions.create(
    model="gpt-4",  # 使用代理中配置的模型别名
    messages=[
        {"role": "user", "content": "你好!"}
    ]
)

print(response.choices[0].message.content)
```

### 请求头说明

| 请求头 | 说明 | 必填 |
|--------|------|------|
| `Authorization` | `Bearer your-secret-key` | 是 |
| `X-Forwarded-For` | 客户端真实 IP (自动转发) | 否 |
| `X-Trace-ID` | 请求追踪 ID (自动生成) | 否 |

---

## 🏗️ 架构设计

### 请求处理流程

```
客户端请求
    ↓
[API Key 验证]
    ↓
[中间件链]
    ├── RecoveryMiddleware (Panic 恢复)
    ├── RateLimiter (限流控制)
    └── ConcurrencyLimiter (并发控制)
    ↓
[代理层 Proxy]
    ├── 请求体解析
    ├── 系统提示词注入
    └── 模型路由解析
    ↓
[路由层 Router]
    ├── 模型别名 → 后端映射
    ├── 优先级排序
    └── 冷却状态过滤
    ↓
[重试循环 (L1 回退)]
    ├── 后端 1 (优先级 1)
    ├── 后端 2 (优先级 1, 随机负载均衡)
    └── 后端 3 (优先级 2)
    ↓
[错误检测 Detector]
    ├── HTTP 状态码检查
    └── 响应体模式匹配
    ↓
[故障转移]
    ├── 触发冷却 (CooldownManager)
    └── L2 跨模型回退
    ↓
[响应处理]
    ├── 流式输出 (SSE)
    └── 非流式输出
    ↓
返回客户端
```

### 模块说明

| 模块 | 路径 | 职责 |
|------|------|------|
| **Proxy** | `src/proxy/proxy.go` | HTTP 请求处理、重试逻辑 |
| **Router** | `src/proxy/router.go` | 模型路由解析、负载均衡 |
| **Detector** | `src/proxy/detector.go` | 错误检测、回退判断 |
| **CooldownManager** | `src/backend/cooldown.go` | 后端冷却状态管理 |
| **RateLimiter** | `src/middleware/ratelimit.go` | 多级限流控制 |
| **ConcurrencyLimiter** | `src/middleware/concurrency.go` | 并发队列管理 |
| **ConfigManager** | `src/config/config.go` | 配置热重载 |
| **Logging** | `src/logging/*.go` | 结构化日志、脱敏 |

---

## 🧪 开发与测试

### 运行测试

```bash
# 所有测试
make test

# 指定包
cd src && go test -v ./proxy

# 单个测试
cd src && go test -v -run TestDetector_Wildcard ./proxy

# 覆盖率报告
cd src && go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 代码检查

```bash
# 格式化
cd src && gofmt -s -w .

# 静态分析
cd src && go vet ./...

# 依赖整理
cd src && go mod tidy
```

### 调试模式

```yaml
# config.yaml
logging:
  level: debug
  console_level: debug
  debug_mode: true
```

---

## 📊 监控与日志

### 日志文件结构

```
logs/
├── general.log         # 通用日志
├── requests/           # 请求日志 (按日期分割)
│   ├── 2026-01-18.log
│   └── 2026-01-19.log
└── errors/             # 错误日志
    └── 2026-01-18.log
```

### 日志字段

```json
{
  "level": "info",
  "ts": "2026-01-18T12:00:00.000Z",
  "msg": "请求成功",
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "model": "gpt-4",
  "backend": "openai",
  "status": 200,
  "duration_ms": 1234,
  "attempts": 1
}
```

---

## ⚙️ 高级配置

### 系统提示词注入

在 `system_prompts/` 目录创建 `<模型别名>.txt`:

```
system_prompts/
├── gpt-4.txt
└── claude.txt
```

内容示例:
```
你是一个专业的 AI 助手,遵循以下原则:
1. 回答简洁准确
2. 避免生成有害内容
3. 拒绝违法请求
```

### 超时配置 (计划支持)

```yaml
timeout:
  connect_timeout: 10s
  read_timeout: 60s
  write_timeout: 60s
  total_timeout: 10m
```

> ⚠️ **已知问题**: 当前超时配置未生效,HTTP 客户端硬编码为 5 分钟超时。修复中。

---

## 🤝 贡献指南

欢迎提交 Issue 和 Pull Request!

### 提交规范

```
类型(范围): 简短描述

详细说明

关联问题: #123
```

**类型**: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`

### 开发流程

1. Fork 本仓库
2. 创建特性分支: `git checkout -b feature/my-feature`
3. 提交代码: `git commit -m 'feat(proxy): 添加 XXX 功能'`
4. 推送分支: `git push origin feature/my-feature`
5. 创建 Pull Request

---

## 📄 许可证

本项目采用 [MIT License](LICENSE) 开源。

---

## 🔗 相关资源

- [AGENTS.md](AGENTS.md) - AI 编码助手开发指南
- [系统提示词注入使用说明](docs/系统提示词注入使用说明.md)
- [配置示例](src/config.example.yaml)

---

## 📮 联系方式

- 提交问题: [GitHub Issues](https://github.com/ococl/llm-proxy/issues)
- 讨论区: [GitHub Discussions](https://github.com/ococl/llm-proxy/discussions)

---

**项目状态**: 🚧 活跃开发中  
**最后更新**: 2026-01-18
