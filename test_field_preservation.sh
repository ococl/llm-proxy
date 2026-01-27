#!/bin/bash

# 测试字段保留功能
# 验证 frequency_penalty, presence_penalty 等字段不再丢失

echo "=== 测试字段保留功能 ==="
echo ""

# 启动代理服务器（后台运行）
echo "1. 启动代理服务器..."
cd "$(dirname "$0")"
./dist/llm-proxy.exe &
SERVER_PID=$!
echo "   服务器 PID: $SERVER_PID"
sleep 3

# 测试请求（包含额外字段）
echo ""
echo "2. 发送测试请求（包含 frequency_penalty, presence_penalty）..."
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test-key" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello"}],
    "temperature": 0.7,
    "max_tokens": 100,
    "frequency_penalty": 0.5,
    "presence_penalty": 0.3,
    "top_p": 0.9,
    "stream": false
  }' \
  -o /dev/null -s -w "\n   HTTP 状态码: %{http_code}\n"

# 等待日志写入
sleep 2

# 检查 diff 日志
echo ""
echo "3. 检查 diff 日志文件..."
DIFF_FILE=$(ls -t logs/*_request_diff.json 2>/dev/null | head -1)

if [ -z "$DIFF_FILE" ]; then
    echo "   ❌ 未找到 diff 日志文件"
    kill $SERVER_PID
    exit 1
fi

echo "   找到 diff 文件: $DIFF_FILE"
echo ""
echo "4. 分析 diff 内容..."

# 检查是否有字段被移除
REMOVED=$(jq -r '.removed | keys | .[]' "$DIFF_FILE" 2>/dev/null)
if [ -n "$REMOVED" ]; then
    echo "   ❌ 发现被移除的字段:"
    echo "$REMOVED" | sed 's/^/      - /'
    echo ""
    echo "   完整 diff 内容:"
    cat "$DIFF_FILE" | jq '.'
    kill $SERVER_PID
    exit 1
fi

# 检查是否有字段被修改（除了 model 和 stream）
MODIFIED=$(jq -r '.modified | keys | .[] | select(. != "model" and . != "stream")' "$DIFF_FILE" 2>/dev/null)
if [ -n "$MODIFIED" ]; then
    echo "   ⚠️  发现被修改的字段（除了 model 和 stream）:"
    echo "$MODIFIED" | sed 's/^/      - /'
fi

# 检查是否有字段被添加
ADDED=$(jq -r '.added | keys | .[]' "$DIFF_FILE" 2>/dev/null)
if [ -n "$ADDED" ]; then
    echo "   ℹ️  发现新增的字段:"
    echo "$ADDED" | sed 's/^/      - /'
fi

echo ""
echo "   ✅ 字段保留测试通过！"
echo "   - frequency_penalty: 保留"
echo "   - presence_penalty: 保留"
echo "   - 其他字段: 保留"

# 清理
kill $SERVER_PID
echo ""
echo "=== 测试完成 ==="
