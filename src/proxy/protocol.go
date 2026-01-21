package proxy

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"
	"time"
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
		switch v := stop.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				outputStop = []string{v}
				anthropicBody["stop_sequences"] = outputStop
			}
		case []interface{}:
			stopSequences := make([]string, 0, len(v))
			for _, s := range v {
				if str, ok := s.(string); ok {
					if strings.TrimSpace(str) != "" {
						stopSequences = append(stopSequences, str)
					}
				}
			}
			if len(stopSequences) > 0 {
				outputStop = stopSequences
				anthropicBody["stop_sequences"] = outputStop
			}
		case []string:
			stopSequences := make([]string, 0, len(v))
			for _, str := range v {
				if strings.TrimSpace(str) != "" {
					stopSequences = append(stopSequences, str)
				}
			}
			if len(stopSequences) > 0 {
				outputStop = stopSequences
				anthropicBody["stop_sequences"] = outputStop
			}
		default:
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

		if toolChoice, ok := openAIBody["tool_choice"]; ok {
			convertedChoice := pc.convertToolChoice(toolChoice)
			anthropicBody["tool_choice"] = convertedChoice
		}
	} else {
		hasToolCalls := false
		if messages, ok := openAIBody["messages"].([]interface{}); ok {
			for _, msgInterface := range messages {
				if msg, ok := msgInterface.(map[string]interface{}); ok {
					if toolCalls, ok := msg["tool_calls"].([]interface{}); ok && len(toolCalls) > 0 {
						hasToolCalls = true
						break
					}
				}
			}
		}

		if hasToolCalls {
			anthropicBody["tools"] = []map[string]interface{}{
				{
					"name":        "dummy_tool",
					"description": "Placeholder tool for tool_calls without tools definition",
					"input_schema": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					},
				},
			}
		}
	}

	if parallelToolCalls, ok := openAIBody["parallel_tool_calls"].(bool); ok {
		if !parallelToolCalls {
			if toolChoice, ok := anthropicBody["tool_choice"].(map[string]interface{}); ok {
				toolChoice["disable_parallel_tool_use"] = true
			} else {
				anthropicBody["tool_choice"] = map[string]interface{}{
					"type":                      "auto",
					"disable_parallel_tool_use": true,
				}
			}
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

		// Handle tool role messages - convert to user role with tool_result
		if role == "tool" {
			toolCallID, _ := msg["tool_call_id"].(string)
			if toolCallID == "" {
				return nil, "", fmt.Errorf("tool_call_id is required for tool role messages")
			}

			toolResultBlock := map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": toolCallID,
			}

			if contentStr, ok := content.(string); ok {
				toolResultBlock["content"] = contentStr
			} else if contentArr, ok := content.([]interface{}); ok {
				toolResultBlock["content"] = contentArr
			} else {
				toolResultBlock["content"] = ""
			}

			anthropicMsg := map[string]interface{}{
				"role":    "user",
				"content": []interface{}{toolResultBlock},
			}
			anthropicMessages = append(anthropicMessages, anthropicMsg)
			continue
		}

		// Handle assistant messages with tool_calls
		if role == "assistant" {
			contentBlocks := []interface{}{}

			if contentStr, ok := content.(string); ok && contentStr != "" {
				contentBlocks = append(contentBlocks, map[string]interface{}{
					"type": "text",
					"text": contentStr,
				})
			} else if contentArr, ok := content.([]interface{}); ok {
				contentBlocks = append(contentBlocks, contentArr...)
			}

			if toolCalls, ok := msg["tool_calls"].([]interface{}); ok {
				for _, toolCallInterface := range toolCalls {
					if toolCall, ok := toolCallInterface.(map[string]interface{}); ok {
						toolUseBlock, err := pc.convertToolCallToToolUse(toolCall)
						if err == nil {
							contentBlocks = append(contentBlocks, toolUseBlock)
						}
					}
				}
			}

			anthropicMsg := map[string]interface{}{
				"role":    "assistant",
				"content": contentBlocks,
			}
			anthropicMessages = append(anthropicMessages, anthropicMsg)
			continue
		}

		// Convert user messages
		anthropicMsg := map[string]interface{}{
			"role":    role,
			"content": content,
		}

		anthropicMessages = append(anthropicMessages, anthropicMsg)
	}

	return anthropicMessages, systemPrompt, nil
}

