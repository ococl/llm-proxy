package entity

import (
	"encoding/json"
	"testing"
)

func TestMessage_JSONSerialization_EmptyRole(t *testing.T) {
	tests := []struct {
		name     string
		message  Message
		wantRole bool
	}{
		{
			name: "Empty role should be omitted",
			message: Message{
				Role:    "",
				Content: "Hello",
			},
			wantRole: false,
		},
		{
			name: "Non-empty role should be included",
			message: Message{
				Role:    "assistant",
				Content: "Hello",
			},
			wantRole: true,
		},
		{
			name: "Empty content with role",
			message: Message{
				Role:    "assistant",
				Content: "",
			},
			wantRole: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.message)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}

			var result map[string]interface{}
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatalf("json.Unmarshal failed: %v", err)
			}

			_, hasRole := result["role"]
			if hasRole != tt.wantRole {
				t.Errorf("role field presence = %v, want %v. JSON: %s", hasRole, tt.wantRole, string(data))
			}
		})
	}
}

func TestChoice_DeltaJSONSerialization(t *testing.T) {
	tests := []struct {
		name      string
		choice    Choice
		wantRole  bool
		wantDelta bool
	}{
		{
			name: "Delta with empty role should omit role",
			choice: Choice{
				Index: 0,
				Delta: &Message{
					Role:    "",
					Content: "Hello",
				},
			},
			wantRole:  false,
			wantDelta: true,
		},
		{
			name: "Delta with role should include role",
			choice: Choice{
				Index: 0,
				Delta: &Message{
					Role:    "assistant",
					Content: "Hello",
				},
			},
			wantRole:  true,
			wantDelta: true,
		},
		{
			name: "Nil delta should be omitted",
			choice: Choice{
				Index: 0,
				Delta: nil,
			},
			wantDelta: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.choice)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}

			var result map[string]interface{}
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatalf("json.Unmarshal failed: %v", err)
			}

			deltaObj, hasDelta := result["delta"]
			if hasDelta != tt.wantDelta {
				t.Errorf("delta field presence = %v, want %v. JSON: %s", hasDelta, tt.wantDelta, string(data))
			}

			if tt.wantDelta {
				deltaMap, ok := deltaObj.(map[string]interface{})
				if !ok {
					t.Fatalf("delta is not a map: %T", deltaObj)
				}

				_, hasRole := deltaMap["role"]
				if hasRole != tt.wantRole {
					t.Errorf("delta.role field presence = %v, want %v. JSON: %s", hasRole, tt.wantRole, string(data))
				}
			}
		})
	}
}

func TestResponse_StreamingChunkSerialization(t *testing.T) {
	response := &Response{
		ID:      "chatcmpl-123",
		Object:  "chat.completion.chunk",
		Created: 1234567890,
		Model:   "gpt-4",
		Choices: []Choice{
			{
				Index: 0,
				Delta: &Message{
					Role:    "",
					Content: "Hello",
				},
			},
		},
		Usage: Usage{
			PromptTokens:     0,
			CompletionTokens: 0,
			TotalTokens:      0,
		},
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	choices, ok := result["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		t.Fatalf("choices is not an array or empty")
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		t.Fatalf("choice is not a map")
	}

	delta, ok := choice["delta"].(map[string]interface{})
	if !ok {
		t.Fatalf("delta is not a map")
	}

	if _, hasRole := delta["role"]; hasRole {
		t.Errorf("Empty role should be omitted from delta. JSON: %s", string(data))
	}

	if content, ok := delta["content"].(string); !ok || content != "Hello" {
		t.Errorf("delta.content = %v, want 'Hello'", delta["content"])
	}
}
