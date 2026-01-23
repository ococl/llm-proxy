package service

import (
	"llm-proxy/domain/entity"
	domainerror "llm-proxy/domain/error"
	"llm-proxy/domain/types"
)

// ProtocolConverter converts requests and responses between protocols.
type ProtocolConverter struct {
	systemPrompts map[string]string
}

// NewProtocolConverter creates a new protocol converter.
func NewProtocolConverter(systemPrompts map[string]string) *ProtocolConverter {
	if systemPrompts == nil {
		systemPrompts = make(map[string]string)
	}
	return &ProtocolConverter{
		systemPrompts: systemPrompts,
	}
}

// ToBackend converts a request to the backend protocol format.
func (c *ProtocolConverter) ToBackend(req *entity.Request, protocol types.Protocol) (*entity.Request, error) {
	if req == nil {
		return nil, domainerror.NewInvalidRequest("request is nil")
	}

	// For now, we just pass through the request
	// In a full implementation, we would transform the request based on protocol
	switch protocol {
	case types.ProtocolOpenAI:
		return c.toOpenAIFormat(req)
	case types.ProtocolAnthropic:
		return c.toAnthropicFormat(req)
	default:
		return req, nil
	}
}

// FromBackend converts a response from the backend protocol format.
func (c *ProtocolConverter) FromBackend(resp *entity.Response, protocol types.Protocol) (*entity.Response, error) {
	if resp == nil {
		return nil, domainerror.NewInvalidRequest("response is nil")
	}

	switch protocol {
	case types.ProtocolOpenAI:
		return c.fromOpenAIFormat(resp)
	case types.ProtocolAnthropic:
		return c.fromAnthropicFormat(resp)
	default:
		return resp, nil
	}
}

// toOpenAIFormat converts a request to OpenAI format.
func (c *ProtocolConverter) toOpenAIFormat(req *entity.Request) (*entity.Request, error) {
	// Inject system prompt if configured for this model
	modelKey := req.Model().String()
	if systemPrompt, ok := c.systemPrompts[modelKey]; ok && systemPrompt != "" {
		// Prepend system message
		messages := make([]entity.Message, 0, len(req.Messages())+1)
		messages = append(messages, entity.NewMessage("system", systemPrompt))
		messages = append(messages, req.Messages()...)

		// Create new request with injected system prompt
		builder := entity.NewRequestBuilder().
			ID(req.ID()).
			Model(req.Model()).
			Messages(messages).
			MaxTokens(req.MaxTokens()).
			Temperature(req.Temperature()).
			TopP(req.TopP()).
			Stream(req.IsStream()).
			Stop(req.Stop()).
			Tools(req.Tools()).
			ToolChoice(req.ToolChoice()).
			User(req.User()).
			Context(req.Context()).
			StreamHandler(req.StreamHandler())

		return builder.BuildUnsafe(), nil
	}

	return req, nil
}

// toAnthropicFormat converts a request to Anthropic format.
func (c *ProtocolConverter) toAnthropicFormat(req *entity.Request) (*entity.Request, error) {
	// Anthropic requires specific transformations
	// For now, pass through
	return req, nil
}

// fromOpenAIFormat converts a response from OpenAI format.
func (c *ProtocolConverter) fromOpenAIFormat(resp *entity.Response) (*entity.Response, error) {
	// For now, pass through
	return resp, nil
}

// fromAnthropicFormat converts a response from Anthropic format.
func (c *ProtocolConverter) fromAnthropicFormat(resp *entity.Response) (*entity.Response, error) {
	// For now, pass through
	return resp, nil
}

// DefaultProtocolConverter returns a converter with no system prompts.
func DefaultProtocolConverter() *ProtocolConverter {
	return NewProtocolConverter(nil)
}
