package proxy

import (
	"encoding/json"
	"fmt"
)

// ConversionMetadata stores parameter conversion details for logging
type ConversionMetadata struct {
	InputMaxTokens    interface{}
	OutputMaxTokens   int
	MaxTokensSource   string
	InputTemperature  interface{}
	OutputTemperature interface{}
	InputTopP         interface{}
	OutputTopP        interface{}
	InputStream       interface{}
	OutputStream      interface{}
	InputStop         interface{}
	OutputStop        interface{}
	InputTools        interface{}
	OutputTools       interface{}
	SystemPromptLen   int
}

// ProtocolConverter handles conversion between OpenAI and Anthropic API formats
type ProtocolConverter struct {
	lastConversion *ConversionMetadata
}

func NewProtocolConverter() *ProtocolConverter {
	return &ProtocolConverter{}
}

// GetLastConversion returns the metadata from the last conversion
func (pc *ProtocolConverter) GetLastConversion() *ConversionMetadata {
	return pc.lastConversion
}

// ConvertToAnthropic converts OpenAI format request to Anthropic format
func (pc *ProtocolConverter) ConvertToAnthropic(openAIBody map[string]interface{}) ([]byte, error) {
	anthropicBody := make(map[string]interface{})

	// Extract model
	if model, ok := openAIBody["model"].(string); ok {
		anthropicBody["model"] = model
	}

	// Extract max_tokens (required by Anthropic)
	var inputMaxTokens interface{}
	var outputMaxTokens int
	var maxTokensSource string

	// Try to extract max_tokens with support for both int and float64 types
	if maxTokens := openAIBody["max_tokens"]; maxTokens != nil {
		switch v := maxTokens.(type) {
		case float64:
			inputMaxTokens = v
			outputMaxTokens = int(v)
			maxTokensSource = "max_tokens"
			anthropicBody["max_tokens"] = outputMaxTokens
		case int:
			inputMaxTokens = v
			outputMaxTokens = v
			maxTokensSource = "max_tokens"
			anthropicBody["max_tokens"] = outputMaxTokens
		}
	} else if maxCompletionTokens := openAIBody["max_completion_tokens"]; maxCompletionTokens != nil {
		// Try max_completion_tokens with support for both types
		switch v := maxCompletionTokens.(type) {
		case float64:
			inputMaxTokens = v
			outputMaxTokens = int(v)
			maxTokensSource = "max_completion_tokens"
			anthropicBody["max_tokens"] = outputMaxTokens
		case int:
			inputMaxTokens = v
			outputMaxTokens = v
			maxTokensSource = "max_completion_tokens"
			anthropicBody["max_tokens"] = outputMaxTokens
		}
	} else {
		// Anthropic requires max_tokens, default to 4096
		inputMaxTokens = nil
		outputMaxTokens = 4096
		maxTokensSource = "default"
		anthropicBody["max_tokens"] = outputMaxTokens
	}

	// Extract temperature
	var inputTemp, outputTemp interface{}
	if temp, ok := openAIBody["temperature"].(float64); ok {
		inputTemp = temp
		outputTemp = temp
		anthropicBody["temperature"] = temp
	}

	// Extract top_p
	var inputTopP, outputTopP interface{}
	if topP, ok := openAIBody["top_p"].(float64); ok {
		inputTopP = topP
		outputTopP = topP
		anthropicBody["top_p"] = topP
	}

	// Extract stream
	var inputStream, outputStream interface{}
	if stream, ok := openAIBody["stream"].(bool); ok {
		inputStream = stream
		outputStream = stream
		anthropicBody["stream"] = stream
	}

	// Extract stop sequences - convert OpenAI stop to Anthropic stop_sequences
	var inputStop, outputStop interface{}
	if stop, ok := openAIBody["stop"]; ok {
		inputStop = stop
		// Convert stop to stop_sequences format
		switch v := stop.(type) {
		case string:
			// Single stop sequence -> array with one element
			outputStop = []string{v}
			anthropicBody["stop_sequences"] = outputStop
		case []interface{}:
			// Array of stop sequences -> convert to []string
			stopSequences := make([]string, len(v))
			for i, s := range v {
				if str, ok := s.(string); ok {
					stopSequences[i] = str
				}
			}
			outputStop = stopSequences
			anthropicBody["stop_sequences"] = outputStop
		case []string:
			// Already string array
			outputStop = v
			anthropicBody["stop_sequences"] = outputStop
		default:
			// Unsupported format, skip stop_sequences
			outputStop = nil
		}
	}

	// Convert messages from OpenAI to Anthropic format
	messages, system, err := pc.convertMessages(openAIBody["messages"])
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages: %w", err)
	}

	var systemPromptLen int
	if system != "" {
		anthropicBody["system"] = system
		systemPromptLen = len(system)
	}
	anthropicBody["messages"] = messages

	// Convert tools if present
	var inputTools, outputTools interface{}
	if tools, ok := openAIBody["tools"].([]interface{}); ok && len(tools) > 0 {
		inputTools = len(tools)
		anthropicTools, err := pc.convertTools(tools)
		if err != nil {
			return nil, fmt.Errorf("failed to convert tools: %w", err)
		}
		outputTools = len(anthropicTools)
		anthropicBody["tools"] = anthropicTools

		// Convert tool_choice if present
		if toolChoice, ok := openAIBody["tool_choice"]; ok {
			anthropicBody["tool_choice"] = pc.convertToolChoice(toolChoice)
		}
	}

	// Store conversion metadata for logging
	pc.lastConversion = &ConversionMetadata{
		InputMaxTokens:    inputMaxTokens,
		OutputMaxTokens:   outputMaxTokens,
		MaxTokensSource:   maxTokensSource,
		InputTemperature:  inputTemp,
		OutputTemperature: outputTemp,
		InputTopP:         inputTopP,
		OutputTopP:        outputTopP,
		InputStream:       inputStream,
		OutputStream:      outputStream,
		InputStop:         inputStop,
		OutputStop:        outputStop,
		InputTools:        inputTools,
		OutputTools:       outputTools,
		SystemPromptLen:   systemPromptLen,
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

		if toolType, _ := tool["type"].(string); toolType == "function" {
			if function, ok := tool["function"].(map[string]interface{}); ok {
				// Sanitize parameters to create a valid input_schema
				inputSchema := pc.sanitizeInputSchema(function["parameters"])

				anthropicTool := map[string]interface{}{
					"name":         function["name"],
					"description":  function["description"],
					"input_schema": inputSchema,
				}
				anthropicTools = append(anthropicTools, anthropicTool)
			}
		}
	}

	return anthropicTools, nil
}

