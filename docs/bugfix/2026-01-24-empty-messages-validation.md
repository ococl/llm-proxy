# 空消息数组验证问题修复

## 问题描述

**日期**: 2026-01-24  
**严重程度**: 高  
**影响范围**: LLM 代理请求处理

### 症状
上游 LLM 服务提供商反馈我们的代理发送了**空消息数组**，导致请求失败。

### 根本原因
在 `src/adapter/http/handler.go` 的 `extractMessages()` 函数中存在以下问题：

1. **缺少空数组检查**: 没有验证 `messages` 数组是否为空
2. **消息格式验证不严格**: 
   - 对于格式错误的消息使用 `continue` 跳过，而不是返回错误
   - `role` 字段可以缺失或为空字符串，没有强制验证
3. **可能导致的后果**: 
   - 客户端发送 `{"messages": []}` → 代理接受并转发空数组
   - 客户端发送格式错误的消息 → 被静默跳过，最终可能产生空数组

## 修复方案

### 1. 添加空数组验证

**文件**: `src/adapter/http/handler.go`  
**函数**: `extractMessages()`

```go
// 修复前
func (h *ProxyHandler) extractMessages(reqBody map[string]interface{}) ([]entity.Message, error) {
	messagesRaw, ok := reqBody["messages"].([]interface{})
	if !ok {
		return nil, domainerror.NewBadRequest("缺少 messages 字段")
	}
	// 缺少对空数组的检查

	messages := make([]entity.Message, 0, len(messagesRaw))
	for _, msgRaw := range messagesRaw {
		msgMap, ok := msgRaw.(map[string]interface{})
		if !ok {
			continue  // 问题：静默跳过错误
		}

		role, _ := msgMap["role"].(string)  // 问题：role 可以为空
		content, _ := msgMap["content"].(string)
		// ...
	}
	return messages, nil
}
```

```go
// 修复后
func (h *ProxyHandler) extractMessages(reqBody map[string]interface{}) ([]entity.Message, error) {
	messagesRaw, ok := reqBody["messages"].([]interface{})
	if !ok {
		return nil, domainerror.NewBadRequest("缺少 messages 字段")
	}

	// ✅ 添加：验证数组不能为空
	if len(messagesRaw) == 0 {
		return nil, domainerror.NewBadRequest("messages 数组不能为空")
	}

	messages := make([]entity.Message, 0, len(messagesRaw))
	for i, msgRaw := range messagesRaw {
		msgMap, ok := msgRaw.(map[string]interface{})
		if !ok {
			// ✅ 改进：返回错误而不是跳过
			return nil, domainerror.NewBadRequest(fmt.Sprintf("messages[%d] 必须是一个对象", i))
		}

		role, ok := msgMap["role"].(string)
		// ✅ 改进：强制要求 role 字段存在且非空
		if !ok || role == "" {
			return nil, domainerror.NewBadRequest(fmt.Sprintf("messages[%d] 缺少有效的 role 字段", i))
		}

		content, _ := msgMap["content"].(string)
		// ...
	}
	return messages, nil
}
```

### 2. 添加测试覆盖

**文件**: `src/adapter/http/http_test.go`  
**新增测试**: `TestExtractMessages`

测试覆盖以下场景：
- ✅ 拒绝空 `messages` 数组
- ✅ 拒绝缺少 `role` 字段的消息
- ✅ 拒绝 `role` 为空字符串的消息
- ✅ 接受有效的消息数组

## 验证结果

### 测试通过
```bash
$ cd src/adapter/http && go test -v -run TestExtractMessages
=== RUN   TestExtractMessages
=== RUN   TestExtractMessages/拒绝空_messages_数组
=== RUN   TestExtractMessages/拒绝缺少_role_的消息
=== RUN   TestExtractMessages/拒绝_role_为空字符串的消息
=== RUN   TestExtractMessages/接受有效的消息数组
--- PASS: TestExtractMessages (0.00s)
PASS
ok  	llm-proxy/adapter/http	3.232s
```

### 全量测试通过
```bash
$ cd src && go test ./...
ok  	llm-proxy/adapter/backend	0.275s
ok  	llm-proxy/adapter/http	0.383s
ok  	llm-proxy/adapter/http/middleware	0.790s
ok  	llm-proxy/application/service	0.558s
ok  	llm-proxy/application/usecase	0.362s
ok  	llm-proxy/domain/entity	0.515s
ok  	llm-proxy/domain/service	0.616s
# ... 所有测试通过
```

## 影响范围

### 向后兼容性
- ✅ **兼容性良好**: 只增加了验证逻辑，原本有效的请求不受影响
- ❌ **破坏性变更**: 之前允许的**空数组**或**格式错误的消息**现在会被拒绝
  - 这是**预期行为**，因为这些请求本身就是无效的

### 错误响应示例

**请求空数组**:
```bash
POST /v1/chat/completions
{
  "model": "gpt-4",
  "messages": []
}
```

**响应**:
```json
{
  "error": {
    "message": "messages 数组不能为空",
    "type": "client:BAD_REQUEST",
    "code": "BAD_REQUEST"
  }
}
```

**请求缺少 role**:
```bash
POST /v1/chat/completions
{
  "model": "gpt-4",
  "messages": [
    {"content": "hello"}
  ]
}
```

**响应**:
```json
{
  "error": {
    "message": "messages[0] 缺少有效的 role 字段",
    "type": "client:BAD_REQUEST",
    "code": "BAD_REQUEST"
  }
}
```

## 相关验证层

### 已存在的验证（未受影响）
1. **应用层验证** (`src/application/service/request_validator.go`):
   - ✅ 验证 `len(req.Messages()) == 0` (第 37 行)
   - ✅ 验证消息内容不为空 (第 53 行)
   - ✅ 验证 role 在允许列表中 (第 58 行)

2. **领域层验证** (`src/domain/entity/request.go`):
   - ✅ RequestBuilder.Build() 要求 `len(rb.messages) > 0` (第 345 行)

### 为何需要在 HTTP 层增加验证？

**多层防御策略**：
1. **HTTP 适配器层** (本次修复): 尽早拒绝明显无效的请求，节省处理成本
2. **应用服务层**: 验证业务逻辑规则（消息数量限制、token 限制等）
3. **领域层**: 确保实体构建时的完整性

**好处**：
- 🚀 **快速失败**: 在 JSON 解析后立即验证，避免无效数据进入业务逻辑
- 📊 **清晰错误**: 精确指出是哪个消息索引出现问题
- 🔒 **深度防御**: 即使一层验证失败，其他层仍可拦截

## 后续建议

### 短期
1. ✅ **已完成**: 添加空数组和 role 字段验证
2. ✅ **已完成**: 添加相应测试用例
3. 🔄 **建议**: 监控上游错误率，确认问题已解决

### 长期
1. **考虑添加集成测试**: 模拟完整的请求-响应流程
2. **API 文档更新**: 明确标注 `messages` 数组必须非空，每条消息必须包含 `role` 字段
3. **错误码标准化**: 统一错误消息格式，便于客户端解析

## 总结

**问题**: 代理向上游发送空消息数组  
**原因**: HTTP 适配器缺少输入验证  
**修复**: 在 `extractMessages()` 中添加空数组和 role 字段验证  
**影响**: 拒绝无效请求，保护上游服务  
**状态**: ✅ 已修复并测试通过
