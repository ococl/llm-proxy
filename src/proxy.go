package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Proxy struct {
	configMgr *ConfigManager
	router    *Router
	cooldown  *CooldownManager
	detector  *Detector
}

func NewProxy(cfg *ConfigManager, router *Router, cd *CooldownManager, det *Detector) *Proxy {
	return &Proxy{configMgr: cfg, router: router, cooldown: cd, detector: det}
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
			NetworkSugar.Warnw("API Key验证失败", "client", r.RemoteAddr)
			WriteJSONError(w, ErrUnauthorized, http.StatusUnauthorized, "")
			return
		}
	}

	if r.URL.Path == "/v1/models" || r.URL.Path == "/models" {
		p.handleModels(w, r)
		return
	}

	reqID := "req_" + strings.ReplaceAll(uuid.New().String()[:18], "-", "")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		NetworkSugar.Errorw("读取请求体失败", "reqID", reqID, "error", err)
		WriteJSONError(w, ErrBadRequest, http.StatusBadRequest, reqID)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	var reqBody map[string]interface{}
	if err := json.Unmarshal(body, &reqBody); err != nil {
		NetworkSugar.Warnw("解析请求体失败", "reqID", reqID, "error", err)
		WriteJSONError(w, ErrInvalidJSON, http.StatusBadRequest, reqID)
		return
	}

	// Inject system prompt if file exists
	reqBody = ProcessSystemPrompt(reqBody)

	modelAlias, _ := reqBody["model"].(string)
	if modelAlias == "" {
		NetworkSugar.Warnw("请求缺少model字段", "reqID", reqID)
		WriteJSONError(w, ErrMissingModel, http.StatusBadRequest, reqID)
		return
	}

	ProxySugar.Infow("收到请求", "reqID", reqID, "model", modelAlias, "client", r.RemoteAddr)

	routes, err := p.router.Resolve(modelAlias)
	if err != nil {
		ProxySugar.Warnw("解析模型别名失败", "reqID", reqID, "error", err)
		WriteJSONErrorWithMsg(w, ErrBadRequest, http.StatusBadRequest, reqID, fmt.Sprintf("解析模型别名失败: %v", err))
		return
	}
	if len(routes) == 0 {
		ProxySugar.Warnw("未知的模型别名", "reqID", reqID, "model", modelAlias)
		WriteJSONErrorWithMsg(w, ErrUnknownModel, http.StatusBadRequest, reqID, fmt.Sprintf("未知的模型别名: %s", modelAlias))
		return
	}

	ProxySugar.Debugw("解析到可用路由", "reqID", reqID, "count", len(routes))

	isStream := false
	if s, ok := reqBody["stream"].(bool); ok {
		isStream = s
	}

	var logBuilder strings.Builder
	logBuilder.WriteString(fmt.Sprintf("================== 请求日志 ==================\n"))
	logBuilder.WriteString(fmt.Sprintf("请求ID: %s\n时间: %s\n客户端: %s\n\n", reqID, time.Now().Format(time.RFC3339), r.RemoteAddr))
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

	metrics := NewRequestMetrics(reqID, modelAlias)
	var finalBackend string

	for i, route := range routes {
		if i >= maxRetries {
			break
		}

		logBuilder.WriteString(fmt.Sprintf("\n--- 尝试 %d ---\n", i+1))
		logBuilder.WriteString(fmt.Sprintf("后端: %s\n模型: %s\n", route.BackendName, route.Model))
		ProxySugar.Infow("尝试后端", "reqID", reqID, "backend", route.BackendName, "model", route.Model)

		modifiedBody := make(map[string]interface{})
		for k, v := range reqBody {
			modifiedBody[k] = v
		}
		modifiedBody["model"] = route.Model

		newBody, err := json.Marshal(modifiedBody)
		if err != nil {
			lastErr = err
			logBuilder.WriteString(fmt.Sprintf("序列化请求体失败: %v\n", err))
			ProxySugar.Errorw("序列化请求体失败", "reqID", reqID, "error", err)
			continue
		}

		targetURL, err := url.Parse(route.BackendURL)
		if err != nil {
			lastErr = err
			logBuilder.WriteString(fmt.Sprintf("解析后端URL失败: %v\n", err))
			ProxySugar.Errorw("解析后端URL失败", "reqID", reqID, "error", err)
			continue
		}

		backendPath := targetURL.Path
		reqPath := r.URL.Path
		if backendPath != "" && strings.HasPrefix(reqPath, backendPath) {
			targetURL.Path = reqPath
		} else {
			targetURL.Path = backendPath + reqPath
		}
		targetURL.RawQuery = r.URL.RawQuery

		logBuilder.WriteString(fmt.Sprintf("目标URL: %s\n", targetURL.String()))

		proxyReq, err := http.NewRequest(r.Method, targetURL.String(), bytes.NewReader(newBody))
		if err != nil {
			lastErr = err
			logBuilder.WriteString(fmt.Sprintf("创建代理请求失败: %v\n", err))
			ProxySugar.Errorw("创建代理请求失败", "reqID", reqID, "error", err)
			continue
		}
		for k, v := range r.Header {
			proxyReq.Header[k] = v
		}
		proxyReq.Header.Set("Content-Length", fmt.Sprintf("%d", len(newBody)))

		backend := p.configMgr.GetBackend(route.BackendName)
		if backend != nil && backend.APIKey != "" {
			proxyReq.Header.Set("Authorization", "Bearer "+backend.APIKey)
		}

		client := &http.Client{Timeout: 5 * time.Minute}
		backendStart := time.Now()
		resp, err := client.Do(proxyReq)
		backendDuration := time.Since(backendStart)
		metrics.RecordBackendTime(route.BackendName, backendDuration)

		if err != nil {
			lastErr = err
			logBuilder.WriteString(fmt.Sprintf("\n--- 响应详情 ---\n"))
			logBuilder.WriteString(fmt.Sprintf("错误: %v\n", err))
			logBuilder.WriteString(fmt.Sprintf("耗时: %dms\n", backendDuration.Milliseconds()))
			NetworkSugar.Warnw("后端请求失败", "reqID", reqID, "backend", route.BackendName, "error", err, "duration_ms", backendDuration.Milliseconds())

			errorContent := fmt.Sprintf("================== 错误日志 ==================\n请求ID: %s\n时间: %s\n后端: %s\n模型: %s\n错误: %v\n耗时: %dms\n",
				reqID, time.Now().Format(time.RFC3339), route.BackendName, route.Model, err, backendDuration.Milliseconds())
			WriteErrorLog(cfg, fmt.Sprintf("%s_%s", reqID, route.BackendName), errorContent)

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
			ProxySugar.Infow("请求成功", "reqID", reqID, "backend", route.BackendName, "status", resp.StatusCode, "duration_ms", backendDuration.Milliseconds(), "attempts", metrics.Attempts)
			WriteRequestLog(cfg, reqID, logBuilder.String())

			finalBackend = route.BackendName
			metrics.Finish(true, finalBackend)

			for k, v := range resp.Header {
				w.Header()[k] = v
			}

			if isStream {
				w.Header().Set("Cache-Control", "no-cache")
				w.Header().Set("Connection", "keep-alive")
				w.Header().Set("X-Accel-Buffering", "no")
				// 添加额外的SSE头部以确保兼容性
				w.Header().Set("Transfer-Encoding", "chunked")
			}
			w.WriteHeader(resp.StatusCode)

			if isStream {
				ProxySugar.Infow("开始流式传输", "reqID", reqID, "backend", route.BackendName, "model", route.Model)
				// 记录后端响应的头部信息以进行调试
				ProxySugar.Debugw("后端响应头部", "reqID", reqID, "backend", route.BackendName, "headers", resp.Header)
				p.streamResponse(w, resp.Body, route.BackendName)
				ProxySugar.Infow("完成流式传输", "reqID", reqID, "backend", route.BackendName, "model", route.Model)
			} else {
				// 对于非流式响应，记录响应长度
				bodyBytes, err := io.ReadAll(resp.Body)
				if err != nil {
					ProxySugar.Errorw("读取非流式响应失败", "reqID", reqID, "error", err)
				} else {
					ProxySugar.Debugw("非流式响应", "reqID", reqID, "backend", route.BackendName, "response_size", len(bodyBytes))
					_, writeErr := w.Write(bodyBytes)
					if writeErr != nil {
						ProxySugar.Errorw("写入非流式响应失败", "reqID", reqID, "error", writeErr)
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
			NetworkSugar.Warnw("读取响应体失败", "reqID", reqID, "error", err)
		}
		lastStatus = resp.StatusCode
		lastBody = string(respBody)

		logBuilder.WriteString(fmt.Sprintf("状态: %d\n响应: %s\n", resp.StatusCode, lastBody))
		NetworkSugar.Warnw("后端返回错误", "reqID", reqID, "backend", route.BackendName, "status", resp.StatusCode, "duration_ms", backendDuration.Milliseconds())

		// 记录每个后端错误到独立错误日志
		errorContent := fmt.Sprintf("================== 错误日志 ==================\n请求ID: %s\n时间: %s\n后端: %s\n模型: %s\n状态码: %d\n\n--- 响应内容 ---\n%s\n",
			reqID, time.Now().Format(time.RFC3339), route.BackendName, route.Model, resp.StatusCode, lastBody)
		WriteErrorLog(cfg, fmt.Sprintf("%s_%s", reqID, route.BackendName), errorContent)

		if p.detector.ShouldFallback(resp.StatusCode, lastBody) {
			key := p.cooldown.Key(route.BackendName, route.Model)
			p.cooldown.SetCooldown(key, time.Duration(cfg.Fallback.CooldownSeconds)*time.Second)
			logBuilder.WriteString(fmt.Sprintf("操作: 冷却 %s，尝试下一个后端\n", key))
			ProxySugar.Infow("触发回退", "reqID", reqID, "backend", key, "action", "进入冷却")
			continue
		}

		WriteRequestLog(cfg, reqID, logBuilder.String())
		finalBackend = route.BackendName
		metrics.Finish(false, finalBackend)
		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)
		return
	}

	logBuilder.WriteString("\n--- 最终结果 ---\n所有后端均失败\n")
	NetworkSugar.Errorw("所有后端均失败", "reqID", reqID)
	WriteRequestLog(cfg, reqID, logBuilder.String())
	WriteErrorLog(cfg, reqID, logBuilder.String())

	var backendDetails []string
	for backend, duration := range metrics.BackendTimes {
		backendDetails = append(backendDetails, fmt.Sprintf("%s=%dms", backend, duration.Milliseconds()))
	}
	NetworkSugar.Errorw("所有后端均失败详情", "reqID", reqID, "model", modelAlias, "attempts", metrics.Attempts, "backend_details", strings.Join(backendDetails, ", "))

	metrics.Finish(false, "")

	if lastErr != nil {
		WriteJSONErrorWithMsg(w, ErrNoBackend, http.StatusBadGateway, reqID, fmt.Sprintf("所有后端均失败: %v", lastErr))
		return
	}
	w.WriteHeader(lastStatus)
	w.Write([]byte(lastBody))
}

func (p *Proxy) streamResponse(w http.ResponseWriter, body io.ReadCloser, backendName string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		io.Copy(w, body)
		return
	}

	cfg := p.configMgr.Get()
	needsSpecialHandling := detectSpecialHandlingNeeded(backendName, cfg)

	// 使用较小的缓冲区以减少延迟并提高实时性
	buf := make([]byte, 4*1024) // 4KB buffer instead of 32KB
	bytesProcessed := 0
	chunksReceived := 0

	for {
		// 添加调试日志记录每次读取的数据
		n, err := body.Read(buf)
		chunksReceived++
		if n > 0 {
			bytesProcessed += n
			ProxySugar.Debugw("接收SSE数据块", "chunk_number", chunksReceived, "size", n, "total_bytes", bytesProcessed, "backend", backendName, "needs_special_handling", needsSpecialHandling)

			// 应用供应商特定的处理（用于解决某些供应商的重复发言问题）
			var processedChunk []byte
			if needsSpecialHandling {
				processedChunk = p.processChunkWithVendorSpecificHandling(buf[:n], backendName)
			} else {
				processedChunk = buf[:n]
			}

			// 直接写入响应，避免额外的缓冲
			if _, writeErr := w.Write(processedChunk); writeErr != nil {
				ProxySugar.Errorw("写入响应失败", "error", writeErr, "chunk_number", chunksReceived)
				break
			}
			flusher.Flush()
			ProxySugar.Debugw("刷新响应", "chunk_number", chunksReceived)
		}
		if err != nil {
			if err == io.EOF {
				ProxySugar.Debugw("SSE流结束", "total_bytes", bytesProcessed, "total_chunks", chunksReceived, "backend", backendName)
				break
			} else {
				ProxySugar.Errorw("读取SSE流错误", "error", err, "chunk_number", chunksReceived, "backend", backendName)
				break
			}
		}
	}
	ProxySugar.Infow("SSE流传输完成", "total_bytes", bytesProcessed, "total_chunks", chunksReceived, "backend", backendName)
}

