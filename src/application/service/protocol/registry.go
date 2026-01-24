package protocol

import (
	"llm-proxy/domain/port"
	"llm-proxy/domain/types"
)

// StrategyRegistry 策略注册表，管理所有可用的协议转换策略。
// 提供策略的注册、查找和枚举功能。
type StrategyRegistry struct {
	requestStrategies  map[types.Protocol]RequestConverterStrategy
	responseStrategies map[types.Protocol]ResponseConverterStrategy
	chunkStrategies    map[types.Protocol]StreamChunkConverterStrategy
	errorStrategies    map[types.Protocol]ErrorConverterStrategy
	logger             port.Logger
}

// NewStrategyRegistry 创建新的策略注册表。
//
// 参数：
//   - logger: 日志记录器（可选，为 nil 时使用 NopLogger）
//
// 返回：
//   - 初始化的策略注册表
func NewStrategyRegistry(logger port.Logger) *StrategyRegistry {
	if logger == nil {
		logger = &port.NopLogger{}
	}
	return &StrategyRegistry{
		requestStrategies:  make(map[types.Protocol]RequestConverterStrategy),
		responseStrategies: make(map[types.Protocol]ResponseConverterStrategy),
		chunkStrategies:    make(map[types.Protocol]StreamChunkConverterStrategy),
		errorStrategies:    make(map[types.Protocol]ErrorConverterStrategy),
		logger:             logger,
	}
}

// RegisterRequestStrategy 注册请求转换策略。
//
// 参数：
//   - strategy: 请求转换策略实现
func (r *StrategyRegistry) RegisterRequestStrategy(strategy RequestConverterStrategy) {
	protocol := strategy.Protocol()
	r.requestStrategies[protocol] = strategy
	r.logger.Debug("注册请求转换策略",
		port.String("protocol", string(protocol)),
		port.String("name", strategy.Name()),
	)
}

// RegisterResponseStrategy 注册响应转换策略。
//
// 参数：
//   - strategy: 响应转换策略实现
func (r *StrategyRegistry) RegisterResponseStrategy(strategy ResponseConverterStrategy) {
	protocol := strategy.Protocol()
	r.responseStrategies[protocol] = strategy
	r.logger.Debug("注册响应转换策略",
		port.String("protocol", string(protocol)),
		port.String("name", strategy.Name()),
	)
}

// RegisterChunkStrategy 注册流式块转换策略。
//
// 参数：
//   - strategy: 流式块转换策略实现
func (r *StrategyRegistry) RegisterChunkStrategy(strategy StreamChunkConverterStrategy) {
	protocol := strategy.Protocol()
	r.chunkStrategies[protocol] = strategy
	r.logger.Debug("注册流式块转换策略",
		port.String("protocol", string(protocol)),
		port.String("name", strategy.Name()),
	)
}

// RegisterErrorStrategy 注册错误转换策略。
//
// 参数：
//   - strategy: 错误转换策略实现
func (r *StrategyRegistry) RegisterErrorStrategy(strategy ErrorConverterStrategy) {
	protocol := strategy.Protocol()
	r.errorStrategies[protocol] = strategy
	r.logger.Debug("注册错误转换策略",
		port.String("protocol", string(protocol)),
		port.String("name", strategy.Name()),
	)
}

// GetRequestStrategy 获取指定协议的请求转换策略。
//
// 参数：
//   - protocol: 协议类型
//
// 返回：
//   - 请求转换策略（未找到返回 nil）
func (r *StrategyRegistry) GetRequestStrategy(protocol types.Protocol) RequestConverterStrategy {
	return r.requestStrategies[protocol]
}

// GetResponseStrategy 获取指定协议的响应转换策略。
//
// 参数：
//   - protocol: 协议类型
//
// 返回：
//   - 响应转换策略（未找到返回 nil）
func (r *StrategyRegistry) GetResponseStrategy(protocol types.Protocol) ResponseConverterStrategy {
	return r.responseStrategies[protocol]
}

// GetChunkStrategy 获取指定协议的流式块转换策略。
//
// 参数：
//   - protocol: 协议类型
//
// 返回：
//   - 流式块转换策略（未找到返回 nil）
func (r *StrategyRegistry) GetChunkStrategy(protocol types.Protocol) StreamChunkConverterStrategy {
	return r.chunkStrategies[protocol]
}

// GetErrorStrategy 获取指定协议的错误转换策略。
//
// 参数：
//   - protocol: 协议类型
//
// 返回：
//   - 错误转换策略（未找到返回 nil）
func (r *StrategyRegistry) GetErrorStrategy(protocol types.Protocol) ErrorConverterStrategy {
	return r.errorStrategies[protocol]
}

// GetAllProtocols 获取所有已注册的协议类型。
//
// 返回：
//   - 协议类型列表
func (r *StrategyRegistry) GetAllProtocols() []types.Protocol {
	protocols := make([]types.Protocol, 0, len(r.requestStrategies))
	for p := range r.requestStrategies {
		protocols = append(protocols, p)
	}
	return protocols
}

// Count 返回已注册的策略数量。
//
// 返回：
//   - 请求策略数量
func (r *StrategyRegistry) Count() int {
	return len(r.requestStrategies)
}
