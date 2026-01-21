package translator

import (
	"encoding/json"
	"fmt"

	"llm-proxy/logging"

	"github.com/tokligence/openai-anthropic-endpoint-translation/pkg/translator"
)

// Adapter wraps tokligence translator for use in llm-proxy
type Adapter struct {
	t translator.Translator
}

// NewAdapter creates a new tokligence-based translator adapter
func NewAdapter() *Adapter {
	return &Adapter{
		t: translator.NewTranslator(
			translator.WithStopSanitizer(true),
			translator.WithDummyTool("auto_added_tool"),
			translator.WithDefaultMaxTokens(4096),
		),
	}
}

// ConvertToAnthropic converts OpenAI request to Anthropic format using tokligence
func (a *Adapter) ConvertToAnthropic(openAIBody map[string]interface{}) ([]byte, error) {
	// Convert map to tokligence OpenAIChatRequest format
	req, err := mapToOpenAIRequest(openAIBody)
	if err != nil {
		logging.ProxySugar.Errorw("Failed to map to OpenAI request", "error", err)
		return nil, fmt.Errorf("failed to map request: %w", err)
	}

	// Use tokligence to build Anthropic request
	anthropicReq, warnings, err := a.t.BuildRequest(req)
	if err != nil {
		logging.ProxySugar.Errorw("tokligence BuildRequest failed", "error", err)
		return nil, fmt.Errorf("translation failed: %w", err)
	}

	// Log warnings
	for _, w := range warnings {
		logging.ProxySugar.Warnw("Translation warning", "warning", string(w))
	}

	// Convert Anthropic request back to JSON
	return json.Marshal(anthropicReq)
}

// ConvertFromAnthropic converts Anthropic response to OpenAI format using tokligence
func (a *Adapter) ConvertFromAnthropic(anthropicResp []byte) ([]byte, error) {
	// Parse Anthropic response
	var anthropicBody map[string]interface{}
	if err := json.Unmarshal(anthropicResp, &anthropicBody); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Anthropic response: %w", err)
	}

	// Convert to tokligence AnthropicResponse format
	anthropicRespObj := mapToAnthropicResponse(anthropicResp)

	// Use tokligence to convert response
	openAIResp, err := a.t.ConvertResponse(anthropicRespObj, "", false)
	if err != nil {
		logging.ProxySugar.Errorw("tokligence ConvertResponse failed", "error", err)
		return nil, fmt.Errorf("response conversion failed: %w", err)
	}

	// Convert back to JSON
	return json.Marshal(openAIResp)
}

// mapToOpenAIRequest converts llm-proxy's map format to tokligence OpenAIChatRequest
func mapToOpenAIRequest(body map[string]interface{}) (translator.OpenAIChatRequest, error) {
	req := translator.OpenAIChatRequest{}

	// Model
	if model, ok := body["model"].(string); ok {
		req.Model = model
	}

	// Messages
	if messages, ok := body["messages"].([]interface{}); ok {
		openAIMessages := make([]translator.OpenAIMessage, 0, len(messages))
		for _, msg := range messages {
			if msgMap, ok := msg.(map[string]interface{}); ok {
				openAIMsg := mapToOpenAIMessage(msgMap)
				openAIMessages = append(openAIMessages, openAIMsg)
			}
		}
		req.Messages = openAIMessages
	}

	// Tools
	if tools, ok := body["tools"].([]interface{}); ok && len(tools) > 0 {
		toolList := make([]translator.Tool, 0, len(tools))
		for _, tool := range tools {
			if toolMap, ok := tool.(map[string]interface{}); ok {
				toolList = append(toolList, mapToTool(toolMap))
			}
		}
		req.Tools = toolList
	}

	// Tool Choice
	if toolChoice, ok := body["tool_choice"].(interface{}); ok {
		req.ToolChoice = mapToToolChoice(toolChoice)
	}

	// Parallel Tool Calls
	if parallel, ok := body["parallel_tool_calls"].(bool); ok {
		req.ParallelToolCalls = &parallel
	}

	// Stream
	if stream, ok := body["stream"].(bool); ok {
		req.Stream = stream
	}

	// Max Tokens
	if maxTokens, ok := body["max_tokens"].(float64); ok {
		mt := int(maxTokens)
		req.MaxTokens = &mt
	} else if maxTokens, ok := body["max_tokens"].(int); ok {
		req.MaxTokens = &maxTokens
	}

	// Max Completion Tokens
	if maxComp, ok := body["max_completion_tokens"].(float64); ok {
		mc := int(maxComp)
		req.MaxCompletion = &mc
	} else if maxComp, ok := body["max_completion_tokens"].(int); ok {
		req.MaxCompletion = &maxComp
	}

	// Stop
	if stop, ok := body["stop"].(interface{}); ok {
		req.Stop = mapToStopSetting(stop)
	}

	// Temperature
	if temp, ok := body["temperature"].(float64); ok {
		req.Temperature = &temp
	}

	// Top P
	if topP, ok := body["top_p"].(float64); ok {
		req.TopP = &topP
	}

	// Response Format
	if rf, ok := body["response_format"].(map[string]interface{}); ok {
		req.ResponseFormat = rf
	}

	// Metadata
	if meta, ok := body["metadata"].(map[string]interface{}); ok {
		req.Metadata = meta
	}

	// User
	if user, ok := body["user"].(string); ok {
		req.User = user
	}

	return req, nil
}

