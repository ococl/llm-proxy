package entity

import (
	"fmt"
	"time"
)

// Usage represents token usage.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// NewUsage creates a new usage.
func NewUsage(prompt, completion int) Usage {
	return Usage{
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      prompt + completion,
	}
}

// IsEmpty returns true if usage is zero.
func (u Usage) IsEmpty() bool {
	return u.TotalTokens == 0
}

// Choice represents a completion choice.
type Choice struct {
	Index        int      `json:"index"`
	Message      Message  `json:"message"`
	FinishReason string   `json:"finish_reason,omitempty"`
	Delta        *Message `json:"delta,omitempty"`
}

// NewChoice creates a new choice.
func NewChoice(index int, message Message, finishReason string) Choice {
	return Choice{
		Index:        index,
		Message:      message,
		FinishReason: finishReason,
	}
}

// IsComplete returns true if the choice is complete.
func (c Choice) IsComplete() bool {
	return c.FinishReason != ""
}

// Response represents a chat completion response.
type Response struct {
	ID            string              `json:"id"`
	Object        string              `json:"object"`
	Created       int64               `json:"created"`
	Model         string              `json:"model"`
	Choices       []Choice            `json:"choices"`
	Usage         Usage               `json:"usage"`
	StopReason    string              `json:"stop_reason,omitempty"`
	StopSequences []string            `json:"stop_sequences,omitempty"`
	Headers       map[string][]string `json:"-"` // HTTP headers from upstream, not serialized to JSON
}

// NewResponse creates a new response.
func NewResponse(id, model string, choices []Choice, usage Usage) *Response {
	return &Response{
		ID:      id,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: choices,
		Usage:   usage,
	}
}

// FirstChoice returns the first choice or nil.
func (r *Response) FirstChoice() *Choice {
	if len(r.Choices) == 0 {
		return nil
	}
	return &r.Choices[0]
}

// String returns a string representation.
func (r *Response) String() string {
	return fmt.Sprintf("Response(%s, model=%s, choices=%d)",
		r.ID, r.Model, len(r.Choices))
}

// ResponseBuilder is a builder for creating Response entities.
type ResponseBuilder struct {
	id            string
	object        string
	created       int64
	model         string
	choices       []Choice
	usage         Usage
	stopReason    string
	stopSequences []string
	headers       map[string][]string
}

// NewResponseBuilder creates a new response builder.
func NewResponseBuilder() *ResponseBuilder {
	return &ResponseBuilder{
		object:  "chat.completion",
		created: time.Now().Unix(),
		choices: []Choice{},
	}
}

// ID sets the response ID.
func (rb *ResponseBuilder) ID(id string) *ResponseBuilder {
	rb.id = id
	return rb
}

// Object sets the object type.
func (rb *ResponseBuilder) Object(object string) *ResponseBuilder {
	rb.object = object
	return rb
}

// Created sets the creation timestamp.
func (rb *ResponseBuilder) Created(created int64) *ResponseBuilder {
	rb.created = created
	return rb
}

// Model sets the model name.
func (rb *ResponseBuilder) Model(model string) *ResponseBuilder {
	rb.model = model
	return rb
}

// Choices sets the choices.
func (rb *ResponseBuilder) Choices(choices []Choice) *ResponseBuilder {
	if choices == nil {
		rb.choices = []Choice{}
	} else {
		rb.choices = choices
	}
	return rb
}

// Usage sets the usage.
func (rb *ResponseBuilder) Usage(usage Usage) *ResponseBuilder {
	rb.usage = usage
	return rb
}

// StopReason sets the stop reason.
func (rb *ResponseBuilder) StopReason(stopReason string) *ResponseBuilder {
	rb.stopReason = stopReason
	return rb
}

// StopSequences sets the stop sequences.
func (rb *ResponseBuilder) StopSequences(stopSequences []string) *ResponseBuilder {
	rb.stopSequences = stopSequences
	return rb
}

// Headers sets the HTTP headers.
func (rb *ResponseBuilder) Headers(headers map[string][]string) *ResponseBuilder {
	rb.headers = headers
	return rb
}

// Build creates the response entity.
func (rb *ResponseBuilder) Build() (*Response, error) {
	if rb.id == "" {
		return nil, fmt.Errorf("response ID is required")
	}
	if rb.model == "" {
		return nil, fmt.Errorf("model is required")
	}
	return &Response{
		ID:            rb.id,
		Object:        rb.object,
		Created:       rb.created,
		Model:         rb.model,
		Choices:       rb.choices,
		Usage:         rb.usage,
		StopReason:    rb.stopReason,
		StopSequences: rb.stopSequences,
		Headers:       rb.headers,
	}, nil
}

// BuildUnsafe creates the response entity without validation.
// It ensures choices is never nil to prevent JSON serialization issues.
func (rb *ResponseBuilder) BuildUnsafe() *Response {
	// Ensure choices is never nil
	if rb.choices == nil {
		rb.choices = []Choice{}
	}
	return &Response{
		ID:            rb.id,
		Object:        rb.object,
		Created:       rb.created,
		Model:         rb.model,
		Choices:       rb.choices,
		Usage:         rb.usage,
		StopReason:    rb.stopReason,
		StopSequences: rb.stopSequences,
		Headers:       rb.headers,
	}
}
