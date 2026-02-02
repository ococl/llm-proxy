# 迁移指南: 日志配置简化 (v2.0 → v2.1)

## ⚠️ 破坏性变更

此版本包含对日志配置的**完全重构**，旧配置格式不再兼容。请按照本指南迁移您的配置。

---

## 快速对比

### 旧配置 (40+ 选项)

```yaml
logging:
  # 多个复杂的 targets 定义
  targets:
    - name: console
      type: console
      encoder:
        type: console
        time_format: "2006-01-02 15:04:05"
        use_color: true
        colorize_level: true
      level: info
    - name: general_file
      type: file
      path: logs/general.log
      encoder:
        type: json
        time_format: "2006-01-02T15:04:05.000Z"
      level: info
      rotation:
        max_size_mb: 100
        max_age_days: 7
        max_backups: 10
        compress: true
    # ... 更多 targets
  
  # 复杂的路由规则
  routers:
    - match:
        category: general
      targets: [console, general_file]
    - match:
        category: request
      targets: [request_file]
    # ... 更多路由
```

### 新配置 (10 个核心选项)

```yaml
logging:
  base_dir: "./logs"
  mask_sensitive: true
  async:
    enabled: true
    buffer_size: 10000
    flush_interval_seconds: 5
    drop_on_full: false
  rotation:
    max_size_mb: 100
    time_strategy: "daily"  # daily | hourly | none
    max_age_days: 7
    max_backups: 21
    compress: true
  categories:
    general: {level: "info", target: "both", path: "general.log"}
    request: {level: "info", target: "file", path: "requests/requests.log"}
    error: {level: "error", target: "both", path: "errors/errors.log"}
    network: {level: "debug", target: "file", path: "network/network.log"}
    debug: {level: "debug", target: "file", path: "debug/debug.log"}
    request_body: {level: "debug", target: "file", path: "request_body/{date}/{time}_{req_id}_{type}.httpdump", include_body: true}
```

---

## 配置项映射表

| 旧配置 | 新配置 | 说明 |
|--------|--------|------|
| `targets[].encoder.type` | `categories[].target` | 简化为 `console`/`file`/`both` |
| `targets[].level` | `categories[].level` | 直接映射 |
| `targets[].rotation` | `rotation` | 统一全局配置，支持分类覆盖 |
| `routers[].match.category` | `categories` 的 key | 分类名称直接作为配置键 |
| `targets[].path` | `categories[].path` | 相对路径，自动拼接 `base_dir` |
| `targets[].encoder.use_color` | (自动) | 控制台始终启用 Markdown 彩色输出 |

---

## 分类配置详解

每个分类支持以下字段：

```yaml
categories:
  <分类名称>:
    level: "debug" | "info" | "warn" | "error" | "none"
    target: "console" | "file" | "both"
    path: "相对于 base_dir 的路径"
    include_body: true | false  # 仅 request_body 分类有效
    # 可选：覆盖全局轮转配置
    max_size_mb: 100
    max_age_days: 7
    max_backups: 21
    compress: true
```

**注意：**
- `level: "none"` 会完全禁用该分类的日志
- `path` 支持模板变量：`{date}` (YYYYMMDD), `{time}` (HHMMSS), `{req_id}` (请求ID)

---

## 迁移步骤

### 1. 备份旧配置

```bash
cp config.yaml config.yaml.backup
```

### 2. 删除旧日志配置

删除整个 `logging:` 块下的所有内容。

### 3. 添加新配置

将 `config.example.yaml` 中的新日志配置复制到您的配置文件中：

```yaml
logging:
  base_dir: "./logs"
  mask_sensitive: true
  async:
    enabled: true
    buffer_size: 10000
    flush_interval_seconds: 5
    drop_on_full: false
  rotation:
    max_size_mb: 100
    time_strategy: "daily"
    max_age_days: 7
    max_backups: 21
    compress: true
  categories:
    general: {level: "info", target: "both", path: "general.log"}
    request: {level: "info", target: "file", path: "requests/requests.log"}
    error: {level: "error", target: "both", path: "errors/errors.log"}
    network: {level: "debug", target: "file", path: "network/network.log"}
    debug: {level: "debug", target: "file", path: "debug/debug.log"}
    request_body: {level: "debug", target: "file", path: "request_body/{date}/{time}_{req_id}_{type}.httpdump", include_body: true}
```

### 4. 调整参数

根据您的需求调整：
- `base_dir`: 日志根目录
- `rotation.time_strategy`: 时间轮转策略
- `categories.<name>.level`: 各分类的日志级别

### 5. 启动验证

```bash
./llm-proxy -config config.yaml
```

---

## 新功能亮点

### 1. 异步日志写入

所有文件日志现在使用异步写入，显著提升性能：
- 缓冲队列避免阻塞请求处理
- 可配置丢弃策略防止内存溢出
- 自动刷新确保数据安全

### 2. Markdown 彩色控制台输出

控制台日志现在使用 Markdown 格式，更易读：
- 自动为每个请求 ID 分配唯一颜色
- 结构化字段清晰展示
- 敏感数据自动脱敏

### 3. 时间 + 大小双策略轮转

支持灵活的日志轮转：
- **时间策略**: `daily` (每天)、`hourly` (每小时)、`none` (仅大小)
- **大小策略**: 超过 `max_size_mb` 自动轮转
- **自动压缩**: 旧日志自动 gzip 压缩

### 4. 敏感数据脱敏

自动脱敏敏感信息：
- API keys、tokens、passwords
- 可在 `logging.mask_sensitive` 中启用/禁用

---

## 常见问题

### Q: 旧日志文件会保留吗？
A: 会。迁移不会删除任何现有日志文件，但新日志将按新配置写入。

### Q: 如何完全禁用某个分类？
A: 设置 `level: "none"`：
```yaml
categories:
  debug: {level: "none"}  # 完全禁用 debug 分类
```

### Q: 可以自定义分类吗？
A: 目前支持 6 个固定分类：`general`, `request`, `error`, `network`, `debug`, `request_body`。

### Q: 为什么移除了 encoder 配置？
A: 为了简化配置，现在：
- 控制台始终使用 Markdown 彩色格式
- 文件始终使用 JSON 格式

---

## 回滚方法

如需回滚到旧版本：

```bash
cp config.yaml.backup config.yaml
git checkout v2.0.x
```

---

## 技术支持

如有问题，请：
1. 检查日志输出中的错误信息
2. 对比您的配置与 `config.example.yaml`
3. 提交 Issue 到项目仓库

---

**迁移完成！** 享受更简洁、更高效的日志系统。
