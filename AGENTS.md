# AI 编码助手开发指南

**本文档专为 AI 编码助手设计,提供项目开发规范、命令参考和架构指导。**

---

## ⚠️ 核心原则

### 语言要求
- **所有交流、推理、输出必须使用中文**
- **所有代码注释必须使用中文**
- **所有日志消息、错误提示必须使用中文**
- **所有文档必须使用中文**

### 注释规范
- **必须保留必要的注释**,包括:
  - 接口和公开类型的文档注释
  - 复杂业务逻辑的解释
  - 非显而易见的实现细节
  - 重要配置项的说明
- **鼓励添加合理的注释**,帮助理解代码意图
- **避免明显多余的注释**,如 `i++  // 自增`

---

## 📦 项目概览

**llm-proxy** 是一个高性能的 LLM API 代理服务,采用 Clean Architecture 架构设计,提供负载均衡、故障转移、限流、并发控制等企业级功能。

- **语言**: Go 1.25.5
- **架构**: Clean Architecture (分层架构)
- **核心层级**:
  - `domain/` - 领域层(实体、端口接口、领域服务)
  - `application/` - 应用层(用例、应用服务)
  - `adapter/` - 适配器层(HTTP、配置、后端客户端、日志)
  - `infrastructure/` - 基础设施层(HTTP 服务器、配置加载、日志实现)

---

## 🛠️ 构建与测试命令

### 开发构建
```bash
# 快速开发构建(当前平台)
make dev
# 输出: dist/llm-proxy.exe

# 完整多平台构建
make build-all
# 输出: dist/llm-proxy-{platform}-{arch}.exe

# 清理构建产物
make clean
```

### 测试命令

#### 运行所有测试
```bash
make test
# 等同于: cd src && go test -v ./...
```

#### 运行指定包的测试
```bash
cd src && go test -v ./domain/entity
cd src && go test -v ./application/usecase
cd src && go test -v ./adapter/http/middleware
```

#### 运行单个测试函数
```bash
# 测试函数命名规范: Test<功能>_<场景>
cd src && go test -v -run TestBackend_New ./domain/entity
cd src && go test -v -run TestProxyRequestUseCase_ValidateRequest ./application/usecase
cd src && go test -v -run TestRateLimiter_Allow ./adapter/http/middleware
```

#### 测试覆盖率
```bash
# 显示覆盖率
cd src && go test -v -cover ./...

# 生成覆盖率报告
cd src && go test -coverprofile=coverage.out ./...
cd src && go tool cover -html=coverage.out
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

**严格遵守三段式导入**:

```go
import (
	// 1. 标准库(按字母排序)
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	
	// 2. 本项目包(使用 llm-proxy/ 前缀,按分层排序)
	"llm-proxy/domain/entity"
	"llm-proxy/domain/port"
	domain_service "llm-proxy/domain/service"  // 使用别名避免冲突
	"llm-proxy/application/usecase"
	http_adapter "llm-proxy/adapter/http"
	
	// 3. 第三方库(按字母排序)
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)
```

**别名命名规范**:
- 同名包冲突时使用 `<层级>_<包名>` 格式
- 例: `domain_service`, `http_adapter`, `infra_config`

### 命名约定

- **包名**: 小写单词,无下划线 (`entity`, `usecase`, `middleware`)
- **导出标识符**: 首字母大写 (`type Backend struct`, `func NewBackend()`)
- **私有标识符**: 首字母小写 (`func validateRequest()`, `type requestContext struct`)
- **接口**: 名词或形容词,通常以 -er 结尾 (`Logger`, `ConfigProvider`, `BackendClient`)
- **常量**: 驼峰命名 (`const maxRetries = 3`, `const defaultTimeout = 30 * time.Second`)

### 类型定义

```go
// ✅ 推荐: 明确的值对象
type BackendID string

func NewBackendID(name string) BackendID {
	return BackendID(name)
}

