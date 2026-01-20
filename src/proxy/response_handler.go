package proxy

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"llm-proxy/config"
	"llm-proxy/logging"
	"llm-proxy/middleware"
)

// ProxyRequestBuilder 代理请求构建器
type ProxyRequestBuilder struct{}

// NewProxyRequestBuilder 创建代理请求构建器
func NewProxyRequestBuilder() *ProxyRequestBuilder {
	return &ProxyRequestBuilder{}
}

// BuildRequest 构建代理请求
func (b *ProxyRequestBuilder) BuildRequest(
	r *http.Request,
	targetURL *url.URL,
	body []byte,
	protocol string,
	bkend interface{ GetAPIKey() string },
	cfg *config.Config,
	reqID string,
	apiPath string,
) (*http.Request, error) {
	return b.BuildRequestWithAPIKey(r, targetURL, body, protocol, bkend.GetAPIKey(), cfg, reqID, apiPath)
}

// BuildRequestWithAPIKey 构建代理请求（使用 API Key 字符串）
func (b *ProxyRequestBuilder) BuildRequestWithAPIKey(
	r *http.Request,
	targetURL *url.URL,
	body []byte,
	protocol string,
	apiKey string,
	cfg *config.Config,
	reqID string,
	apiPath string,
) (*http.Request, error) {
	// 构建完整的目标 URL
	targetURL = b.buildTargetURL(targetURL, apiPath, r.URL.RawQuery)

	// 创建代理请求
	proxyReq, err := http.NewRequest(r.Method, targetURL.String(), bytes.NewReader(body))
	if err != nil {
		logging.ProxySugar.Errorw("创建代理请求失败", "reqID", reqID, "error", err)
		return nil, err
	}

	// 复制请求头
	for k, v := range r.Header {
		proxyReq.Header[k] = v
	}
	proxyReq.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))

	// 设置协议特定的请求头
	b.setProtocolHeaders(proxyReq, protocol)

	// 设置客户端 IP 转发
	b.setClientIPHeaders(proxyReq, r, cfg)

	// 设置 API Key
	b.setAPIKey(proxyReq, protocol, apiKey)

	logging.ProxySugar.Infow("发送请求到后端",
		"reqID", reqID,
		"method", proxyReq.Method,
		"url", targetURL.String(),
		"body_size", len(body),
		"protocol", protocol)

	return proxyReq, nil
}

// buildTargetURL 构建目标 URL
func (b *ProxyRequestBuilder) buildTargetURL(targetURL *url.URL, apiPath string, rawQuery string) *url.URL {
	backendPath := targetURL.Path
	if backendPath != "" && strings.HasPrefix(apiPath, backendPath) {
		targetURL.Path = apiPath
	} else {
		targetURL.Path = backendPath + apiPath
	}
	targetURL.RawQuery = rawQuery
	return targetURL
}

// setProtocolHeaders 设置协议特定的请求头
func (b *ProxyRequestBuilder) setProtocolHeaders(proxyReq *http.Request, protocol string) {
	if protocol == "anthropic" {
		proxyReq.Header.Set("anthropic-version", anthropicVersionHeader)
		proxyReq.Header.Set("Content-Type", "application/json")
		// 移除 OpenAI 特定的头部
		proxyReq.Header.Del("OpenAI-Organization")
		proxyReq.Header.Del("OpenAI-Project")
	}
}

// setClientIPHeaders 设置客户端 IP 转发头
func (b *ProxyRequestBuilder) setClientIPHeaders(proxyReq *http.Request, r *http.Request, cfg *config.Config) {
	if cfg.Proxy.GetForwardClientIP() {
		clientIP := middleware.ExtractIP(r)
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			proxyReq.Header.Set("X-Forwarded-For", xff+", "+clientIP)
		} else {
			proxyReq.Header.Set("X-Forwarded-For", clientIP)
		}
		if r.Header.Get("X-Real-IP") == "" {
			proxyReq.Header.Set("X-Real-IP", clientIP)
		}
	}
}

// setAPIKey 设置 API Key
func (b *ProxyRequestBuilder) setAPIKey(proxyReq *http.Request, protocol string, apiKey string) {
	if apiKey == "" {
		return
	}

	if protocol == "anthropic" {
		proxyReq.Header.Set("x-api-key", apiKey)
		proxyReq.Header.Del("Authorization") // 移除 OpenAI 的 Authorization 头
	} else {
		proxyReq.Header.Set("Authorization", "Bearer "+apiKey)
	}
}

// GetAPIPath 获取 API 路径
func GetAPIPath(protocol string, originalPath string) string {
	if protocol == "anthropic" {
		return anthropicAPIPath
	}
	return originalPath
}

// ResponseConverter 响应转换器
type ResponseConverter struct {
	converter       *ProtocolConverter
	requestDetector *RequestDetector
}

// NewResponseConverter 创建响应转换器
func NewResponseConverter(converter *ProtocolConverter, detector *RequestDetector) *ResponseConverter {
	return &ResponseConverter{
		converter:       converter,
		requestDetector: detector,
	}
}

// ConvertResponse 转换响应
func (c *ResponseConverter) ConvertResponse(
	bodyBytes []byte,
	protocol string,
	clientProtocol RequestProtocol,
	isPassthrough bool,
	reqID string,
	backendName string,
) ([]byte, error) {
	// 协议直通场景，不需要转换响应
	if isPassthrough {
		logging.ProxySugar.Infow("协议直通响应",
			"reqID", reqID,
			"protocol", protocol,
			"backend", backendName,
			"response_size", len(bodyBytes),
			"note", "客户端与后端协议相同，直接返回")
		return bodyBytes, nil
	}

	// 需要协议转换的场景
	if protocol == "anthropic" && clientProtocol == ProtocolOpenAI {
		// 后端 Anthropic → 客户端 OpenAI
		logging.ProxySugar.Infow("转换后端响应格式",
			"reqID", reqID,
			"from", "anthropic",
			"to", "openai",
			"backend", backendName,
			"response_size", len(bodyBytes))

		convertedBytes, err := c.converter.ConvertFromAnthropic(bodyBytes)
		if err != nil {
			logging.ProxySugar.Errorw("后端响应转换失败",
				"reqID", reqID,
				"protocol", "anthropic",
				"backend", backendName,
				"error", err)
			return bodyBytes, err
		}

		logging.ProxySugar.Infow("后端响应转换成功",
			"reqID", reqID,
			"from", "anthropic",
			"to", "openai",
			"backend", backendName,
			"size", len(convertedBytes))
		return convertedBytes, nil
	}

	if protocol == "openai" && clientProtocol == ProtocolAnthropic {
		// 后端 OpenAI → 客户端 Anthropic
		logging.ProxySugar.Infow("转换响应为客户端协议",
			"reqID", reqID,
			"from", "openai",
			"to", "anthropic",
			"client_protocol", clientProtocol)

		convertedBytes, err := c.requestDetector.ConvertOpenAIToAnthropicResponse(bodyBytes)
		if err != nil {
			logging.ProxySugar.Errorw("客户端协议转换失败",
				"reqID", reqID,
				"error", err)
			return bodyBytes, err
		}

		logging.ProxySugar.Infow("客户端协议转换成功",
			"reqID", reqID,
			"size", len(convertedBytes))
		return convertedBytes, nil
	}

	// 其他情况直接返回
	return bodyBytes, nil
}