func mapToOpenAIMessage(msg map[string]interface{}) translator.OpenAIMessage {
	openAIMsg := translator.OpenAIMessage{}

	// Role
	if role, ok := msg["role"].(string); ok {
		openAIMsg.Role = role
	}

	// Content - can be string or array
	if content, ok := msg["content"]; ok {
		switch c := content.(type) {
		case string:
			openAIMsg.Content = []translator.ContentBlock{translator.NewTextBlock(c)}
		case []interface{}:
			blocks := make([]translator.ContentBlock, 0, len(c))
			for _, item := range c {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if blockType, ok := itemMap["type"].(string); ok {
						blocks = append(blocks, translator.NewContentBlock(blockType, itemMap))
					}
				}
			}
			openAIMsg.Content = blocks
		}
	}

	// Tool Calls
	if toolCalls, ok := msg["tool_calls"].([]interface{}); ok {
		calls := make([]translator.ToolCall, 0, len(toolCalls))
		for _, tc := range toolCalls {
			if tcMap, ok := tc.(map[string]interface{}); ok {
				calls = append(calls, mapToToolCall(tcMap))
			}
		}
		openAIMsg.ToolCalls = calls
	}

	// Tool Call ID
	if toolCallID, ok := msg["tool_call_id"].(string); ok {
		openAIMsg.ToolCallID = toolCallID
	}

	// Name
	if name, ok := msg["name"].(string); ok {
		openAIMsg.Name = name
	}

	// Prefix
	if prefix, ok := msg["prefix"].(bool); ok {
		openAIMsg.Prefix = prefix
	}

	// Cache Control
	if cc, ok := msg["cache_control"].(map[string]interface{}); ok {
		openAIMsg.CacheControl = cc
	}

	return openAIMsg
}

func mapToToolCall(tc map[string]interface{}) translator.ToolCall {
	call := translator.ToolCall{}

	if id, ok := tc["id"].(string); ok {
		call.ID = id
	}
	if t, ok := tc["type"].(string); ok {
		call.Type = t
	}
	if fn, ok := tc["function"].(map[string]interface{}); ok {
		call.Function = translator.ToolCallFunction{}
		if name, ok := fn["name"].(string); ok {
			call.Function.Name = name
		}
		if args, ok := fn["arguments"].(string); ok {
			call.Function.Arguments = args
		}
	}

	return call
}

func mapToTool(tool map[string]interface{}) translator.Tool {
	t := translator.Tool{}

	if toolType, ok := tool["type"].(string); ok {
		t.Type = toolType
	}
	t.Raw = tool

	return t
}

func mapToToolChoice(choice interface{}) *translator.ToolChoice {
	if choiceMap, ok := choice.(map[string]interface{}); ok {
		tc := &translator.ToolChoice{}
		if kind, ok := choiceMap["type"].(string); ok {
			tc.Kind = kind
		}
		if fn, ok := choiceMap["function"].(map[string]interface{}); ok {
			if name, ok := fn["name"].(string); ok {
				tc.FunctionName = name
			}
		}
		return tc
	}
	if kind, ok := choice.(string); ok {
		return &translator.ToolChoice{Kind: kind}
	}
	return nil
}

func mapToStopSetting(stop interface{}) *translator.StopSetting {
	setting := &translator.StopSetting{}

	switch s := stop.(type) {
	case string:
		setting.Values = []string{s}
	case []interface{}:
		values := make([]string, 0, len(s))
		for _, v := range s {
			if str, ok := v.(string); ok {
				values = append(values, str)
			}
		}
		setting.Values = values
	case []string:
		setting.Values = s
	}

	return setting
}

func mapToAnthropicResponse(data []byte) translator.AnthropicResponse {
	var resp translator.AnthropicResponse
	json.Unmarshal(data, &resp)
	return resp
}
