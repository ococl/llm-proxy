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
├── infrastructure/   # 基础设施层（技术实现）🚧
│   ├── http/         # HTTP 服务器/客户端 ⚠️ 部分完成
│   ├── config/       # 配置加载器 ❌
│   ├── logging/      # 日志工厂 ❌
│   └── di/           # 依赖注入 ❌
└── main.go           # 应用入口 🚧 需重构
```

## 注意事项

1. **依赖方向**: 依赖应该从外向内（Infrastructure → Adapter → Application → Domain）
2. **端口接口**: Domain 层定义接口，外层实现接口
3. **测试优先**: 每个新模块都应该有对应的测试
4. **渐进式重构**: 保持代码可编译可运行，逐步替换旧代码

## 下一步行动

1. 实现 `infrastructure/http/server.go`
2. 整合 `config/` 到 `infrastructure/config/`
3. 整合 `logging/` 到 `infrastructure/logging/`
4. 重构 `main.go` 使用新的 Infrastructure 层
5. 删除旧代码包
6. 增加测试覆盖率

---

**创建时间**: 2026-01-22  
**最后更新**: 2026-01-22
