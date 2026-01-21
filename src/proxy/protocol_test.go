package proxy

import (
	"encoding/json"
	"testing"
)

func TestConvertToAnthropic_MaxTokensTypes(t *testing.T) {
	converter := NewProtocolConverter()

	tests := []struct {
		name     string
		input    map[string]interface{}
		expected int
	}{
		{
			name: "max_tokens as float64",
			input: map[string]interface{}{
				"model":      "claude-3-opus-20240229",
				"max_tokens": float64(1000),
				"messages":   []interface{}{},
			},
			expected: 1000,
		},
		{
			name: "max_tokens as int",
			input: map[string]interface{}{
				"model":      "claude-3-opus-20240229",
				"max_tokens": 2000,
				"messages":   []interface{}{},
			},
			expected: 2000,
		},
		{
			name: "max_completion_tokens as float64",
			input: map[string]interface{}{
				"model":                 "claude-3-opus-20240229",
				"max_completion_tokens": float64(1500),
				"messages":              []interface{}{},
			},
			expected: 1500,
		},
		{
			name: "max_completion_tokens as int",
			input: map[string]interface{}{
				"model":                 "claude-3-opus-20240229",
				"max_completion_tokens": 2500,
				"messages":              []interface{}{},
			},
			expected: 2500,
		},
		{
			name: "no max_tokens - use default",
			input: map[string]interface{}{
				"model":    "claude-3-opus-20240229",
				"messages": []interface{}{},
			},
			expected: 4096,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertToAnthropic(tt.input)
			if err != nil {
				t.Fatalf("ConvertToAnthropic failed: %v", err)
			}

			var anthropicBody map[string]interface{}
			if err := json.Unmarshal(result, &anthropicBody); err != nil {
				t.Fatalf("Failed to unmarshal result: %v", err)
			}

			maxTokensFloat, ok := anthropicBody["max_tokens"].(float64)
			if !ok {
				t.Fatalf("max_tokens not found or not float64 in result")
			}
			maxTokens := int(maxTokensFloat)

			if maxTokens != tt.expected {
				t.Errorf("Expected max_tokens=%d, got %d", tt.expected, maxTokens)
			}
		})
	}
}

func TestConvertToAnthropic_StopSequences(t *testing.T) {
	converter := NewProtocolConverter()

	tests := []struct {
		name     string
		input    map[string]interface{}
		expected []string
	}{
		{
			name: "stop as string",
			input: map[string]interface{}{
				"model":      "claude-3-opus-20240229",
				"max_tokens": 1000,
				"stop":       "END",
				"messages":   []interface{}{},
			},
			expected: []string{"END"},
		},
		{
			name: "stop as string array",
			input: map[string]interface{}{
				"model":      "claude-3-opus-20240229",
				"max_tokens": 1000,
				"stop":       []string{"END", "STOP"},
				"messages":   []interface{}{},
			},
			expected: []string{"END", "STOP"},
		},
		{
			name: "stop as interface array",
			input: map[string]interface{}{
				"model":      "claude-3-opus-20240229",
				"max_tokens": 1000,
				"stop":       []interface{}{"END", "STOP"},
				"messages":   []interface{}{},
			},
			expected: []string{"END", "STOP"},
		},
		{
			name: "no stop field",
			input: map[string]interface{}{
				"model":      "claude-3-opus-20240229",
				"max_tokens": 1000,
				"messages":   []interface{}{},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertToAnthropic(tt.input)
			if err != nil {
				t.Fatalf("ConvertToAnthropic failed: %v", err)
			}

			var anthropicBody map[string]interface{}
			if err := json.Unmarshal(result, &anthropicBody); err != nil {
				t.Fatalf("Failed to unmarshal result: %v", err)
			}

			stopSequences := anthropicBody["stop_sequences"]
			if tt.expected == nil {
				if stopSequences != nil {
					t.Errorf("Expected stop_sequences=nil, got %v", stopSequences)
				}
				return
			}

			if stopSequences == nil {
				t.Fatalf("Expected stop_sequences=%v, got nil", tt.expected)
			}

			actual, ok := stopSequences.([]interface{})
			if !ok {
				t.Fatalf("stop_sequences not []interface{}, got %T", stopSequences)
			}

			// Convert []interface{} to []string for comparison
			actualStr := make([]string, len(actual))
			for i, item := range actual {
				if s, ok := item.(string); ok {
					actualStr[i] = s
				} else {
					t.Fatalf("stop_sequences item %d is not string, got %T", i, item)
				}
			}

			if len(actualStr) != len(tt.expected) {
				t.Errorf("Expected %d stop sequences, got %d", len(tt.expected), len(actualStr))
			}

			for i, expected := range tt.expected {
				if i >= len(actualStr) || actualStr[i] != expected {
					t.Errorf("Expected stop_sequences[%d]=%s, got %s", i, expected, actualStr[i])
				}
			}
		})
	}
}
