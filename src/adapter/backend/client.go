package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"llm-proxy/domain/entity"
	domainerror "llm-proxy/domain/error"
	"llm-proxy/domain/types"
)

type HTTPClient struct {
	client *http.Client
}

func NewHTTPClient(client *http.Client) *HTTPClient {
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPClient{
		client: client,
	}
}

type BackendRequest struct {
	Backend *entity.Backend
	Body    map[string]interface{}
	Headers map[string][]string
	Path    string
	Stream  bool
}

func (h *HTTPClient) Send(ctx context.Context, backendReq *BackendRequest) (*http.Response, error) {
	if backendReq == nil {
		return nil, domainerror.NewBadRequest("backend request cannot be nil")
	}

	if backendReq.Backend == nil {
		return nil, domainerror.NewBadRequest("backend cannot be nil")
	}

	httpReq, err := h.buildHTTPRequest(ctx, backendReq)
	if err != nil {
		return nil, err
	}

	resp, err := h.client.Do(httpReq)
	if err != nil {
		return nil, domainerror.NewBackendError(
			backendReq.Backend.ID().String(),
			err,
		)
	}

	return resp, nil
}

func (h *HTTPClient) GetHTTPClient() *http.Client {
	return h.client
}

func (h *HTTPClient) buildHTTPRequest(ctx context.Context, backendReq *BackendRequest) (*http.Request, error) {
	var bodyReader io.Reader
	if backendReq.Body != nil {
		bodyBytes, err := json.Marshal(backendReq.Body)
		if err != nil {
			return nil, domainerror.NewInvalidJSON(err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	fullURL := backendReq.Backend.URL().String()
	if backendReq.Path != "" {
		fullURL = strings.TrimSuffix(fullURL, "/") + "/" + strings.TrimPrefix(backendReq.Path, "/")
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", fullURL, bodyReader)
	if err != nil {
		return nil, domainerror.NewBadRequest(fmt.Sprintf("failed to create HTTP request: %v", err))
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// 默认使用中国大陆、简体中文，除非后端显式配置了 locale
	locale := backendReq.Backend.Locale()
	if locale == "" {
		locale = "zh-CN"
	}
	httpReq.Header.Set("Accept-Language", locale)

	apiKey := backendReq.Backend.APIKey()
	if !apiKey.IsEmpty() {
		keyStr := string(apiKey)
		switch backendReq.Backend.Protocol() {
		case types.ProtocolAnthropic:
			httpReq.Header.Set("x-api-key", keyStr)
			httpReq.Header.Set("anthropic-version", "2023-06-01")
		case types.ProtocolOpenAI:
			httpReq.Header.Set("Authorization", "Bearer "+keyStr)
		default:
			httpReq.Header.Set("Authorization", "Bearer "+keyStr)
		}
	}

	for key, values := range backendReq.Headers {
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}

	return httpReq, nil
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