func (pc *ProtocolConverter) convertToolCallToToolUse(toolCall map[string]interface{}) (map[string]interface{}, error) {
	toolCallID, _ := toolCall["id"].(string)
	if toolCallID == "" {
		return nil, fmt.Errorf("tool_call_id is required but empty")
	}

	toolType, _ := toolCall["type"].(string)
	if toolType != "function" {
		return nil, fmt.Errorf("unsupported tool call type: %s", toolType)
	}

	function, ok := toolCall["function"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("tool call missing function field")
	}

	name, _ := function["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("tool name is required but empty")
	}

	argumentsStr, _ := function["arguments"].(string)

	var input interface{}
	if argumentsStr != "" {
		if err := json.Unmarshal([]byte(argumentsStr), &input); err != nil {
			return nil, fmt.Errorf("invalid tool arguments JSON: %w", err)
		}
	} else {
		input = map[string]interface{}{}
	}

	toolUseBlock := map[string]interface{}{
		"type":  "tool_use",
		"id":    toolCallID,
		"name":  name,
		"input": input,
	}

	return toolUseBlock, nil
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
	defaultSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}

	paramsMap, ok := parameters.(map[string]interface{})
	if !ok {
		return defaultSchema
	}

	return pc.sanitizeSchemaRecursive(paramsMap)
}

func (pc *ProtocolConverter) sanitizeSchemaRecursive(schema map[string]interface{}) map[string]interface{} {
	cleanSchema := map[string]interface{}{}

	supportedFields := []string{"type", "properties", "required", "items", "enum", "description", "default", "minimum", "maximum", "minLength", "maxLength", "pattern", "minItems", "maxItems"}

	for _, field := range supportedFields {
		if value, ok := schema[field]; ok {
			if field == "properties" {
				if propsMap, ok := value.(map[string]interface{}); ok {
					cleanProps := make(map[string]interface{})
					for propName, propSchema := range propsMap {
						if propSchemaMap, ok := propSchema.(map[string]interface{}); ok {
							cleanProps[propName] = cloneMap(pc.sanitizeSchemaRecursive(propSchemaMap))
						} else {
							cleanProps[propName] = propSchema
						}
					}
					cleanSchema[field] = cleanProps
				}
			} else if field == "items" {
				if itemsMap, ok := value.(map[string]interface{}); ok {
					cleanSchema[field] = cloneMap(pc.sanitizeSchemaRecursive(itemsMap))
				} else {
					cleanSchema[field] = value
				}
			} else {
				cleanSchema[field] = value
			}
		}
	}

	if _, ok := cleanSchema["type"]; !ok {
		cleanSchema["type"] = "object"
	}

	if schemaType, ok := cleanSchema["type"].(string); ok && schemaType == "object" {
		if _, hasProps := cleanSchema["properties"]; !hasProps {
			cleanSchema["properties"] = map[string]interface{}{}
		}
	}

	return cleanSchema
}

func cloneMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		switch val := v.(type) {
		case map[string]interface{}:
			dst[k] = cloneMap(val)
		case []interface{}:
			dst[k] = cloneSlice(val)
		default:
			dst[k] = v
		}
	}
	return dst
}

