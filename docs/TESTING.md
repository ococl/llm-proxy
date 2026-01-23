# Testing Guide

LLM-Proxy 项目的测试指南，包含单元测试、集成测试和端到端测试的完整说明。

---

## 📋 目录

- [快速开始](#快速开始)
- [测试类型](#测试类型)
- [端到端测试](#端到端测试)
- [单元测试](#单元测试)
- [测试覆盖率](#测试覆盖率)
- [CI/CD 集成](#cicd-集成)

---

## 🚀 快速开始

### 运行所有测试

```bash
cd src
go test ./...
```

### 快速验证（构建 + 简单 E2E 测试）

```powershell
# Windows
.\scripts\quick-test.ps1

# 跳过构建，使用已有二进制
.\scripts\quick-test.ps1 -SkipBuild
```

```bash
# Linux/macOS
./scripts/quick-test.sh
```

### 完整端到端测试

```powershell
# Windows - 运行所有测试
.\scripts\e2e-test.ps1

# 仅运行健康检查
.\scripts\e2e-test.ps1 -HealthCheck

# 仅测试正常请求
.\scripts\e2e-test.ps1 -NormalRequest

# 仅测试流式请求
.\scripts\e2e-test.ps1 -StreamingRequest
```

```bash
# Linux/macOS
./scripts/e2e-test.sh
./scripts/e2e-test.sh --health-check
./scripts/e2e-test.sh --normal-request
./scripts/e2e-test.sh --streaming
```

---

## 🧪 测试类型

### 1. 单元测试 (Unit Tests)

测试单个函数或方法的行为，不依赖外部资源。

**位置**: `src/*_test.go`

**运行**:
```bash
cd src
go test ./domain/entity/...
go test ./domain/service/...
```

**示例**:
```go
func TestBackend_IsEnabled(t *testing.T) {
    backend := &entity.Backend{Enabled: true}
    if !backend.IsEnabled() {
        t.Error("Expected backend to be enabled")
    }
}
```

### 2. 集成测试 (Integration Tests)

测试多个组件协作的行为，可能涉及 HTTP 调用或文件 I/O。

**位置**: `src/adapter/*_test.go`, `src/application/*_test.go`

**运行**:
```bash
cd src
go test ./adapter/...
go test ./application/...
```

### 3. 端到端测试 (E2E Tests)

测试完整的用户场景，从 HTTP 请求到响应。

**脚本位置**: `scripts/e2e-test.ps1`, `scripts/e2e-test.sh`

**运行**:
```powershell
.\scripts\e2e-test.ps1
```

---

## 🌐 端到端测试

### 测试脚本功能

E2E 测试脚本会自动执行以下步骤：

1. **环境检查**
   - 验证二进制文件存在
   - 验证配置文件存在
   - 检查端口占用

2. **服务启动**
   - 使用 `dist/config.yaml` 启动服务
   - 等待服务就绪（最多 10 秒）

3. **健康检查测试**
   - GET `/health`
   - 验证服务状态和后端数量

4. **正常请求测试**
   - POST `/v1/chat/completions`
   - 测试多个模型（DeepSeek V3, GLM-4.7）
   - 验证响应格式和内容

5. **流式请求测试**
   - POST `/v1/chat/completions` (stream=true)
   - 验证 SSE 数据流
   - 统计数据块数量

6. **错误处理测试**
   - 测试无效模型请求
   - 验证错误码和错误消息

7. **日志验证**
   - 检查日志文件存在性
   - 显示最新日志内容

8. **服务停止**
   - 优雅关闭服务进程

### 测试配置

测试使用的配置文件：`dist/config.yaml`

关键配置项：
- **监听地址**: `:8765`
- **API Key**: `sk-aNbDRYsSMcbdVUptFyy9yWpeN6agx`
- **日志级别**: `debug`
- **后端数量**: 5 个
- **模型别名**: 14 个

### 测试结果示例

```
========================================
 Test Report
========================================
  Total tests: 5
  Passed: 5
  Failed: 0
  Pass rate: 100%

╔════════════════════════════════════════╗
║          All tests passed! ✓           ║
╚════════════════════════════════════════╝
```

### 自定义测试

#### 添加新测试用例

编辑 `scripts/e2e-test.ps1`：

```powershell
function Test-MyNewFeature {
    Write-Header "My New Feature Test"

    # 测试逻辑
    $result = Invoke-APIRequest -Endpoint "/my-endpoint" -Method "POST"

    if ($result.Success) {
        Write-Success "Test passed"
        return $true
    } else {
        Write-Failure "Test failed"
        return $false
    }
}

# 在 Start-E2ETest 函数中添加
$results["MyNewFeature"] = Test-MyNewFeature
```

#### 修改超时时间

```powershell
# 在脚本顶部修改
$HealthTimeout = 5000       # 健康检查超时
$RequestTimeout = 30000     # 普通请求超时
$StreamTimeout = 60000      # 流式请求超时
```

---

## 🔬 单元测试

### 测试覆盖的模块

#### Domain Layer
- **Entity**: `domain/entity/*_test.go`
  - Backend, Request, Response, Message
  - 实体创建、验证、构建器模式
- **Service**: `domain/service/*_test.go`
  - LoadBalancer, CooldownManager, CircuitBreaker
  - FallbackStrategy, WeightedLoadBalancer

#### Application Layer
- **UseCase**: `application/usecase/*_test.go`
  - ProxyRequestUseCase
  - 重试策略、负载均衡
- **Service**: `application/service/*_test.go`
  - ProtocolConverter

#### Adapter Layer
- **HTTP**: `adapter/http/*_test.go`
  - HealthHandler, ErrorPresenter
  - RecoveryMiddleware
- **Middleware**: `adapter/http/middleware/*_test.go`
  - RateLimiter, ConcurrencyLimiter
- **Config**: `adapter/config/*_test.go`
  - ConfigAdapter, BackendRepository

#### Infrastructure Layer
- **Config**: `infrastructure/config/*_test.go`
  - 配置加载、默认值、验证
- **Logging**: `infrastructure/logging/*_test.go`
  - 日志级别、脱敏、格式化

### 运行特定测试

```bash
# 运行单个测试文件
go test ./domain/entity/backend_test.go

# 运行单个测试用例
go test -run TestBackend_IsEnabled ./domain/entity

# 详细输出
go test -v ./domain/service

# 并行运行
go test -parallel 4 ./...
```

### 编写测试的最佳实践

#### 1. 使用表驱动测试

```go
func TestBackend_IsEnabled(t *testing.T) {
    tests := []struct {
        name    string
        enabled *bool
        want    bool
    }{
        {"nil (default true)", nil, true},
        {"explicit true", boolPtr(true), true},
        {"explicit false", boolPtr(false), false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            b := &entity.Backend{Enabled: tt.enabled}
            if got := b.IsEnabled(); got != tt.want {
                t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

#### 2. 使用 Mock 对象

```go
type MockLogger struct{}

func (m *MockLogger) Info(msg string, fields ...port.Field) {}
func (m *MockLogger) Error(msg string, fields ...port.Field) {}

func TestWithMock(t *testing.T) {
    service := NewService(&MockLogger{})
    // 测试逻辑
}
```

#### 3. 测试错误场景

```go
func TestBackend_InvalidURL(t *testing.T) {
    _, err := entity.NewBackend("test", "://invalid", "", "")
    if err == nil {
        t.Error("Expected error for invalid URL")
    }
}
```

---

## 📊 测试覆盖率

### 生成覆盖率报告

```bash
cd src

# 生成覆盖率文件
go test -coverprofile=coverage.out ./...

# 查看总体覆盖率
go tool cover -func=coverage.out

# 生成 HTML 报告
go tool cover -html=coverage.out -o coverage.html
```

### 按包查看覆盖率

```bash
go test -cover ./domain/entity
go test -cover ./domain/service
go test -cover ./application/usecase
```

### 覆盖率目标

| 层级 | 目标覆盖率 | 当前状态 |
|------|-----------|---------|
| Domain Entity | > 90% | ✅ 95% |
| Domain Service | > 85% | ✅ 92% |
| Application | > 80% | ✅ 85% |
| Adapter | > 75% | ✅ 80% |
| Infrastructure | > 70% | ✅ 75% |

---

## 🔄 CI/CD 集成

### GitHub Actions 配置示例

创建 `.github/workflows/test.yml`:

```yaml
name: Test

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Run tests
        run: |
          cd src
          go test -v -coverprofile=coverage.out ./...

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./src/coverage.out

  e2e-test:
    runs-on: ubuntu-latest
    needs: test
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Build binary
        run: |
          cd src
          go build -o ../dist/llm-proxy-latest .

      - name: Run E2E tests
        run: |
          chmod +x scripts/e2e-test.sh
          ./scripts/e2e-test.sh
```

### Makefile 集成

创建 `Makefile`:

```makefile
.PHONY: test test-unit test-e2e test-coverage

test: test-unit test-e2e

test-unit:
	cd src && go test -v ./...

test-e2e:
	./scripts/e2e-test.sh

test-coverage:
	cd src && go test -coverprofile=coverage.out ./...
	cd src && go tool cover -html=coverage.out -o coverage.html

quick-test:
	./scripts/quick-test.sh
```

---

## 🐛 故障排查

### 测试失败常见原因

#### 1. 端口占用

**症状**: "Service failed to start" 或 "Port 8765 already in use"

**解决方案**:
```powershell
# Windows
netstat -ano | findstr :8765
taskkill /PID <PID> /F

# Linux/macOS
lsof -ti:8765 | xargs kill -9
```

#### 2. 配置文件缺失

**症状**: "Config not found"

**解决方案**:
```bash
# 确保 dist/config.yaml 存在
ls dist/config.yaml

# 从示例创建
cp src/config.example.yaml dist/config.yaml
```

#### 3. 二进制文件过期

**症状**: 测试失败，但代码已修改

**解决方案**:
```bash
# 重新构建
cd src
go build -o ../dist/llm-proxy-latest.exe .

# 或使用脚本自动构建
./scripts/quick-test.ps1
```

#### 4. API Key 无效

**症状**: "Unauthorized" 或 401 错误

**解决方案**:
```yaml
# 检查 dist/config.yaml
proxy_api_key: "sk-aNbDRYsSMcbdVUptFyy9yWpeN6agx"
```

#### 5. 超时错误

**症状**: "Request timeout" 或 "Context deadline exceeded"

**解决方案**:
```powershell
# 增加超时时间
$RequestTimeout = 60000  # 改为 60 秒
```

---

## 📝 日志分析

### 查看测试日志

测试运行时会生成日志文件：

```
logs/
├── general.log         # 通用日志
├── requests/           # 请求日志
│   └── request.log
└── errors/             # 错误日志
    └── error.log
```

### 分析日志内容

```bash
# 查看最新请求
tail -f logs/requests/request.log

# 查看错误日志
cat logs/errors/error.log

# 搜索特定 trace_id
grep "trace_id=req_xxx" logs/general.log

# 统计错误数量
grep -c "ERROR" logs/general.log
```

### 日志级别说明

| 级别 | 用途 | 示例 |
|------|------|------|
| DEBUG | 详细调试信息 | "backend selected", "routes resolved" |
| INFO | 重要事件 | "proxy request started", "request completed" |
| WARN | 警告信息 | "all backends in cooldown", "retry exceeded" |
| ERROR | 错误事件 | "route resolution failed", "backend error" |

---

## 🎯 性能测试

### 基准测试

```bash
cd src

# 运行所有基准测试
go test -bench=. -benchmem ./...

# 运行特定基准测试
go test -bench=BenchmarkLoadBalancer -benchmem ./domain/service
```

### 压力测试

使用 `wrk` 或 `hey` 工具：

```bash
# 安装 hey
go install github.com/rakyll/hey@latest

# 压力测试
hey -n 10000 -c 100 -m POST \
  -H "Authorization: Bearer sk-aNbDRYsSMcbdVUptFyy9yWpeN6agx" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek/deepseek-v3.2","messages":[{"role":"user","content":"test"}],"max_tokens":10}' \
  http://localhost:8765/v1/chat/completions
```

---

## 📚 参考资源

- [Go Testing 官方文档](https://golang.org/pkg/testing/)
- [测试驱动开发 (TDD)](https://en.wikipedia.org/wiki/Test-driven_development)
- [Mock 对象模式](https://martinfowler.com/articles/mocksArentStubs.html)
- [测试覆盖率最佳实践](https://testing.googleblog.com/2020/08/code-coverage-best-practices.html)

---

**最后更新**: 2026-01-23
