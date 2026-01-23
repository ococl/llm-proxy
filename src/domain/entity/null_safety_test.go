package entity

import (
	"encoding/json"
	"testing"
)

func TestResponseBuilder_NullSafety_UpstreamNullChoices(t *testing.T) {
	t.Run("Unmarshal upstream null choices", func(t *testing.T) {
		upstreamJSON := `{"id":"test","model":"gpt-4","choices":null}`

		var data map[string]interface{}
		if err := json.Unmarshal([]byte(upstreamJSON), &data); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		choicesRaw, exists := data["choices"]
		if !exists {
			t.Fatal("choices field should exist")
		}

		if choicesRaw != nil {
			t.Errorf("Expected nil, got %v (type: %T)", choicesRaw, choicesRaw)
		}

		builder := NewResponseBuilder().
			ID("test").
			Model("gpt-4")

		if choices, ok := data["choices"].([]interface{}); ok {
			builder.Choices(convertChoices(choices))
		} else {
			builder.Choices(nil)
		}

		resp := builder.BuildUnsafe()
		if resp == nil {
			t.Fatal("BuildUnsafe should never return nil")
		}

		if resp.Choices == nil {
			t.Error("Choices should never be nil after BuildUnsafe")
		}

		jsonData, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}

		jsonStr := string(jsonData)
		if contains(jsonStr, `"choices":null`) {
			t.Errorf("JSON should not contain null choices: %s", jsonStr)
		}
	})
}

func TestResponseBuilder_NullSafety_DirectNilAssignment(t *testing.T) {
	t.Run("Direct nil assignment to Choices", func(t *testing.T) {
		builder := NewResponseBuilder().
			ID("test").
			Model("gpt-4").
			Choices(nil)

		resp := builder.BuildUnsafe()

		if resp.Choices == nil {
			t.Error("Choices should be empty array, not nil")
		}

		if len(resp.Choices) != 0 {
			t.Errorf("Expected empty array, got length %d", len(resp.Choices))
		}

		jsonData, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(jsonData, &result); err != nil {
			t.Fatalf("Failed to unmarshal result: %v", err)
		}

		choicesVal, exists := result["choices"]
		if !exists {
			t.Error("choices field should exist in JSON")
		}

		if choicesVal == nil {
			t.Error("choices should not be null in JSON")
		}

		choicesArray, ok := choicesVal.([]interface{})
		if !ok {
			t.Errorf("choices should be array, got %T", choicesVal)
		}

		if len(choicesArray) != 0 {
			t.Errorf("choices array should be empty, got length %d", len(choicesArray))
		}
	})
}

func TestResponseBuilder_NullSafety_BuildUnsafeWithoutChoices(t *testing.T) {
	t.Run("BuildUnsafe without calling Choices", func(t *testing.T) {
		builder := NewResponseBuilder().
			ID("test").
			Model("gpt-4")

		resp := builder.BuildUnsafe()

		if resp.Choices == nil {
			t.Error("Choices should be initialized as empty array by constructor")
		}

		jsonData, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}

		if contains(string(jsonData), `"choices":null`) {
			t.Errorf("JSON should not contain null choices: %s", string(jsonData))
		}
	})
}

func TestChoice_NullSafety_DeltaField(t *testing.T) {
	t.Run("Choice with nil Delta", func(t *testing.T) {
		choice := Choice{
			Index:        0,
			Delta:        nil,
			FinishReason: "stop",
		}

		jsonData, err := json.Marshal(choice)
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(jsonData, &result); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		if _, exists := result["delta"]; exists {
			t.Error("delta field should be omitted when nil")
		}
	})

	t.Run("Choice with empty Delta", func(t *testing.T) {
		choice := Choice{
			Index: 0,
			Delta: &Message{
				Role:    "",
				Content: "",
			},
			FinishReason: "stop",
		}

		jsonData, err := json.Marshal(choice)
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(jsonData, &result); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		deltaVal, exists := result["delta"]
		if !exists {
			t.Error("delta field should exist even when empty")
		}

		deltaMap, ok := deltaVal.(map[string]interface{})
		if !ok {
			t.Fatalf("delta should be object, got %T", deltaVal)
		}

		if _, hasRole := deltaMap["role"]; hasRole {
			t.Error("empty role should be omitted from delta")
		}
	})
}

func convertChoices(choices []interface{}) []Choice {
	result := make([]Choice, 0, len(choices))
	for _, c := range choices {
		if choiceMap, ok := c.(map[string]interface{}); ok {
			choice := Choice{}
			if idx, ok := choiceMap["index"].(float64); ok {
				choice.Index = int(idx)
			}
			if fr, ok := choiceMap["finish_reason"].(string); ok {
				choice.FinishReason = fr
			}
			result = append(result, choice)
		}
	}
	return result
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
