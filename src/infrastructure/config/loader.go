package config

import (
	"llm-proxy/config"
)

// Loader 配置加载器，封装 config.Manager
type Loader struct {
	manager *config.Manager
}

// NewLoader 创建配置加载器
func NewLoader(configPath string) (*Loader, error) {
	manager, err := config.NewManager(configPath)
	if err != nil {
		return nil, err
	}
	return &Loader{manager: manager}, nil
}

// Get 获取配置
func (l *Loader) Get() *config.Config {
	return l.manager.Get()
}

// GetBackend 获取指定后端配置
func (l *Loader) GetBackend(name string) *config.Backend {
	return l.manager.GetBackend(name)
}

// GetManager 获取底层 Manager（用于兼容旧代码）
func (l *Loader) GetManager() *config.Manager {
	return l.manager
}
