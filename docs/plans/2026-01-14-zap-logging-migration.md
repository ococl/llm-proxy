# 日志系统迁移到zap框架实施计划

## 目标

将当前自定义的日志系统迁移到专业的zap日志框架，实现：
1. 多目录结构化日志（general/system/network/proxy/llm_debug）
2. 按日期+大小自动轮转，7天自动清理
3. 控制台Markdown着色，支持禁用
4. JSON格式文件日志，易于解析
5. 完整的敏感信息脱敏
6. 异步高性能日志输出

## 涉及文件

### 新增文件
- `src/logging_schema.go` - Logger类型定义和全局实例
- `src/logging_factory.go` - zap多Logger初始化工厂
- `src/logging_config.go` - Logging配置结构体扩展
- `src/logger_test.go` - 新的单元测试

### 修改文件
- `src/logger.go` - 替换InitLogger，集成zap
- `src/config.go` - 添加新配置字段
- `src/config.example.yaml` - 完整logging配置示例
- `src/main.go` - 添加-colorize命令行选项
- `src/proxy.go` - 迁移35条日志调用（最复杂）
- `src/router.go` - 迁移6条日志调用
- `src/config.go` - 迁移3条日志调用
- `src/middleware.go` - 迁移1条日志调用
- `src/system_prompt.go` - 迁移1条日志调用

### 删除/废弃
- `src/colors.go` - 手工着色代码（zap处理）
- 旧logger.go中的AsyncLogger、LogTarget等

## 日志目录结构

```
llm-proxy/
├── logs/                          # 所有日志根目录
│   ├── general.log               # 通用日志（启动、关闭等）
│   ├── system/                   # 系统和配置类日志
│   │   ├── system.log            # 配置加载、验证、panic
│   │   ├── startup.log           # 启动日志
│   │   └── shutdown.log          # 关闭日志
│   ├── network/                  # 网络和HTTP异常日志
│   │   ├── network.log           # 连接错误、超时等
│   │   ├── http_errors.log       # HTTP 4xx/5xx错误
│   │   └── api_validation.log    # API Key验证失败
│   ├── proxy/                    # 代理业务逻辑日志
│   │   ├── requests.log          # 请求开始/完成
│   │   ├── routing.log           # 路由解析、回退
│   │   ├── backend.log           # 后端请求、响应
│   │   └── fallback.log          # 回退策略执行
│   ├── llm_debug/                # 大模型调试日志（debug_mode控制）
│   │   ├── system_prompt.log     # system_prompt注入调试
│   │   ├── request_body.log      # 请求体详情（调试阶段）
│   │   └── response_body.log     # 响应体详情（调试阶段）
│   └── archive/                  # 轮转清理的旧日志（7天自动清理）
```

## 日志分类映射

| Logger | 目录 | 用途 | 迁移自 |
|--------|------|------|--------|
| GeneralLogger | `logs/` | 启动、关闭、性能指标 | LogGeneral(6条) |
| SystemLogger | `logs/system/` | 配置加载、验证、panic | LogGeneral(4条) |
| NetworkLogger | `logs/network/` | HTTP异常、连接错误、验证失败 | LogGeneral/WARN(8条) |
| ProxyLogger | `logs/proxy/` | 请求、路由、后端、回退 | LogGeneral(30条) |
| DebugLogger | `logs/llm_debug/` | system_prompt注入详情（debug_mode控制） | LogGeneral(1条) |

## 实现步骤

### Phase 1: 基础设施建设

- [ ] **1.1** 添加zap依赖
  ```bash
  go get go.uber.org/zap
  go get go.uber.org/zap/zapcore
  go get github.com/natefinch/lumberjack/v2
  ```

- [ ] **1.2** 创建 `src/logging_schema.go`
  - 定义LoggerType枚举
  - 定义全局Logger实例变量（GeneralLogger, SystemLogger, NetworkLogger, ProxyLogger, DebugLogger）
  - 定义Sugar接口变量

- [ ] **1.3** 创建 `src/logging_factory.go`
  - 实现InitLoggers()函数
  - 实现createLogger()函数
  - 实现parseLevel()辅助函数
  - 实现ShutdownLoggers()函数
  - 集成lumberjack日志轮转

- [ ] **1.4** 创建 `src/logging_config.go`
  - 定义LoggingNew结构体
  - 实现GetLevel(), GetConsoleLevel(), GetBaseDir()等方法
  - 实现MaxFileSizeMB, MaxAgeDays, MaxBackups默认值

### Phase 2: 配置和集成

- [ ] **2.1** 更新 `src/logger.go`
  - 替换InitLogger()为调用InitLoggers()
  - 替换ShutdownLogger()为调用ShutdownLoggers()
  - 保持向后兼容