func (id BackendID) String() string {
	return string(id)
}

// ✅ 推荐: 使用 Builder 模式构建复杂对象
type Backend struct {
	id       BackendID
	name     string
	url      BackendURL
	apiKey   APIKey
	protocol types.Protocol
	enabled  bool
}

func NewBackendBuilder() *BackendBuilder {
	return &BackendBuilder{
		enabled: true,  // 默认值
	}
}

type BackendBuilder struct {
	id       BackendID
	name     string
	// ... 其他字段
}

func (b *BackendBuilder) WithName(name string) *BackendBuilder {
	b.name = name
	return b
}

func (b *BackendBuilder) Build() (*Backend, error) {
	// 验证必填字段
	if b.name == "" {
		return nil, fmt.Errorf("后端名称不能为空")
	}
	// 返回不可变对象
	return &Backend{
		id:       NewBackendID(b.name),
		name:     b.name,
		url:      b.url,
		apiKey:   b.apiKey,
		protocol: b.protocol,
		enabled:  b.enabled,
	}, nil
}

// ✅ 推荐: 使用指针区分零值和未设置
type Config struct {
	Enabled *bool `yaml:"enabled,omitempty"`
}

func (c *Config) IsEnabled() bool {
	return c.Enabled == nil || *c.Enabled
}
```

### 错误处理

```go
// ✅ 推荐: 使用自定义错误类型(domain/error/types.go)
import domainerror "llm-proxy/domain/error"

func processRequest(req *Request) error {
	if req.Model == "" {
		return domainerror.ErrMissingModel
	}
	
	backend, err := repo.GetBackend(req.Model)
	if err != nil {
		return domainerror.Wrap(err, domainerror.CodeNoBackend, "获取后端失败")
	}
	
	return nil
}

// ✅ 推荐: 提前返回,避免嵌套
func validate(req *Request) error {
	if req == nil {
		return domainerror.ErrInvalidRequest
	}
	if req.Model == "" {
		return domainerror.ErrMissingModel
	}
	// 正常路径
	return nil
}

// ❌ 避免: 忽略错误
body, _ := io.ReadAll(resp.Body)  // 缺少错误检查

// ❌ 避免: 过度嵌套
if err == nil {
	if data != nil {
		if valid {
			// 处理
		}
	}
}
```

### 日志记录

**使用结构化日志,所有消息和字段名必须是中文**:

```go
// ✅ 推荐: 结构化日志 (port.Logger 接口)
logger.Info("代理请求成功",
	port.Field{Key: "请求ID", Value: reqID},
	port.Field{Key: "后端", Value: backend.Name()},
	port.Field{Key: "模型", Value: req.Model()},
	port.Field{Key: "状态码", Value: resp.StatusCode()},
	port.Field{Key: "耗时毫秒", Value: duration.Milliseconds()},
)

// ✅ 推荐: 错误日志包含上下文
logger.Error("后端请求失败",
	port.Field{Key: "错误", Value: err.Error()},
	port.Field{Key: "后端", Value: backend.Name()},
	port.Field{Key: "重试次数", Value: retryCount},
	port.Field{Key: "请求ID", Value: reqID},
)

// ❌ 避免: 非结构化日志
log.Println("请求成功 backend=" + backend)

// ❌ 避免: 使用英文字段名
logger.Info("Request success", 
	port.Field{Key: "backend", Value: backend},  // 错误: 字段名必须是中文
)
```

### 注释规范

```go
// ✅ 推荐: 接口和公开类型的文档注释(中文)
// Logger 提供结构化日志记录功能。
// 所有日志消息和字段名必须使用中文。
type Logger interface {
	// Info 记录信息级别日志
	Info(msg string, fields ...Field)
	
	// Error 记录错误级别日志
	Error(msg string, fields ...Field)
}

