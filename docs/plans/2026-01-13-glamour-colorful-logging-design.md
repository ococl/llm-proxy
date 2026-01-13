# Glamour 彩色日志改造设计

## 概述

引入 [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss) 库，为 LLM Proxy 实现控制台彩色日志输出，提升日志可读性和用户体验。

> 注：glamour 主要用于 Markdown 渲染，日志着色使用 lipgloss 更轻量高效。

## 需求确认

| 需求项 | 选择 |
|--------|------|
| 日志级别颜色 | 主题配色（Dracula） |
| 文件日志格式 | 控制台彩色 + 文件纯文本 |
| 启动画面 | 添加 ASCII 艺术字横幅 |
| 请求 ID 显示 | 高亮显示 `[req_xxx]` |

## 设计方案

### 1. 依赖引入

```bash
cd src && go get github.com/charmbracelet/lipgloss
```

### 2. 日志格式对比

**当前格式（纯文本）：**
```
[2026-01-13 09:41:00] [INFO] LLM Proxy 启动，监听地址: :8080
[2026-01-13 09:41:05] [INFO] [req_abc123] 收到请求: 模型=anthropic/claude-opus-4-5
```

**改造后（控制台）：**
```
2026-01-13 09:41:00  INFO   LLM Proxy 启动，监听地址: :8080
2026-01-13 09:41:05  INFO   [req_abc123] 收到请求: 模型=anthropic/claude-opus-4-5
                            ↑ 青色高亮    ↑ 绿色
```

**文件日志保持不变：**
```
[2026-01-13 09:41:00] [INFO] 消息内容
```

### 3. 配色方案（Dracula 主题）

| 级别 | 颜色 | Hex | 使用场景 |
|------|------|-----|----------|
| ERROR | 红色 | `#FF5555` | 后端失败、配置错误 |
| WARN | 橙色 | `#FFB86C` | API Key 验证失败、后端返回错误 |
| INFO | 绿色 | `#50FA7B` | 启动信息、请求成功 |
| DEBUG | 紫色 | `#BD93F9` | 路由解析、调试信息 |
| 请求ID | 青色 | `#8BE9FD` | `[req_xxx]` 高亮 |
| 时间戳 | 灰色 | `#6272A4` | 时间信息 |

### 4. 启动横幅

```
╦  ╦  ╔╦╗  ╔═╗┬─┐┌─┐─┐ ┬┬ ┬
║  ║  ║║║  ╠═╝├┬┘│ │┌┴┬┘└┬┘
╩═╝╩═╝╩ ╩  ╩  ┴└─└─┘┴ └─ ┴ 

Version: 1.0.0
Listen:  :8080
Backends: 4 loaded
Models:   13 aliases
```

### 5. 配置扩展

在 `logging` 配置段添加：

```yaml
logging:
  # ... 现有配置 ...
  colorize: true           # 控制台彩色输出（默认 true）
```

## 技术实现细节

### TTY 检测实现

```go
import (
    "os"
    "github.com/mattn/go-isatty"
)

func shouldUseColor() bool {
    // 配置强制禁用
    if loggingConfig != nil && loggingConfig.Colorize != nil && !*loggingConfig.Colorize {
        return false
    }
    // 检测是否为终端
    return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}
```

> 备选方案：使用 `golang.org/x/term` 的 `term.IsTerminal()`

### 请求 ID 高亮正则

```go
var reqIDPattern = regexp.MustCompile(`\[req_[a-zA-Z0-9]+\]`)

func highlightRequestID(msg string) string {
    if !shouldUseColor() {
        return msg
    }
    return reqIDPattern.ReplaceAllStringFunc(msg, func(match string) string {
        return reqIDStyle.Render(match)
    })
}
```

### lipgloss 样式定义

```go
import "github.com/charmbracelet/lipgloss"

var (
    // Dracula 主题颜色
    colorError   = lipgloss.Color("#FF5555")
    colorWarn    = lipgloss.Color("#FFB86C")
    colorInfo    = lipgloss.Color("#50FA7B")
    colorDebug   = lipgloss.Color("#BD93F9")
    colorReqID   = lipgloss.Color("#8BE9FD")
    colorTime    = lipgloss.Color("#6272A4")

    // 样式定义
    errorStyle = lipgloss.NewStyle().Foreground(colorError).Bold(true)
    warnStyle  = lipgloss.NewStyle().Foreground(colorWarn).Bold(true)
    infoStyle  = lipgloss.NewStyle().Foreground(colorInfo)
    debugStyle = lipgloss.NewStyle().Foreground(colorDebug)
    reqIDStyle = lipgloss.NewStyle().Foreground(colorReqID)
    timeStyle  = lipgloss.NewStyle().Foreground(colorTime)
)

func colorLevel(level string) string {
    switch strings.ToUpper(level) {
    case "ERROR":
        return errorStyle.Render("ERROR")
    case "WARN":
        return warnStyle.Render("WARN ")
    case "INFO":
        return infoStyle.Render("INFO ")
    case "DEBUG":
        return debugStyle.Render("DEBUG")
    default:
        return level
    }
}
```

