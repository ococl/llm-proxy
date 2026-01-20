package proxy

import (
	"encoding/json"
	"net/http"
	"strings"
)

// RequestProtocol represents the detected protocol of an incoming request
type RequestProtocol string

const (
	ProtocolOpenAI    RequestProtocol = "openai"
	ProtocolAnthropic RequestProtocol = "anthropic"
	ProtocolUnknown   RequestProtocol = "unknown"
)

// RequestDetector detects the protocol type of incoming requests
type RequestDetector struct{}

func NewRequestDetector() *RequestDetector {
	return &RequestDetector{}
}

// DetectProtocol determines the protocol type based on request path and headers
func (rd *RequestDetector) DetectProtocol(r *http.Request) RequestProtocol {
	// Method 1: Check request path
	path := r.URL.Path
	if path == "/v1/messages" || strings.HasPrefix(path, "/v1/messages/") {
		return ProtocolAnthropic
	}

	if path == "/v1/chat/completions" ||
		path == "/v1/completions" ||
		strings.HasPrefix(path, "/v1/chat/") {
		return ProtocolOpenAI
	}

	// Method 2: Check anthropic-version header (Anthropic-specific)
	if r.Header.Get("anthropic-version") != "" {
		return ProtocolAnthropic
	}

	// Method 3: Check x-api-key vs Authorization header pattern
	apiKey := r.Header.Get("x-api-key")
	authHeader := r.Header.Get("Authorization")

	// Anthropic typically uses x-api-key
	if apiKey != "" && !strings.HasPrefix(authHeader, "Bearer sk-") {
		return ProtocolAnthropic
	}

	// Default to OpenAI for backward compatibility
	return ProtocolOpenAI
}

// IsAnthropicRequest checks if request body matches Anthropic format
func (rd *RequestDetector) IsAnthropicRequest(body []byte) bool {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return false
	}

	// Anthropic-specific fields
	if _, hasSystem := data["system"]; hasSystem {
		// If has 'system' as top-level field (not in messages), likely Anthropic
		if messages, ok := data["messages"].([]interface{}); ok {
			if len(messages) > 0 {
				if msg, ok := messages[0].(map[string]interface{}); ok {
					// Anthropic messages don't have 'system' role
					if role, _ := msg["role"].(string); role == "system" {
						return false
					}
				}
			}
			return true
		}
	}

	// Check for stop_sequences (Anthropic) vs stop (OpenAI)
	if _, hasStopSeq := data["stop_sequences"]; hasStopSeq {
		return true
	}

	return false
}

// NormalizeRequestPath converts request path to standard format
func (rd *RequestDetector) NormalizeRequestPath(r *http.Request, protocol RequestProtocol) string {
	path := r.URL.Path

	// Already normalized
	if path == "/v1/messages" || path == "/v1/chat/completions" {
		return path
	}

	// Normalize Anthropic paths
	if protocol == ProtocolAnthropic {
		if strings.HasPrefix(path, "/v1/messages") {
			return path
		}
		// Default Anthropic endpoint
		return "/v1/messages"
	}

	// Normalize OpenAI paths
	if protocol == ProtocolOpenAI {
		if strings.Contains(path, "/chat/completions") ||
			strings.Contains(path, "/completions") {
			return path
		}
		// Default OpenAI endpoint
		return "/v1/chat/completions"
	}

	return path
}

// ConvertOpenAIToAnthropicResponse converts OpenAI response to Anthropic format
func (rd *RequestDetector) ConvertOpenAIToAnthropicResponse(openAIResp []byte) ([]byte, error) {
	var openAIBody map[string]interface{}
	if err := json.Unmarshal(openAIResp, &openAIBody); err != nil {
		return nil, err
	}

	anthropicBody := make(map[string]interface{})

	// Map basic fields
	if id, ok := openAIBody["id"].(string); ok {
		anthropicBody["id"] = id
	}

	if model, ok := openAIBody["model"].(string); ok {
		anthropicBody["model"] = model
	}

	anthropicBody["type"] = "message"
	anthropicBody["role"] = "assistant"

	// Map usage
	if usage, ok := openAIBody["usage"].(map[string]interface{}); ok {
		anthropicBody["usage"] = map[string]interface{}{
			"input_tokens":  usage["prompt_tokens"],
			"output_tokens": usage["completion_tokens"],
		}
	}

	// Extract content from choices
	if choices, ok := openAIBody["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			var content []map[string]interface{}

			if message, ok := choice["message"].(map[string]interface{}); ok {
				// Extract text content
				if textContent, ok := message["content"].(string); ok && textContent != "" {
					content = append(content, map[string]interface{}{
						"type": "text",
						"text": textContent,
					})
				}

				// Handle tool calls
				if toolCalls, ok := message["tool_calls"].([]interface{}); ok {
					for _, tc := range toolCalls {
						if tcMap, ok := tc.(map[string]interface{}); ok {
							if function, ok := tcMap["function"].(map[string]interface{}); ok {
								content = append(content, map[string]interface{}{
									"type":  "tool_use",
									"id":    tcMap["id"],
									"name":  function["name"],
									"input": function["arguments"],
								})
							}
						}
					}
				}
			}

			anthropicBody["content"] = content

			// Map finish_reason to stop_reason
			if finishReason, ok := choice["finish_reason"].(string); ok {
				anthropicBody["stop_reason"] = finishReason
			}
		}
	}

	return json.Marshal(anthropicBody)
}

// ConvertAnthropicToOpenAI converts Anthropic request to OpenAI format
func (rd *RequestDetector) ConvertAnthropicToOpenAI(anthropicBody map[string]interface{}) (map[string]interface{}, error) {
	openAIBody := make(map[string]interface{})

	// Copy model
	if model, ok := anthropicBody["model"].(string); ok {
		openAIBody["model"] = model
	}

	// Convert max_tokens to max_completion_tokens (OpenAI prefers this for chat)
	if maxTokens, ok := anthropicBody["max_tokens"]; ok {
		openAIBody["max_completion_tokens"] = maxTokens
	}

	// Copy temperature, top_p, stream directly
	for _, field := range []string{"temperature", "top_p", "stream"} {
		if val, ok := anthropicBody[field]; ok {
			openAIBody[field] = val
		}
	}

	// Convert stop_sequences to stop
	if stopSeq, ok := anthropicBody["stop_sequences"]; ok {
		openAIBody["stop"] = stopSeq
	}

	// Convert messages: add system as first message if present
	var openAIMessages []map[string]interface{}

	if system, ok := anthropicBody["system"].(string); ok && system != "" {
		openAIMessages = append(openAIMessages, map[string]interface{}{
			"role":    "system",
			"content": system,
		})
	}

	if messages, ok := anthropicBody["messages"].([]interface{}); ok {
		for _, msg := range messages {
			if msgMap, ok := msg.(map[string]interface{}); ok {
				openAIMessages = append(openAIMessages, msgMap)
			}
		}
	}

	openAIBody["messages"] = openAIMessages

	return openAIBody, nil
}
