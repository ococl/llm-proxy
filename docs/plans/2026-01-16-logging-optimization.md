# 日志系统优化实施方案

## 现状分析

当前项目使用自定���日志系统,运行正常但存在以下待优化点:
1. 简单的文件轮转机制(按日期手动轮转)
2. 单一的general.log文件,缺乏分类
3. 控制台输出着色功能缺失(有占位函数但未实现)
4. 缺乏结构化日志支持

## 优化目标

基于现有可运行代码,渐进式优化日志系统:

1. **增强控制台输出** - 实现Markdown风格的彩色控制台输出(非JSON)
2. **改进日志轮转** - 集成lumberjack实现按大小+日期自动轮转
3. **简化日志分类** - 保持单一general.log,通过字段区分类型
4. **保持向后兼容** - 不破坏现有LogGeneral等函数
5. **优化敏感信息脱敏** - 增强现有正则表达式

## 文件涉及

### 修改文件
- `src/logger.go` - 增强控制台着色、集成lumberjack
- `src/config.example.yaml` - 优化配置说明和默认值
- `src/README.md` - 更新日志配置文档

### 无需新增文件
保持现有文件结构,避免大规模重构

## 实施步骤

### Phase 1: 增强控制台彩色输出 (30分钟)

**目标**: 实现清晰美观的Markdown风格控制台日志

**任务**:

1.1 实现 `colorLevelSimple` 函数
```go
// 使用ANSI颜色码为日志级别着色
// ERROR: 红色, WARN: 黄色, INFO: 绿色, DEBUG: 蓝色
```

1.2 实现 `colorTimeStrSimple` 函数  
```go
// 时间显示为灰色
```

1.3 实现 `highlightRequestIDSimple` 函数
```go
// 高亮显示 [req_xxx] 格式的请求ID
// 使用正则匹配 `\[req_[a-zA-Z0-9]+\]` 并着色为青色
```

1.4 优化 `logMessage` 函数的控制台输出格式
```go
// 当前: 15:04:05  INFO  消息内容
// 优化为: 15:04:05  INFO   消息内容  (对齐更好)
```

**验收标准**:
- ✅ 不同日志级别显示不同颜色
- ✅ 请求ID高亮显示
- ✅ `-no-color` 选项能禁用颜色
- ✅ 颜色仅在TTY终端显示

---

### Phase 2: 集成lumberjack日志轮转 (30分钟)

**目标**: 使用专业库实现按大小+时间自动轮转

**任务**:

2.1 已有依赖检查
```bash
# go.mod中已包含 gopkg.in/natefinch/lumberjack.v2
```

2.2 修改 `InitLogger` 函数
```go
import "gopkg.in/natefinch/lumberjack.v2"

// 将 generalLogger (*os.File) 改为 lumberjack.Logger
var generalLoggerLumberjack *lumberjack.Logger

// 配置lumberjack
generalLoggerLumberjack = &lumberjack.Logger{
    Filename:   cfg.Logging.GeneralFile,
    MaxSize:    cfg.Logging.GetMaxFileSizeMB(), // MB
    MaxBackups: cfg.Logging.GetMaxBackups(),
    MaxAge:     cfg.Logging.GetMaxAgeDays(), // days
    Compress:   cfg.Logging.Compress,
}
```

2.3 简化 `logMessage` 函数
```go
// 移除 rotateLogIfNeeded 调用(lumberjack自动处理)
// 移除 currentLogDate, currentLogSize 变量
```

2.4 更新 `ShutdownLogger` 函数
```go
if generalLoggerLumberjack != nil {
    generalLoggerLumberjack.Close()
}
```

**验收标准**:
- ✅ 日志文件超过100MB自动轮转
- ✅ 保留最近7天的日志
- ✅ 旧日志自动压缩为.gz
- ✅ 无需手动删除旧日志

---

### Phase 3: 优化配置和文档 (20分钟)

**目标**: 提供清晰的配置说明和合理默认值

**任务**:

3.1 优化 `config.example.yaml`
```yaml
logging:
  level: "info"              # 文件日志级别: debug/info/warn/error
  console_level: "info"      # 控制台日志级别(默认继承level)
  general_file: "./logs/general.log"
  
  # 日志轮转策略
  max_file_size_mb: 100      # 单文件最大大小(MB),超过自动轮转
  max_age_days: 7            # 保留天数,过期自动删除
  max_backups: 21            # 保留备份数量(7天*3次/天≈21)
  compress: true             # 压缩旧日志为.gz节省空间
  
  # 控制台着色
  colorize: true             # 启用彩色输出(仅TTY终端)
  console_style: "compact"   # 输出风格: compact(简洁)/verbose(详细)
  
  # 敏感信息保护
  mask_sensitive: true       # 自动脱敏API Key, Token等
  
  # 性能选项
  async: true                # 异步写入(推荐)
  buffer_size: 10000         # 异步缓冲区大小
  drop_on_full: false        # 缓冲区满时丢弃日志(false=阻塞等待)
  
  # 高级选项(一般无需修改)
  separate_files: false      # 为每个请求创建独立日志文件
  request_dir: "./logs/requests"
  error_dir: "./logs/errors"
  enable_metrics: false      # 记录性能指标
```

