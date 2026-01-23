package http

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Server 封装 HTTP 服务器，提供优雅关闭功能
type Server struct {
	server *http.Server
}

// ServerConfig HTTP 服务器配置
type ServerConfig struct {
	Addr         string
	Handler      http.Handler
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// NewServer 创建新的 HTTP 服务器
func NewServer(cfg ServerConfig) *Server {
	return &Server{
		server: &http.Server{
			Addr:         cfg.Addr,
			Handler:      cfg.Handler,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
	}
}

// Start 启动服务器（非阻塞）
func (s *Server) Start() error {
	go func() {
		if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			// 错误会在 Shutdown 时处理
		}
	}()
	return nil
}

// Shutdown 优雅关闭服务器
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// DefaultServerConfig 返回默认服务器配置
func DefaultServerConfig(addr string, handler http.Handler) ServerConfig {
	return ServerConfig{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}
