package proxy

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

	"llm-proxy/backend"
	"llm-proxy/config"
	"llm-proxy/errors"
	"llm-proxy/logging"
	"llm-proxy/middleware"
	"llm-proxy/prompt"

	"github.com/google/uuid"
)

type Proxy struct {
	configMgr *config.Manager
	router    *Router
	cooldown  *backend.CooldownManager
	detector  *Detector
}

func NewProxy(cfg *config.Manager, router *Router, cd *backend.CooldownManager, det *Detector) *Proxy {
	return &Proxy{configMgr: cfg, router: router, cooldown: cd, detector: det}
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

	body, err := io.ReadAll(r.Body)
	if err != nil {
		logging.NetworkSugar.Errorw("读取请求体失败", "reqID", reqID, "error", err)
		errors.WriteJSONError(w, errors.ErrBadRequest, http.StatusBadRequest, reqID)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	var reqBody map[string]interface{}
	if err := json.Unmarshal(body, &reqBody); err != nil {
		logging.NetworkSugar.Warnw("解析请求体失败", "reqID", reqID, "error", err)
		errors.WriteJSONError(w, errors.ErrInvalidJSON, http.StatusBadRequest, reqID)
		return
	}

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
	logging.ProxySugar.Infow("收到请求", "reqID", reqID, "model", modelAlias, "client", clientIP)

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

	isStream := false
	if s, ok := reqBody["stream"].(bool); ok {
		isStream = s
	}

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
		logging.ProxySugar.Debugw("尝试后端", "reqID", reqID, "backend", route.BackendName, "model", route.Model)

		modifiedBody := make(map[string]interface{})
		for k, v := range reqBody {
			modifiedBody[k] = v
		}
		modifiedBody["model"] = route.Model

		newBody, err := json.Marshal(modifiedBody)
		if err != nil {
			lastErr = err
			logBuilder.WriteString(fmt.Sprintf("序列化请求体失败: %v\n", err))
			logging.ProxySugar.Errorw("序列化请求体失败", "reqID", reqID, "error", err)
			continue
		}

		targetURL, err := url.Parse(route.BackendURL)
		if err != nil {
			lastErr = err
			logBuilder.WriteString(fmt.Sprintf("解析后端URL失败: %v\n", err))
			logging.ProxySugar.Errorw("解析后端URL失败", "reqID", reqID, "error", err)
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
			logging.ProxySugar.Errorw("创建代理请求失败", "reqID", reqID, "error", err)
			continue
		}
		for k, v := range r.Header {
			proxyReq.Header[k] = v
		}
		proxyReq.Header.Set("Content-Length", fmt.Sprintf("%d", len(newBody)))

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

		bkend := p.configMgr.GetBackend(route.BackendName)
		if bkend != nil && bkend.APIKey != "" {
			proxyReq.Header.Set("Authorization", "Bearer "+bkend.APIKey)
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
			logging.ProxySugar.Infow("请求成功", "reqID", reqID, "backend", route.BackendName, "status", resp.StatusCode, "duration_ms", backendDuration.Milliseconds(), "attempts", attempts)
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
				logging.ProxySugar.Debugw("开始流式传输", "reqID", reqID, "backend", route.BackendName, "model", route.Model)
				logging.FileOnlySugar.Debugw("后端响应头部", "reqID", reqID, "backend", route.BackendName, "headers", resp.Header)
				p.streamResponse(w, resp.Body, route.BackendName)
				logging.ProxySugar.Debugw("完成流式传输", "reqID", reqID, "backend", route.BackendName, "model", route.Model)
			} else {
				bodyBytes, err := io.ReadAll(resp.Body)
				if err != nil {
					logging.ProxySugar.Errorw("读取非流式响应失败", "reqID", reqID, "error", err)
				} else {
					logging.ProxySugar.Debugw("非流式响应", "reqID", reqID, "backend", route.BackendName, "response_size", len(bodyBytes))
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

func (p *Proxy) streamResponse(w http.ResponseWriter, body io.ReadCloser, backendName string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		io.Copy(w, body)
		return
	}

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
