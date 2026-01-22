package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// validateToolResult validates that a tool result message has required fields
func validateToolResult(result map[string]interface{}) error {
	toolCallID, ok := result["tool_call_id"].(string)
	if !ok || toolCallID == "" {
		return fmt.Errorf("tool result missing or invalid tool_call_id")
	}
	return nil
}

// validateToolUse validates that a tool_use block has required fields
func validateToolUse(block map[string]interface{}) error {
	// Check required fields: id, name, input
	if _, ok := block["id"].(string); !ok || block["id"] == "" {
		return fmt.Errorf("tool_use missing required field: id")
	}
	if _, ok := block["name"].(string); !ok || block["name"] == "" {
		return fmt.Errorf("tool_use missing required field: name")
	}
	if _, ok := block["input"].(map[string]interface{}); !ok || block["input"] == nil {
		return fmt.Errorf("tool_use missing required field: input")
	}
	return nil
}

// ConvertAnthropicErrorToOpenAI converts Anthropic API errors to OpenAI format
func ConvertAnthropicErrorToOpenAI(anthropicResp []byte, statusCode int) ([]byte, error) {
	var anthropicErr map[string]interface{}
	if err := json.Unmarshal(anthropicResp, &anthropicErr); err != nil {
		// If we can't parse the error, return a generic error
		return createOpenAIErrorResponse("server_error", "Anthropic API returned an error", statusCode)
	}

	// Extract Anthropic error details
	var errType, errMsg string
	if t, ok := anthropicErr["type"].(string); ok {
		errType = t
	}
	if m, ok := anthropicErr["error"].(map[string]interface{}); ok {
		if msg, ok := m["message"].(string); ok {
			errMsg = msg
		}
	} else if msg, ok := anthropicErr["message"].(string); ok {
		errMsg = msg
	}

	// Map Anthropic error types to OpenAI error types
	openAIErrorType := mapAnthropicErrorToOpenAI(errType, statusCode)

	return createOpenAIErrorResponse(openAIErrorType, errMsg, statusCode)
}

func mapAnthropicErrorToOpenAI(anthropicType string, statusCode int) string {
	switch anthropicType {
	case "invalid_request_error":
		return "invalid_request_error"
	case "authentication_error":
		return "authentication_error"
	case "permission_error":
		return "permission_denied"
	case "rate_limit_error":
		return "rate_limit_exceeded"
	case "overloaded_error":
		return "service_unavailable_error"
	default:
		if statusCode == 400 {
			return "invalid_request_error"
		} else if statusCode == 401 {
			return "authentication_error"
		} else if statusCode == 403 {
			return "permission_denied"
		} else if statusCode == 429 {
			return "rate_limit_exceeded"
		} else if statusCode >= 500 {
			return "service_unavailable_error"
		}
		return "server_error"
	}
}

func createOpenAIErrorResponse(errType, message string, statusCode int) ([]byte, error) {
	errResp := map[string]interface{}{
		"object": "error",
		"type":   errType,
		"message": func() string {
			if message != "" {
				return message
			}
			return fmt.Sprintf("API request failed with status %d", statusCode)
		}(),
	}

	return json.Marshal(errResp)
}

// sanitizeStopSequences removes whitespace-only stop sequences that Anthropic rejects
func sanitizeStopSequences(stop interface{}) interface{} {
	switch s := stop.(type) {
	case string:
		if strings.TrimSpace(s) == "" {
			return nil
		}
		return s
	case []interface{}:
		var result []interface{}
		for _, item := range s {
			if item != nil {
				sanitized := sanitizeStopSequences(item)
				if sanitized != nil {
					result = append(result, sanitized)
				}
			}
		}
		if len(result) == 0 {
			return nil
		}
		return result
	default:
		return s
	}
}

// WriteOpenAIError writes an error response in OpenAI format
func WriteOpenAIError(w http.ResponseWriter, status int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := map[string]interface{}{
		"object":  "error",
		"type":    errType,
		"message": message,
	}
	json.NewEncoder(w).Encode(resp)
}

// ValidateToolCallID validates that tool_call_id exists and is valid
func ValidateToolCallID(toolCallID interface{}) error {
	if toolCallID == nil {
		return fmt.Errorf("tool_call_id is required")
	}
	id, ok := toolCallID.(string)
	if !ok || id == "" {
		return fmt.Errorf("tool_call_id must be a non-empty string")
	}
	return nil
}
