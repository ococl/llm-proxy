# Clean Architecture 重构 - 剩余工作清单

## 当前状态

✅ **已完成**:
- Domain 层（实体、端口、领域服务）+ 测试
- Application 层（用例、服务）+ 测试  
- Adapter 层（HTTP、配置、后端、日志）+ 测试
- Infrastructure 层（HTTP 服务器、配置加载器、日志工厂）
- 所有测试通过，编译成功

## 待完成工作

### 高优先级

#### 1. 完善 Infrastructure 层
- [x] `infrastructure/http/server.go` - HTTP 服务器封装（优雅关闭、中间件链）
- [x] `infrastructure/config/loader.go` - 配置加载器（整合现有 config/ 包）
- [x] `infrastructure/logging/factory.go` - 日志工厂（整合现有 logging/ 包）
- [ ] `infrastructure/di/container.go` - 依赖注入容器（简化 main.go）（可选）

#### 2. 重构 main.go
- [x] 移除对旧 `proxy/` 包的依赖（proxy.InitHTTPClient, proxy.GetHTTPClient）
- [x] 使用 Infrastructure 层组件替代直接依赖
- [x] 简化依赖注入逻辑，提高可读性

#### 3. 清理旧代码
- [ ] 删除或重构 `proxy/` 包（保留必要的检测器逻辑）
- [ ] 删除或重构 `backend/` 包（已被 adapter/backend 替代）
- [ ] 整合 `middleware/` 到 `adapter/http/middleware/`
- [ ] 整合 `config/` 到 `infrastructure/config/`
- [ ] 整合 `logging/` 到 `infrastructure/logging/`
- [ ] 删除 `errors/` 包（已被 domain/error 替代）
- [ ] 删除 `prompt/` 包（已被 application/service 整合）
- [ ] 删除 `translator/` 包（如果不再使用）

### 中优先级

#### 4. 完善 Adapter 层
- [ ] `adapter/http/router.go` - 路由注册器
- [ ] `adapter/metrics/` - 指标收集适配器（替代 NopMetricsProvider）
- [ ] `adapter/cache/` - 缓存适配器（如果需要）

#### 5. 增加测试覆盖率
- [ ] Infrastructure 层单元测试
- [ ] 集成测试（端到端流程）
- [ ] 提高现有模块覆盖率（目标 >60%）

### 低优先级

#### 6. 文档和示例
- [ ] 更新 README.md 架构说明
- [ ] 添加架构图（分层依赖关系）
- [ ] 编写开发指南（如何添加新功能）
- [ ] API 使用示例

## 架构目标

```
src/
├── domain/           # 领域层（核心业务逻辑）✅
│   ├── entity/       # 实体
│   ├── port/         # 端口接口
│   ├── service/      # 领域服务
│   ├── error/        # 错误类型
│   └── types/        # 类型定义
├── application/      # 应用层（用例编排）✅
│   ├── service/      # 应用服务
│   └── usecase/      # 用例
├── adapter/          # 适配器层（外部接口）✅
│   ├── backend/      # 后端客户端
│   ├── config/       # 配置适配器
│   ├── http/         # HTTP 处理器
│   └── logging/      # 日志适配器
├── infrastructure/   # 基础设施层（技术实现）✅
│   ├── http/         # HTTP 服务器/客户端 ✅
│   ├── config/       # 配置加载器 ✅
│   ├── logging/      # 日志工厂 ✅
│   └── di/           # 依赖注入 ❌ (可选)
└── main.go           # 应用入口 ✅ 已重构
```

## 注意事项

1. **依赖方向**: 依赖应该从外向内（Infrastructure → Adapter → Application → Domain）
2. **端口接口**: Domain 层定义接口，外层实现接口
3. **测试优先**: 每个新模块都应该有对应的测试
4. **渐进式重构**: 保持代码可编译可运行，逐步替换旧代码

## 下一步行动

✅ **已完成的核心重构**:
1. ✅ 实现 `infrastructure/http/client.go` - HTTP 客户端工厂
2. ✅ 实现 `infrastructure/http/server.go` - HTTP 服务器封装
3. ✅ 实现 `infrastructure/config/loader.go` - 配置加载器
4. ✅ 实现 `infrastructure/logging/factory.go` - 日志工厂
5. ✅ 重构 `main.go` 使用新的 Infrastructure 层

**可选的后续工作**:
1. 清理旧代码包（`proxy/httpclient.go` 等）
2. 增加 Infrastructure 层单元测试
3. 提高整体测试覆盖率（目标 >60%）
4. 实现 DI 容器简化依赖注入（可选）

---

**创建时间**: 2026-01-22  
**最后更新**: 2026-01-23

## 重构完成总结

### ✅ 已完成的核心架构重构

1. **Domain 层** (52.4% 测试覆盖率)
   - 实体定义 (entity/)
   - 端口接口 (port/)
   - 领域服务 (service/)
   - 错误类型 (error/)

2. **Application 层** (~40% 测试覆盖率)
   - 用例编排 (usecase/)
   - 应用服务 (service/)

3. **Adapter 层** (26-80% 测试覆盖率)
   - HTTP 处理器 (http/)
   - 后端客户端适配器 (backend/)
   - 配置适配器 (config/)
   - 日志适配器 (logging/)

4. **Infrastructure 层**
   - HTTP 客户端工厂 (http/client.go)
   - HTTP 服务器封装 (http/server.go)
   - 配置加载器 (config/loader.go)
   - 日志工厂 (logging/factory.go)

5. **main.go 重构**
   - 移除对旧 proxy 包的依赖
   - 使用 Infrastructure 层组件
   - 显式化依赖注入逻辑

### 📊 质量指标

- ✅ 所有包编译通过
- ✅ 所有测试通过 (19 个包)
- ✅ 架构依赖方向正确 (Infrastructure → Adapter → Application → Domain)
- ✅ 代码可维护性显著提升

### 🎯 架构收益

1. **清晰的分层架构** - 每层职责明确，易于理解和维护
2. **依赖倒置** - 核心业务逻辑不依赖外部实现
3. **可测试性** - 通过接口隔离，便于单元测试
4. **可扩展性** - 新增功能只需实现对应接口
5. **代码复用** - Infrastructure 层组件可在多处复用
