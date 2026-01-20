package proxy

import (
	"encoding/json"
	"fmt"
)

// ProtocolConverter handles conversion between OpenAI and Anthropic API formats
type ProtocolConverter struct{}

func NewProtocolConverter() *ProtocolConverter {
	return &ProtocolConverter{}
}

// ConvertToAnthropic converts OpenAI format request to Anthropic format
func (pc *ProtocolConverter) ConvertToAnthropic(openAIBody map[string]interface{}) ([]byte, error) {
	anthropicBody := make(map[string]interface{})

	// Extract model
	if model, ok := openAIBody["model"].(string); ok {
		anthropicBody["model"] = model
	}

	// Extract max_tokens (required by Anthropic)
	if maxTokens, ok := openAIBody["max_tokens"].(float64); ok {
		anthropicBody["max_tokens"] = int(maxTokens)
	} else if maxCompletionTokens, ok := openAIBody["max_completion_tokens"].(float64); ok {
		anthropicBody["max_tokens"] = int(maxCompletionTokens)
	} else {
		// Anthropic requires max_tokens, default to 4096
		anthropicBody["max_tokens"] = 4096
	}

	// Extract temperature
	if temp, ok := openAIBody["temperature"].(float64); ok {
		anthropicBody["temperature"] = temp
	}

	// Extract top_p
	if topP, ok := openAIBody["top_p"].(float64); ok {
		anthropicBody["top_p"] = topP
	}

	// Extract stream
	if stream, ok := openAIBody["stream"].(bool); ok {
		anthropicBody["stream"] = stream
	}

	// Extract stop sequences
	if stop, ok := openAIBody["stop"]; ok {
		anthropicBody["stop_sequences"] = stop
	}

	// Convert messages from OpenAI to Anthropic format
	messages, system, err := pc.convertMessages(openAIBody["messages"])
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages: %w", err)
	}

	if system != "" {
		anthropicBody["system"] = system
	}
	anthropicBody["messages"] = messages

	// Convert tools if present
	if tools, ok := openAIBody["tools"].([]interface{}); ok && len(tools) > 0 {
		anthropicTools, err := pc.convertTools(tools)
		if err != nil {
			return nil, fmt.Errorf("failed to convert tools: %w", err)
		}
		anthropicBody["tools"] = anthropicTools

		// Convert tool_choice if present
		if toolChoice, ok := openAIBody["tool_choice"]; ok {
			anthropicBody["tool_choice"] = pc.convertToolChoice(toolChoice)
		}
	}

	return json.Marshal(anthropicBody)
}

// convertMessages converts OpenAI messages to Anthropic format
// Returns (messages, system_prompt, error)
func (pc *ProtocolConverter) convertMessages(messagesInterface interface{}) ([]map[string]interface{}, string, error) {
	messages, ok := messagesInterface.([]interface{})
	if !ok {
		return nil, "", fmt.Errorf("messages must be an array")
	}

	var anthropicMessages []map[string]interface{}
	var systemPrompt string

	for _, msgInterface := range messages {
		msg, ok := msgInterface.(map[string]interface{})
		if !ok {
			continue
		}

		role, _ := msg["role"].(string)
		content := msg["content"]

		// Extract system messages separately
		if role == "system" {
			if contentStr, ok := content.(string); ok {
				if systemPrompt != "" {
					systemPrompt += "\n\n" + contentStr
				} else {
					systemPrompt = contentStr
				}
			}
			continue
		}

		// Convert user and assistant messages
		anthropicMsg := map[string]interface{}{
			"role":    role,
			"content": content,
		}

		anthropicMessages = append(anthropicMessages, anthropicMsg)
	}

	return anthropicMessages, systemPrompt, nil
}

