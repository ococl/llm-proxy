package logging

import (
	"fmt"
	"strings"
	"testing"
)

func TestCompareJSON_NoChanges(t *testing.T) {
	original := map[string]interface{}{
		"model":       "gpt-4",
		"temperature": 0.7,
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "hello"},
		},
	}

	modified := map[string]interface{}{
		"model":       "gpt-4",
		"temperature": 0.7,
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "hello"},
		},
	}

	result := CompareJSON(original, modified, nil)

	if !result.IsEmpty() {
		t.Errorf("期望无差异，但得到: %+v", result)
	}
}

func TestCompareJSON_ModelChange(t *testing.T) {
	original := map[string]interface{}{
		"model":       "gpt-4",
		"temperature": 0.7,
	}

	modified := map[string]interface{}{
		"model":       "gpt-3.5-turbo",
		"temperature": 0.7,
	}

	result := CompareJSON(original, modified, DefaultDiffOptions())

	if !result.IsEmpty() {
		t.Errorf("model 字段应该被忽略，但得到差异: %+v", result)
	}
}

func TestCompareJSON_AddedFields(t *testing.T) {
	original := map[string]interface{}{
		"model":       "gpt-4",
		"temperature": 0.7,
	}

	modified := map[string]interface{}{
		"model":       "gpt-4",
		"temperature": 0.7,
		"max_tokens":  100,
		"stream":      true,
	}

	result := CompareJSON(original, modified, nil)

	if len(result.Added) != 2 {
		t.Errorf("期望 2 个新增字段，实际 %d", len(result.Added))
	}

	if result.Added["max_tokens"] != 100 {
		t.Errorf("期望 max_tokens=100，实际 %v", result.Added["max_tokens"])
	}

	if result.Added["stream"] != true {
		t.Errorf("期望 stream=true，实际 %v", result.Added["stream"])
	}
}

func TestCompareJSON_RemovedFields(t *testing.T) {
	original := map[string]interface{}{
		"model":             "gpt-4",
		"temperature":       0.7,
		"frequency_penalty": 0.5,
		"presence_penalty":  0.3,
	}

	modified := map[string]interface{}{
		"model":       "gpt-4",
		"temperature": 0.7,
	}

	result := CompareJSON(original, modified, nil)

	if len(result.Removed) != 2 {
		t.Errorf("期望 2 个删除字段，实际 %d", len(result.Removed))
	}

	if result.Removed["frequency_penalty"] != 0.5 {
		t.Errorf("期望 frequency_penalty=0.5，实际 %v", result.Removed["frequency_penalty"])
	}

	if result.Removed["presence_penalty"] != 0.3 {
		t.Errorf("期望 presence_penalty=0.3，实际 %v", result.Removed["presence_penalty"])
	}
}

func TestCompareJSON_ModifiedFields(t *testing.T) {
	original := map[string]interface{}{
		"model":       "gpt-4",
		"temperature": 0.7,
		"max_tokens":  100,
	}

	modified := map[string]interface{}{
		"model":       "gpt-4",
		"temperature": 0.9,
		"max_tokens":  200,
	}

	result := CompareJSON(original, modified, nil)

	if len(result.Modified) != 2 {
		t.Errorf("期望 2 个修改字段，实际 %d", len(result.Modified))
	}

	tempDiff := result.Modified["temperature"]
	if tempDiff.Old != 0.7 || tempDiff.New != 0.9 {
		t.Errorf("temperature 差异错误: old=%v, new=%v", tempDiff.Old, tempDiff.New)
	}

	tokensDiff := result.Modified["max_tokens"]
	if tokensDiff.Old != 100 || tokensDiff.New != 200 {
		t.Errorf("max_tokens 差异错误: old=%v, new=%v", tokensDiff.Old, tokensDiff.New)
	}
}

func TestCompareJSON_ComplexScenario(t *testing.T) {
	original := map[string]interface{}{
		"model":             "gpt-4",
		"temperature":       0.7,
		"frequency_penalty": 0.5,
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "hello"},
		},
	}

	modified := map[string]interface{}{
		"model":       "gpt-3.5-turbo",
		"temperature": 0.9,
		"max_tokens":  100,
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "hello"},
		},
	}

	result := CompareJSON(original, modified, DefaultDiffOptions())

	if len(result.Added) != 1 {
		t.Errorf("期望 1 个新增字段，实际 %d", len(result.Added))
	}

	if len(result.Removed) != 1 {
		t.Errorf("期望 1 个删除字段，实际 %d", len(result.Removed))
	}

	if len(result.Modified) != 1 {
		t.Errorf("期望 1 个修改字段，实际 %d (model 应被忽略)", len(result.Modified))
	}
}

func TestDiffResult_ToJSON(t *testing.T) {
	result := &DiffResult{
		Added: map[string]interface{}{
			"max_tokens": 100,
		},
		Removed: map[string]interface{}{
			"frequency_penalty": 0.5,
		},
		Modified: map[string]DiffValue{
			"temperature": {Old: 0.7, New: 0.9},
		},
	}

	jsonStr, err := result.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON 失败: %v", err)
	}

	if jsonStr == "" {
		t.Error("ToJSON 返回空字符串")
	}
}