// ✅ 推荐: 复杂业务逻辑的解释
func (s *FallbackStrategy) GetBackoffDelay(attempt int) time.Duration {
	// 计算指数退避延迟: initialDelay * multiplier^(attempt-1)
	delay := s.initialDelay
	for i := 1; i < attempt; i++ {
		delay = time.Duration(float64(delay) * s.multiplier)
		if delay > s.maxDelay {
			delay = s.maxDelay
			break
		}
	}
	
	// 添加随机抖动,避免雷鸣群效应
	jitter := time.Duration(float64(delay) * s.jitter * (rand.Float64()*2 - 1))
	return delay + jitter
}

// ✅ 推荐: 非显而易见的实现细节
// 注意: 这里使用深拷贝,避免并发修改原始路由列表
routes := make([]*port.Route, len(original))
copy(routes, original)

// ❌ 避免: 明显多余的注释
i++  // 自增
if err != nil {  // 如果有错误
	return err  // 返回错误
}
```

---

## 🏗️ Clean Architecture 架构

### 核心原则
- **依赖方向**: 外层依赖内层,内层对外部无感知
- **依赖倒置**: 内层定义接口(port),外层实现接口
- **业务逻辑隔离**: 核心业务逻辑在 domain 和 application 层,与框架解耦

### 分层结构

```
┌─────────────────────────────────────────────────────────┐
│  Infrastructure Layer (基础设施层)                       │
│  - HTTP 服务器、配置文件加载、Zap 日志实现              │
│  - 依赖: adapter/, application/, domain/                │
└─────────────────────────────────────────────────────────┘
                        ⬇️ 依赖
┌─────────────────────────────────────────────────────────┐
│  Adapter Layer (适配器层)                                │
│  - HTTP 处理器、中间件、配置适配器、后端客户端           │
│  - 依赖: application/, domain/                          │
└─────────────────────────────────────────────────────────┘
                        ⬇️ 依赖
┌─────────────────────────────────────────────────────────┐
│  Application Layer (应用层)                              │
│  - 用例编排(ProxyRequestUseCase, RouteResolveUseCase)  │
│  - 应用服务(协议转换、请求验证、响应转换)                │
│  - 依赖: domain/                                        │
└─────────────────────────────────────────────────────────┘
                        ⬇️ 依赖
