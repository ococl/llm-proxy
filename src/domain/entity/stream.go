package entity

// StreamChunk 表示流式响应中的单个数据块。
// 用于在流式传输过程中传递增量内容。
type StreamChunk struct {
	// Finished 表示流是否结束
	Finished bool

	// Content 是当前块的文本内容
	Content string

	// StopReason 是流结束的原因（如 "stop", "length", "content_filter"）
	StopReason string

	// Error 是错误信息（如果有）
	Error string
}

// NewStreamChunk 创建一个新的流式块。
func NewStreamChunk(content string, finished bool) *StreamChunk {
	return &StreamChunk{
		Content:  content,
		Finished: finished,
	}
}

// NewFinishedStreamChunk 创建一个已结束的流式块。
func NewFinishedStreamChunk(content, stopReason string) *StreamChunk {
	return &StreamChunk{
		Content:    content,
		Finished:   true,
		StopReason: stopReason,
	}
}

// NewErrorStreamChunk 创建一个错误流式块。
func NewErrorStreamChunk(errMsg string) *StreamChunk {
	return &StreamChunk{
		Finished: true,
		Error:    errMsg,
	}
}

// IsEmpty 检查流式块是否为空。
func (sc *StreamChunk) IsEmpty() bool {
	return sc.Content == "" && !sc.Finished && sc.Error == ""
}

// WithContent 设置流式块的内容。
func (sc *StreamChunk) WithContent(content string) *StreamChunk {
	sc.Content = content
	return sc
}

// WithStopReason 设置流式块的结束原因。
func (sc *StreamChunk) WithStopReason(reason string) *StreamChunk {
	sc.StopReason = reason
	return sc
}
