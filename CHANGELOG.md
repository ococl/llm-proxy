# 更新日志 / Changelog

本文档记录 LLM Proxy 的所有重要变更。
This document records all notable changes to LLM Proxy.

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，
遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [1.0.0] - 2026-01-13

### 新增 / Added

#### 核心功能 / Core Features

- **统一 API Key / Unified API Key**
  - 用户只需配置一个端点和密钥，代理自动处理后端认证
  - Users only need to configure one endpoint and key, proxy handles backend authentication automatically

- **多对多模型别名 / Many-to-Many Model Aliases**
  - 统一不同提供商的模型命名（如 `anthropic/claude-opus-4-5`）
  - Unify model naming across different providers (e.g., `anthropic/claude-opus-4-5`)

- **多级回退策略 / Multi-Level Fallback Strategy**
  - L1：别名内后端优先级回退
  - L1: Backend priority fallback within alias
  - L2：别名间跨模型回退（通过 `alias_fallback` 配置）
  - L2: Cross-model fallback between aliases (via `alias_fallback` config)

- **负载均衡 / Load Balancing**
  - 同优先级后端自动随机分配
  - Automatic random distribution for same-priority backends

- **三级启用控制 / Three-Level Enable Control**
  - 后端级 `enabled` 开关
  - Backend-level `enabled` switch
  - 别名级 `enabled` 开关
  - Alias-level `enabled` switch
  - 路由级 `enabled` 开关
  - Route-level `enabled` switch

- **冷却机制 / Cooldown Mechanism**
  - 失败后端自动冷却，可配置时长
  - Failed backends automatically cool down with configurable duration
  - 定时清理过期冷却记录，防止内存泄漏
  - Periodic cleanup of expired cooldown records to prevent memory leaks

- **错误码通配符 / Error Code Wildcards**
  - 支持 `4xx`、`5xx` 等通配符匹配
  - Support `4xx`, `5xx` wildcard matching

- **完全透传 / Full Passthrough**
  - Headers、Body 完全透传，支持 SSE 流式响应
  - Headers and Body fully passed through, supports SSE streaming

- **配置热加载 / Hot Reload**
  - 修改配置后下次请求自动生效
  - Configuration changes take effect on next request

#### 日志系统 / Logging System

- **滚动日志 / Rolling Logs**
  - 按日期自动分割，单文件最大 100MB
  - Auto-split by date, max 100MB per file

- **敏感信息脱敏 / Sensitive Data Masking**
  - API Key 自动脱敏显示（如 `sk-ab****cdef`）
  - API Keys automatically masked (e.g., `sk-ab****cdef`)
  - 可通过 `mask_sensitive` 配置开关
  - Configurable via `mask_sensitive` option

- **性能指标 / Performance Metrics**
  - 可选记录请求耗时、后端耗时、尝试次数
  - Optional recording of request latency, backend timing, attempt count
  - 通过 `enable_metrics` 配置开关
  - Configurable via `enable_metrics` option

- **日志合并 / Log Consolidation**
  - 默认所有请求日志打印在一起，减少碎片
  - All request logs consolidated by default, reducing fragmentation
  - 可通过 `separate_files` 切换为独立文件模式
  - Can switch to separate files mode via `separate_files` option

- **中文日志 / Chinese Logs**
  - 所有日志消息使用中文
  - All log messages in Chinese

#### API 端点 / API Endpoints

- **健康检查 / Health Check**
  - `/health` 和 `/healthz`（K8s 兼容）
  - `/health` and `/healthz` (K8s compatible)

- **模型列表 / Model List**
  - `/v1/models` 和 `/models`
  - `/v1/models` and `/models`

#### 构建与部署 / Build & Deployment

- **多平台支持 / Multi-Platform Support**
  - Windows (amd64/arm64)
  - Linux (amd64/arm64)
  - macOS (amd64/arm64)

- **构建脚本 / Build Scripts**
  - `build.bat` (Windows)
  - `Makefile` (Linux/macOS)

- **GitHub CI/CD**
  - main 分支：测试 + 构建（产物可下载）
  - main branch: test + build (artifacts downloadable)
  - tag 推送：测试 + 构建 + 自动发布 Release
  - tag push: test + build + auto release

#### 测试 / Testing

- **单元测试 / Unit Tests**
  - 32 个测试用例覆盖核心逻辑
  - 32 test cases covering core logic
  - 测试模式下不产生日志文件
  - No log files generated in test mode

### 安全 / Security

- **循环回退检测 / Circular Fallback Detection**
  - 防止 A→B→A 死循环配置
  - Prevents A→B→A infinite loop configurations

- **API Key 验证 / API Key Validation**
  - 可选的统一 API Key 验证
  - Optional unified API Key validation
  - 空配置时跳过验证（向后兼容）
  - Skips validation when empty (backward compatible)

---

## 版本说明 / Version Notes

### 语义化版本 / Semantic Versioning

- **主版本号 / Major**: 不兼容的 API 变更 / Incompatible API changes
- **次版本号 / Minor**: 向后兼容的功能新增 / Backward compatible features
- **修订号 / Patch**: 向后兼容的问题修复 / Backward compatible bug fixes

### 变更类型 / Change Types

- **新增 / Added**: 新功能 / New features
- **变更 / Changed**: 现有功能的变更 / Changes to existing features
- **弃用 / Deprecated**: 即将移除的功能 / Features to be removed
- **移除 / Removed**: 已移除的功能 / Removed features
- **修复 / Fixed**: Bug 修复 / Bug fixes
- **安全 / Security**: 安全相关修复 / Security fixes