┌─────────────────────────────────────────────────────────┐
│  Domain Layer (领域层) - 核心业务规则                     │
│  - 实体(Backend, Request, Response)                     │
│  - 端口接口(Logger, ConfigProvider, BackendClient)     │
│  - 领域服务(LoadBalancer, FallbackStrategy)            │
│  - 无外部依赖                                           │
└─────────────────────────────────────────────────────────┘
```

### 目录对照

| 目录 | 职责 | 示例 |
|------|------|------|
| `domain/entity/` | 领域实体(业务对象) | Backend, Request, Response |
| `domain/port/` | 端口接口(依赖倒置) | Logger, ConfigProvider, BackendClient |
| `domain/service/` | 领域服务(核心业务逻辑) | LoadBalancer, FallbackStrategy, CooldownManager |
| `domain/error/` | 错误类型定义 | LLMProxyError, ErrorCode |
| `application/usecase/` | 用例编排 | ProxyRequestUseCase, RouteResolveUseCase |
| `application/service/` | 应用服务 | ProtocolConverter, RequestValidator |
| `adapter/http/` | HTTP 适配器 | ProxyHandler, HealthHandler, Middleware |
| `adapter/config/` | 配置适配器 | ConfigAdapter, BackendRepository |
| `adapter/backend/` | 后端客户端适配器 | HTTPClient, BackendClientAdapter |
| `adapter/logging/` | 日志适配器 | ZapLoggerAdapter |
| `infrastructure/` | 基础设施实现 | HTTP Server, Config Loader, Zap Logger |

### 依赖注入示例

```go
// main.go - 组装所有依赖
func main() {
	// 1. 基础设施层
	configMgr, _ := infra_config.NewManager("config.yaml")
	infra_logging.InitLogger(configMgr.Get())
	
	// 2. 适配器层
	configAdapter := config_adapter.NewConfigAdapter(configMgr)
	proxyLogger := logging_adapter.NewZapLoggerAdapter(infra_logging.ProxySugar)
	httpClient := infra_http.NewClient(clientConfig)
	backendClient := backend_adapter.NewBackendClientAdapter(httpClient, proxyLogger)
	
	// 3. 领域服务
	cooldownMgr := domain_service.NewCooldownManager(5 * time.Minute)
	loadBalancer := domain_service.NewLoadBalancer()
	fallbackStrategy := domain_service.NewFallbackStrategy(configAdapter, cooldownMgr)
	
	// 4. 应用层
	protocolConverter := service.NewProtocolConverter()
	routeResolver := usecase.NewRouteResolveUseCase(configAdapter, loadBalancer)
	retryStrategy := usecase.NewRetryStrategy(fallbackStrategy, configAdapter)
	
	proxyUseCase := usecase.NewProxyRequestUseCase(
		backendClient,
		routeResolver,
		retryStrategy,
		protocolConverter,
		configAdapter,
		proxyLogger,
		&MockMetricsProvider{},
	)
	
	// 5. HTTP 层
	proxyHandler := http_adapter.NewProxyHandler(proxyUseCase, proxyLogger, configAdapter)
	mux := http.NewServeMux()
	mux.Handle("/v1/chat/completions", proxyHandler)
	
	// 6. 启动服务器
	server := infra_http.NewServer(cfg.Server.Port, mux)
	server.Start()
}
```

---

## 🧪 测试编写指南

### 测试文件命名
- 测试文件: `<原文件名>_test.go` (如 `backend.go` → `backend_test.go`)
- 放置位置: 与被测文件同目录
- 包名: 与被测包相同 (白盒测试)

### 测试函数命名

**规范**: `Test<功能>_<场景>` 或 `Test<类型>_<方法>_<场景>`

```go
// ✅ 推荐: 清晰的测试名称
func TestBackend_New(t *testing.T)                          // 测试 Backend 构造函数
func TestBackendURL_NewBackendURL_InvalidURL(t *testing.T)  // 测试 URL 验证失败场景
func TestRateLimiter_Allow_BurstFactor(t *testing.T)        // 测试限流器的突发因子
func TestProxyRequestUseCase_ValidateRequest_EmptyModel(t *testing.T)  // 测试用例验证逻辑
```

### 表格驱动测试(推荐)

```go
func TestBackendURL_NewBackendURL(t *testing.T) {
	tests := []struct {
		name        string    // 测试用例名称
		input       string    // 输入参数
		expectError bool      // 是否期望错误
		expected    string    // 期望输出
	}{
		{
			name:        "完整的 HTTPS URL",
			input:       "https://api.example.com/v1",
			expectError: false,
			expected:    "https://api.example.com/v1",
		},
		{
			name:        "自动添加 HTTPS",
			input:       "api.example.com",
			expectError: false,
			expected:    "https://api.example.com",
		},
		{
			name:        "无效的 URL",
			input:       "://invalid",
			expectError: true,
			expected:    "",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := NewBackendURL(tt.input)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("期望错误,但成功返回: %v", url)
				}
				return
			}
			
			if err != nil {
				t.Fatalf("意外错误: %v", err)
			}
			
			if url.String() != tt.expected {
				t.Errorf("期望 %q, 实际 %q", tt.expected, url.String())
			}
		})
	}
}
```

### Mock 对象规范

```go
// Mock 对象命名: Mock<接口名>
type MockBackendClient struct {
	sendFunc func(ctx context.Context, req *entity.Request, backend *entity.Backend, backendModel string) (*entity.Response, error)
}

func (m *MockBackendClient) Send(ctx context.Context, req *entity.Request, backend *entity.Backend, backendModel string) (*entity.Response, error) {
	if m.sendFunc != nil {
		return m.sendFunc(ctx, req, backend, backendModel)
	}
	return nil, nil
}

