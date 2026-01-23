package logging

import (
	"llm-proxy/config"
	"llm-proxy/logging"
)

// Factory 日志工厂，封装 logging 包的初始化逻辑
type Factory struct{}

// NewFactory 创建日志工厂
func NewFactory() *Factory {
	return &Factory{}
}

// InitLogger 初始化日志系统
func (f *Factory) InitLogger(cfg *config.Config) error {
	return logging.InitLogger(cfg)
}

// ShutdownLogger 关闭日志系统
func (f *Factory) ShutdownLogger() {
	logging.ShutdownLogger()
}

// GetProxySugar 获取代理日志记录器
func (f *Factory) GetProxySugar() interface{} {
	return logging.ProxySugar
}

// GetGeneralSugar 获取通用日志记录器
func (f *Factory) GetGeneralSugar() interface{} {
	return logging.GeneralSugar
}
