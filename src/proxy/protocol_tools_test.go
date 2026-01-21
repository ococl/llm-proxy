package proxy

import (
	"testing"
)

func TestConvertTools(t *testing.T) {
	converter := NewProtocolConverter()

	tests := []struct {
		name          string
		tools         []interface{}
		expectedCount int
		checkFirst    func(*testing.T, map[string]interface{})
	}{
		{
			name: "OpenAI tools to Anthropic tools",
			tools: []interface{}{
				map[string]interface{}{
					"type": "function",
					"function": map[string]interface{}{
						"name":        "get_weather",
						"description": "Get the current weather in a location",
						"parameters": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"location": map[string]interface{}{
									"type":        "string",
									"description": "The city and state",
								},
							},
							"required": []interface{}{"location"},
						},
					},
				},
			},
			expectedCount: 1,
			checkFirst: func(t *testing.T, tool map[string]interface{}) {
				if name, ok := tool["name"].(string); !ok || name != "get_weather" {
					t.Errorf("Expected name=get_weather, got %v", tool["name"])
				}
				if desc, ok := tool["description"].(string); !ok || desc != "Get the current weather in a location" {
					t.Errorf("Expected description, got %v", tool["description"])
				}
				if inputSchema, ok := tool["input_schema"].(map[string]interface{}); !ok {
					t.Errorf("Expected input_schema as map, got %T", tool["input_schema"])
				} else {
					if schemaType, ok := inputSchema["type"].(string); !ok || schemaType != "object" {
						t.Errorf("Expected input_schema.type=object, got %v", inputSchema["type"])
					}
				}
			},
		},
		{
			name: "Multiple tools",
			tools: []interface{}{
				map[string]interface{}{
					"type": "function",
					"function": map[string]interface{}{
						"name":        "tool1",
						"description": "First tool",
						"parameters": map[string]interface{}{
							"type":       "object",
							"properties": map[string]interface{}{},
						},
					},
				},
				map[string]interface{}{
					"type": "function",
					"function": map[string]interface{}{
						"name":        "tool2",
						"description": "Second tool",
						"parameters": map[string]interface{}{
							"type":       "object",
							"properties": map[string]interface{}{},
						},
					},
				},
			},
			expectedCount: 2,
			checkFirst: func(t *testing.T, tool map[string]interface{}) {
				if name, ok := tool["name"].(string); !ok || name != "tool1" {
					t.Errorf("Expected name=tool1, got %v", tool["name"])
				}
			},
		},
		{
			name:          "Empty tools",
			tools:         []interface{}{},
			expectedCount: 0,
			checkFirst:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.convertTools(tt.tools)
			if err != nil {
				t.Fatalf("convertTools failed: %v", err)
			}

			if len(result) != tt.expectedCount {
				t.Errorf("Expected %d tools, got %d", tt.expectedCount, len(result))
			}

			if tt.expectedCount > 0 && tt.checkFirst != nil {
				firstTool := result[0]
				tt.checkFirst(t, firstTool)
			}
		})
	}
}

func TestConvertToolChoice(t *testing.T) {
	converter := NewProtocolConverter()

	tests := []struct {
		name     string
		input    interface{}
		expected map[string]interface{}
	}{
		{
			name:  "auto",
			input: "auto",
			expected: map[string]interface{}{
				"type": "auto",
			},
		},
		{
			name:  "required",
			input: "required",
			expected: map[string]interface{}{
				"type": "any",
			},
		},
		{
			name:  "none",
			input: "none",
			expected: map[string]interface{}{
				"type": "auto",
			},
		},
		{
			name: "specific function",
			input: map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name": "get_weather",
				},
			},
			expected: map[string]interface{}{
				"type": "tool",
				"name": "get_weather",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.convertToolChoice(tt.input)

			resultMap, ok := result.(map[string]interface{})
			if !ok {
				t.Fatalf("Result is not map[string]interface{}, got %T", result)
			}

			if resultMap["type"] != tt.expected["type"] {
				t.Errorf("Expected type=%v, got %v", tt.expected["type"], resultMap["type"])
			}

			if expectedName, hasName := tt.expected["name"]; hasName {
				if resultMap["name"] != expectedName {
					t.Errorf("Expected name=%v, got %v", expectedName, resultMap["name"])
				}
			}
		})
	}
}

func TestConvertOpenAIStreamChunkToAnthropic(t *testing.T) {
	converter := NewProtocolConverter()

	tests := []struct {
		name     string
		chunk    map[string]interface{}
		expected map[string]interface{}
		isNil    bool
	}{
		{
			name: "text delta",
			chunk: map[string]interface{}{
				"choices": []interface{}{
					map[string]interface{}{
						"delta": map[string]interface{}{
							"content": "Hello",
						},
					},
				},
			},
			expected: map[string]interface{}{
				"type":  "content_block_delta",
				"index": 0,
				"delta": map[string]interface{}{
					"type": "text_delta",
					"text": "Hello",
				},
			},
		},
		{
			name: "finish reason stop",
			chunk: map[string]interface{}{
				"choices": []interface{}{
					map[string]interface{}{
						"delta":         map[string]interface{}{},
						"finish_reason": "stop",
					},
				},
			},
			expected: map[string]interface{}{
				"type": "message_delta",
				"delta": map[string]interface{}{
					"stop_reason":   "end_turn",
					"stop_sequence": nil,
				},
			},
		},
		{
			name: "finish reason length",
			chunk: map[string]interface{}{
				"choices": []interface{}{
					map[string]interface{}{
						"delta":         map[string]interface{}{},
						"finish_reason": "length",
					},
				},
			},
			expected: map[string]interface{}{
				"type": "message_delta",
				"delta": map[string]interface{}{
					"stop_reason":   "max_tokens",
					"stop_sequence": nil,
				},
			},
		},
		{
			name: "empty delta",
			chunk: map[string]interface{}{
				"choices": []interface{}{
					map[string]interface{}{
						"delta": map[string]interface{}{},
					},
				},
			},
			isNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertOpenAIStreamChunkToAnthropic(tt.chunk)
			if err != nil {
				t.Fatalf("ConvertOpenAIStreamChunkToAnthropic failed: %v", err)
			}

			if tt.isNil {
				if result != nil {
					t.Errorf("Expected nil result, got %v", result)
				}
				return
			}

			if result == nil {
				t.Fatalf("Expected result, got nil")
			}

			if result["type"] != tt.expected["type"] {
				t.Errorf("Expected type=%v, got %v", tt.expected["type"], result["type"])
			}
		})
	}
}