// 使用示例
func TestProxyRequestUseCase_Execute(t *testing.T) {
	mockClient := &MockBackendClient{
		sendFunc: func(ctx context.Context, req *entity.Request, backend *entity.Backend, backendModel string) (*entity.Response, error) {
			// 模拟成功响应
			return entity.NewResponseBuilder().
				WithModel(req.Model()).
				Build(), nil
		},
	}
	
	uc := usecase.NewProxyRequestUseCase(mockClient, ...)
	resp, err := uc.Execute(context.Background(), testRequest)
	
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if resp == nil {
		t.Fatal("响应不应为 nil")
	}
}
```

---

## 🔍 关键设计模式

### 1. Builder 模式(构建复杂对象)

```go
// 用于构建 Request, Response, Backend 等复杂对象
req := entity.NewRequestBuilder().
	WithRequestID(reqID).
	WithModel(model).
	WithMessages(messages).
	WithStream(true).
	Build()

resp := entity.NewResponseBuilder().
	WithModel(model).
	WithChoices(choices).
	WithUsage(usage).
	Build()
```

### 2. Strategy 模式(可替换算法)

```go
// FallbackStrategy 封装故障转移逻辑
type FallbackStrategy interface {
	ShouldRetry(statusCode int, body string) bool
	GetBackoffDelay(attempt int) time.Duration
	GetMaxRetries() int
}

// 在用例中注入策略
type ProxyRequestUseCase struct {
	retryStrategy RetryStrategy  // 可替换的重试策略
}
```

### 3. Repository 模式(数据访问抽象)

```go
// domain/port/backend_repository.go
type BackendRepository interface {
	GetAll() []*entity.Backend
	GetByName(name string) (*entity.Backend, error)
	GetEnabled() []*entity.Backend
}

// adapter/config/adapter.go 实现
type BackendRepositoryImpl struct {
	configProvider port.ConfigProvider
}

func (r *BackendRepositoryImpl) GetByName(name string) (*entity.Backend, error) {
	// 从配置中查找后端
	return r.configProvider.GetBackend(name)
}
```

---

## ⚙️ 配置热重载

配置文件(`config.yaml`)支持热重载,无需重启服务:

```go
// infrastructure/config/config.go
type Manager struct {
	// 每次 Get() 检查文件修改时间
	// 检测到变化时自动重新加载
}

// main.go 注册日志配置变更回调
infra_config.LoggingConfigChangedFunc = func(c *infra_config.Config) error {
	infra_logging.ShutdownLogger()
	return infra_logging.InitLogger(c)
}
```

---

## 📚 参考资料

- [Go 代码审查建议](https://github.com/golang/go/wiki/CodeReviewComments)
- [Effective Go](https://go.dev/doc/effective_go)
- [Uber Go 风格指南](https://github.com/uber-go/guide/blob/master/style.md)
- [Clean Architecture - Uncle Bob](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- 项目设计文档: `docs/plans/2026-01-22-clean-arch-design.md`

---

## 📝 Git 提交规范

### 提交消息格式
```
<类型>(<范围>): <简短描述>

<详细说明(可选)>

关联问题: #123
```

**类型**:
- `feat`: 新功能
- `fix`: 错误修复
- `refactor`: 重构(不改变功能)
- `test`: 测试相关
- `docs`: 文档更新
- `chore`: 构建/工具配置

**范围**: `domain`, `application`, `adapter`, `infrastructure`, `http`, `config`

### 示例
```
feat(adapter/http): 添加并发限流中间件

- 实现基于 channel 的信号量机制
- 支持队列超时配置
- 添加单元测试覆盖

关联问题: #42
```

---

**最后更新**: 2026-01-23  
**项目版本**: 根据 git tag 自动生成  
**架构版本**: Clean Architecture v1.0