func cloneSlice(src []interface{}) []interface{} {
	if src == nil {
		return nil
	}
	dst := make([]interface{}, len(src))
	for i, v := range src {
		switch val := v.(type) {
		case map[string]interface{}:
			dst[i] = cloneMap(val)
		case []interface{}:
			dst[i] = cloneSlice(val)
		default:
			dst[i] = v
		}
	}
	return dst
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
		inputTokens, _ := usage["input_tokens"].(float64)
		outputTokens, _ := usage["output_tokens"].(float64)

		openAIBody["usage"] = map[string]interface{}{
			"prompt_tokens":     int(inputTokens),
			"completion_tokens": int(outputTokens),
			"total_tokens":      int(inputTokens) + int(outputTokens),
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
					argumentsJSON, err := json.Marshal(item["input"])
					if err != nil {
						argumentsJSON = []byte("{}")
					}

					toolCall := map[string]interface{}{
						"id":   item["id"],
						"type": "function",
						"function": map[string]interface{}{
							"name":      item["name"],
							"arguments": string(argumentsJSON),
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
			if message, ok := choice["message"].(map[string]interface{}); ok {
				message["tool_calls"] = toolCalls
			}
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
		return pc.createOpenAIStreamEvent("", "", ""), nil

	case "content_block_start":
		if block, ok := data["content_block"].(map[string]interface{}); ok {
			if blockType, _ := block["type"].(string); blockType == "tool_use" {
				toolUseID, _ := block["id"].(string)
				toolName, _ := block["name"].(string)
				return pc.createOpenAIToolCallStart(toolUseID, toolName), nil
			}
		}
		return "", nil

	case "content_block_delta":
		if delta, ok := data["delta"].(map[string]interface{}); ok {
			deltaType, _ := delta["type"].(string)
			if deltaType == "text_delta" {
				if text, ok := delta["text"].(string); ok {
					return pc.createOpenAIStreamEvent(text, "", ""), nil
				}
			} else if deltaType == "input_json_delta" {
				if partialJSON, ok := delta["partial_json"].(string); ok {
					return pc.createOpenAIToolCallDelta(partialJSON), nil
				}
			}
		}

	case "content_block_stop":
		return "", nil

	case "message_delta":
		if delta, ok := data["delta"].(map[string]interface{}); ok {
			if stopReason, ok := delta["stop_reason"].(string); ok {
				return pc.createOpenAIStreamEvent("", stopReason, ""), nil
			}
		}

	case "message_stop":
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
		if delta, ok := choice["delta"].(map[string]interface{}); ok {
			delta["content"] = content
		}
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

func (pc *ProtocolConverter) createOpenAIToolCallStart(toolUseID, toolName string) string {
	choice := map[string]interface{}{
		"index": 0,
		"delta": map[string]interface{}{
			"tool_calls": []interface{}{
				map[string]interface{}{
					"index": 0,
					"id":    toolUseID,
					"type":  "function",
					"function": map[string]interface{}{
						"name":      toolName,
						"arguments": "",
					},
				},
			},
		},
		"finish_reason": nil,
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

func (pc *ProtocolConverter) createOpenAIToolCallDelta(partialJSON string) string {
	choice := map[string]interface{}{
		"index": 0,
		"delta": map[string]interface{}{
			"tool_calls": []interface{}{
				map[string]interface{}{
					"index": 0,
					"function": map[string]interface{}{
						"arguments": partialJSON,
					},
				},
			},
		},
		"finish_reason": nil,
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

	if toolCalls, hasToolCalls := delta["tool_calls"].([]interface{}); hasToolCalls && len(toolCalls) > 0 {
		toolCall, ok := toolCalls[0].(map[string]interface{})
		if !ok {
			return nil, nil
		}

		if function, ok := toolCall["function"].(map[string]interface{}); ok {
			if arguments, ok := function["arguments"].(string); ok && arguments != "" {
				return map[string]interface{}{
					"type":  "content_block_delta",
					"index": 0,
					"delta": map[string]interface{}{
						"type":         "input_json_delta",
						"partial_json": arguments,
					},
				}, nil
			}
		}
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
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return fmt.Sprintf("msg_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("msg_%x", b)
}