## 实施步骤

### 步骤 1：添加依赖

```bash
cd src && go get github.com/charmbracelet/lipgloss
cd src && go get github.com/mattn/go-isatty
```

### 步骤 2：创建颜色工具模块

新建 `src/colors.go`：

| 函数 | 说明 |
|------|------|
| `shouldUseColor() bool` | TTY 检测 + 配置检查 |
| `colorLevel(level string) string` | 日志级别着色 |
| `colorTime(t string) string` | 时间戳着色 |
| `highlightRequestID(msg string) string` | 请求 ID 高亮 |
| `printBanner(version, listen string, backends, models int)` | 启动横幅 |

### 步骤 3：修改 logger.go

修改 `logInternal()` 函数（第 133-169 行）：

```go
func logInternal(level string, target LogTarget, format string, args ...interface{}) {
    // ... 现有逻辑 ...

    msg := fmt.Sprintf(format, args...)
    if maskSensitive {
        msg = MaskSensitiveData(msg)
    }

    // 文件日志：保持纯文本格式
    fileLine := fmt.Sprintf("[%s] [%s] %s\n", 
        time.Now().Format("2006-01-02 15:04:05"), 
        strings.ToUpper(level), 
        msg)

    // 控制台日志：彩色格式
    if shouldLogConsole {
        if shouldUseColor() {
            consoleLine := fmt.Sprintf("%s  %s  %s\n",
                colorTime(time.Now().Format("2006-01-02 15:04:05")),
                colorLevel(level),
                highlightRequestID(msg))
            fmt.Print(consoleLine)
        } else {
            fmt.Print(fileLine)
        }
    }

    if shouldLogFile && generalLogger != nil {
        // ... 文件写入逻辑不变 ...
        generalLogger.WriteString(fileLine)
    }
}
```

### 步骤 4：修改 config.go

在 `Logging` 结构体（第 53 行）添加字段：

```go
type Logging struct {
    // ... 现有字段 ...
    Colorize      *bool  `yaml:"colorize,omitempty"`      // 新增
}
```

### 步骤 5：修改 main.go

在 `main()` 函数启动日志前添加横幅：

```go
func main() {
    // ... 配置加载 ...

    // 打印启动横幅
    printBanner(Version, cfg.GetListen(), len(cfg.Backends), len(cfg.Models))

    // 原有启动日志
    LogGeneral("INFO", "LLM Proxy %s", Version)
    // ...
}
```

### 步骤 6：更新配置示例

修改 `src/config.example.yaml`（第 48-56 行）：

```yaml
logging:
  level: "info"
  colorize: true              # 新增：控制台彩色输出
  general_file: "./logs/proxy.log"
  # ... 其他配置 ...
```

### 步骤 7：测试验证

| 测试项 | 验证方法 |
|--------|----------|
| 单元测试通过 | `cd src && go test -v ./...` |
| 测试模式无输出 | `SetTestMode(true)` 时无颜色代码 |
| 文件日志纯文本 | 检查日志文件无 ANSI 转义码 |
| 非 TTY 自动禁用 | `go run . 2>&1 \| cat` 无颜色 |
| 配置禁用生效 | `colorize: false` 时无颜色 |

## 文件变更清单

| 文件 | 操作 | 变更说明 |
|------|------|----------|
| `src/go.mod` | 修改 | 添加 lipgloss、go-isatty 依赖 |
| `src/colors.go` | **新建** | 颜色工具函数模块 |
| `src/logger.go` | 修改 | 集成彩色输出（第 133-169 行） |
| `src/config.go` | 修改 | Logging 结构体添加 Colorize 字段（第 53 行） |
| `src/main.go` | 修改 | 添加启动横幅调用（第 52 行前） |
| `src/config.example.yaml` | 修改 | 添加 colorize 配置示例（第 49 行） |

## 涉及 LogGeneral 调用的文件

| 文件 | 调用次数 | 说明 |
|------|----------|------|
| `proxy.go` | 15 | 请求处理、错误日志 |
| `router.go` | 5 | 路由解析、回退日志 |
| `config.go` | 2 | 配置重载日志 |
| `main.go` | 3 | 启动信息 |
| `backend.go` | 1 | 冷却设置日志 |
| `logger.go` | 1 | 性能指标日志 |

> 这些文件无需修改，颜色由 `logInternal()` 统一处理。

## 兼容性考虑

1. **TTY 检测**：使用 `go-isatty` 库，支持 Windows/Linux/macOS
2. **配置覆盖**：`colorize: false` 可强制禁用
3. **测试模式**：`SetTestMode(true)` 时跳过所有输出
4. **Windows 支持**：lipgloss 自动处理 Windows 终端兼容性
5. **向后兼容**：文件日志格式不变，不影响现有日志解析工具

## 预期效果

- ✅ 控制台日志按级别着色，提升可读性
- ✅ 请求 ID 高亮，便于追踪
- ✅ 启动时显示美观的横幅
- ✅ 文件日志保持纯文本，便于解析
- ✅ 自动适配终端环境
