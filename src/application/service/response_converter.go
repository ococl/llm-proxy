package service

import (
	"llm-proxy/domain/entity"
)

// ResponseConverter handles response format conversions and transformations.
type ResponseConverter struct{}

// NewResponseConverter creates a new response converter.
func NewResponseConverter() *ResponseConverter {
	return &ResponseConverter{}
}

// NormalizeResponse normalizes a response to ensure consistent format.
func (rc *ResponseConverter) NormalizeResponse(resp *entity.Response) *entity.Response {
	// Ensure response has required fields
	if resp == nil {
		return nil
	}

	// Normalize choices
	choices := resp.Choices()
	if len(choices) == 0 {
		// Create empty choice if none exist
		choices = []entity.Choice{
			entity.NewChoice(0, entity.NewMessage("assistant", ""), "stop"),
		}
	}

	// Build normalized response
	builder := entity.NewResponseBuilder().
		ID(resp.ID()).
		Model(resp.Model()).
		Created(resp.Created()).
		Choices(choices).
		Usage(resp.Usage())

	if resp.StopReason() != "" {
		builder = builder.StopReason(resp.StopReason())
	}
	if len(resp.StopSequences()) > 0 {
		builder = builder.StopSequences(resp.StopSequences())
	}

	normalized, err := builder.Build()
	if err != nil {
		// If normalization fails, return original
		return resp
	}

	return normalized
}

// MergeStreamChunks merges multiple streaming response chunks.
func (rc *ResponseConverter) MergeStreamChunks(chunks []*entity.Response) *entity.Response {
	if len(chunks) == 0 {
		return nil
	}

	// Use first chunk as base
	base := chunks[0]
	if len(chunks) == 1 {
		return base
	}

	// Merge content from all chunks
	var mergedContent string
	var lastFinishReason string
	var totalUsage entity.Usage

	for _, chunk := range chunks {
		if firstChoice := chunk.FirstChoice(); firstChoice != nil {
			if firstChoice.Delta != nil {
				mergedContent += firstChoice.Delta.Content
			}
			if firstChoice.FinishReason != "" {
				lastFinishReason = firstChoice.FinishReason
			}
		}
		// Accumulate usage
		usage := chunk.Usage()
		totalUsage = entity.NewUsage(
			totalUsage.PromptTokens+usage.PromptTokens,
			totalUsage.CompletionTokens+usage.CompletionTokens,
		)
	}

	// Build merged response
	mergedChoice := entity.NewChoice(
		0,
		entity.NewMessage("assistant", mergedContent),
		lastFinishReason,
	)

	return entity.NewResponse(
		base.ID(),
		base.Model(),
		[]entity.Choice{mergedChoice},
		totalUsage,
	)
}