// detectSpecialHandlingNeeded 检测是否需要特殊处理的供应商
func detectSpecialHandlingNeeded(backendName string, config *Config) bool {
	// 某些供应商可能需要特殊的处理方式
	// 这里可以根据后端名称来判断是否需要特殊处理
	// 可以根据用户配置动态调整

	// 检查配置中是否有特殊处理设置
	if config.Logging.DebugMode {
		ProxySugar.Debugw("调试模式：检测特殊处理需求", "backend", backendName)
	}

	// 从配置中获取特殊处理的后端列表
	specialBackends := make(map[string]bool)

	// 从配置中读取有问题的后端列表
	for _, backend := range config.Logging.ProblematicBackends {
		specialBackends[backend] = true
	}

	// 默认的一些可能有问题的供应商
	defaultProblematic := []string{"special-vendor", "problematic-provider"}
	for _, backend := range defaultProblematic {
		specialBackends[backend] = true
	}

	result := specialBackends[backendName]
	if result && config.Logging.DebugMode {
		ProxySugar.Debugw("检测到需要特殊处理的后端", "backend", backendName)
	}

	return result
}

// processChunkWithVendorSpecificHandling 根据供应商特性处理响应块
func (p *Proxy) processChunkWithVendorSpecificHandling(chunk []byte, backendName string) []byte {
	cfg := p.configMgr.Get()
	if !detectSpecialHandlingNeeded(backendName, cfg) {
		return chunk
	}

	// 这里可以添加针对特定供应商的特殊处理逻辑
	// 比如：过滤重复内容、调整数据格式等
	ProxySugar.Debugw("应用特殊处理", "backend", backendName)

	// 针对某些供应商的重复发言问题，我们可以实施以下策略：
	// 1. 检查数据块是否包含重复的SSE格式数据
	// 2. 确保数据正确分块传输
	// 3. 处理可能的格式问题

	// 示例：实现基本的重复内容检测和过滤（如果需要）
	// 这里只是占位符，实际实现可以根据具体需求定制
	return chunk
}

// processChunkForRepeatDetection 检测并处理重复内容的高级函数
func (p *Proxy) processChunkForRepeatDetection(chunk []byte, backendName string, sessionID string) []byte {
	cfg := p.configMgr.Get()
	if !detectSpecialHandlingNeeded(backendName, cfg) {
		return chunk
	}

	// 这是一个高级功能，用于检测和处理重复内容
	// 可以通过会话ID跟踪对话历史，避免重复发言
	ProxySugar.Debugw("执行重复内容检测", "backend", backendName, "session", sessionID)

	return chunk
}

func (p *Proxy) handleModels(w http.ResponseWriter, r *http.Request) {
	cfg := p.configMgr.Get()
	ProxySugar.Debugw("收到模型列表请求", "client", r.RemoteAddr)

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

	// Sort models by ID (name) ascending
	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})

	ProxySugar.Debugw("返回可用模型", "count", len(models))
	resp := Response{Object: "list", Data: models}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
