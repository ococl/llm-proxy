package logging

import (
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