- [ ] **2.2** 更新 `src/config.example.yaml`
  - 添加完整logging配置示例
  - 添加base_dir, max_file_size_mb, max_age_days等
  - 添加console_colorize, console_style
  - 添加debug_mode配置

- [ ] **2.3** 更新 `src/main.go`
  - 添加 `-colorize` / `-no-color` 命令行选项
  - 在InitLogger之前应用配置

### Phase 3: 日志调用迁移

- [ ] **3.1** 迁移 `src/main.go` (6条)
  - 启动日志 → GeneralLogger
  - 关闭日志 → GeneralLogger

- [ ] **3.2** 迁移 `src/config.go` (3条)
  - 配置加载日志 → SystemLogger

- [ ] **3.3** 迁移 `src/proxy.go` (35条，最复杂)
  - API Key验证失败 → NetworkLogger
  - 请求处理 → ProxyLogger
  - 后端请求 → ProxyLogger
  - 错误日志 → NetworkLogger + WriteErrorLog

- [ ] **3.4** 迁移 `src/router.go` (6条)
  - 路由解析日志 → ProxyLogger
  - 回退日志 → ProxyLogger

- [ ] **3.5** 迁移 `src/middleware.go` (1条)
  - Panic捕获日志 → SystemLogger

- [ ] **3.6** 迁移 `src/system_prompt.go` (1条)
  - System prompt调试日志 → DebugLogger

### Phase 4: 测试和验证

- [ ] **4.1** 创建 `src/logger_test.go`
  - 测试Logger初始化
  - 测试日志轮转
  - 测试敏感信息脱敏
  - 测试Debug模式开关

- [ ] **4.2** 运行集成测试
  ```bash
  cd src && go test -v ./...
  ```

- [ ] **4.3** 手动验证
  - [ ] 应用启动正常
  - [ ] logs/下各子目录正确创建
  - [ ] 文件日志有JSON格式内容
  - [ ] 控制台着色正常
  - [ ] 禁用着色后无颜色
  - [ ] 敏感信息被脱敏
  - [ ] Debug模式开关正常

### Phase 5: 清理和文档

- [ ] **5.1** 清理旧代码
  - 删除 `src/colors.go`
  - 标记旧LogGeneral/LogFile为deprecated
  - 删除logger.go中的冗余代码

- [ ] **5.2** 更新文档
  - 更新README.md日志配置章节
  - 添加日志目录结构说明
  - 添加debug_mode说明
  - 添加-colorize选项说明

## 风险点

### 风险1: proxy.go日志迁移最复杂（35条）
**应对**: 逐行迁移，每迁移一个函数就编译验证一次，避免大规模修改后难以定位问题。

### 风险2: 敏感信息脱敏可能遗漏
**应对**: 使用zap Hook机制，在写日志前统一处理；添加单元测试验证脱敏效果。

### 风险3: 性能影响
**应对**: zap本身高性能，但需注意异步配置；使用buffer_size和flush_interval调优。

### 风险4: 向后兼容性
**应对**: 保留旧的LogGeneral函数但标记为deprecated，新代码全部使用新Logger。

### 风险5: 测试覆盖
**应对**: 创建完整的logger_test.go，覆盖所有日志场景。

## 验收标准

### 功能验收
- [ ] 多目录结构化日志正常生成
- [ ] 按日期+大小自动轮转（测试：创建>100MB日志文件）
- [ ] 7天自动清理（配置max_age_days=7）
- [ ] 控制台Markdown着色正常
- [ ] -no-color命令行选项生效
- [ ] debug_mode开关正常控制llm_debug日志
- [ ] 敏感信息（API Key）正确脱敏

### 代码验收
- [ ] 编译通过（go build -o dist/llm-proxy ./src）
- [ ] 所有单元测试通过（go test ./src/...）
- [ ] 代码风格符合项目规范（linter检查）
- [ ] 无新的编译警告

### 文档验收
- [ ] README.md更新完整
- [ ] config.example.yaml配置示例完整
- [ ] 日志目录结构文档清晰

## 时间预估

- Phase 1-2（基础设施）: 30分钟
- Phase 3（日志迁移）: 1-1.5小时
- Phase 4（测试验证）: 30分钟
- Phase 5（清理文档）: 15分钟
- **总计**: 约2.5-3小时

## 后续优化（可选）

- 性能指标日志（MetricsLogger）
- 安全审计日志（AuditLogger）
- 业务统计日志（StatsLogger）
- 熔断器日志（CircuitLogger）

---

**计划制定时间**: 2025-01-14
**预计完成时间**: 2025-01-14
**状态**: 待用户确认后开始实施