func TestDiffResult_ToJSON_Empty(t *testing.T) {
	result := &DiffResult{
		Added:    make(map[string]interface{}),
		Removed:  make(map[string]interface{}),
		Modified: make(map[string]DiffValue),
	}

	jsonStr, err := result.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON 失败: %v", err)
	}

	expected := "# 无差异：client_request 和 upstream_request 完全一致（忽略预期差异字段）"
	if jsonStr != expected {
		t.Errorf("期望 %q，实际 %q", expected, jsonStr)
	}
}

func TestDiffResult_ToSummary(t *testing.T) {
	result := &DiffResult{
		Added: map[string]interface{}{
			"max_tokens": 100,
			"stream":     true,
		},
		Removed: map[string]interface{}{
			"frequency_penalty": 0.5,
		},
		Modified: map[string]DiffValue{
			"temperature": {Old: 0.7, New: 0.9},
		},
	}

	summary := result.ToSummary()

	if summary == "" {
		t.Error("ToSummary 返回空字符串")
	}

	if summary == "无差异" {
		t.Error("ToSummary 不应返回'无差异'")
	}
}

func TestDiffResult_ToSummary_Empty(t *testing.T) {
	result := &DiffResult{
		Added:    make(map[string]interface{}),
		Removed:  make(map[string]interface{}),
		Modified: make(map[string]DiffValue),
	}

	summary := result.ToSummary()

	if summary != "无差异" {
		t.Errorf("期望'无差异'，实际 %q", summary)
	}
}

func TestSummarizeValue_LargeArray(t *testing.T) {
	options := &DiffOptions{
		MaxValueSize:          1000,
		ArraySummaryThreshold: 5,
	}

	largeArray := make([]interface{}, 10)
	for i := 0; i < 10; i++ {
		largeArray[i] = map[string]interface{}{"role": "user", "content": "test"}
	}

	result := summarizeValue(largeArray, options)

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("期望 map 类型，实际 %T", result)
	}

	if resultMap["_summary"] != "数组过大已摘要" {
		t.Errorf("期望摘要标记，实际 %v", resultMap["_summary"])
	}
	if resultMap["_length"] != 10 {
		t.Errorf("期望长度 10，实际 %v", resultMap["_length"])
	}
}

func TestSummarizeValue_SmallArray(t *testing.T) {
	options := &DiffOptions{
		MaxValueSize:          1000,
		ArraySummaryThreshold: 5,
	}

	smallArray := make([]interface{}, 3)
	for i := 0; i < 3; i++ {
		smallArray[i] = map[string]interface{}{"role": "user", "content": "test"}
	}

	result := summarizeValue(smallArray, options)

	resultArray, ok := result.([]interface{})
	if !ok {
		t.Fatalf("期望数组类型，实际 %T", result)
	}

	if len(resultArray) != 3 {
		t.Errorf("期望 3 个元素，实际 %d", len(resultArray))
	}
}

func TestSummarizeValue_LargeValue(t *testing.T) {
	options := &DiffOptions{
		MaxValueSize:          100,
		ArraySummaryThreshold: 5,
	}

	largeValue := make(map[string]interface{})
	for i := 0; i < 100; i++ {
		largeValue[fmt.Sprintf("key_%d", i)] = strings.Repeat("x", 10)
	}

	result := summarizeValue(largeValue, options)

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("期望 map 类型，实际 %T", result)
	}

	if resultMap["_summary"] != "值过大已截断" {
		t.Errorf("期望截断标记，实际 %v", resultMap["_summary"])
	}
}

func TestCompareJSON_SameMessagesArray(t *testing.T) {
	original := map[string]interface{}{
		"model": "gpt-4",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "hello"},
			map[string]interface{}{"role": "assistant", "content": "hi"},
		},
	}

	modified := map[string]interface{}{
		"model": "gpt-3.5-turbo",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "hello"},
			map[string]interface{}{"role": "assistant", "content": "hi"},
		},
	}

	result := CompareJSON(original, modified, DefaultDiffOptions())

	if !result.IsEmpty() {
		t.Errorf("期望无差异（messages 相同，model 已忽略），但得到: %+v", result)
	}
}

func TestCompareJSON_DifferentMessagesArray(t *testing.T) {
	original := map[string]interface{}{
		"model": "gpt-4",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "hello"},
		},
	}

	modified := map[string]interface{}{
		"model": "gpt-4",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "goodbye"},
		},
	}

	result := CompareJSON(original, modified, nil)

	if len(result.Modified) != 1 {
		t.Fatalf("期望 1 个修改字段，实际 %d", len(result.Modified))
	}

	msgDiff, ok := result.Modified["messages"]
	if !ok {
		t.Fatalf("messages 字段应该存在")
	}
	if msgDiff.Old == nil || msgDiff.New == nil {
		t.Errorf("messages 差异应该有 Old 和 New 值")
	}
}
