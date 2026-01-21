package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"llm-proxy/backend"
	"llm-proxy/config"
	"llm-proxy/errors"
	"llm-proxy/logging"
	"llm-proxy/middleware"
	"llm-proxy/prompt"

	"github.com/google/uuid"
)

// Helper function to get minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// 常量定义
const (
	streamBufferSize       = 32 * 1024      // 流式响应缓冲区大小
	anthropicVersionHeader = "2023-06-01"   // Anthropic API 版本
	anthropicAPIPath       = "/v1/messages" // Anthropic API 路径
	defaultLogBuilderSize  = 4096           // 日志构建器默认容量
	reqIDPrefix            = "req_"         // 请求ID前缀
	reqIDLength            = 18             // 请求ID长度
)

type Proxy struct {
	configMgr         *config.Manager
	router            *Router
	cooldown          *backend.CooldownManager
	detector          *Detector
	converter         *ProtocolConverter
	requestDetector   *RequestDetector
	bodyPreparer      *RequestBodyPreparer
	requestBuilder    *ProxyRequestBuilder
	responseConverter *ResponseConverter
}

func NewProxy(cfg *config.Manager, router *Router, cd *backend.CooldownManager, det *Detector) *Proxy {
	converter := NewProtocolConverter()
	requestDetector := NewRequestDetector()

	return &Proxy{
		configMgr:         cfg,
		router:            router,
		cooldown:          cd,
		detector:          det,
		converter:         converter,
		requestDetector:   requestDetector,
		bodyPreparer:      NewRequestBodyPreparer(converter, requestDetector),
		requestBuilder:    NewProxyRequestBuilder(),
		responseConverter: NewResponseConverter(converter, requestDetector),
	}
}

var hopByHopHeaders = map[string]bool{
	"Connection":          true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Te":                  true,
	"Trailer":             true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
}

