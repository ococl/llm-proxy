# LLM Proxy - AI 代理协作指南

本文档为 AI 编码代理提供项目开发指南。

## 项目概述

LLM Proxy 是一个轻量级 LLM API 代理服务器，使用 Go 语言编写，支持多提供商负载均衡和自动回退。

## 构建/测试命令

```bash
# 运行所有测试
cd src && go test -v ./...

# 运行单个测试文件（按文件名匹配）
cd src && go test -v -run TestRouter ./...

# 运行特定测试函数
cd src && go test -v -run TestRouter_Resolve_Basic ./...

# 运行带覆盖率的测试
cd src && go test -v -cover ./...

# 本地开发构建
cd src && go build -o ../dist/llm-proxy.exe .

# Release 构建（去除调试信息）
cd src && go build -ldflags "-s -w" -o ../dist/llm-proxy.exe .

# 多平台构建
make build-all          # Linux/macOS
.\build.ps1 all         # Windows PowerShell

# 依赖管理
cd src && go mod tidy
```

## 项目结构

```
llm-proxy/
├── src/                    # 源代码目录（go.mod 在此）
│   ├── main.go             # 入口点
│   ├── config.go           # 配置管理和结构体定义
│   ├── proxy.go            # HTTP 代理处理
│   ├── router.go           # 路由解析和负载均衡
│   ├── backend.go          # 后端冷却管理
│   ├── detector.go         # 错误检测
│   ├── logger.go           # 日志系统
│   └── *_test.go           # 单元测试
├── dist/                   # 构建输出
└── Makefile                # 构建脚本
```

## 代码风格指南

### 导入顺序

```go
import (
    // 1. 标准库
    "fmt"
    "net/http"
    "sync"
    
    // 2. 外部依赖（空行分隔）
    "github.com/google/uuid"
    "gopkg.in/yaml.v3"
)
```

### 命名约定

| 类型 | 约定 | 示例 |
|------|------|------|
| 导出类型/函数 | PascalCase | `ConfigManager`, `NewProxy()` |
| 未导出类型/函数 | camelCase | `configPath`, `tryReload()` |
| 常量 | PascalCase | `MaxRetries` |
| 测试函数 | `Test<Type>_<Method>_<Scenario>` | `TestRouter_Resolve_Basic` |

### 结构体定义

```go
// 使用 yaml tag 定义配置字段
// 使用指针实现可选布尔字段（nil = 默认 true）
type Backend struct {
    Name    string `yaml:"name"`
    URL     string `yaml:"url"`
    APIKey  string `yaml:"api_key,omitempty"`
    Enabled *bool  `yaml:"enabled,omitempty"`
}

// 为可选布尔字段提供默认值方法
func (b *Backend) IsEnabled() bool {
    return b.Enabled == nil || *b.Enabled
}
```

### 错误处理

```go
// 正确：返回错误并记录日志
if err != nil {
    LogGeneral("ERROR", "操作失败: %v", err)
    return nil, err
}

// 正确：HTTP 错误响应
if modelAlias == "" {
    LogGeneral("WARN", "[%s] 请求缺少 model 字段", reqID)
    http.Error(w, "缺少 model 字段", http.StatusBadRequest)
    return
}

// 禁止：空的错误处理块
// if err != nil { }
```

### 日志规范

```go
// 使用 LogGeneral 统一日志，日志消息使用中文
LogGeneral("DEBUG", "调试信息: %s", detail)
LogGeneral("INFO", "业务事件: %s", event)
LogGeneral("WARN", "潜在问题: %s", warning)
LogGeneral("ERROR", "严重错误: %v", err)

// 包含请求 ID 便于追踪
LogGeneral("INFO", "[%s] 收到请求: 模型=%s", reqID, model)
```

### 测试规范

```go
// 使用 TestMain 启用测试模式（禁用日志输出）
func TestMain(m *testing.M) {
    SetTestMode(true)
    os.Exit(m.Run())
}

// 表驱动测试
func TestDetector_MatchStatusCode(t *testing.T) {
    tests := []struct {
        name     string
        code     int
        expected bool
    }{
        {"4xx match", 404, true},
        {"2xx no match", 200, false},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := d.ShouldFallback(tt.code, "")
            if got != tt.expected {
                t.Errorf("got %v, want %v", got, tt.expected)
            }
        })
    }
}
```

### 并发安全

```go
// 使用 sync.RWMutex 保护共享状态
type CooldownManager struct {
    cooldowns map[CooldownKey]time.Time
    mu        sync.RWMutex
}

func (cm *CooldownManager) IsCoolingDown(key CooldownKey) bool {
    cm.mu.RLock()
    defer cm.mu.RUnlock()
    // ...
}
```

## 关键模式

- **配置热重载**: `ConfigManager.Get()` 自动检测文件变更并重载
- **多级回退**: L1 别名内后端优先级回退，L2 跨别名回退（`alias_fallback`）
- **负载均衡**: 同优先级路由使用 `rand.Shuffle` 随机选择

## 禁止事项

- 空的错误处理块
- 删除失败的测试来"通过"构建
- 未经请求自动提交代码
- 在 `.gitignore` 中忽略 `go.sum`
- 硬编码敏感信息（API Key 等）

## CI/CD

- **main 分支**: 测试 + 构建（产物可下载）
- **v* 标签**: 测试 + 构建 + 发布 Release
