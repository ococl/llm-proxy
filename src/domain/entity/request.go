package entity

import (
	"context"
	"fmt"
	"time"
)

// RequestID is a value object for request identifier.
type RequestID string

// NewRequestID creates a new request ID.
func NewRequestID(id string) RequestID {
	return RequestID(id)
}

// String returns the string representation.
func (id RequestID) String() string {
	return string(id)
}

// IsEmpty returns true if the ID is empty.
func (id RequestID) IsEmpty() bool {
	return string(id) == ""
}

// Message represents a chat message.
type Message struct {
	Role       string     `json:"role,omitempty"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// NewMessage creates a new message.
func NewMessage(role, content string) Message {
	return Message{
		Role:    role,
		Content: content,
	}
}

// IsEmpty returns true if the message has no content.
func (m Message) IsEmpty() bool {
	return m.Content == "" && len(m.ToolCalls) == 0
}

// IsToolCall returns true if the message contains tool calls.
func (m Message) IsToolCall() bool {
	return len(m.ToolCalls) > 0
}

// IsToolResult returns true if the message is a tool result.
func (m Message) IsToolResult() bool {
	return m.ToolCallID != ""
}

// ToolCall represents a tool call from the model.
type ToolCall struct {
	ID       string
	Type     string
	Function ToolCallFunction
}

// ToolCallFunction represents a function tool call.
type ToolCallFunction struct {
	Name      string
	Arguments string
}

// Tool represents a tool definition.
type Tool struct {
	Type     string
	Function ToolFunction
}

// ToolFunction represents a function tool.
type ToolFunction struct {
	Name        string
	Description string
	Parameters  map[string]any
}

// Request represents a chat completion request.
type Request struct {
	id             RequestID
	model          ModelAlias
	messages       []Message
	maxTokens      int
	temperature    float64
	topP           float64
	stream         bool
	stop           []string
	tools          []Tool
	toolChoice     any
	user           string
	ctx            context.Context
	streamHandler  func(chunk []byte) error
	headers        map[string][]string // Client headers to forward to backend
	clientProtocol string              // Client protocol (openai/anthropic)
}

// NewRequest creates a new request.
func NewRequest(id RequestID, model ModelAlias, messages []Message) *Request {
	return &Request{
		id:          id,
		model:       model,
		messages:    messages,
		temperature: 1.0,
		topP:        1.0,
		ctx:         context.Background(),
	}
}

// ID returns the request ID.
func (r *Request) ID() RequestID {
	return r.id
}

// Model returns the model alias.
func (r *Request) Model() ModelAlias {
	return r.model
}

// Messages returns the messages.
func (r *Request) Messages() []Message {
	return r.messages
}

// MaxTokens returns the max tokens.
func (r *Request) MaxTokens() int {
	return r.maxTokens
}

// Temperature returns the temperature.
func (r *Request) Temperature() float64 {
	return r.temperature
}

// TopP returns the top_p.
func (r *Request) TopP() float64 {
	return r.topP
}

// IsStream returns true if streaming is enabled.
func (r *Request) IsStream() bool {
	return r.stream
}

// Stop returns the stop sequences.
func (r *Request) Stop() []string {
	return r.stop
}

// Tools returns the tools.
func (r *Request) Tools() []Tool {
	return r.tools
}

// ToolChoice returns the tool choice.
func (r *Request) ToolChoice() any {
	return r.toolChoice
}

// User returns the user ID.
func (r *Request) User() string {
	return r.user
}

// Context returns the context.
func (r *Request) Context() context.Context {
	if r.ctx == nil {
		return context.Background()
	}
	return r.ctx
}

// StreamHandler returns the stream handler.
func (r *Request) StreamHandler() func(chunk []byte) error {
	return r.streamHandler
}

// Headers returns the client headers to forward to backend.
func (r *Request) Headers() map[string][]string {
	return r.headers
}

// ClientProtocol returns the client protocol.
func (r *Request) ClientProtocol() string {
	return r.clientProtocol
}

// WithModel creates a new request with a different model.
func (r *Request) WithModel(model ModelAlias) *Request {
	newReq := *r
	newReq.model = model
	return &newReq
}

// WithContext creates a new request with a different context.
func (r *Request) WithContext(ctx context.Context) *Request {
	newReq := *r
	newReq.ctx = ctx
	return &newReq
}

// WithStreamHandler creates a new request with a stream handler.
func (r *Request) WithStreamHandler(handler func(chunk []byte) error) *Request {
	newReq := *r
	newReq.streamHandler = handler
	return &newReq
}

// String returns a string representation.
func (r *Request) String() string {
	return fmt.Sprintf("Request(%s, model=%s, messages=%d, stream=%v)",
		r.id, r.model, len(r.messages), r.stream)
}

// RequestBuilder is a builder for creating Request entities.
type RequestBuilder struct {
	id             RequestID
	model          ModelAlias
	messages       []Message
	maxTokens      int
	temperature    float64
	topP           float64
	stream         bool
	stop           []string
	tools          []Tool
	toolChoice     any
	user           string
	ctx            context.Context
	streamHandler  func(chunk []byte) error
	headers        map[string][]string
	clientProtocol string
}

// NewRequestBuilder creates a new request builder.
func NewRequestBuilder() *RequestBuilder {
	return &RequestBuilder{
		temperature: 1.0,
		topP:        1.0,
		ctx:         context.Background(),
	}
}

// ID sets the request ID.
func (rb *RequestBuilder) ID(id RequestID) *RequestBuilder {
	rb.id = id
	return rb
}

// Model sets the model.
func (rb *RequestBuilder) Model(model ModelAlias) *RequestBuilder {
	rb.model = model
	return rb
}

// Messages sets the messages.
func (rb *RequestBuilder) Messages(messages []Message) *RequestBuilder {
	rb.messages = messages
	return rb
}

// MaxTokens sets the max tokens.
func (rb *RequestBuilder) MaxTokens(maxTokens int) *RequestBuilder {
	rb.maxTokens = maxTokens
	return rb
}

// Temperature sets the temperature.
func (rb *RequestBuilder) Temperature(temperature float64) *RequestBuilder {
	rb.temperature = temperature
	return rb
}

// TopP sets the top_p.
func (rb *RequestBuilder) TopP(topP float64) *RequestBuilder {
	rb.topP = topP
	return rb
}

// Stream sets the stream flag.
func (rb *RequestBuilder) Stream(stream bool) *RequestBuilder {
	rb.stream = stream
	return rb
}

// Stop sets the stop sequences.
func (rb *RequestBuilder) Stop(stop []string) *RequestBuilder {
	rb.stop = stop
	return rb
}

// Tools sets the tools.
func (rb *RequestBuilder) Tools(tools []Tool) *RequestBuilder {
	rb.tools = tools
	return rb
}

// ToolChoice sets the tool choice.
func (rb *RequestBuilder) ToolChoice(toolChoice any) *RequestBuilder {
	rb.toolChoice = toolChoice
	return rb
}

// User sets the user ID.
func (rb *RequestBuilder) User(user string) *RequestBuilder {
	rb.user = user
	return rb
}

// Context sets the context.
func (rb *RequestBuilder) Context(ctx context.Context) *RequestBuilder {
	rb.ctx = ctx
	return rb
}

// StreamHandler sets the stream handler.
func (rb *RequestBuilder) StreamHandler(handler func(chunk []byte) error) *RequestBuilder {
	rb.streamHandler = handler
	return rb
}

// Headers sets the client headers to forward to backend.
func (rb *RequestBuilder) Headers(headers map[string][]string) *RequestBuilder {
	rb.headers = headers
	return rb
}

func (rb *RequestBuilder) ClientProtocol(protocol string) *RequestBuilder {
	rb.clientProtocol = protocol
	return rb
}

// Build creates the request entity.
func (rb *RequestBuilder) Build() (*Request, error) {
	if rb.id.IsEmpty() {
		return nil, fmt.Errorf("request ID is required")
	}
	if rb.model.IsEmpty() {
		return nil, fmt.Errorf("model is required")
	}
	if len(rb.messages) == 0 {
		return nil, fmt.Errorf("messages are required")
	}
	return &Request{
		id:             rb.id,
		model:          rb.model,
		messages:       rb.messages,
		maxTokens:      rb.maxTokens,
		temperature:    rb.temperature,
		topP:           rb.topP,
		stream:         rb.stream,
		stop:           rb.stop,
		tools:          rb.tools,
		toolChoice:     rb.toolChoice,
		user:           rb.user,
		ctx:            rb.ctx,
		streamHandler:  rb.streamHandler,
		headers:        rb.headers,
		clientProtocol: rb.clientProtocol,
	}, nil
}

// BuildUnsafe creates the request entity without validation.
func (rb *RequestBuilder) BuildUnsafe() *Request {
	req, _ := rb.Build()
	return req
}

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
