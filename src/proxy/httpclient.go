package proxy

import (
	"net"
	"net/http"
	"time"

	"llm-proxy/config"
)

// HTTP 客户端单例，带连接池和超时配置
var httpClient *http.Client

// InitHTTPClient 初始化全局 HTTP 客户端
// 针对 LLM 代理优化，特别是 SSE 流式响应场景
func InitHTTPClient(cfg *config.Config) {
	timeouts := cfg.Timeout

	// 连接池大小：根据后端数量保守估算
	// LLM 后端通常有限，不需要过大的连接池
	maxConnsPerHost := len(cfg.Backends) * 5
	if maxConnsPerHost < 10 {
		maxConnsPerHost = 10
	}
	if maxConnsPerHost > 50 {
		maxConnsPerHost = 50 // 保守上限
	}

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   timeouts.GetConnectTimeout(),
			KeepAlive: 5 * time.Minute, // SSE 需要较长的保活时间
		}).DialContext,
		TLSHandshakeTimeout: timeouts.GetConnectTimeout(),
		// SSE 流式响应需要较长的读取超时
		// 避免在 LLM 生成期间触发超时
		ResponseHeaderTimeout: 3 * time.Minute,
		IdleConnTimeout:       10 * time.Minute, // SSE 可能长时间空闲
		MaxConnsPerHost:       maxConnsPerHost,
		MaxIdleConns:          20,
		MaxIdleConnsPerHost:   maxConnsPerHost / 4,
	}

	// TotalTimeout 控制整个请求生命周期
	// 对于复杂 LLM 查询，设置较长时间
	totalTimeout := timeouts.GetTotalTimeout()
	if totalTimeout < 15*time.Minute {
		totalTimeout = 15 * time.Minute // 最小 15 分钟
	}

	httpClient = &http.Client{
		Timeout:   totalTimeout,
		Transport: transport,
	}
}

// GetHTTPClient 获取全局 HTTP 客户端
// 如果未初始化，返回默认客户端
func GetHTTPClient() *http.Client {
	if httpClient == nil {
		return http.DefaultClient
	}
	return httpClient
}
