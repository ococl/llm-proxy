package service

import (
	"llm-proxy/domain/entity"
	domainerror "llm-proxy/domain/error"
)

// RequestValidator validates incoming requests.
type RequestValidator struct {
	maxMessagesPerRequest int
	maxTokensPerRequest   int
	allowedRoles          map[string]bool
}

// NewRequestValidator creates a new request validator.
func NewRequestValidator(maxMessages, maxTokens int) *RequestValidator {
	return &RequestValidator{
		maxMessagesPerRequest: maxMessages,
		maxTokensPerRequest:   maxTokens,
		allowedRoles: map[string]bool{
			"system":    true,
			"user":      true,
			"assistant": true,
			"tool":      true,
		},
	}
}

// Validate validates a request entity.
func (v *RequestValidator) Validate(req *entity.Request) error {
	// Check if model is specified
	if req.Model().IsEmpty() {
		return domainerror.NewMissingModel()
	}

	// Check if messages are provided
	if len(req.Messages()) == 0 {
		return domainerror.NewInvalidRequest("messages array is empty")
	}

	// Check message count
	if v.maxMessagesPerRequest > 0 && len(req.Messages()) > v.maxMessagesPerRequest {
		return domainerror.NewInvalidRequest("too many messages in request")
	}

	// Check max tokens
	if v.maxTokensPerRequest > 0 && req.MaxTokens() > v.maxTokensPerRequest {
		return domainerror.NewInvalidRequest("max_tokens exceeds limit")
	}

	// Validate messages
	for i, msg := range req.Messages() {
		if msg.IsEmpty() {
			return domainerror.NewInvalidRequest("message at index %d is empty", i)
		}

		// Check role
		if !v.allowedRoles[msg.Role] {
			return domainerror.NewInvalidRequest("invalid role '%s' at message index %d", msg.Role, i)
		}

		// Validate tool calls
		if msg.IsToolCall() {
			for j, tc := range msg.ToolCalls {
				if tc.ID == "" {
					return domainerror.NewInvalidRequest("tool_call at message %d index %d missing id", i, j)
				}
				if tc.Function.Name == "" {
					return domainerror.NewInvalidRequest("tool_call at message %d index %d missing function name", i, j)
				}
			}
		}
	}

	// Validate temperature
	if req.Temperature() < 0 || req.Temperature() > 2 {
		return domainerror.NewInvalidRequest("temperature must be between 0 and 2")
	}

	// Validate top_p
	if req.TopP() < 0 || req.TopP() > 1 {
		return domainerror.NewInvalidRequest("top_p must be between 0 and 1")
	}

	return nil
}

// DefaultRequestValidator returns a validator with default limits.
func DefaultRequestValidator() *RequestValidator {
	return NewRequestValidator(100, 4096)
}
