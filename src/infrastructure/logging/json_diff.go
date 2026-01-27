package logging

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
)

// DiffResult 表示 JSON diff 的结果
type DiffResult struct {
	Added    map[string]interface{} `json:"added,omitempty"`    // 新增的字段
	Removed  map[string]interface{} `json:"removed,omitempty"`  // 删除的字段
	Modified map[string]DiffValue   `json:"modified,omitempty"` // 修改的字段
}

// DiffValue 表示修改字段的前后值
type DiffValue struct {
	Old interface{} `json:"old"` // 原始值
	New interface{} `json:"new"` // 新值
}

// DiffOptions 配置 diff 行为
type DiffOptions struct {
	// IgnoreFields 忽略的字段列表（预期的差异）
	IgnoreFields []string

	// MaxValueSize 单个字段值的最大记录大小（字节）
	// 超过此大小的值将被截断为摘要
	// 默认值：1000 字节
	MaxValueSize int

	// ArraySummaryThreshold 数组摘要阈值
	// 当数组元素数量超过此值时，只记录摘要信息
	// 默认值：5 个元素
	ArraySummaryThreshold int
}

// DefaultDiffOptions 返回默认的 diff 配置
// 默认忽略预期会被修改的字段
func DefaultDiffOptions() *DiffOptions {
	return &DiffOptions{
		IgnoreFields: []string{
			"model", // model 字段会被路由重写，这是预期行为
		},
		MaxValueSize:          1000, // 单个字段值最大 1000 字节
		ArraySummaryThreshold: 5,    // 数组超过 5 个元素时使用摘要
	}
}

// CompareJSON 比较两个 JSON 对象，返回差异
// original: 原始 JSON 对象（client_request）
// modified: 修改后的 JSON 对象（upstream_request）
// options: diff 配置选项
func CompareJSON(original, modified map[string]interface{}, options *DiffOptions) *DiffResult {
	if options == nil {
		options = DefaultDiffOptions()
	}

	result := &DiffResult{
		Added:    make(map[string]interface{}),
		Removed:  make(map[string]interface{}),
		Modified: make(map[string]DiffValue),
	}

	// 构建忽略字段的快速查找表
	ignoreMap := make(map[string]bool)
	for _, field := range options.IgnoreFields {
		ignoreMap[field] = true
	}

	// 检查删除和修改的字段
	for key, oldValue := range original {
		// 跳过忽略的字段
		if ignoreMap[key] {
			continue
		}

		newValue, exists := modified[key]
		if !exists {
			// 字段被删除
			result.Removed[key] = summarizeValue(oldValue, options)
		} else if !deepEqual(oldValue, newValue) {
			// 字段值被修改
			result.Modified[key] = DiffValue{
				Old: summarizeValue(oldValue, options),
				New: summarizeValue(newValue, options),
			}
		}
	}

	// 检查新增的字段
	for key, newValue := range modified {
		// 跳过忽略的字段
		if ignoreMap[key] {
			continue
		}

		if _, exists := original[key]; !exists {
			// 字段被新增
			result.Added[key] = summarizeValue(newValue, options)
		}
	}

	return result
}

// IsEmpty 检查 diff 结果是否为空（没有差异）
func (r *DiffResult) IsEmpty() bool {
	return len(r.Added) == 0 && len(r.Removed) == 0 && len(r.Modified) == 0
}

// ToJSON 将 diff 结果转换为格式化的 JSON 字符串
func (r *DiffResult) ToJSON() (string, error) {
	if r.IsEmpty() {
		return "# 无差异：client_request 和 upstream_request 完全一致（忽略预期差异字段）", nil
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化 diff 结果失败: %w", err)
	}

	return string(data), nil
}

// ToSummary 生成人类可读的差异摘要
func (r *DiffResult) ToSummary() string {
	if r.IsEmpty() {
		return "无差异"
	}

	summary := ""

	if len(r.Added) > 0 {
		keys := sortedKeys(r.Added)
		summary += fmt.Sprintf("新增字段 (%d): %v\n", len(r.Added), keys)
	}

	if len(r.Removed) > 0 {
		keys := sortedKeys(r.Removed)
		summary += fmt.Sprintf("删除字段 (%d): %v\n", len(r.Removed), keys)
	}

	if len(r.Modified) > 0 {
		keys := make([]string, 0, len(r.Modified))
		for k := range r.Modified {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		summary += fmt.Sprintf("修改字段 (%d): %v\n", len(r.Modified), keys)
	}

	return summary
}

// deepEqual 深度比较两个值是否相等
// 使用 JSON 序列化比较，避免 reflect.DeepEqual 对 map/slice 指针的误判
func deepEqual(a, b interface{}) bool {
	// 处理 nil 情况
	if a == nil || b == nil {
		return a == b
	}

	// 对于复杂类型（map、slice），使用 JSON 序列化比较
	// 这样可以正确比较内容相同但指针不同的对象
	aJSON, aErr := json.Marshal(a)
	bJSON, bErr := json.Marshal(b)

	// 如果序列化失败，回退到 reflect.DeepEqual
	if aErr != nil || bErr != nil {
		return reflect.DeepEqual(a, b)
	}

	// 比较 JSON 字符串
	return string(aJSON) == string(bJSON)
}

// sortedKeys 返回 map 的排序键列表
func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// summarizeValue 对值进行摘要处理，防止 diff 文件过大
func summarizeValue(value interface{}, options *DiffOptions) interface{} {
	if value == nil {
		return nil
	}

	if arr, ok := value.([]interface{}); ok {
		return summarizeArray(arr, options)
	}

	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return value
	}

	if len(jsonBytes) <= options.MaxValueSize {
		return value
	}

	return map[string]interface{}{
		"_summary": "值过大已截断",
		"_type":    fmt.Sprintf("%T", value),
		"_size":    len(jsonBytes),
		"_preview": truncateString(string(jsonBytes), 200),
	}
}

// summarizeArray 对数组进行摘要处理
func summarizeArray(arr []interface{}, options *DiffOptions) interface{} {
	if len(arr) <= options.ArraySummaryThreshold {
		return arr
	}

	return map[string]interface{}{
		"_summary":     "数组过大已摘要",
		"_length":      len(arr),
		"_first_items": arr[:min(3, len(arr))],
	}
}

// truncateString 截断字符串到指定长度
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
