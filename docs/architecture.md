# LLM Proxy 架构设计文档

## 1. 系统概述

### 1.1 项目定位
LLM Proxy 是一个高性能、可扩展的 LLM API 代理服务，支持多后端路由、协议转换、流式响应和智能回退。

### 1.2 核心特性
- **多后端路由**: 支持配置多个 LLM 后端，按优先级和负载均衡策略路由
- **协议转换**: 自动在 OpenAI 和 Anthropic API 格式之间转换
- **智能回退**: 基于错误检测的自动回退机制，支持后端冷却
- **流式响应**: 完整的 SSE (Server-Sent Events) 流式传输支持
- **并发控制**: 全局和 per-IP 并发限制，防止过载
- **速率限制**: 多级速率限制 (全局/IP/模型)
- **系统提示注入**: 支持动态系统提示词注入和变量替换

### 1.3 技术栈
- **语言**: Go 1.21+
- **配置**: YAML
- **日志**: Zap (高性能结构化日志)
- **HTTP**: 标准库 net/http + 自定义连接池
- **并发**: Goroutines + Channels

---

## 2. 架构设计

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                        Client Request                        │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    Middleware Chain                          │
├─────────────────────────────────────────────────────────────┤
│  1. Recovery (Panic Handler)                                │
│  2. Rate Limiter (Global/IP/Model)                          │
│  3. Concurrency Limiter (Queue + Max Concurrent)            │
│  4. Auth Validator (API Key Check)                          │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                      Proxy Handler                           │
├─────────────────────────────────────────────────────────────┤
│  • Request Protocol Detection (OpenAI/Anthropic)            │
│  • Model Alias Resolution (Router)                          │
│  • System Prompt Injection                                  │
│  • Request Body Processing                                  │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    Backend Selection                         │
├─────────────────────────────────────────────────────────────┤
│  • Priority-based Routing                                   │
│  • Cooldown Check (Failed backend skip)                    │
│  • Protocol Conversion (if needed)                          │
└───────────────────────────┬─────────────────────────────────┘
                            │
                ┌───────────┴───────────┐
                ▼                       ▼
┌───────────────────────┐   ┌───────────────────────┐
│   Backend A (OpenAI)  │   │ Backend B (Anthropic) │
└───────────┬───────────┘   └───────────┬───────────┘
            │                           │
            └───────────┬───────────────┘
                        ▼
        ┌───────────────────────────┐
        │  Response Processing      │
        ├───────────────────────────┤
        │  • Error Detection        │
        │  • Fallback Triggering    │
        │  • Protocol Conversion    │
        │  • Stream/Non-stream      │
        └───────────┬───────────────┘
                    │
                    ▼
        ┌───────────────────────────┐
        │    Client Response        │
        └───────────────────────────┘
```

### 2.2 目录结构

```
llm-proxy/
├── src/
│   ├── main.go                    # 应用入口
│   ├── auth/                      # 认证模块
│   │   └── validator.go           # API Key 验证
│   ├── backend/                   # 后端管理
│   │   └── cooldown.go            # 后端冷却管理
│   ├── config/                    # 配置管理
│   │   ├── config.go              # 配置结构定义
│   │   └── manager.go             # 配置热重载管理
│   ├── errors/                    # 错误处理
│   │   └── errors.go              # 统一错误定义和响应
│   ├── logging/                   # 日志模块
│   │   └── logger.go              # Zap 日志封装
│   ├── middleware/                # 中间件
│   │   ├── recovery.go            # Panic 恢复
│   │   ├── ratelimit.go           # 速率限制
│   │   └── concurrency.go         # 并发控制
│   ├── prompt/                    # 系统提示管理
│   │   └── system_prompt.go       # 提示词注入逻辑
│   └── proxy/                     # 核心代理逻辑
│       ├── proxy.go               # 主代理处理器
│       ├── router.go              # 路由解析
│       ├── detector.go            # 错误检测
│       ├── protocol.go            # 协议转换
│       ├── request_detector.go    # 请求协议检测
│       └── httpclient.go          # HTTP 客户端配置
├── configs/
│   └── config.yaml                # 主配置文件
├── logs/                          # 日志目录
└── docs/
    └── architecture.md            # 本文档
```

---

## 3. 核心模块设计

### 3.1 配置管理 (config/)

**职责**: 
- 加载和解析 YAML 配置
- 支持配置热重载 (SIGHUP)
- 提供线程安全的配置访问

**关键组件**:

