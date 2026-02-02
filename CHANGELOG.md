# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.1.0] - 2026-02-02

### ✨ Added

- **异步日志写入系统**
  - 基于 channel 的高性能异步写入
  - 可配置缓冲区大小和自动刷新间隔
  - 支持队列满时丢弃策略，防止内存溢出
  - 显著提升高并发场景下的日志性能

- **Markdown 彩色控制台输出**
  - 全新的 Markdown 格式控制台编码器
  - 为每个请求 ID 自动分配唯一颜色，便于追踪
  - 结构化字段清晰展示，提升可读性
  - 支持终端宽度自适应

- **时间 + 大小双策略轮转**
  - 新增 `time_strategy` 配置项: `daily`, `hourly`, `none`
  - 支持按时间（每天/每小时）自动轮转
  - 保留原有的按大小（MB）轮转
  - 双策略可组合使用，更灵活

- **敏感数据自动脱敏**
  - 自动识别并脱敏 API keys、tokens、passwords 等敏感信息
  - 可配置启用/禁用脱敏功能
  - 支持多种数据格式识别

### 🔧 Changed

- **日志配置完全重构** (⚠️ Breaking Change)
  - 从 40+ 个配置选项简化为 10 个核心选项
  - 新的简洁配置结构：
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
  - 移除复杂的 `targets` 和 `routers` 配置
  - 简化分类配置，支持 6 个固定分类
  - 支持分类级别的配置覆盖

- **改进的分类系统**
  - 统一使用 6 个固定分类: `general`, `request`, `error`, `network`, `debug`, `request_body`
  - 支持 `level: "none"` 完全禁用某个分类
  - 路径支持模板变量: `{date}`, `{time}`, `{req_id}`

### 🗑️ Removed

- **旧的多目标日志系统**
  - 移除 `targets` 配置结构
  - 移除 `routers` 路由配置
  - 移除 `multi_target.go` 和相关测试
  - 移除 `logger.go` 和相关测试
  - 移除 `schema.go` 和相关测试
  - 移除旧工厂 `factory.go` (已重写)

### 📚 Documentation

- 新增 [MIGRATION.md](MIGRATION.md) 迁移指南
- 更新 `config.example.yaml` 为新配置格式
- 本 CHANGELOG 首次创建

### 🧪 Testing

- 新增异步写入器和轮转功能的单元测试
- 更新所有配置相关测试
- 更新请求体日志测试
- 所有测试通过，覆盖率保持

---

## [2.0.0] - 2026-01-XX

_Previous version baseline. See git history for earlier changes._

---

## 升级指南

### 从 2.0 升级到 2.1

⚠️ **这是破坏性变更！** 必须更新配置文件。

1. 备份现有 `config.yaml`
2. 参考 [MIGRATION.md](MIGRATION.md) 进行配置迁移
3. 删除旧的日志目录（可选）
4. 启动服务验证

