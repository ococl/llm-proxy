package protocol

import (
	"sync"

	"llm-proxy/application/service/protocol/anthropic"
	"llm-proxy/application/service/protocol/azure"
	"llm-proxy/application/service/protocol/google"
	"llm-proxy/application/service/protocol/openai"
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

// StrategyFactory 协议策略工厂。
// 负责创建和注册所有默认的协议转换策略。
type StrategyFactory struct {
	logger port.Logger
}

// NewStrategyFactory 创建策略工厂实例。
//
// 参数：
//   - logger: 日志记录器（可选）
//
// 返回：
//   - 初始化后的策略工厂
func NewStrategyFactory(logger port.Logger) *StrategyFactory {
	if logger == nil {
		logger = &port.NopLogger{}
	}
	return &StrategyFactory{
		logger: logger,
	}
}

// CreateDefaultRegistry 创建默认的策略注册表。
// 自动注册所有支持的协议策略。
func (f *StrategyFactory) CreateDefaultRegistry() *StrategyRegistry {
	registry := NewStrategyRegistry(f.logger)

	// 注册 OpenAI 兼容协议策略（OpenAI, DeepSeek, Groq, Mistral, Cohere）
	openAIRequest := openai.NewRequestConverter(nil, f.logger)
	openAIResponse := openai.NewResponseConverter(f.logger)
	openAIStream := openai.NewStreamChunkConverter(f.logger)
	openAIError := openai.NewErrorConverter(f.logger)

	// OpenAI 及其兼容协议（请求、响应、流式共享，错误转换器独立）
	registry.RegisterRequestStrategy(openAIRequest)
	registry.RegisterResponseStrategy(openAIResponse)
	registry.RegisterChunkStrategy(openAIStream)
	registry.RegisterErrorStrategy(openAIError)

	// 注册 Azure OpenAI 专用错误转换器
	// Azure 使用 OpenAI 兼容的请求/响应/流式格式，但错误格式不同
	azureError := azure.NewErrorConverter(f.logger)
	registry.RegisterErrorStrategy(azureError)

	// 注册 Anthropic 协议策略
	anthropicRequest := anthropic.NewRequestConverter(nil, f.logger)
	anthropicResponse := anthropic.NewResponseConverter(f.logger)
	anthropicStream := anthropic.NewStreamChunkConverter(f.logger)
	anthropicError := anthropic.NewErrorConverter(f.logger)

	registry.RegisterRequestStrategy(anthropicRequest)
	registry.RegisterResponseStrategy(anthropicResponse)
	registry.RegisterChunkStrategy(anthropicStream)
	registry.RegisterErrorStrategy(anthropicError)

	// 注册 Google Vertex AI 协议策略
	googleRequest := google.NewRequestConverter(f.logger)
	googleResponse := google.NewResponseConverter(f.logger)
	googleStream := google.NewStreamChunkConverter(f.logger)
	googleError := google.NewErrorConverter(f.logger)

	registry.RegisterRequestStrategy(googleRequest)
	registry.RegisterResponseStrategy(googleResponse)
	registry.RegisterChunkStrategy(googleStream)
	registry.RegisterErrorStrategy(googleError)

	f.logger.Debug("默认策略注册表创建完成",
		port.Int("request_strategies", len(registry.requestStrategies)),
		port.Int("response_strategies", len(registry.responseStrategies)),
		port.Int("stream_strategies", len(registry.chunkStrategies)),
		port.Int("error_strategies", len(registry.errorStrategies)),
	)

	return registry
}

// CreateRegistryWithCustomStrategies 创建带有自定义策略的注册表。
func (f *StrategyFactory) CreateRegistryWithCustomStrategies(customStrategies map[types.Protocol]StrategySet) *StrategyRegistry {
	registry := f.CreateDefaultRegistry()

	// 添加自定义策略（覆盖默认策略）
	for protocol, strategies := range customStrategies {
		if strategies.Request != nil {
			registry.RegisterRequestStrategy(strategies.Request)
		}
		if strategies.Response != nil {
			registry.RegisterResponseStrategy(strategies.Response)
		}
		if strategies.StreamChunk != nil {
			registry.RegisterChunkStrategy(strategies.StreamChunk)
		}
		if strategies.Error != nil {
			registry.RegisterErrorStrategy(strategies.Error)
		}

		f.logger.Debug("自定义策略已注册",
			port.String("protocol", string(protocol)),
		)
	}

	return registry
}

// StrategySet 策略集合，用于配置自定义策略。
type StrategySet struct {
	Request     RequestConverterStrategy
	Response    ResponseConverterStrategy
	StreamChunk StreamChunkConverterStrategy
	Error       ErrorConverterStrategy
}

func NewOpenAIStrategies(logger port.Logger) StrategySet {
	return StrategySet{
		Request:     openai.NewRequestConverter(nil, logger),
		Response:    openai.NewResponseConverter(logger),
		StreamChunk: openai.NewStreamChunkConverter(logger),
		Error:       openai.NewErrorConverter(logger),
	}
}

// NewAnthropicStrategies 创建 Anthropic 协议策略集合。
func NewAnthropicStrategies(logger port.Logger, systemPrompts map[string]string) StrategySet {
	return StrategySet{
		Request:     anthropic.NewRequestConverter(systemPrompts, logger),
		Response:    anthropic.NewResponseConverter(logger),
		StreamChunk: anthropic.NewStreamChunkConverter(logger),
		Error:       anthropic.NewErrorConverter(logger),
	}
}

// NewGoogleStrategies 创建 Google Vertex AI 协议策略集合。
func NewGoogleStrategies(logger port.Logger) StrategySet {
	return StrategySet{
		Request:     google.NewRequestConverter(logger),
		Response:    google.NewResponseConverter(logger),
		StreamChunk: google.NewStreamChunkConverter(logger),
		Error:       google.NewErrorConverter(logger),
	}
}

// ProtocolRegistry 协议注册表管理器。
// 提供线程安全的策略注册和查询功能。
type ProtocolRegistry struct {
	factory    *StrategyFactory
	registries map[types.Protocol]*StrategySet
	mu         sync.RWMutex
}

// NewProtocolRegistry 创建协议注册表管理器。
func NewProtocolRegistry(factory *StrategyFactory) *ProtocolRegistry {
	return &ProtocolRegistry{
		factory:    factory,
		registries: make(map[types.Protocol]*StrategySet),
	}
}

// Register 注册指定协议的策略集合。
func (r *ProtocolRegistry) Register(protocol types.Protocol, strategies StrategySet) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.registries[protocol] = &strategies
}

// Get 获取指定协议的策略集合。
func (r *ProtocolRegistry) Get(protocol types.Protocol) *StrategySet {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.registries[protocol]
}

// GetRequest 获取指定协议的请求转换策略。
func (r *ProtocolRegistry) GetRequest(protocol types.Protocol) RequestConverterStrategy {
	if strategies := r.Get(protocol); strategies != nil {
		return strategies.Request
	}
	return nil
}

// GetResponse 获取指定协议的响应转换策略。
func (r *ProtocolRegistry) GetResponse(protocol types.Protocol) ResponseConverterStrategy {
	if strategies := r.Get(protocol); strategies != nil {
		return strategies.Response
	}
	return nil
}

// GetStreamChunk 获取指定协议的流式块转换策略。
func (r *ProtocolRegistry) GetStreamChunk(protocol types.Protocol) StreamChunkConverterStrategy {
	if strategies := r.Get(protocol); strategies != nil {
		return strategies.StreamChunk
	}
	return nil
}

// GetError 获取指定协议的错误转换策略。
func (r *ProtocolRegistry) GetError(protocol types.Protocol) ErrorConverterStrategy {
	if strategies := r.Get(protocol); strategies != nil {
		return strategies.Error
	}
	return nil
}
