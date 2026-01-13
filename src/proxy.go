package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	if r.URL.Path == "/v1/models" || r.URL.Path == "/models" {
		p.handleModels(w, r)
		return
	}

	cfg := p.configMgr.Get()

	if cfg.ProxyAPIKey != "" {
		auth := r.Header.Get("Authorization")
		expected := "Bearer " + cfg.ProxyAPIKey
		if auth != expected {
			http.Error(w, "Invalid API key", http.StatusUnauthorized)
			return
		}
	}

	reqID := time.Now().Format("2006-01-02_15-04-05") + "_" + uuid.New().String()[:8]

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	var reqBody map[string]interface{}
	json.Unmarshal(body, &reqBody)

	modelAlias, _ := reqBody["model"].(string)
	if modelAlias == "" {
		http.Error(w, "Missing model field", http.StatusBadRequest)
		return
	}

	routes, _ := p.router.Resolve(modelAlias)
	if len(routes) == 0 {
		http.Error(w, fmt.Sprintf("Unknown model alias: %s", modelAlias), http.StatusBadRequest)
		return
	}

	isStream := false
	if s, ok := reqBody["stream"].(bool); ok {
		isStream = s
	}

	var logBuilder strings.Builder
	logBuilder.WriteString(fmt.Sprintf("================== REQUEST ==================\n"))
	logBuilder.WriteString(fmt.Sprintf("ID: %s\nTime: %s\nClient: %s\n\n", reqID, time.Now().Format(time.RFC3339), r.RemoteAddr))
	logBuilder.WriteString("--- Headers ---\n")
	for k, v := range r.Header {
		logBuilder.WriteString(fmt.Sprintf("%s: %s\n", k, strings.Join(v, ", ")))
	}
	logBuilder.WriteString("\n--- Body ---\n")
	logBuilder.WriteString(string(body))
	logBuilder.WriteString("\n")

	var lastErr error
	var lastStatus int
	var lastBody string

	for i, route := range routes {
		if i >= cfg.Fallback.MaxRetries {
			break
		}

		logBuilder.WriteString(fmt.Sprintf("\n--- Attempt %d ---\n", i+1))
		logBuilder.WriteString(fmt.Sprintf("Backend: %s\nModel: %s\n", route.BackendName, route.Model))

		modifiedBody := make(map[string]interface{})
		for k, v := range reqBody {
			modifiedBody[k] = v
		}
		modifiedBody["model"] = route.Model

		newBody, _ := json.Marshal(modifiedBody)

		targetURL, err := url.Parse(route.BackendURL)
		if err != nil {
			lastErr = err
			logBuilder.WriteString(fmt.Sprintf("Error parsing backend URL: %v\n", err))
			continue
		}

		// Smart path join: avoid duplicate prefix (e.g., /v1/v1)
		backendPath := targetURL.Path
		reqPath := r.URL.Path
		if backendPath != "" && strings.HasPrefix(reqPath, backendPath) {
			targetURL.Path = reqPath
		} else {
			targetURL.Path = backendPath + reqPath
		}
		targetURL.RawQuery = r.URL.RawQuery

		logBuilder.WriteString(fmt.Sprintf("URL: %s\n", targetURL.String()))

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
		resp, err := client.Do(proxyReq)
		if err != nil {
			lastErr = err
			logBuilder.WriteString(fmt.Sprintf("Error: %v\n", err))
			key := p.cooldown.Key(route.BackendName, route.Model)
			p.cooldown.SetCooldown(key, time.Duration(cfg.Fallback.CooldownSeconds)*time.Second)
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			logBuilder.WriteString(fmt.Sprintf("Status: %d OK\n", resp.StatusCode))
			WriteRequestLog(cfg, reqID, logBuilder.String())

			for k, v := range resp.Header {
				w.Header()[k] = v
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

		logBuilder.WriteString(fmt.Sprintf("Status: %d\nResponse: %s\n", resp.StatusCode, lastBody))

		if p.detector.ShouldFallback(resp.StatusCode, lastBody) {
			key := p.cooldown.Key(route.BackendName, route.Model)
			p.cooldown.SetCooldown(key, time.Duration(cfg.Fallback.CooldownSeconds)*time.Second)
			logBuilder.WriteString(fmt.Sprintf("Action: Cooldown %s, trying next\n", key))
			continue
		}

		WriteRequestLog(cfg, reqID, logBuilder.String())
		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)
		return
	}

	logBuilder.WriteString("\n--- Final Result ---\nAll backends failed\n")
	WriteRequestLog(cfg, reqID, logBuilder.String())
	WriteErrorLog(cfg, reqID, logBuilder.String())

	if lastErr != nil {
		http.Error(w, fmt.Sprintf("All backends failed: %v", lastErr), http.StatusBadGateway)
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

	buf := make([]byte, 4096)
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
	for alias := range cfg.Models {
		models = append(models, Model{
			ID:      alias,
			Object:  "model",
			Created: time.Now().Unix(),
			OwnedBy: "llm-proxy",
		})
	}

	resp := Response{Object: "list", Data: models}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
