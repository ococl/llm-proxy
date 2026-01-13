# LLM Proxy

轻量级 LLM API 代理服务器，支持多提供商负载均衡、自动回退和异常检测。

## 功能特性

- **多对多模型别名**：统一不同提供商的模型命名
- **优先级路由**：每个别名独立配置后端优先级
- **自动回退**：后端失败时自动切换到下一优先级
- **冷却机制**：失败后端自动冷却 5 分钟
- **完全透传**：Headers、Body 完全透传，绕过转发检测
- **SSE 流式**：支持流式响应实时转发
- **配置热加载**：修改配置后下次请求自动生效
- **详细日志**：每个请求独立日志文件

## 快速开始

```bash
# 1. 进入 dist 目录
cd dist

# 2. 复制示例配置
cp config.example.yaml config.yaml

# 3. 编辑配置（添加你的后端和模型映射）
# vim config.yaml

# 4. 启动代理
./llm-proxy.exe -config config.yaml
```

## 配置说明

```yaml
listen: ":8080"

# 后端定义
backends:
  - name: "provider-a"
    url: "https://api.provider-a.com/v1"  # 完整基础路径
    api_key: "sk-xxx"                      # 可选，仅记录用途

# 模型别名（多对多映射）
models:
  "claude-opus":                           # 本地别名（客户端使用）
    - backend: "provider-a"
      model: "claude-opus-4"               # 该后端的实际模型名
      priority: 1                          # 优先级（数字越小越优先）
    - backend: "provider-b"
      model: "abab6.5-chat"
      priority: 2

# 回退配置
fallback:
  cooldown_seconds: 300                    # 冷却时间（秒）
  max_retries: 3                           # 单次请求最大尝试次数

# 异常检测
detection:
  error_codes: [402, 429, 500, 502, 503, 504]
  error_patterns:
    - "insufficient_quota"
    - "rate_limit"

# 日志配置
logging:
  level: "debug"                           # debug/info/warn/error
  request_dir: "./logs/requests"
  error_dir: "./logs/errors"
  general_file: "./logs/proxy.log"
```

## 客户端使用

```bash
# 使用本地别名请求
curl http://localhost:8080/chat/completions \
  -H "Authorization: Bearer sk-your-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-opus",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": true
  }'
```

代理会自动：
1. 将 `claude-opus` 解析为后端实际模型名
2. 按优先级选择可用后端
3. 失败时自动回退到下一个后端

## 目录结构

```
llm-proxy/
├── src/                    # 源代码
├── dist/                   # 构建产物
│   ├── llm-proxy.exe
│   ├── config.example.yaml
│   └── logs/               # 运行时生成
├── docs/                   # 设计文档
├── .editorconfig
├── .gitignore
└── README.md
```

## 构建

```bash
cd src
go build -o ../dist/llm-proxy.exe .
```

## 日志

| 类型 | 路径 | 内容 |
|------|------|------|
| 请求日志 | `logs/requests/` | 完整请求（headers, body） |
| 异常日志 | `logs/errors/` | 失败详情 |
| 常规日志 | `logs/proxy.log` | 启动、路由信息 |

## License

MIT
