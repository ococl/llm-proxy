# AGENTS.md - AI 编码助手指南

本文档为 AI 编码助手提供项目开发规范和命令参考。

---

## 📦 项目概览

**llm-proxy** 是一个高性能的 LLM API 代理服务,提供负载均衡、故障转移、限流、并发控制等企业级功能。

- **语言**: Go 1.25.5
- **架构**: 分层中间件 + 代理模式
- **主要模块**: proxy, router, middleware, config, backend, logging

---

## 🛠️ 构建与测试命令

### 开发构建
```bash
# 快速开发构建(当前平台)
make dev

# 完整多平台构建
make build-all

# 清理构建产物
make clean
```

### 测试命令
```bash
# 运行所有测试
make test
# 等同于: cd src && go test -v ./...

# 运行指定包的测试
cd src && go test -v ./proxy
cd src && go test -v ./config

# 运行单个测试用例
cd src && go test -v -run TestDetector_EmptyConfig ./proxy
cd src && go test -v -run TestFallback_L2 ./proxy

# 运行测试并显示覆盖率
cd src && go test -v -cover ./...

# 生成覆盖率报告
cd src && go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 代码检查
```bash
# 代码格式化(自动修复)
cd src && gofmt -s -w .

# 静态分析
cd src && go vet ./...

# 依赖管理
cd src && go mod tidy
cd src && go mod verify
```

---

## 📐 代码风格指南

### 格式化
- **缩进**: Tab (4 空格显示宽度)
- **YAML 文件**: 2 空格缩进
- **行尾**: LF (Unix 风格)
- **文件结尾**: 必须有空行
- **工具**: 使用 `gofmt -s` 格式化

### 导入规范
```go
import (
	// 1. 标准库
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	
	// 2. 本项目包 (使用 llm-proxy/ 前缀)
	"llm-proxy/backend"
	"llm-proxy/config"
	"llm-proxy/errors"
	"llm-proxy/logging"
	
	// 3. 第三方库
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)
```

### 命名约定
- **包名**: 小写单词,无下划线 (`proxy`, `config`, `middleware`)
- **导出**: 首字母大写 (`type Proxy struct`, `func NewProxy()`)
- **私有**: 首字母小写 (`func isHopByHopHeader()`)
- **接口**: 名词或形容词 (`type Manager interface`)
- **常量**: 驼峰命名 (`const maxRetries = 3`)

### 类型定义
```go
// ✅ 推荐: 显式字段类型,YAML 标签清晰
type Backend struct {
	Name    string `yaml:"name"`
	URL     string `yaml:"url"`
	APIKey  string `yaml:"api_key,omitempty"`
	Enabled *bool  `yaml:"enabled,omitempty"` // 使用指针区分零值和未设置
}

// ✅ 推荐: 为配置项提供默认值获取方法
func (b *Backend) IsEnabled() bool {
	return b.Enabled == nil || *b.Enabled
}
```

### 错误处理
```go
// ✅ 标准错误检查模式
resp, err := client.Do(proxyReq)
if err != nil {
	logging.ProxySugar.Errorw("请求失败", "error", err, "backend", route.BackendName)
	continue // 故障转移到下一个后端
}
defer resp.Body.Close()

// ✅ 使用自定义错误类型 (见 src/errors/errors.go)
errors.WriteJSONError(w, errors.ErrNoBackend, http.StatusBadGateway, traceID)

// ❌ 避免: 忽略错误
io.ReadAll(resp.Body) // 缺少错误检查

// ❌ 避免: 过度嵌套
if err == nil {
	if data != nil {
		// 处理
	}
}
// ✅ 推荐: 提前返回
if err != nil {
	return err
}
if data == nil {
	return errors.New("data is nil")
}
// 处理正常路径
```

### 日志记录
```go
// ✅ 使用结构化日志 (go.uber.org/zap)
logging.ProxySugar.Infow("请求成功",
	"trace_id", traceID,
	"backend", route.BackendName,
	"model", route.Model,
	"status", resp.StatusCode,
	"duration_ms", time.Since(start).Milliseconds(),
)

// ✅ 错误日志包含上下文
logging.ProxySugar.Errorw("路由解析失败",
	"error", err,
	"model", model,
	"trace_id", traceID,
)

