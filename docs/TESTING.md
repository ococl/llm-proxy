# 测试指南

LLM-Proxy 项目的测试指南，包含单元测试、集成测试、端到端测试和协议测试的完整说明。

---

## 📋 目录

- [快速开始](#快速开始)
- [测试类型](#测试类型)
- [端到端测试](#端到端测试)
- [协议测试](#协议测试)
- [单元测试](#单元测试)
- [测试覆盖率](#测试覆盖率)
- [CI/CD 集成](#cicd-集成)

---

## 🚀 快速开始

### 首选方法：使用 Python3（推荐）

```powershell
# Windows
python3 scripts/e2e-test.py

# 或直接运行（会自动调用 Python3）
.\scripts\e2e-test.ps1
```

```bash
# Linux/macOS
python3 scripts/e2e-test.py
./scripts/e2e-test.py
```

### 快速验证（构建 + 简单测试）

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

### 运行所有测试

```bash
cd src
go test ./...
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

### 2. 集成测试 (Integration Tests)

测试多个组件协作的行为，可能涉及 HTTP 调用或文件 I/O。

**位置**: `src/adapter/*_test.go`, `src/application/*_test.go`

### 3. 端到端测试 (E2E Tests)

测试完整的用户场景，从 HTTP 请求到响应。

### 4. 协议测试 (Protocol Tests)

测试不同协议（OpenAI、Anthropic）的直通和转换功能。

---

## 🌐 端到端测试

### 使用 Python3 测试脚本（推荐）

```powershell
# Windows / Linux / macOS
python3 scripts/e2e-test.py

# 运行所有测试
python3 scripts/e2e-test.py --all

# 仅健康检查
python3 scripts/e2e-test.py --health

# 仅正常请求测试
python3 scripts/e2e-test.py --normal

# 仅流式请求测试
python3 scripts/e2e-test.py --streaming

# 仅协议测试
python3 scripts/e2e-test.py --protocol

# 仅 OpenAI 协议透传测试
python3 scripts/e2e-test.py --openai

# 仅 Anthropic 协议透传测试
python3 scripts/e2e-test.py --anthropic

# 详细输出
python3 scripts/e2e-test.py -v
```

### 使用 PowerShell 脚本（自动转发到 Python3）

```powershell
# Windows PowerShell
.\scripts\e2e-test.ps1

# 参数与 Python3 版本相同
.\scripts\e2e-test.ps1 --all
.\scripts\e2e-test.ps1 --health
.\scripts\e2e-test.ps1 --protocol
```

### 测试脚本功能

E2E 测试脚本会自动执行以下步骤：

1. **环境检查**
   - 验证二进制文件存在
   - 验证配置文件存在
   - 检查端口占用

2. **服务启动**（如果未运行）
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

6. **协议测试**（可选）
   - OpenAI 协议透传测试
   - Anthropic 协议透传测试
   - 协议转换测试

7. **错误处理测试**
   - 测试无效模型请求
   - 验证错误码和错误消息

8. **日志验证**
   - 检查日志文件存在性
   - 显示最新日志内容

9. **服务停止**
   - 优雅关闭服务进程（如果由脚本启动）

---

## 🔄 协议测试

### 协议测试脚本

协议测试用于验证 LLM-Proxy 的协议直通和转换功能。

### 使用 Python3（推荐）

```powershell
# 运行所有协议测试
python3 scripts/protocol-test.py

# 仅 OpenAI 协议测试
python3 scripts/protocol-test.py --openai

# 仅 Anthropic 协议测试
python3 scripts/protocol-test.py --anthropic

# 仅协议转换测试
python3 scripts/protocol-test.py --conversion

# 详细输出
python3 scripts/protocol-test.py -v
```

### 使用 PowerShell（自动转发）

```powershell
.\scripts\protocol-test.ps1
.\scripts\protocol-test.ps1 --openai
.\scripts\protocol-test.ps1 --anthropic
```

### 测试的协议类型

#### 1. OpenAI 协议

测试使用 OpenAI 格式请求的模型：

| 模型别名 | 后端 | 协议 |
|---------|------|------|
| `deepseek/deepseek-v3.2` | GROUP_2 | openai |
| `z-ai/glm-4.7` | GROUP_1 | openai |
| `google/gemini-3-flash` | GROUP_1 | openai |
| `minimax/minimax-m2.1` | GROUP_1 | openai |
| `qwen/qwen3-coder-480b-a35b-instruct` | GROUP_2 | openai |

#### 2. Anthropic 协议

测试 Claude 模型：

| 模型别名 | 后端 | 协议 | 说明 |
|---------|------|------|------|
| `anthropic/claude-opus-4-5` | GROUP_HB5S | anthropic | Anthropic 原生协议 |
| `anthropic/claude-sonnet-4-5` | GROUP_1 | openai | OpenAI 格式请求 |
| `anthropic/claude-haiku-4-5` | GROUP_1 | openai | OpenAI 格式请求 |

### 测试场景

#### 直通测试 (Passthrough)

验证请求直接从客户端透传到对应协议的后端：

```
客户端 (OpenAI格式) → LLM-Proxy → OpenAI后端
客户端 (OpenAI格式) → LLM-Proxy → Anthropic后端
```

#### 转换测试 (Conversion)

验证协议转换功能：

```
客户端请求 (OpenAI格式) → LLM-Proxy → 不同协议后端
```

#### 混合协议路由

测试多后端回退场景：

```
Claude Opus → oocc (OpenAI) → GROUP_HB5S (Anthropic) → GROUP_1 (OpenAI) → NVIDIA (OpenAI)
```

### 测试结果示例

```
========================================
 Test Report
========================================
  Total tests: 10
  Passed: 10
  Failed: 0
  Pass rate: 100%

========================================
 Protocol Summary
========================================
 ✓ OpenAI Protocol: tested
 ✓ Anthropic Protocol: tested
 ↔ Protocol Conversion: tested
 ↔ Mixed Protocol Routes: tested
 ↔ System Prompt Injection: tested

╔════════════════════════════════════════╗
║       All protocol tests passed! ✓     ║
╚════════════════════════════════════════╝
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

#### 使用表驱动测试

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

### 覆盖率目标

| 层级 | 目标覆盖率 | 当前状态 |
|------|-----------|---------|
| Domain Entity | > 90% | ✅ 95% |
| Domain Service | > 85% | ✅ 92% |
| Application | > 80% | ✅ 85% |
| Adapter | > 75% | ✅ 80% |

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

      - name: Set up Python
        uses: actions/setup-python@v4
        with:
          python-version: '3.10'

      - name: Run unit tests
        run: |
          cd src
          go test -v -coverprofile=coverage.out ./...

      - name: Run E2E tests
        run: |
          cd src
          go build -o ../dist/llm-proxy-latest .
          cd ..
          python3 scripts/e2e-test.py --all

      - name: Run protocol tests
        run: |
          python3 scripts/protocol-test.py

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./src/coverage.out
```

### Makefile 集成

```makefile
.PHONY: test test-unit test-e2e test-protocol test-coverage

test: test-unit test-e2e test-protocol

test-unit:
	cd src && go test -v ./...

test-e2e:
	python3 scripts/e2e-test.py --all

test-protocol:
	python3 scripts/protocol-test.py

test-coverage:
	cd src && go test -coverprofile=coverage.out ./...
	cd src && go tool cover -html=coverage.out -o coverage.html

quick-test:
	python3 scripts/e2e-test.py
```

---

## 🐛 故障排查

### 测试失败常见原因

#### 1. 端口占用

```powershell
# Windows
netstat -ano | findstr :8765
taskkill /PID <PID> /F

# Linux/macOS
lsof -ti:8765 | xargs kill -9
```

#### 2. Python3 未找到

```
症状: "Python3 未找到" 错误
解决方案: 安装 Python 3.8+
```

```powershell
# Windows - 检查 Python
where python
where python3

# Linux/macOS
which python3
```

#### 3. 配置文件缺失

```bash
# 确保 dist/config.yaml 存在
ls dist/config.yaml
```

#### 4. 二进制文件过期

```bash
# 重新构建
cd src
go build -o ../dist/llm-proxy-latest.exe .
```

#### 5. 超时错误

增加超时时间（在脚本参数中）。

---

## 📝 日志分析

### 查看测试日志

```
logs/
├── general.log         # 通用日志
├── requests/           # 请求日志
│   └── request.log
└── errors/             # 错误日志
    └── error.log
```

---

## 🎯 性能测试

### 基准测试

```bash
cd src
go test -bench=. -benchmem ./...
```

### 压力测试

```bash
# 使用 hey 工具
hey -n 1000 -c 10 \
  -H "Authorization: Bearer sk-aNbDRYsSMcbdVUptFyy9yWpeN6agx" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek/deepseek-v3.2","messages":[{"role":"user","content":"test"}]}' \
  http://localhost:8765/v1/chat/completions
```

---

## 📚 参考资源

- [Go Testing 官方文档](https://golang.org/pkg/testing/)
- [Python urllib 文档](https://docs.python.org/3/library/urllib.html)
- [测试驱动开发 (TDD)](https://en.wikipedia.org/wiki/Test-driven_development)

---

**最后更新**: 2026-01-23