func isHopByHopHeader(header string) bool {
	return hopByHopHeaders[header]
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/health" || r.URL.Path == "/healthz" {
		cfg := p.configMgr.Get()
		health := map[string]interface{}{
			"status":   "healthy",
			"backends": len(cfg.Backends),
			"models":   len(cfg.Models),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(health)
		return
	}

	cfg := p.configMgr.Get()

	// 检测请求协议类型（客户端使用的协议）- 必须在API Key验证之前
	clientProtocol := p.requestDetector.DetectProtocol(r)

	// 根据协议验证API Key
	if cfg.ProxyAPIKey != "" {
		var isValid bool
		var providedKey string

		if clientProtocol == ProtocolAnthropic {
			// Anthropic客户端通常使用x-api-key
			providedKey = r.Header.Get("x-api-key")
			isValid = providedKey == cfg.ProxyAPIKey
		} else {
			// OpenAI客户端使用Authorization Bearer
			auth := r.Header.Get("Authorization")
			expected := "Bearer " + cfg.ProxyAPIKey
			isValid = auth == expected
			providedKey = auth // 用于日志记录
		}

		if !isValid {
			clientIP := middleware.ExtractIP(r)
			logging.NetworkSugar.Warnw("API Key验证失败",
				"client", clientIP,
				"protocol", string(clientProtocol),
				"provided_key_preview", providedKey[:min(20, len(providedKey))])
			errors.WriteJSONError(w, errors.ErrUnauthorized, http.StatusUnauthorized, "")
			return
		}
	}

	if r.URL.Path == "/v1/models" || r.URL.Path == "/models" {
		p.handleModels(w, r)
		return
	}

	reqID := reqIDPrefix + strings.ReplaceAll(uuid.New().String()[:reqIDLength], "-", "")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		logging.NetworkSugar.Errorw("读取请求体失败", "reqID", reqID, "error", err)
		errors.WriteJSONError(w, errors.ErrBadRequest, http.StatusBadRequest, reqID)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	// 记录请求体大小，用于诊断
	requestBodySize := len(body)
	logging.ProxySugar.Infow("请求体大小",
		"reqID", reqID,
		"size_bytes", requestBodySize,
		"size_kb", requestBodySize/1024,
		"client_protocol", clientProtocol)

	var reqBody map[string]interface{}
	if err := json.Unmarshal(body, &reqBody); err != nil {
		logging.NetworkSugar.Warnw("解析请求体失败", "reqID", reqID, "error", err)
		errors.WriteJSONError(w, errors.ErrInvalidJSON, http.StatusBadRequest, reqID)
		return
	}

	// 保存原始请求体，用于直通场景
	originalBody := body

	if cfg.Proxy.EnableSystemPrompt {
		reqBody = prompt.ProcessSystemPrompt(reqBody)
	}

	modelAlias, _ := reqBody["model"].(string)
	if modelAlias == "" {
		logging.NetworkSugar.Warnw("请求缺少model字段", "reqID", reqID)
		errors.WriteJSONError(w, errors.ErrMissingModel, http.StatusBadRequest, reqID)
		return
	}

	clientIP := middleware.ExtractIP(r)

	// 判断是否为流式请求
	isStream := false
	if s, ok := reqBody["stream"].(bool); ok {
		isStream = s
	}

	logging.ProxySugar.Infow("收到请求",
		"reqID", reqID,
		"model", modelAlias,
		"client", clientIP,
		"stream", isStream,
		"client_protocol", clientProtocol)

	routes, err := p.router.Resolve(modelAlias)
	if err != nil {
		logging.ProxySugar.Warnw("解析模型别名失败", "reqID", reqID, "error", err)
		errors.WriteJSONErrorWithMsg(w, errors.ErrBadRequest, http.StatusBadRequest, reqID, fmt.Sprintf("解析模型别名失败: %v", err))
		return
	}
	if len(routes) == 0 {
		logging.ProxySugar.Warnw("未知的模型别名", "reqID", reqID, "model", modelAlias)
		errors.WriteJSONErrorWithMsg(w, errors.ErrUnknownModel, http.StatusBadRequest, reqID, fmt.Sprintf("未知的模型别名: %s", modelAlias))
		return
	}

	logging.FileOnlySugar.Debugw("解析到可用路由", "reqID", reqID, "count", len(routes))

	var logBuilder strings.Builder
	logBuilder.Grow(defaultLogBuilderSize) // 预分配容量
	logBuilder.WriteString(fmt.Sprintf("================== 请求日志 ==================\n"))
	logBuilder.WriteString(fmt.Sprintf("请求ID: %s\n时间: %s\n客户端: %s\n\n", reqID, time.Now().Format(time.RFC3339), clientIP))
	logBuilder.WriteString("--- 请求头 ---\n")
	for k, v := range r.Header {
		logBuilder.WriteString(fmt.Sprintf("%s: %s\n", k, strings.Join(v, ", ")))
	}
	logBuilder.WriteString("\n--- 请求体 ---\n")
	logBuilder.WriteString(string(body))
	logBuilder.WriteString("\n")

	var lastErr error
	var lastStatus int
	var lastBody string

	maxRetries := cfg.Fallback.MaxRetries
	if maxRetries <= 0 {
		maxRetries = len(routes)
	}

	// Metrics 只在 enable_metrics 开启时创建
	var metrics *logging.RequestMetrics
	if cfg.Logging.EnableMetrics {
		metrics = logging.NewRequestMetrics(reqID, modelAlias)
	}
	var finalBackend string

	var backoff *BackoffStrategy
	if cfg.Fallback.IsBackoffEnabled() {
		backoff = NewBackoffStrategy(
			time.Duration(cfg.Fallback.GetBackoffInitialDelay())*time.Millisecond,
			time.Duration(cfg.Fallback.GetBackoffMaxDelay())*time.Millisecond,
			cfg.Fallback.GetBackoffMultiplier(),
			cfg.Fallback.GetBackoffJitter(),
			maxRetries,
		)
	}

	for i, route := range routes {
		if i >= maxRetries {
			break
		}

		if backoff != nil && i > 0 {
			delay := backoff.CalculateDelay(i)
			if delay > 0 {
				logging.ProxySugar.Debugw("指数退避等待",
					"reqID", reqID,
					"attempt", i+1,
					"delay_ms", delay.Milliseconds())
				time.Sleep(delay)
			}
		}

		logBuilder.WriteString(fmt.Sprintf("\n--- 尝试 %d ---\n", i+1))
		logBuilder.WriteString(fmt.Sprintf("后端: %s\n模型: %s\n", route.BackendName, route.Model))

		// 获取后端配置以确定协议
		bkend := p.configMgr.GetBackend(route.BackendName)
		if bkend == nil {
			lastErr = fmt.Errorf("backend %s not found", route.BackendName)
			logBuilder.WriteString(fmt.Sprintf("后端配置未找到: %v\n", lastErr))
			logging.ProxySugar.Errorw("后端配置未找到", "reqID", reqID, "backend", route.BackendName)
			continue
		}

		// 确定使用的协议（模型级别优先）
		protocol := route.GetProtocol(bkend.GetProtocol())
		logBuilder.WriteString(fmt.Sprintf("协议: %s\n", protocol))

		// 检测是否为直通场景（客户端协议 == 后端协议）
		isPassthrough := (clientProtocol == ProtocolAnthropic && protocol == "anthropic") ||
			(clientProtocol == ProtocolOpenAI && protocol == "openai")

		if isPassthrough {
			logging.ProxySugar.Infow("协议直通模式",
				"reqID", reqID,
				"protocol", protocol,
				"backend", route.BackendName,
				"note", "客户端与后端协议相同，无需转换")
		}

		// 在控制台明确打印协议类型
		logging.ProxySugar.Infow("转发请求",
			"reqID", reqID,
			"attempt", i+1,
			"backend", route.BackendName,
			"model", route.Model,
			"protocol", protocol,
			"client_protocol", clientProtocol,
			"passthrough", isPassthrough,
			"stream", isStream)

		var prepareResult *PrepareResult
		var err error

		// 使用 RequestBodyPreparer 准备请求体
		prepareResult, err = p.bodyPreparer.PrepareRequestBody(
			reqBody,
			originalBody,
			&route,
			protocol,
			clientProtocol,
			reqID,
			&logBuilder,
		)
		if err != nil {
			lastErr = err
			continue
		}

		newBody := prepareResult.Body
		isPassthrough = prepareResult.IsPassthrough

		targetURL, err := url.Parse(route.BackendURL)
		if err != nil {
			lastErr = err
			logBuilder.WriteString(fmt.Sprintf("解析后端URL失败: %v\n", err))
			logging.ProxySugar.Errorw("解析后端URL失败", "reqID", reqID, "error", err)
			continue
		}

		// 根据协议确定端点路径
		apiPath := GetAPIPath(protocol, r.URL.Path)
		logBuilder.WriteString(fmt.Sprintf("API路径: %s\n", apiPath))

		// 使用 ProxyRequestBuilder 构建代理请求
		apiKey := bkend.APIKey
		proxyReq, err := p.requestBuilder.BuildRequestWithAPIKey(
			r,
			targetURL,
			newBody,
			protocol,
			apiKey,
			cfg,
			reqID,
			apiPath,
		)
		if err != nil {
			lastErr = err
			logBuilder.WriteString(fmt.Sprintf("构建代理请求失败: %v\n", err))
			continue
		}

		client := GetHTTPClient()
		backendStart := time.Now()
		resp, err := client.Do(proxyReq)
		backendDuration := time.Since(backendStart)
		if metrics != nil {
			metrics.RecordBackendTime(route.BackendName, backendDuration)
		}

		if err != nil {
			lastErr = err
			logBuilder.WriteString(fmt.Sprintf("\n--- 响应详情 ---\n"))
			logBuilder.WriteString(fmt.Sprintf("错误: %v\n", err))
			logBuilder.WriteString(fmt.Sprintf("耗时: %dms\n", backendDuration.Milliseconds()))
			logging.NetworkSugar.Warnw("后端请求失败", "reqID", reqID, "backend", route.BackendName, "error", err, "duration_ms", backendDuration.Milliseconds())

			errorContent := fmt.Sprintf("================== 错误日志 ==================\n请求ID: %s\n时间: %s\n后端: %s\n模型: %s\n错误: %v\n耗时: %dms\n",
				reqID, time.Now().Format(time.RFC3339), route.BackendName, route.Model, err, backendDuration.Milliseconds())
			logging.WriteErrorLogFile(cfg, fmt.Sprintf("%s_%s", reqID, route.BackendName), errorContent)

			key := p.cooldown.Key(route.BackendName, route.Model)
			p.cooldown.SetCooldown(key, time.Duration(cfg.Fallback.CooldownSeconds)*time.Second)
			continue
		}

		logBuilder.WriteString(fmt.Sprintf("\n--- 响应详情 ---\n"))
		logBuilder.WriteString(fmt.Sprintf("状态码: %d\n", resp.StatusCode))
		logBuilder.WriteString(fmt.Sprintf("响应头:\n"))
		for k, v := range resp.Header {
			logBuilder.WriteString(fmt.Sprintf("  %s: %s\n", k, strings.Join(v, ", ")))
		}
		logBuilder.WriteString(fmt.Sprintf("耗时: %dms\n", backendDuration.Milliseconds()))

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			logBuilder.WriteString(fmt.Sprintf("结果: 成功\n"))
			attempts := 0
			if metrics != nil {
				attempts = metrics.Attempts
			}

			// 记录成功信息，包含协议类型
			logging.ProxySugar.Infow("请求成功",
				"reqID", reqID,
				"backend", route.BackendName,
				"model", route.Model,
				"protocol", protocol,
				"stream", isStream,
				"status", resp.StatusCode,
				"duration_ms", backendDuration.Milliseconds(),
				"attempts", attempts)
			logging.WriteRequestLogFile(cfg, reqID, logBuilder.String())

			finalBackend = route.BackendName
			if metrics != nil {
				metrics.Finish(true, finalBackend)
			}

			for k, v := range resp.Header {
				if isHopByHopHeader(k) {
					continue
				}
				w.Header()[k] = v
			}

			if isStream {
				w.Header().Set("Cache-Control", "no-cache")
				w.Header().Set("X-Accel-Buffering", "no")
			}
			w.WriteHeader(resp.StatusCode)

			if isStream {
				logging.ProxySugar.Infow("开始流式传输",
					"reqID", reqID,
					"backend", route.BackendName,
					"protocol", protocol,
					"model", route.Model)
				logging.FileOnlySugar.Debugw("后端响应头部", "reqID", reqID, "backend", route.BackendName, "headers", resp.Header)
				p.streamResponse(w, resp.Body, route.BackendName, protocol, clientProtocol, prepareResult)
				logging.ProxySugar.Infow("完成流式传输",
					"reqID", reqID,
					"backend", route.BackendName,
					"protocol", protocol,
					"model", route.Model)
			} else {
				bodyBytes, err := io.ReadAll(resp.Body)
				if err != nil {
					logging.ProxySugar.Errorw("读取非流式响应失败",
						"reqID", reqID,
						"backend", route.BackendName,
						"protocol", protocol,
						"error", err)
				} else {
					// 使用 ResponseConverter 转换响应
					convertedBytes, convErr := p.responseConverter.ConvertResponse(
						bodyBytes,
						protocol,
						clientProtocol,
						isPassthrough,
						reqID,
						route.BackendName,
					)
					if convErr != nil {
						logging.ProxySugar.Warnw("响应转换失败，使用原始响应",
							"reqID", reqID,
							"error", convErr)
					} else {
						bodyBytes = convertedBytes
					}

					logging.ProxySugar.Infow("非流式响应",
						"reqID", reqID,
						"backend", route.BackendName,
						"protocol", protocol,
						"client_protocol", clientProtocol,
						"passthrough", isPassthrough,
						"response_size", len(bodyBytes))
					_, writeErr := w.Write(bodyBytes)
					if writeErr != nil {
						logging.ProxySugar.Errorw("写入非流式响应失败", "reqID", reqID, "error", writeErr)
					}
				}
			}
			resp.Body.Close()
			return
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			logBuilder.WriteString(fmt.Sprintf("读取响应体失败: %v\n", err))
			logging.NetworkSugar.Warnw("读取响应体失败", "reqID", reqID, "error", err)
		}
		lastStatus = resp.StatusCode
		lastBody = string(respBody)

		logBuilder.WriteString(fmt.Sprintf("状态: %d\n响应: %s\n", resp.StatusCode, lastBody))
		logging.NetworkSugar.Warnw("后端返回错误", "reqID", reqID, "backend", route.BackendName, "status", resp.StatusCode, "duration_ms", backendDuration.Milliseconds())

		errorContent := fmt.Sprintf("================== 错误日志 ==================\n请求ID: %s\n时间: %s\n后端: %s\n模型: %s\n状态码: %d\n\n--- 响应内容 ---\n%s\n",
			reqID, time.Now().Format(time.RFC3339), route.BackendName, route.Model, resp.StatusCode, lastBody)
		logging.WriteErrorLogFile(cfg, fmt.Sprintf("%s_%s", reqID, route.BackendName), errorContent)

		if p.detector.ShouldFallback(resp.StatusCode, lastBody) {
			key := p.cooldown.Key(route.BackendName, route.Model)
			p.cooldown.SetCooldown(key, time.Duration(cfg.Fallback.CooldownSeconds)*time.Second)
			logBuilder.WriteString(fmt.Sprintf("操作: 冷却 %s，尝试下一个后端\n", key))
			logging.ProxySugar.Debugw("触发回退", "reqID", reqID, "backend", key, "action", "进入冷却")
			continue
		}

		logging.WriteRequestLogFile(cfg, reqID, logBuilder.String())
		finalBackend = route.BackendName
		if metrics != nil {
			metrics.Finish(false, finalBackend)
		}
		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)
		return
	}

	logBuilder.WriteString("\n--- 最终结果 ---\n所有后端均失败\n")
	logging.NetworkSugar.Errorw("所有后端均失败", "reqID", reqID)
	logging.WriteRequestLogFile(cfg, reqID, logBuilder.String())
	logging.WriteErrorLogFile(cfg, reqID, logBuilder.String())

	if metrics != nil {
		var backendDetails []string
		for bkend, duration := range metrics.BackendTimes {
			backendDetails = append(backendDetails, fmt.Sprintf("%s=%dms", bkend, duration.Milliseconds()))
		}
		logging.NetworkSugar.Errorw("所有后端均失败详情", "reqID", reqID, "model", modelAlias, "attempts", metrics.Attempts, "backend_details", strings.Join(backendDetails, ", "))
		metrics.Finish(false, "")
	} else {
		logging.NetworkSugar.Errorw("所有后端均失败详情", "reqID", reqID, "model", modelAlias)
	}

	if lastErr != nil {
		errors.WriteJSONErrorWithMsg(w, errors.ErrNoBackend, http.StatusBadGateway, reqID, fmt.Sprintf("所有后端均失败: %v", lastErr))
		return
	}
	w.WriteHeader(lastStatus)
	w.Write([]byte(lastBody))
}

func (p *Proxy) streamResponse(w http.ResponseWriter, body io.ReadCloser, backendName string, protocol string, clientProtocol RequestProtocol, prepResult *PrepareResult) {
	logging.ProxySugar.Infow("开始流式响应处理", "backend", backendName, "protocol", protocol, "client_protocol", clientProtocol)

	flusher, ok := w.(http.Flusher)
	if !ok {
		logging.ProxySugar.Warnw("不支持流式响应", "backend", backendName, "protocol", protocol)
		io.Copy(w, body)
		return
	}

	// 场景2: 后端 Anthropic → 客户端 OpenAI (已实现)
	if protocol == "anthropic" && clientProtocol == ProtocolOpenAI {
		logging.ProxySugar.Infow("使用 Anthropic→OpenAI 流式转换",
			"backend", backendName,
			"protocol", protocol)
		p.streamAnthropicResponse(w, body, backendName, flusher)
		return
	}

	// 场景3: 后端 OpenAI → 客户端 Anthropic (新实现)
	if protocol == "openai" && clientProtocol == ProtocolAnthropic {
		logging.ProxySugar.Infow("使用 OpenAI→Anthropic 流式转换",
			"backend", backendName,
			"client_protocol", clientProtocol)
		p.streamOpenAIToAnthropicResponse(w, body, backendName, flusher, prepResult)
		return
	}

	// 场景1和场景4: 直通场景,原始流式传输
	logging.ProxySugar.Infow("使用原始流式传输(直通)",
		"backend", backendName,
		"protocol", protocol)
	buf := make([]byte, streamBufferSize)
	bytesProcessed := 0
	chunksReceived := 0

	for {
		n, err := body.Read(buf)
		chunksReceived++
		if n > 0 {
			bytesProcessed += n
			chunk := buf[:n]

			logging.FileOnlySugar.Debugw("接收SSE数据块", "chunk_number", chunksReceived, "size", n, "total_bytes", bytesProcessed, "backend", backendName)

			if _, writeErr := w.Write(chunk); writeErr != nil {
				logging.ProxySugar.Errorw("写入响应失败", "error", writeErr, "chunk_number", chunksReceived)
				break
			}
			flusher.Flush()
		}
		if err != nil {
			if err == io.EOF {
				logging.FileOnlySugar.Debugw("SSE流结束", "total_bytes", bytesProcessed, "total_chunks", chunksReceived, "backend", backendName)
				break
			} else {
				logging.ProxySugar.Errorw("读取SSE流错误", "error", err, "chunk_number", chunksReceived, "backend", backendName)
				break
			}
		}
	}
	logging.FileOnlySugar.Infow("SSE流传输完成", "total_bytes", bytesProcessed, "total_chunks", chunksReceived, "backend", backendName)
}

func (p *Proxy) streamAnthropicResponse(w http.ResponseWriter, body io.ReadCloser, backendName string, flusher http.Flusher) {
	scanner := bufio.NewScanner(body)
	var eventBuffer strings.Builder
	linesProcessed := 0

	for scanner.Scan() {
		line := scanner.Text()
		linesProcessed++

		// SSE events are separated by empty lines
		if line == "" {
			if eventBuffer.Len() > 0 {
				// Convert the complete event
				convertedEvent, err := p.converter.ConvertAnthropicStreamToOpenAI(eventBuffer.String())
				if err == nil && convertedEvent != "" {
					if _, writeErr := w.Write([]byte(convertedEvent)); writeErr != nil {
						logging.ProxySugar.Errorw("写入转换后的SSE事件失败", "backend", backendName, "error", writeErr)
						return
					}
					flusher.Flush()
				}
				eventBuffer.Reset()
			}
			continue
		}

		// Accumulate lines for the current event
		if eventBuffer.Len() > 0 {
			eventBuffer.WriteString("\n")
		}
		eventBuffer.WriteString(line)
	}

	// Handle any remaining event
	if eventBuffer.Len() > 0 {
		convertedEvent, err := p.converter.ConvertAnthropicStreamToOpenAI(eventBuffer.String())
		if err == nil && convertedEvent != "" {
			w.Write([]byte(convertedEvent))
			flusher.Flush()
		}
	}

	if err := scanner.Err(); err != nil {
		logging.ProxySugar.Errorw("读取Anthropic SSE流错误", "backend", backendName, "error", err)
	}

	logging.FileOnlySugar.Infow("Anthropic SSE流传输完成", "lines_processed", linesProcessed, "backend", backendName)
}

func (p *Proxy) streamOpenAIToAnthropicResponse(w http.ResponseWriter, body io.ReadCloser, backendName string, flusher http.Flusher, prepResult *PrepareResult) {
	scanner := bufio.NewScanner(body)
	var eventsSent int
	var model string
	if prepResult != nil && prepResult.ConversionMeta != nil {
		logging.ProxySugar.Debugw("OpenAI→Anthropic流式转换开始",
			"backend", backendName,
			"有转换元数据", prepResult.ConversionMeta != nil)
	}

	messageStartSent := false

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		data = strings.TrimSpace(data)

		if data == "[DONE]" {
			events := p.converter.ConvertOpenAIStreamEndToAnthropic()
			for _, event := range events {
				eventJSON, _ := json.Marshal(event)
				w.Write([]byte("event: " + event["type"].(string) + "\ndata: " + string(eventJSON) + "\n\n"))
				flusher.Flush()
				eventsSent++
			}
			break
		}

		if data == "" {
			continue
		}

		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			logging.ProxySugar.Warnw("解析OpenAI SSE chunk失败",
				"backend", backendName,
				"error", err,
				"data", data)
			continue
		}

		if !messageStartSent {
			if chunkModel, ok := chunk["model"].(string); ok {
				model = chunkModel
			}
			events := p.converter.ConvertOpenAIStreamStartToAnthropic(model)
			for _, event := range events {
				eventJSON, _ := json.Marshal(event)
				w.Write([]byte("event: " + event["type"].(string) + "\ndata: " + string(eventJSON) + "\n\n"))
				flusher.Flush()
				eventsSent++
			}
			messageStartSent = true
		}

		event, err := p.converter.ConvertOpenAIStreamChunkToAnthropic(chunk)
		if err != nil {
			logging.ProxySugar.Warnw("转换OpenAI chunk失败",
				"backend", backendName,
				"error", err)
			continue
		}

		if event != nil {
			eventJSON, _ := json.Marshal(event)
			eventType := event["type"].(string)
			w.Write([]byte("event: " + eventType + "\ndata: " + string(eventJSON) + "\n\n"))
			flusher.Flush()
			eventsSent++
		}
	}

	if err := scanner.Err(); err != nil {
		logging.ProxySugar.Errorw("读取OpenAI SSE流错误",
			"backend", backendName,
			"error", err)
	}

	logging.FileOnlySugar.Infow("OpenAI→Anthropic SSE流传输完成",
		"backend", backendName,
		"events_sent", eventsSent)
}

func (p *Proxy) handleModels(w http.ResponseWriter, r *http.Request) {
	cfg := p.configMgr.Get()
	clientIP := middleware.ExtractIP(r)
	logging.ProxySugar.Debugw("收到模型列表请求", "client", clientIP)

	type Model struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	}

	type Response struct {
		Object string  `json:"object"`
		Data   []Model `json:"data"`
	}

	var models []Model
	for alias, modelAlias := range cfg.Models {
		if modelAlias == nil || !modelAlias.IsEnabled() {
			continue
		}
		models = append(models, Model{
			ID:      alias,
			Object:  "model",
			Created: time.Now().Unix(),
			OwnedBy: "llm-proxy",
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})

	logging.ProxySugar.Debugw("返回可用模型", "count", len(models))
	resp := Response{Object: "list", Data: models}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
