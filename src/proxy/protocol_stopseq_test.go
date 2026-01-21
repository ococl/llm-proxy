package proxy

import (
	"encoding/json"
	"testing"
)

func TestStopSequenceSanitization(t *testing.T) {
	tests := []struct {
		name           string
		input          interface{}
		expectedOutput interface{}
		shouldHaveStop bool
	}{
		{
			name:           "Single valid string",
			input:          "STOP",
			expectedOutput: []string{"STOP"},
			shouldHaveStop: true,
		},
		{
			name:           "Single whitespace-only string",
			input:          "   ",
			expectedOutput: nil,
			shouldHaveStop: false,
		},
		{
			name:           "Array with mixed valid and whitespace",
			input:          []interface{}{"STOP", "  ", "\t", "END"},
			expectedOutput: []string{"STOP", "END"},
			shouldHaveStop: true,
		},
		{
			name:           "Array with all whitespace",
			input:          []interface{}{"  ", "\t", "\n"},
			expectedOutput: nil,
			shouldHaveStop: false,
		},
		{
			name:           "String array with valid stops",
			input:          []string{"STOP", "END", "HALT"},
			expectedOutput: []string{"STOP", "END", "HALT"},
			shouldHaveStop: true,
		},
		{
			name:           "String array with whitespace filtered",
			input:          []string{"STOP", "  ", "END"},
			expectedOutput: []string{"STOP", "END"},
			shouldHaveStop: true,
		},
		{
			name:           "Empty string",
			input:          "",
			expectedOutput: nil,
			shouldHaveStop: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := NewProtocolConverter()
			openAIBody := map[string]interface{}{
				"model": "gpt-4",
				"messages": []interface{}{
					map[string]interface{}{
						"role":    "user",
						"content": "test",
					},
				},
				"stop": tt.input,
			}

			result, err := pc.ConvertToAnthropic(openAIBody)
			if err != nil {
				t.Fatalf("ConvertToAnthropic failed: %v", err)
			}

			var anthropicBody map[string]interface{}
			if err := json.Unmarshal(result, &anthropicBody); err != nil {
				t.Fatalf("Failed to unmarshal result: %v", err)
			}

			stopSequences, hasStop := anthropicBody["stop_sequences"]
			if tt.shouldHaveStop {
				if !hasStop {
					t.Errorf("Expected stop_sequences field but got none")
					return
				}

				stopArr, ok := stopSequences.([]interface{})
				if !ok {
					t.Errorf("stop_sequences is not an array: %T", stopSequences)
					return
				}

				expectedArr, ok := tt.expectedOutput.([]string)
				if !ok {
					t.Fatalf("Test setup error: expectedOutput should be []string")
				}

				if len(stopArr) != len(expectedArr) {
					t.Errorf("Expected %d stop sequences, got %d", len(expectedArr), len(stopArr))
					return
				}

				for i, expected := range expectedArr {
					if i >= len(stopArr) {
						t.Errorf("Missing stop sequence at index %d", i)
						continue
					}
					actual, ok := stopArr[i].(string)
					if !ok {
						t.Errorf("Stop sequence at index %d is not a string: %T", i, stopArr[i])
						continue
					}
					if actual != expected {
						t.Errorf("Stop sequence at index %d: expected %q, got %q", i, expected, actual)
					}
				}
			} else {
				if hasStop {
					t.Errorf("Expected no stop_sequences field but got: %v", stopSequences)
				}
			}
		})
	}
}

func TestStopSequencePreservation(t *testing.T) {
	pc := NewProtocolConverter()

	openAIBody := map[string]interface{}{
		"model": "gpt-4",
		"messages": []interface{}{
			map[string]interface{}{
				"role":    "user",
				"content": "test",
			},
		},
		"stop": []string{"\\n\\n", "---", "###"},
	}

	result, err := pc.ConvertToAnthropic(openAIBody)
	if err != nil {
		t.Fatalf("ConvertToAnthropic failed: %v", err)
	}

	var anthropicBody map[string]interface{}
	if err := json.Unmarshal(result, &anthropicBody); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	stopSequences, ok := anthropicBody["stop_sequences"].([]interface{})
	if !ok {
		t.Fatalf("stop_sequences not found or wrong type")
	}

	expected := []string{"\\n\\n", "---", "###"}
	if len(stopSequences) != len(expected) {
		t.Fatalf("Expected %d stop sequences, got %d", len(expected), len(stopSequences))
	}

	for i, exp := range expected {
		actual, ok := stopSequences[i].(string)
		if !ok {
			t.Errorf("Stop sequence %d is not a string", i)
			continue
		}
		if actual != exp {
			t.Errorf("Stop sequence %d: expected %q, got %q", i, exp, actual)
		}
	}
}