// sanitizeInputSchema removes fields that Anthropic doesn't support in input_schema
func (pc *ProtocolConverter) sanitizeInputSchema(parameters interface{}) map[string]interface{} {
	// Default schema if parameters is nil or invalid
	defaultSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}

	paramsMap, ok := parameters.(map[string]interface{})
	if !ok {
		return defaultSchema
	}

	// Create a clean schema with only supported fields
	cleanSchema := map[string]interface{}{}

	// Always set type to "object" (required by Anthropic)
	if schemaType, ok := paramsMap["type"].(string); ok && schemaType != "" {
		cleanSchema["type"] = schemaType
	} else {
		cleanSchema["type"] = "object"
	}

	// Copy supported fields
	if properties, ok := paramsMap["properties"]; ok {
		cleanSchema["properties"] = properties
	}

	if required, ok := paramsMap["required"]; ok {
		cleanSchema["required"] = required
	}

	// Add description if present (Anthropic supports this)
	if description, ok := paramsMap["description"]; ok {
		cleanSchema["description"] = description
	}

	return cleanSchema
}

// convertToolChoice converts OpenAI tool_choice to Anthropic format
func (pc *ProtocolConverter) convertToolChoice(toolChoice interface{}) interface{} {
	switch tc := toolChoice.(type) {
	case string:
		if tc == "auto" {
			return map[string]interface{}{"type": "auto"}
		} else if tc == "required" {
			return map[string]interface{}{"type": "any"}
		} else if tc == "none" {
			return map[string]interface{}{"type": "auto"}
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

// ConvertAnthropicStreamToOpenAI converts Anthropic SSE event to OpenAI SSE format
func (pc *ProtocolConverter) ConvertAnthropicStreamToOpenAI(anthropicEvent string) (string, error) {
	// Anthropic SSE events:
	// event: message_start, content_block_start, content_block_delta, content_block_stop, message_delta, message_stop

	if anthropicEvent == "" || anthropicEvent == "event: ping" {
		return "", nil
	}

	// Parse event type and data
	var eventType, dataStr string
	lines := splitLines(anthropicEvent)
	for _, line := range lines {
		if len(line) > 7 && line[:7] == "event: " {
			eventType = line[7:]
		} else if len(line) > 6 && line[:6] == "data: " {
			dataStr = line[6:]
		}
	}

	if dataStr == "" {
		return "", nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
		return "", nil
	}

	// Convert based on event type
	switch eventType {
	case "message_start":
		// Initialize OpenAI stream response
		return pc.createOpenAIStreamEvent("", "", ""), nil

	case "content_block_delta":
		// Extract text delta
		if delta, ok := data["delta"].(map[string]interface{}); ok {
			if deltaType, _ := delta["type"].(string); deltaType == "text_delta" {
				if text, ok := delta["text"].(string); ok {
					return pc.createOpenAIStreamEvent(text, "", ""), nil
				}
			}
		}

	case "message_delta":
		// Handle finish reason
		if delta, ok := data["delta"].(map[string]interface{}); ok {
			if stopReason, ok := delta["stop_reason"].(string); ok {
				return pc.createOpenAIStreamEvent("", stopReason, ""), nil
			}
		}

	case "message_stop":
		// Send [DONE]
		return "data: [DONE]\n\n", nil
	}

	return "", nil
}

func (pc *ProtocolConverter) createOpenAIStreamEvent(content, finishReason, toolCallDelta string) string {
	choice := map[string]interface{}{
		"index": 0,
		"delta": map[string]interface{}{},
	}

	if content != "" {
		choice["delta"].(map[string]interface{})["content"] = content
	}

	if finishReason != "" {
		choice["finish_reason"] = finishReason
	} else {
		choice["finish_reason"] = nil
	}

	event := map[string]interface{}{
		"id":      "chatcmpl-anthropic",
		"object":  "chat.completion.chunk",
		"created": 0,
		"model":   "claude",
		"choices": []interface{}{choice},
	}

	eventJSON, _ := json.Marshal(event)
	return "data: " + string(eventJSON) + "\n\n"
}

func splitLines(s string) []string {
	var lines []string
	var line string
	for _, c := range s {
		if c == '\n' {
			if line != "" {
				lines = append(lines, line)
			}
			line = ""
		} else if c != '\r' {
			line += string(c)
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

func (pc *ProtocolConverter) ConvertOpenAIStreamStartToAnthropic(model string) []map[string]interface{} {
	var events []map[string]interface{}

	messageStart := map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id":            "msg_" + pc.generateID(),
			"type":          "message",
			"role":          "assistant",
			"content":       []interface{}{},
			"model":         model,
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": map[string]interface{}{
				"input_tokens":  0,
				"output_tokens": 0,
			},
		},
	}
	events = append(events, messageStart)

	contentBlockStart := map[string]interface{}{
		"type":  "content_block_start",
		"index": 0,
		"content_block": map[string]interface{}{
			"type": "text",
			"text": "",
		},
	}
	events = append(events, contentBlockStart)

	return events
}

func (pc *ProtocolConverter) ConvertOpenAIStreamChunkToAnthropic(chunk map[string]interface{}) (map[string]interface{}, error) {
	choices, ok := chunk["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return nil, nil
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	delta, ok := choice["delta"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	if content, hasContent := delta["content"].(string); hasContent && content != "" {
		return map[string]interface{}{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]interface{}{
				"type": "text_delta",
				"text": content,
			},
		}, nil
	}

	if finishReason, hasFinish := choice["finish_reason"].(string); hasFinish && finishReason != "" {
		anthropicStopReason := "end_turn"
		if finishReason == "length" {
			anthropicStopReason = "max_tokens"
		} else if finishReason == "tool_calls" {
			anthropicStopReason = "tool_use"
		}

		return map[string]interface{}{
			"type": "message_delta",
			"delta": map[string]interface{}{
				"stop_reason":   anthropicStopReason,
				"stop_sequence": nil,
			},
			"usage": map[string]interface{}{
				"output_tokens": 0,
			},
		}, nil
	}

	return nil, nil
}

func (pc *ProtocolConverter) ConvertOpenAIStreamEndToAnthropic() []map[string]interface{} {
	var events []map[string]interface{}

	contentBlockStop := map[string]interface{}{
		"type":  "content_block_stop",
		"index": 0,
	}
	events = append(events, contentBlockStop)

	messageStop := map[string]interface{}{
		"type": "message_stop",
	}
	events = append(events, messageStop)

	return events
}

func (pc *ProtocolConverter) generateID() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 29)
	for i := range b {
		b[i] = charset[i%len(charset)]
	}
	return string(b)
}
