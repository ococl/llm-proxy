# LLM Proxy 设计文档

## 概述

一个轻量级的 LLM API 代理服务器，支持多提供商负载均衡、自动回退和异常检测。

## 设计决策

| 决策项 | 选择 |
|--------|------|
| 负载均衡 | 优先级模式 |
| 模型映射 | 多对多 + 每个别名独立优先级 |
| 冷却粒度 | 后端+模型级 |
| URL映射 | 完全覆盖式（url 包含完整基础路径） |
| 配置结构 | 别名为中心 |
| 未知模型 | 拒绝请求 (400) |
| API Key | 可选记录，不使用 |
| SSE | 支持流式透传 |
| 日志 | 请求独立文件 + 异常独立 + 常规日志 |
| 热加载 | 每次请求检查，失败保留旧配置 |
| 协议 | 仅HTTP |

## 配置结构

```yaml
listen: ":8080"

backends:
  - name: "oocc"
    url: "https://api.oocc.com/v1"
    api_key: "sk-oocc-xxx"              # 可选
    
  - name: "minimax"
    url: "https://api.minimax.com/api"
    api_key: "sk-minimax-yyy"
    
  - name: "x-aio"
    url: "https://api.x-aio.com/v1"

models:
  "claude-opus":
    - backend: "oocc"
      model: "claude-opus-4"
      priority: 1
    - backend: "minimax"
      model: "abab6.5-chat"
      priority: 2
    - backend: "x-aio"
      model: "claude-latest"
      priority: 3

fallback:
  cooldown_seconds: 300
  max_retries: 3

detection:
  error_codes: [402, 429, 500, 502, 503, 504]
  error_patterns:
    - "insufficient_quota"
    - "rate_limit"
    - "exceeded"
    - "billing"

logging:
  level: "debug"
  request_dir: "./logs/requests"
  error_dir: "./logs/errors"
  general_file: "./logs/proxy.log"
```

## 请求处理流程

1. 收到请求 → 生成请求ID，创建日志文件
2. 解析 model → 查找别名，未找到返回 400
3. 选择后端 → 按优先级，跳过冷却中的
4. 转发请求 → 替换URL和model，透传headers
5. 处理响应 → 成功透传，失败检测回退

## 日志系统

| 类型 | 路径 | 内容 |
|------|------|------|
| 请求日志 | logs/requests/{ts}_{uuid}.log | 完整请求 |
| 异常日志 | logs/errors/{ts}_{uuid}.log | 失败详情 |
| 常规日志 | logs/proxy.log | 启动、路由等 |

## 配置热加载

- 每次请求检查文件修改时间
- 变化则重新加载，失败保留旧配置
- listen 端口变更需重启
- 冷却状态在热加载后保留

## 项目结构

```
llm-proxy/
├── main.go
├── config.go
├── router.go
├── backend.go
├── proxy.go
├── detector.go
├── logger.go
├── config.yaml
└── logs/
```
