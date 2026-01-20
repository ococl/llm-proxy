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

type Proxy struct {
	configMgr       *config.Manager
	router          *Router
	cooldown        *backend.CooldownManager
	detector        *Detector
	converter       *ProtocolConverter
	requestDetector *RequestDetector
}

func NewProxy(cfg *config.Manager, router *Router, cd *backend.CooldownManager, det *Detector) *Proxy {
	return &Proxy{
		configMgr:       cfg,
		router:          router,
		cooldown:        cd,
		detector:        det,
		converter:       NewProtocolConverter(),
		requestDetector: NewRequestDetector(),
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

	if cfg.ProxyAPIKey != "" {
		auth := r.Header.Get("Authorization")
		expected := "Bearer " + cfg.ProxyAPIKey
		if auth != expected {
			clientIP := middleware.ExtractIP(r)
			logging.NetworkSugar.Warnw("API Key验证失败", "client", clientIP)
			errors.WriteJSONError(w, errors.ErrUnauthorized, http.StatusUnauthorized, "")
			return
		}
	}

	if r.URL.Path == "/v1/models" || r.URL.Path == "/models" {
		p.handleModels(w, r)
		return
	}

	reqID := "req_" + strings.ReplaceAll(uuid.New().String()[:18], "-", "")

	// 检测请求协议类型（客户端使用的协议）
	clientProtocol := p.requestDetector.DetectProtocol(r)

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

	for i, route := range routes {
		if i >= maxRetries {
			break
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

		var newBody []byte
		var err error

		// 协议直通：客户端和后端使用相同协议，直接使用原始请求体
		if isPassthrough {
			// 只需要替换 model 字段
			modifiedBody := make(map[string]interface{})
			if err := json.Unmarshal(originalBody, &modifiedBody); err != nil {
				lastErr = err
				logBuilder.WriteString(fmt.Sprintf("解析原始请求体失败: %v\n", err))
				logging.ProxySugar.Errorw("解析原始请求体失败", "reqID", reqID, "error", err)
				continue
			}
			modifiedBody["model"] = route.Model
			newBody, err = json.Marshal(modifiedBody)
			if err != nil {
				lastErr = err
				logBuilder.WriteString(fmt.Sprintf("序列化请求体失败: %v\n", err))
				logging.ProxySugar.Errorw("序列化请求体失败", "reqID", reqID, "error", err)
				continue
			}
			logBuilder.WriteString(fmt.Sprintf("✓ 协议直通 (%s)，仅替换model字段\n", protocol))
			logging.ProxySugar.Infow("协议直通处理完成",
				"reqID", reqID,
				"protocol", protocol,
				"original_size", len(originalBody),
				"modified_size", len(newBody))
		} else if protocol == "anthropic" && clientProtocol == ProtocolOpenAI {
			// OpenAI → Anthropic 转换
			modifiedBody := make(map[string]interface{})
			for k, v := range reqBody {
				modifiedBody[k] = v
			}
			modifiedBody["model"] = route.Model

			newBody, err = p.converter.ConvertToAnthropic(modifiedBody)
			if err != nil {
				lastErr = err
				logBuilder.WriteString(fmt.Sprintf("转换为Anthropic格式失败: %v\n", err))
				logging.ProxySugar.Errorw("协议转换失败",
					"reqID", reqID,
					"protocol", "anthropic",
					"backend", route.BackendName,
					"error", err)
				continue
			}

			// 获取转换元数据
			convMeta := p.converter.GetLastConversion()

			// 记录转换后的请求体大小
			convertedSize := len(newBody)

			// 详细记录参数转换情况（增加空指针检查）
			if convMeta != nil {
				logging.ProxySugar.Infow("参数转换详情",
					"reqID", reqID,
					"backend", route.BackendName,
					"input_max_tokens", convMeta.InputMaxTokens,
					"output_max_tokens", convMeta.OutputMaxTokens,
					"max_tokens_source", convMeta.MaxTokensSource,
					"input_temperature", convMeta.InputTemperature,
					"output_temperature", convMeta.OutputTemperature,
					"input_top_p", convMeta.InputTopP,
					"output_top_p", convMeta.OutputTopP,
					"input_stream", convMeta.InputStream,
					"output_stream", convMeta.OutputStream,
					"input_stop", convMeta.InputStop,
					"output_stop", convMeta.OutputStop,
					"input_tools_count", convMeta.InputTools,
					"output_tools_count", convMeta.OutputTools,
					"system_prompt_length", convMeta.SystemPromptLen)

				logBuilder.WriteString("✓ 已转换为Anthropic协议格式\n")
				logBuilder.WriteString(fmt.Sprintf("  max_tokens: %v → %d (%s)\n",
					convMeta.InputMaxTokens, convMeta.OutputMaxTokens, convMeta.MaxTokensSource))
				if convMeta.SystemPromptLen > 0 {
					logBuilder.WriteString(fmt.Sprintf("  system prompt: %d 字符\n", convMeta.SystemPromptLen))
				}
			} else {
				logging.ProxySugar.Warnw("转换元数据为空",
					"reqID", reqID,
					"backend", route.BackendName)
				logBuilder.WriteString("✓ 已转换为Anthropic协议格式（无元数据）\n")
			}

			logging.ProxySugar.Infow("Anthropic 请求体转换完成",
				"reqID", reqID,
				"original_size_bytes", requestBodySize,
				"converted_size_bytes", convertedSize,
				"backend", route.BackendName,
				"model", route.Model)

			logging.ProxySugar.Infow("协议转换成功",
				"reqID", reqID,
				"from", "openai",
				"to", "anthropic",
				"backend", route.BackendName)
		} else if protocol == "openai" && clientProtocol == ProtocolAnthropic {
			// Anthropic → OpenAI 转换
			logging.ProxySugar.Infow("检测到Anthropic客户端请求OpenAI后端，需要转换",
				"reqID", reqID,
				"original_path", r.URL.Path)

			convertedBody, err := p.requestDetector.ConvertAnthropicToOpenAI(reqBody)
			if err != nil {
				lastErr = err
				logBuilder.WriteString(fmt.Sprintf("转换Anthropic请求失败: %v\n", err))
				logging.NetworkSugar.Errorw("转换Anthropic请求失败", "reqID", reqID, "error", err)
				continue
			}
			convertedBody["model"] = route.Model
			newBody, err = json.Marshal(convertedBody)
			if err != nil {
				lastErr = err
				logBuilder.WriteString(fmt.Sprintf("序列化请求体失败: %v\n", err))
				logging.ProxySugar.Errorw("序列化请求体失败", "reqID", reqID, "error", err)
				continue
			}
			logBuilder.WriteString("✓ 已转换为OpenAI协议格式\n")
			logging.ProxySugar.Infow("协议转换完成",
				"reqID", reqID,
				"from", "anthropic",
				"to", "openai",
				"backend", route.BackendName)
		} else {
			// OpenAI → OpenAI（直通已在上面处理）
			modifiedBody := make(map[string]interface{})
			for k, v := range reqBody {
				modifiedBody[k] = v
			}
			modifiedBody["model"] = route.Model
			newBody, err = json.Marshal(modifiedBody)
			if err != nil {
				lastErr = err
				logBuilder.WriteString(fmt.Sprintf("序列化请求体失败: %v\n", err))
				logging.ProxySugar.Errorw("序列化请求体失败", "reqID", reqID, "error", err)
				continue
			}
			logBuilder.WriteString("✓ 使用OpenAI协议格式\n")
		}

		targetURL, err := url.Parse(route.BackendURL)
		if err != nil {
			lastErr = err
			logBuilder.WriteString(fmt.Sprintf("解析后端URL失败: %v\n", err))
			logging.ProxySugar.Errorw("解析后端URL失败", "reqID", reqID, "error", err)
			continue
		}

		// 根据协议确定端点路径
		var apiPath string
		if protocol == "anthropic" {
			apiPath = "/v1/messages"
		} else {
			apiPath = r.URL.Path // OpenAI 使用原始路径
		}

		backendPath := targetURL.Path
		if backendPath != "" && strings.HasPrefix(apiPath, backendPath) {
			targetURL.Path = apiPath
		} else {
			targetURL.Path = backendPath + apiPath
		}
		targetURL.RawQuery = r.URL.RawQuery

		logBuilder.WriteString(fmt.Sprintf("目标URL: %s\n", targetURL.String()))

		proxyReq, err := http.NewRequest(r.Method, targetURL.String(), bytes.NewReader(newBody))
		if err != nil {
			lastErr = err
			logBuilder.WriteString(fmt.Sprintf("创建代理请求失败: %v\n", err))
			logging.ProxySugar.Errorw("创建代理请求失败", "reqID", reqID, "error", err)
			continue
		}

		// 记录即将发送的请求详情
		logging.ProxySugar.Infow("发送请求到后端",
			"reqID", reqID,
			"method", proxyReq.Method,
			"url", targetURL.String(),
			"body_size", len(newBody),
			"protocol", protocol,
			"backend", route.BackendName)
		for k, v := range r.Header {
			proxyReq.Header[k] = v
		}
		proxyReq.Header.Set("Content-Length", fmt.Sprintf("%d", len(newBody)))

		// 根据协议设置特定的请求头
		if protocol == "anthropic" {
			// Anthropic 需要特定的头部
			proxyReq.Header.Set("anthropic-version", "2023-06-01")
			proxyReq.Header.Set("Content-Type", "application/json")
			// 移除 OpenAI 特定的头部
			proxyReq.Header.Del("OpenAI-Organization")
			proxyReq.Header.Del("OpenAI-Project")
		}

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

		// 设置 API Key（Anthropic 使用 x-api-key 头部）
		if bkend.APIKey != "" {
			if protocol == "anthropic" {
				proxyReq.Header.Set("x-api-key", bkend.APIKey)
				proxyReq.Header.Del("Authorization") // 移除 OpenAI 的 Authorization 头
			} else {
				proxyReq.Header.Set("Authorization", "Bearer "+bkend.APIKey)
			}
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
				p.streamResponse(w, resp.Body, route.BackendName, protocol)
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
					// 协议直通场景，不需要转换响应
					if isPassthrough {
						logging.ProxySugar.Infow("协议直通响应",
							"reqID", reqID,
							"protocol", protocol,
							"backend", route.BackendName,
							"response_size", len(bodyBytes),
							"note", "客户端与后端协议相同，直接返回")
					} else {
						// 需要协议转换的场景
						if protocol == "anthropic" && clientProtocol == ProtocolOpenAI {
							// 后端 Anthropic → 客户端 OpenAI
							logging.ProxySugar.Infow("转换后端响应格式",
								"reqID", reqID,
								"from", "anthropic",
								"to", "openai",
								"backend", route.BackendName,
								"response_size", len(bodyBytes))
							convertedBytes, convErr := p.converter.ConvertFromAnthropic(bodyBytes)
							if convErr != nil {
								logging.ProxySugar.Errorw("后端响应转换失败",
									"reqID", reqID,
									"protocol", "anthropic",
									"backend", route.BackendName,
									"error", convErr)
							} else {
								bodyBytes = convertedBytes
								logging.ProxySugar.Infow("后端响应转换成功",
									"reqID", reqID,
									"from", "anthropic",
									"to", "openai",
									"backend", route.BackendName,
									"size", len(bodyBytes))
							}
						} else if protocol == "openai" && clientProtocol == ProtocolAnthropic {
							// 后端 OpenAI → 客户端 Anthropic
							logging.ProxySugar.Infow("转换响应为客户端协议",
								"reqID", reqID,
								"from", "openai",
								"to", "anthropic",
								"client_protocol", clientProtocol)
							convertedBytes, convErr := p.requestDetector.ConvertOpenAIToAnthropicResponse(bodyBytes)
							if convErr != nil {
								logging.ProxySugar.Errorw("客户端协议转换失败",
									"reqID", reqID,
									"error", convErr)
							} else {
								bodyBytes = convertedBytes
								logging.ProxySugar.Infow("客户端协议转换成功",
									"reqID", reqID,
									"size", len(bodyBytes))
							}
						}
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

func (p *Proxy) streamResponse(w http.ResponseWriter, body io.ReadCloser, backendName string, protocol string) {
	logging.ProxySugar.Infow("开始流式响应处理", "backend", backendName, "protocol", protocol)

	flusher, ok := w.(http.Flusher)
	if !ok {
		logging.ProxySugar.Warnw("不支持流式响应", "backend", backendName, "protocol", protocol)
		io.Copy(w, body)
		return
	}

	// For Anthropic protocol, we need line-by-line parsing
	if protocol == "anthropic" {
		logging.ProxySugar.Infow("使用 Anthropic 流式转换",
			"backend", backendName,
			"protocol", protocol)
		p.streamAnthropicResponse(w, body, backendName, flusher)
		return
	}

	// For OpenAI protocol, use raw byte streaming
	logging.ProxySugar.Infow("使用 OpenAI 原始流式传输",
		"backend", backendName,
		"protocol", protocol)
	buf := make([]byte, 32*1024)
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
