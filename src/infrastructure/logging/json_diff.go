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
}

// DefaultDiffOptions 返回默认的 diff 配置
// 默认忽略预期会被修改的字段
func DefaultDiffOptions() *DiffOptions {
	return &DiffOptions{
		IgnoreFields: []string{
			"model", // model 字段会被路由重写，这是预期行为
		},
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
			result.Removed[key] = oldValue
		} else if !deepEqual(oldValue, newValue) {
			// 字段值被修改
			result.Modified[key] = DiffValue{
				Old: oldValue,
				New: newValue,
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
			result.Added[key] = newValue
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
func deepEqual(a, b interface{}) bool {
	return reflect.DeepEqual(a, b)
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