// ❌ 避免: 非结构化日志
log.Println("请求成功 backend=" + backend)
```

---

## 🏗️ 架构模式

### 中间件链 (见 src/main.go:100)
```
请求流 → RecoveryMiddleware → RateLimiter → ConcurrencyLimiter → Proxy
```

### 故障转移逻辑 (见 src/proxy/proxy.go:141-311)
1. **L1 回退**: 同模型别名内多后端重试 (按优先级)
2. **L2 回退**: 跨模型别名回退 (通过 `alias_fallback` 配置)
3. **冷却机制**: 失败后端进入冷却期 (默认 300 秒)
4. **错误检测**: 通过 HTTP 状态码和响应体模式触发回退

### 配置热重载 (见 src/config/config.go:301-349)
- 每次 `Get()` 检查文件修改时间
- 检测到变化时自动重新加载
- 日志配置变更会触发回调 (`LoggingConfigChangedFunc`)

---

## 🔍 关键组件说明

### 1. Proxy (src/proxy/proxy.go)
- **入口**: `ServeHTTP()` - 处理所有 HTTP 请求
- **核心流程**:
  1. API Key 验证 (line 64-72)
  2. 请求体解析和系统提示词注入 (line 81-96)
  3. 模型路由解析 (line 107)
  4. 多后端重试循环 (line 141-311)
  5. 响应流式/非流式处理 (line 253-277)
- **已知问题**: HTTP 客户端超时硬编码为 5 分钟,`TimeoutConfig` 未生效

### 2. Router (src/proxy/router.go)
- **路由解析**: `Resolve()` - 将模型别名映射到后端列表
- **负载均衡**: 同优先级后端随机打散 (line 50-59)
- **L2 回退**: `ResolutionWithFallback()` - 收集跨别名回退路由

### 3. Middleware
- **限流** (src/middleware/ratelimit.go): Token Bucket 算法,支持全局/IP/模型级限流
- **并发** (src/middleware/concurrency.go): 基于 channel 的信号量,支持队列超时
- **恢复** (src/middleware/recovery.go): Panic 捕获和恢复

### 4. Detector (src/proxy/detector.go)
- **错误检测**: `ShouldFallback()` - 根据 HTTP 状态码和响应体判断是否回退
- **通配符支持**: `4xx`, `5xx` 匹配整个状态码范围
- **默认规则**: 未配置时默认 `["4xx", "5xx"]`

---

## ⚠️ 已知问题与注意事项

1. **超时配置未生效** (src/proxy/proxy.go:208)
   - 当前硬编码 5 分钟: `client := &http.Client{Timeout: 5 * time.Minute}`
   - `TimeoutConfig` 结构体已定义但未应用到 HTTP 客户端
   - 缺少 `http.Transport` 配置 (连接池、TLS 超时等)

2. **HTTP 客户端效率**
   - 每个请求创建新客户端,无连接池复用
   - 建议改用单例客户端 + 自定义 Transport

3. **测试覆盖**
   - 主要模块有单元测试 (detector, router, fallback)
   - 缺少集成测试和端到端测试

---

## 🤖 工具调用规范

**重要**: 在调用任何工具时,必须严格遵循工具列表中的参数命名和说明,切勿臆测参数名称或类型!

### 规范要求

1. **精确参数匹配**
   - 使用工具前,仔细阅读工具描述中的参数定义
   - 参数名称必须与文档完全一致 (区分大小写)
   - 参数类型必须匹配 (string, int, bool, array, object 等)

2. **必填参数检查**
   - 确保所有 `required` 参数都已提供
   - 不要遗漏必填字段,也不要添加不存在的字段

3. **可选参数理解**
   - 可选参数有默认值时,了解默认行为
   - 不确定的参数不要随意传值

### 示例

```go
// ❌ 错误: 参数名称错误
ant_cc_bash(cmd="ls", timeout=5000)  // 应该是 command 而非 cmd

// ✅ 正确: 严格按照文档
ant_cc_bash(command="ls", description="List files", timeout=5000)

// ❌ 错误: 参数类型错误
ant_cc_read(filePath="test.go", line=10)  // line 不是该工具的参数

// ✅ 正确: 只使用定义的参数
ant_cc_read(filePath="test.go", offset=10, limit=50)
```

### 违规后果

- 工具调用失败,浪费 API 调用次数
- 延长任务完成时间
- 可能产生不可预测的行为

**牢记**: 工具文档是权威来源,永远以文档为准!

---

## 📝 提交指南

### 提交消息格式
```
类型(范围): 简短描述

详细说明(可选)

关联问题: #123
```

**类型**: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`  
**范围**: `proxy`, `router`, `config`, `middleware`, `logging`

### 示例
```
fix(proxy): 修复 HTTP 客户端超时配置未生效

- 应用 TimeoutConfig 到 http.Transport
- 添加连接池配置 (MaxIdleConns=100)
- 设置 IdleConnTimeout 为 90 秒

关联问题: #42
```

---

## 🧪 测试编写指南

```go
// 测试命名: Test<功能>_<场景>
func TestDetector_MatchStatusCode_Wildcard(t *testing.T) {
	// 1. 准备测试数据
	d := newDetectorWithConfig([]string{"4xx", "5xx"}, nil)
	
	// 2. 定义测试用例 (表格驱动测试)
	tests := []struct {
		name     string
		code     int
		expected bool
	}{
		{"400 Bad Request", 400, true},
		{"500 Internal Error", 500, true},
		{"200 OK", 200, false},
	}
	
	// 3. 遍历执行
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := d.ShouldFallback(tt.code, "")
			if got != tt.expected {
				t.Errorf("期望 %v, 实际 %v", tt.expected, got)
			}
		})
	}
}
```

---

## 📚 参考资料

- [Go 代码审查建议](https://github.com/golang/go/wiki/CodeReviewComments)
- [Effective Go](https://go.dev/doc/effective_go)
- [Uber Go 风格指南](https://github.com/uber-go/guide/blob/master/style.md)

---

**最后更新**: 2026-01-18  
**项目版本**: 根据 git tag 自动生成