// convertTools converts OpenAI tools to Anthropic format
func (pc *ProtocolConverter) convertTools(tools []interface{}) ([]map[string]interface{}, error) {
	var anthropicTools []map[string]interface{}

	for _, toolInterface := range tools {
		tool, ok := toolInterface.(map[string]interface{})
		if !ok {
			continue
		}

		// OpenAI format: {"type": "function", "function": {...}}
		// Anthropic format: {"name": "...", "description": "...", "input_schema": {...}}
		if toolType, _ := tool["type"].(string); toolType == "function" {
			if function, ok := tool["function"].(map[string]interface{}); ok {
				anthropicTool := map[string]interface{}{
					"name":         function["name"],
					"description":  function["description"],
					"input_schema": function["parameters"],
				}
				anthropicTools = append(anthropicTools, anthropicTool)
			}
		}
	}

	return anthropicTools, nil
}

// convertToolChoice converts OpenAI tool_choice to Anthropic format
func (pc *ProtocolConverter) convertToolChoice(toolChoice interface{}) interface{} {
	// OpenAI: "auto", "none", or {"type": "function", "function": {"name": "..."}}
	// Anthropic: {"type": "auto"}, {"type": "any"}, {"type": "tool", "name": "..."}

	switch tc := toolChoice.(type) {
	case string:
		if tc == "auto" {
			return map[string]interface{}{"type": "auto"}
		} else if tc == "none" {
			// Anthropic doesn't have explicit "none", just omit tool_choice
			return nil
		}
	case map[string]interface{}:
		if tcType, ok := tc["type"].(string); ok && tcType == "function" {
			if function, ok := tc["function"].(map[string]interface{}); ok {
				if name, ok := function["name"].(string); ok {
					return map[string]interface{}{
						"type": "tool",
						"name": name,
					}
				}
			}
		}
	}

	return map[string]interface{}{"type": "auto"}
}

// ConvertFromAnthropic converts Anthropic response to OpenAI format
func (pc *ProtocolConverter) ConvertFromAnthropic(anthropicResp []byte) ([]byte, error) {
	var anthropicBody map[string]interface{}
	if err := json.Unmarshal(anthropicResp, &anthropicBody); err != nil {
		return nil, fmt.Errorf("failed to unmarshal anthropic response: %w", err)
	}

	openAIBody := make(map[string]interface{})

	// Map basic fields
	if id, ok := anthropicBody["id"].(string); ok {
		openAIBody["id"] = id
	}

	openAIBody["object"] = "chat.completion"

	if model, ok := anthropicBody["model"].(string); ok {
		openAIBody["model"] = model
	}

	// Map usage
	if usage, ok := anthropicBody["usage"].(map[string]interface{}); ok {
		openAIBody["usage"] = map[string]interface{}{
			"prompt_tokens":     usage["input_tokens"],
			"completion_tokens": usage["output_tokens"],
			"total_tokens":      int(usage["input_tokens"].(float64)) + int(usage["output_tokens"].(float64)),
		}
	}

	// Map content
	if content, ok := anthropicBody["content"].([]interface{}); ok && len(content) > 0 {
		var messageContent string
		var toolCalls []map[string]interface{}

		for _, contentItem := range content {
			if item, ok := contentItem.(map[string]interface{}); ok {
				if itemType, _ := item["type"].(string); itemType == "text" {
					if text, ok := item["text"].(string); ok {
						messageContent += text
					}
				} else if itemType == "tool_use" {
					// Convert Anthropic tool_use to OpenAI tool_calls
					toolCall := map[string]interface{}{
						"id":   item["id"],
						"type": "function",
						"function": map[string]interface{}{
							"name":      item["name"],
							"arguments": item["input"],
						},
					}
					toolCalls = append(toolCalls, toolCall)
				}
			}
		}

		choice := map[string]interface{}{
			"index": 0,
			"message": map[string]interface{}{
				"role":    "assistant",
				"content": messageContent,
			},
			"finish_reason": anthropicBody["stop_reason"],
		}

		if len(toolCalls) > 0 {
			choice["message"].(map[string]interface{})["tool_calls"] = toolCalls
		}

		openAIBody["choices"] = []interface{}{choice}
	}

	return json.Marshal(openAIBody)
}
