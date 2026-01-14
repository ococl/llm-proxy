package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
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
	client    *http.Client
}

func NewProxy(cfg *ConfigManager, router *Router, cd *CooldownManager, det *Detector) *Proxy {
	return &Proxy{
		configMgr: cfg,
		router:    router,
		cooldown:  cd,
		detector:  det,
		client: &http.Client{
			Timeout: 10 * time.Minute,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
			},
		},
	}
}

func (p *Proxy) createHTTPClient(cfg *Config) *http.Client {
	timeout := cfg.Timeout.GetTotalTimeout()
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			DialContext: (&net.Dialer{
				Timeout:   cfg.Timeout.GetConnectTimeout(),
				KeepAlive: 30 * time.Second,
			}).DialContext,
		},
	}
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
			LogGeneral("WARN", "API Key 验证失败，客户端: %s", r.RemoteAddr)
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
		LogGeneral("ERROR", "[%s] 读取请求体失败: %v", reqID, err)
		WriteJSONError(w, ErrBadRequest, http.StatusBadRequest, reqID)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	var reqBody map[string]interface{}
	if err := json.Unmarshal(body, &reqBody); err != nil {
		LogGeneral("WARN", "[%s] 解析请求体失败: %v", reqID, err)
		WriteJSONError(w, ErrInvalidJSON, http.StatusBadRequest, reqID)
		return
	}

	// Inject system prompt if file exists
	reqBody = ProcessSystemPrompt(reqBody)

	modelAlias, _ := reqBody["model"].(string)
	if modelAlias == "" {
		LogGeneral("WARN", "[%s] 请求缺少 model 字段", reqID)
		WriteJSONError(w, ErrMissingModel, http.StatusBadRequest, reqID)
		return
	}

	LogGeneral("INFO", "[%s] 收到请求: 模型=%s 客户端=%s", reqID, modelAlias, r.RemoteAddr)

	routes, err := p.router.Resolve(modelAlias)
	if err != nil {
		LogGeneral("WARN", "[%s] 解析模型别名失败: %v", reqID, err)
		WriteJSONErrorWithMsg(w, ErrBadRequest, http.StatusBadRequest, reqID, fmt.Sprintf("解析模型别名失败: %v", err))
		return
	}
	if len(routes) == 0 {
		LogGeneral("WARN", "[%s] 未知的模型别名: %s", reqID, modelAlias)
		WriteJSONErrorWithMsg(w, ErrUnknownModel, http.StatusBadRequest, reqID, fmt.Sprintf("未知的模型别名: %s", modelAlias))
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

		newBody, err := json.Marshal(modifiedBody)
		if err != nil {
			lastErr = err
			logBuilder.WriteString(fmt.Sprintf("序列化请求体失败: %v\n", err))
			LogGeneral("ERROR", "[%s] 序列化请求体失败: %v", reqID, err)
			continue
		}

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

		proxyReq, err := http.NewRequest(r.Method, targetURL.String(), bytes.NewReader(newBody))
		if err != nil {
			lastErr = err
			logBuilder.WriteString(fmt.Sprintf("创建代理请求失败: %v\n", err))
			LogGeneral("ERROR", "[%s] 创建代理请求失败: %v", reqID, err)
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

		backendStart := time.Now()
		resp, err := p.client.Do(proxyReq)
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
				if !p.streamResponse(r.Context(), w, resp.Body) {
					resp.Body.Close()
				}
			} else {
				if _, err := io.Copy(w, resp.Body); err != nil {
					LogGeneral("DEBUG", "[%s] 写入响应失败: %v", reqID, err)
				}
				resp.Body.Close()
			}
			return
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			logBuilder.WriteString(fmt.Sprintf("读取响应体失败: %v\n", err))
			LogGeneral("WARN", "[%s] 读取响应体失败: %v", reqID, err)
		}
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
		WriteJSONErrorWithMsg(w, ErrNoBackend, http.StatusBadGateway, reqID, fmt.Sprintf("所有后端均失败: %v", lastErr))
		return
	}
	w.WriteHeader(lastStatus)
	w.Write([]byte(lastBody))
}

func (p *Proxy) streamResponse(ctx context.Context, w http.ResponseWriter, body io.ReadCloser) (closed bool) {
	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case <-ctx.Done():
			// 使用带超时的关闭，防止 body.Close() 阻塞导致 goroutine 泄漏
			closeDone := make(chan struct{})
			go func() {
				body.Close()
				close(closeDone)
			}()
			select {
			case <-closeDone:
			case <-time.After(5 * time.Second):
				// 超时后放弃等待，避免 goroutine 永久阻塞
			}
		case <-done:
		}
	}()

	flusher, ok := w.(http.Flusher)
	if !ok {
		io.Copy(w, body)
		return ctx.Err() != nil
	}

	rc := http.NewResponseController(w)
	rc.SetWriteDeadline(time.Time{})

	buf := make([]byte, 32*1024)
	for {
		n, readErr := body.Read(buf)
		if n > 0 {
			_, writeErr := w.Write(buf[:n])
			if writeErr != nil {
				LogGeneral("DEBUG", "写入客户端失败: %v", writeErr)
				return ctx.Err() != nil
			}
			flusher.Flush()
		}
		if readErr != nil {
			if readErr != io.EOF && ctx.Err() == nil {
				LogGeneral("DEBUG", "读取后端响应结束: %v", readErr)
			}
			return ctx.Err() != nil
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
