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
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
		return
	}

	cfg := p.configMgr.Get()

	if cfg.ProxyAPIKey != "" {
		auth := r.Header.Get("Authorization")
		expected := "Bearer " + cfg.ProxyAPIKey
		if auth != expected {
			LogGeneral("WARN", "API Key 验证失败，客户端: %s", r.RemoteAddr)
			http.Error(w, "无效的 API Key", http.StatusUnauthorized)
			return
		}
	}

	if r.URL.Path == "/v1/models" || r.URL.Path == "/models" {
		p.handleModels(w, r)
		return
	}

	reqID := time.Now().Format("2006-01-02_15-04-05") + "_" + uuid.New().String()[:8]

	body, err := io.ReadAll(r.Body)
	if err != nil {
		LogGeneral("ERROR", "[%s] 读取请求体失败: %v", reqID, err)
		http.Error(w, "读取请求体失败", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	var reqBody map[string]interface{}
	json.Unmarshal(body, &reqBody)

	modelAlias, _ := reqBody["model"].(string)
	if modelAlias == "" {
		LogGeneral("WARN", "[%s] 请求缺少 model 字段", reqID)
		http.Error(w, "缺少 model 字段", http.StatusBadRequest)
		return
	}

	LogGeneral("INFO", "[%s] 收到请求: 模型=%s 客户端=%s", reqID, modelAlias, r.RemoteAddr)

	routes, _ := p.router.Resolve(modelAlias)
	if len(routes) == 0 {
		LogGeneral("WARN", "[%s] 未知的模型别名: %s", reqID, modelAlias)
		http.Error(w, fmt.Sprintf("未知的模型别名: %s", modelAlias), http.StatusBadRequest)
		return
	}

	LogGeneral("DEBUG", "[%s] 解析到 %d 个可用路由", reqID, len(routes))

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
		LogGeneral("INFO", "[%s] 尝试后端 %s (模型: %s)", reqID, route.BackendName, route.Model)

		modifiedBody := make(map[string]interface{})
		for k, v := range reqBody {
			modifiedBody[k] = v
		}
		modifiedBody["model"] = route.Model

		newBody, _ := json.Marshal(modifiedBody)

		targetURL, err := url.Parse(route.BackendURL)
		if err != nil {
			lastErr = err
			logBuilder.WriteString(fmt.Sprintf("解析后端URL失败: %v\n", err))
			LogGeneral("ERROR", "[%s] 解析后端URL失败: %v", reqID, err)
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

		proxyReq, _ := http.NewRequest(r.Method, targetURL.String(), bytes.NewReader(newBody))
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
			logBuilder.WriteString(fmt.Sprintf("请求失败: %v\n", err))
			LogGeneral("WARN", "[%s] 后端 %s 请求失败: %v", reqID, route.BackendName, err)

			// 记录连接错误到独立错误日志
			errorContent := fmt.Sprintf("================== 错误日志 ==================\n请求ID: %s\n时间: %s\n后端: %s\n模型: %s\n错误: %v\n",
				reqID, time.Now().Format(time.RFC3339), route.BackendName, route.Model, err)
			WriteErrorLog(cfg, fmt.Sprintf("%s_%s", reqID, route.BackendName), errorContent)

			key := p.cooldown.Key(route.BackendName, route.Model)
			p.cooldown.SetCooldown(key, time.Duration(cfg.Fallback.CooldownSeconds)*time.Second)
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			logBuilder.WriteString(fmt.Sprintf("状态: %d 成功\n", resp.StatusCode))
			LogGeneral("INFO", "[%s] 请求成功: 后端=%s 状态=%d 耗时=%dms", reqID, route.BackendName, resp.StatusCode, backendDuration.Milliseconds())
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
			}
			w.WriteHeader(resp.StatusCode)

			if isStream {
				p.streamResponse(w, resp.Body)
			} else {
				io.Copy(w, resp.Body)
			}
			resp.Body.Close()
			return
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		lastStatus = resp.StatusCode
		lastBody = string(respBody)

		logBuilder.WriteString(fmt.Sprintf("状态: %d\n响应: %s\n", resp.StatusCode, lastBody))
		LogGeneral("WARN", "[%s] 后端 %s 返回错误: 状态=%d", reqID, route.BackendName, resp.StatusCode)

		// 记录每个后端错误到独立错误日志
		errorContent := fmt.Sprintf("================== 错误日志 ==================\n请求ID: %s\n时间: %s\n后端: %s\n模型: %s\n状态码: %d\n\n--- 响应内容 ---\n%s\n",
			reqID, time.Now().Format(time.RFC3339), route.BackendName, route.Model, resp.StatusCode, lastBody)
		WriteErrorLog(cfg, fmt.Sprintf("%s_%s", reqID, route.BackendName), errorContent)

		if p.detector.ShouldFallback(resp.StatusCode, lastBody) {
			key := p.cooldown.Key(route.BackendName, route.Model)
			p.cooldown.SetCooldown(key, time.Duration(cfg.Fallback.CooldownSeconds)*time.Second)
			logBuilder.WriteString(fmt.Sprintf("操作: 冷却 %s，尝试下一个后端\n", key))
			LogGeneral("INFO", "[%s] 触发回退: %s 进入冷却", reqID, key)
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
	LogGeneral("ERROR", "[%s] 所有后端均失败", reqID)
	WriteRequestLog(cfg, reqID, logBuilder.String())
	WriteErrorLog(cfg, reqID, logBuilder.String())

	metrics.Finish(false, "")

	if lastErr != nil {
		http.Error(w, fmt.Sprintf("所有后端均失败: %v", lastErr), http.StatusBadGateway)
		return
	}
	w.WriteHeader(lastStatus)
	w.Write([]byte(lastBody))
}

func (p *Proxy) streamResponse(w http.ResponseWriter, body io.ReadCloser) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		io.Copy(w, body)
		return
	}

	rc := http.NewResponseController(w)
	rc.SetWriteDeadline(time.Time{})

	buf := make([]byte, 1024)
	for {
		n, err := body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			flusher.Flush()
		}
		if err != nil {
			break
		}
	}
}

func (p *Proxy) handleModels(w http.ResponseWriter, r *http.Request) {
	cfg := p.configMgr.Get()
	LogGeneral("DEBUG", "收到模型列表请求: 客户端=%s", r.RemoteAddr)

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

	LogGeneral("DEBUG", "返回 %d 个可用模型", len(models))
	resp := Response{Object: "list", Data: models}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