3.2 更新 README.md 日志章节
- 添加彩色输出示例截图(可选)
- 说明 `-no-color` 选项用法
- 说明日志轮转机制
- 说明敏感信息脱敏规则

**验收标准**:
- ✅ 配置文件有完整注释
- ✅ README文档清晰易懂
- ✅ 默认配置适合生产环境

---

### Phase 4: 增强敏感信息脱敏 (15分钟)

**目标**: 更全面地保护敏感信息

**任务**:

4.1 扩展 `sensitivePatterns` 正则表达式
```go
var sensitivePatterns = []*regexp.Regexp{
    // 现有规则
    regexp.MustCompile(`(?i)(sk-[a-zA-Z0-9]{20,})`),
    regexp.MustCompile(`(?i)(bearer\s+)([a-zA-Z0-9\-_]{20,})`),
    regexp.MustCompile(`(?i)(api[_-]?key["\s:=]+)([a-zA-Z0-9\-_]{16,})`),
    regexp.MustCompile(`(?i)(authorization["\s:=]+)([a-zA-Z0-9\-_]{16,})`),
    
    // 新增规则
    regexp.MustCompile(`(?i)("api_key"\s*:\s*")([^"]{16,})")`),  // JSON中的api_key
    regexp.MustCompile(`(?i)(password["\s:=]+)([a-zA-Z0-9\-_!@#$%^&*]{8,})`), // 密码
    regexp.MustCompile(`(?i)(token["\s:=]+)([a-zA-Z0-9\-_\.]{20,})`), // 通用token
}
```

4.2 优化 `MaskSensitiveData` 函数
```go
// 保留更多上下文,便于调试
// 旧: sk-abc1234567890 → sk-a****890  
// 新: sk-abc1234567890 → sk-abc****567890 (保留前6位和后6位)
```

**验收标准**:
- ✅ API Key正确脱敏
- ✅ Authorization header正确脱敏  
- ✅ JSON内嵌的api_key正确脱敏
- ✅ 保留足够信息用于调试

---

### Phase 5: 测试和验证 (20分钟)

**目标**: 确保所有优化功能正常工作

**任务**:

5.1 功能测试
```bash
# 启动应用
./llm-proxy -config config.yaml

# 观察控制台输出是否有颜色

# 禁用颜色测试
./llm-proxy -no-color -config config.yaml

# 检查日志文件
ls -lh logs/
# 应该看到 general.log 和可能的轮转文件
```

5.2 轮转测试
```bash
# 修改配置将 max_file_size_mb 设为 1
# 触发大量请求,观察是否自动轮转
# 检查旧日志是否压缩为 .gz
```

5.3 脱敏测试
```bash
# 发送包含API Key的请求
# 检查日志文件,确认敏感信息已脱敏
grep "api_key" logs/general.log
# 应该看到 "api_key": "sk-abc****890"
```

**验收标准**:
- ✅ 编译通过: `go build -o dist/llm-proxy ./src`
- ✅ 应用启动正常
- ✅ 控制台输出有颜色
- ✅ `-no-color` 正常工作
- ✅ 日志文件自动轮转
- ✅ 敏感信息正确脱敏

---

## 不实施的功能

以下功能虽在原计划中,但当前不实施:

❌ **多目录分类日志** (logs/system/, logs/network/, logs/proxy/)
- 理由: 增加复杂度,单一文件+字段分类已足够

❌ **zap框架迁移**  
- 理由: 需要创建多个新文件,迁移风险大,当前系统已满足需求

❌ **JSON格式文件日志**
- 理由: 文本格式更易读,且已有结构化字段(时间、级别、消息)

❌ **Sugar Logger全局变量** (GeneralSugar等)
- 理由: 会破坏现有代码,需大规模迁移

## 风险控制

### 风险1: 颜色在非TTY环境显示乱码
**应对**: 使用 `github.com/mattn/go-isatty` 检测终端类型,非TTY自动禁用

### 风险2: lumberjack与现有file handle冲突
**应对**: 
- 先关闭旧的 `generalLogger (*os.File)`
- 再初始化 `lumberjack.Logger`

### 风险3: 性能影响
**应对**:
- lumberjack本身高性能
- 保持现有AsyncLogger机制
- 颜色渲染仅在控制台输出时进行

## 时间预估

- Phase 1 (控制台着色): 30分钟
- Phase 2 (lumberjack集成): 30分钟  
- Phase 3 (配置文档): 20分钟
- Phase 4 (脱敏增强): 15分钟
- Phase 5 (测试验证): 20分钟
- **总计**: 约2小时

## 后续优化建议

完成本次优化后,未来可考虑:

1. **结构化字段**: 在日志中添加更多结构化字段(backend, model, duration等)
2. **日志查询工具**: 编写简单的日志分析脚本
3. **监控集成**: 输出Prometheus metrics或集成OpenTelemetry
4. **分类标签**: 在单一文件中使用 `[SYSTEM]`, `[NETWORK]`, `[PROXY]` 等前���分类

---

**计划制定时间**: 2026-01-16  
**预计完成时间**: 2026-01-16  
**状态**: 待用户确认后开始实施
