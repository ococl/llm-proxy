package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"time"

	"llm-proxy/domain/entity"
	domainerror "llm-proxy/domain/error"
	"llm-proxy/domain/port"
)

// StreamingAdapter 定义流式处理的适配器接口。
// 通过策略模式消除 executeStreamingWithRetry 和 executeStreamingPassthroughWithRetry 的重复代码。
type StreamingAdapter interface {
	// Execute 执行流式处理，返回错误
	Execute(ctx context.Context, backendReq *entity.Request, backend *entity.Backend, backendModel string) error

	// OnSuccess 重试成功后执行的日志记录
	LogSuccess(reqID, modelName, backendName, backendModel string, attempt int)
}

// ResponseStreamingAdapter 处理标准响应流式响应。
type ResponseStreamingAdapter struct {
	uc      *ProxyRequestUseCase
	req     *entity.Request
	handler func(*entity.Response) error
}

// NewResponseStreamingAdapter 创建响应流适配器。
func NewResponseStreamingAdapter(uc *ProxyRequestUseCase, req *entity.Request, handler func(*entity.Response) error) *ResponseStreamingAdapter {
	return &ResponseStreamingAdapter{
		uc:      uc,
		req:     req,
		handler: handler,
	}
}

// Execute 执行流式处理。
func (a *ResponseStreamingAdapter) Execute(ctx context.Context, backendReq *entity.Request, backend *entity.Backend, backendModel string) error {
	reqID := a.req.ID().String()
	modelName := a.req.Model().String()

	streamHandler := func(chunk []byte) error {
		a.uc.logger.Debug("处理流式数据块",
			port.ReqID(reqID),
			port.Model(modelName),
		)

		var chunkData map[string]interface{}
		if err := json.Unmarshal(chunk, &chunkData); err != nil {
			return err
		}

		responseID, _ := chunkData["id"].(string)
		if responseID == "" {
			responseID = "响应-" + a.req.ID().String()
		}

		model, _ := chunkData["model"].(string)
		if model == "" {
			model = a.req.Model().String()
		}

		builder := entity.NewResponseBuilder().
			ID(responseID).
			Model(model).
			Object("chat.completion.chunk")

		choicesArray := []entity.Choice{}

		choicesRaw, _ := chunkData["choices"]
		if choicesRaw == nil {
			a.uc.logger.Warn("上游返回空choices",
				port.ReqID(reqID),
				port.Model(modelName),
				port.Backend(backend.Name()),
				port.BackendModel(backendModel),
				port.ResponseID(responseID),
			)
		}

		if choices, ok := chunkData["choices"].([]interface{}); ok && len(choices) > 0 {
			if choiceMap, ok := choices[0].(map[string]interface{}); ok {
				index, _ := choiceMap["index"].(float64)
				finishReason, _ := choiceMap["finish_reason"].(string)

				choice := entity.Choice{
					Index:        int(index),
					FinishReason: finishReason,
				}

				if deltaMap, ok := choiceMap["delta"].(map[string]interface{}); ok {
					content, _ := deltaMap["content"].(string)
					role, _ := deltaMap["role"].(string)

					if role != "" || content != "" {
						choice.Delta = &entity.Message{
							Role:    role,
							Content: content,
						}
					}
				}

				choicesArray = append(choicesArray, choice)
			}
		}

		builder.Choices(choicesArray)
		resp := builder.BuildUnsafe()
		return a.handler(resp)
	}

	return a.uc.backendClient.SendStreaming(ctx, backendReq, backend, backendModel, streamHandler)
}

// LogSuccess 记录成功日志。
func (a *ResponseStreamingAdapter) LogSuccess(reqID, modelName, backendName, backendModel string, attempt int) {
	a.uc.logger.Info("重试成功",
		port.ReqID(reqID),
		port.Model(modelName),
		port.Backend(backendName),
		port.BackendModel(backendModel),
		port.Attempt(attempt),
	)
}

// PassthroughStreamingAdapter 处理透传流式响应。
type PassthroughStreamingAdapter struct {
	uc      *ProxyRequestUseCase
	handler func([]byte) error
}

// NewPassthroughStreamingAdapter 创建透传流适配器。
func NewPassthroughStreamingAdapter(uc *ProxyRequestUseCase, handler func([]byte) error) *PassthroughStreamingAdapter {
	return &PassthroughStreamingAdapter{
		uc:      uc,
		handler: handler,
	}
}

func (a *PassthroughStreamingAdapter) maxCaptureBytes() int {
	cfg := a.uc.config.Get()
	if cfg == nil {
		return 0
	}
	return cfg.Logging.MaxLogContentSize
}

// Execute 执行流式处理。
func (a *PassthroughStreamingAdapter) Execute(ctx context.Context, backendReq *entity.Request, backend *entity.Backend, backendModel string) error {
	reqID := backendReq.ID().String()

	httpResp, err := a.uc.backendClient.SendStreamingPassthrough(ctx, backendReq, backend, backendModel)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	a.uc.logger.Debug("开始读取上游流式响应(透传模式)",
		port.String("reqID", reqID),
		port.String("backend", backend.Name()),
	)

	maxCapture := a.maxCaptureBytes()
	capturing := maxCapture != 0
	captureExceeded := false
	var captured bytes.Buffer

	buf := make([]byte, 32*1024)
	totalBytes := 0
	for {
		n, readErr := httpResp.Body.Read(buf)
		if n > 0 {
			totalBytes += n
			chunk := buf[:n]

			if capturing && !captureExceeded {
				remaining := maxCapture - captured.Len()
				if remaining <= 0 {
					captureExceeded = true
				} else if n <= remaining {
					_, _ = captured.Write(chunk)
				} else {
					_, _ = captured.Write(chunk[:remaining])
					captureExceeded = true
				}
			}

			if handlerErr := a.handler(chunk); handlerErr != nil {
				a.uc.logger.Error("处理流式chunk_data失败(透传模式)",
					port.ReqID(reqID),
					port.Backend(backend.Name()),
					port.Error(handlerErr),
				)
				return handlerErr
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				a.uc.logger.Debug("上游流式响应结束(透传模式)",
					port.ReqID(reqID),
					port.Backend(backend.Name()),
					port.TotalBytes(totalBytes),
				)
				break
			}
			a.uc.logger.Error("读取上游流式响应失败(透传模式)",
				port.ReqID(reqID),
				port.Backend(backend.Name()),
				port.Error(readErr),
			)
			return readErr
		}
	}

	if capturing {
		bodyBytes := captured.Bytes()
		if captureExceeded {
			bodyBytes = append(bodyBytes, []byte("\n\n[response body capture truncated]\n")...)
		}
		a.uc.bodyLogger.LogResponseBody(reqID, port.BodyLogTypeUpstreamResponse, httpResp.StatusCode, httpResp.Header, string(bodyBytes))
	}
	a.uc.logger.Info("上游流式请求完成(透传模式)",
		port.ReqID(reqID),
		port.Backend(backend.Name()),
		port.TotalBytes(totalBytes),
	)
	return nil
}

// LogSuccess 记录成功日志。
func (a *PassthroughStreamingAdapter) LogSuccess(reqID, modelName, backendName, backendModel string, attempt int) {
	a.uc.logger.Info("重试成功(透传模式)",
		port.ReqID(reqID),
		port.Model(modelName),
		port.Backend(backendName),
		port.BackendModel(backendModel),
		port.Attempt(attempt),
	)
}

// selectBackendForRetry 从路由列表中选择可用backend。
func (uc *ProxyRequestUseCase) selectBackendForRetry(
	ctx context.Context,
	routes []*port.Route,
	reqID string,
	modelName string,
	attempt int,
) (*entity.Backend, string, error) {
	backend := uc.loadBalancer.Select(routes)
	if backend == nil {
		uc.logger.Error("重试时无可用backend",
			port.ReqID(reqID),
			port.Model(modelName),
			port.Attempt(attempt),
		)
		return nil, "", domainerror.NewNoBackend()
	}

	selectedRoute := findRouteForBackend(routes, backend)
	backendModel := modelName
	if selectedRoute != nil {
		backendModel = selectedRoute.Model
	}

	return backend, backendModel, nil
}

// logRetryAttempt 记录重试尝试日志。
func (uc *ProxyRequestUseCase) logRetryAttempt(
	adapter StreamingAdapter,
	reqID, modelName string,
	backend *entity.Backend,
	backendModel string,
	attempt, maxRetries int,
) {
	if attempt > 0 {
		uc.logger.Debug("重试请求",
			port.ReqID(reqID),
			port.Model(modelName),
			port.Backend(backend.Name()),
			port.BackendModel(backendModel),
			port.Attempt(attempt),
			port.MaxRetries(maxRetries),
		)
	}
}

// executeStreamingWithRetryCommon 执行流式重试的公共框架。
// 通过 Strategy 模式消除两个流式方法中约 70% 的重复代码。
func (uc *ProxyRequestUseCase) executeStreamingWithRetryCommon(
	ctx context.Context,
	req *entity.Request,
	routes []*port.Route,
	backendModelName string,
	adapter StreamingAdapter,
) error {
	var lastErr error
	maxRetries := uc.retryStrategy.GetMaxRetries()
	reqID := req.ID().String()
	modelName := req.Model().String()

	for attempt := 0; attempt <= maxRetries; attempt++ {
		backend, currentBackendModel, err := uc.selectBackendForRetry(ctx, routes, reqID, modelName, attempt)
		if err != nil {
			return err
		}

		uc.logRetryAttempt(adapter, reqID, modelName, backend, currentBackendModel, attempt, maxRetries)

		backendReq, err := uc.protocolConv.ToBackend(req, backend.Protocol())
		if err != nil {
			uc.logger.Error("重试时转换失败",
				port.ReqID(reqID),
				port.Model(modelName),
				port.Backend(backend.Name()),
				port.BackendModel(currentBackendModel),
				port.Attempt(attempt),
				port.Error(err),
			)
			return domainerror.NewProtocolError("request conversion failed", err)
		}

		err = adapter.Execute(ctx, backendReq, backend, currentBackendModel)
		if err == nil {
			if attempt > 0 {
				adapter.LogSuccess(reqID, modelName, backend.Name(), currentBackendModel, attempt)
			}
			return nil
		}

		lastErr = err
		uc.metrics.IncBackendErrors(backend.Name())

		uc.logger.Warn("backend错误，尝试重试",
			port.ReqID(reqID),
			port.Model(modelName),
			port.Backend(backend.Name()),
			port.BackendModel(currentBackendModel),
			port.Attempt(attempt),
			port.Error(err),
		)

		if !uc.retryStrategy.ShouldRetry(attempt, err) {
			uc.logger.Error("重试次数耗尽",
				port.ReqID(reqID),
				port.Model(modelName),
				port.Backend(backend.Name()),
				port.BackendModel(currentBackendModel),
				port.TotalAttempts(attempt+1),
				port.Error(lastErr),
			)
			return domainerror.NewBackendError(backend.Name(), err)
		}

		delay := uc.retryStrategy.GetDelay(attempt)
		if delay > 0 {
			uc.logger.Debug("重试等待",
				port.ReqID(reqID),
				port.Model(modelName),
				port.DelayMSInt(delay.Milliseconds()),
				port.NextAttempt(attempt+1),
			)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				uc.logger.Warn("重试被取消",
					port.ReqID(reqID),
					port.Model(modelName),
					port.Attempt(attempt),
				)
				return ctx.Err()
			}
		}
	}

	if lastErr != nil {
		return domainerror.NewNoBackend().WithCause(lastErr)
	}
	return domainerror.NewNoBackend()
}

// ExecuteStreaming processes a streaming proxy request.
func (uc *ProxyRequestUseCase) ExecuteStreaming(
	ctx context.Context,
	req *entity.Request,
	handler func(*entity.Response) error,
) error {
	startTime := time.Now()
	reqID := req.ID().String()
	modelName := req.Model().String()

	uc.logger.Info("请求开始",
		port.ReqID(reqID),
		port.Model(modelName),
		port.ClientProtocol(req.ClientProtocol()),
	)

	if err := uc.validateRequest(req); err != nil {
		uc.logger.Warn("验证失败",
			port.ReqID(reqID),
			port.Model(modelName),
			port.Error(err),
		)
		return err
	}

	routes, err := uc.routeResolver.Resolve(req.Model().String())
	if err != nil {
		uc.logger.Error("解析路由失败",
			port.ReqID(reqID),
			port.Model(modelName),
			port.Error(err),
		)
		return err
	}

	availableRoutes := uc.fallbackStrategy.FilterAvailableRoutes(routes)
	if len(availableRoutes) == 0 {
		uc.logger.Warn("backend全部冷却，尝试降级",
			port.ReqID(reqID),
			port.Model(modelName),
		)

		fallbackRoutes, err := uc.fallbackStrategy.GetFallbackRoutes(req.Model().String(), uc.routeResolver)
		if err != nil || len(fallbackRoutes) == 0 {
			uc.logger.Error("无可用backend",
				port.ReqID(reqID),
				port.Model(modelName),
			)
			return domainerror.NewNoBackend()
		}

		uc.logger.Info("触发降级",
			port.String("reqID", reqID),
			port.String("original_model", modelName),
			port.Int("降级路由数", len(fallbackRoutes)),
		)
		availableRoutes = fallbackRoutes
	}

	backend := uc.loadBalancer.Select(availableRoutes)
	if backend == nil {
		uc.logger.Error("选择backend失败",
			port.String("reqID", reqID),
			port.String("model", modelName),
		)
		return domainerror.NewNoBackend()
	}

	selectedRoute := findRouteForBackend(availableRoutes, backend)
	backendModelName := modelName
	if selectedRoute != nil {
		backendModelName = selectedRoute.Model
	}

	uc.logger.Debug("backend选择",
		port.String("req_id", reqID),
		port.String("model", modelName),
		port.String("backend", backend.Name()),
		port.String("backend_model", backendModelName),
	)

	backendReq, err := uc.protocolConv.ToBackend(req, backend.Protocol())
	if err != nil {
		uc.logger.Error("转换协议失败",
			port.String("req_id", reqID),
			port.String("model", modelName),
			port.String("backend", backend.Name()),
			port.String("backend_model", backendModelName),
			port.Error(err),
		)
		return domainerror.NewProtocolError("failed to convert request", err)
	}

	err = uc.executeStreamingWithRetry(ctx, backendReq, availableRoutes, backendModelName, handler)
	if err != nil {
		uc.logger.Error("请求失败",
			port.ReqID(reqID),
			port.Model(modelName),
			port.Backend(backend.Name()),
			port.BackendModel(backendModelName),
			port.DurationMSInt(time.Since(startTime).Milliseconds()),
			port.Error(err),
		)
		return err
	}

	uc.logger.Info("请求完成",
		port.ReqID(reqID),
		port.DurationMSInt(time.Since(startTime).Milliseconds()),
	)

	return nil
}

// ExecuteStreamingPassthrough processes a streaming request in passthrough mode.
func (uc *ProxyRequestUseCase) ExecuteStreamingPassthrough(
	ctx context.Context,
	req *entity.Request,
	handler func([]byte) error,
) error {
	startTime := time.Now()
	reqID := req.ID().String()
	modelName := req.Model().String()

	uc.logger.Info("请求开始(透传模式)",
		port.ReqID(reqID),
		port.Model(modelName),
		port.ClientProtocol(req.ClientProtocol()),
	)

	if err := uc.validateRequest(req); err != nil {
		uc.logger.Warn("验证失败",
			port.ReqID(reqID),
			port.Model(modelName),
			port.Error(err),
		)
		return err
	}

	routes, err := uc.routeResolver.Resolve(req.Model().String())
	if err != nil {
		uc.logger.Error("解析路由失败",
			port.ReqID(reqID),
			port.Model(modelName),
			port.Error(err),
		)
		return err
	}

	availableRoutes := uc.fallbackStrategy.FilterAvailableRoutes(routes)
	if len(availableRoutes) == 0 {
		uc.logger.Warn("backend全部冷却，尝试降级",
			port.ReqID(reqID),
			port.Model(modelName),
		)

		fallbackRoutes, err := uc.fallbackStrategy.GetFallbackRoutes(req.Model().String(), uc.routeResolver)
		if err != nil || len(fallbackRoutes) == 0 {
			uc.logger.Error("无可用backend",
				port.ReqID(reqID),
				port.Model(modelName),
			)
			return domainerror.NewNoBackend()
		}

		uc.logger.Info("触发降级",
			port.ReqID(reqID),
			port.OriginalModel(modelName),
			port.FallbackRoutes(len(fallbackRoutes)),
		)
		availableRoutes = fallbackRoutes
	}

	backend := uc.loadBalancer.Select(availableRoutes)
	if backend == nil {
		uc.logger.Error("选择backend失败",
			port.ReqID(reqID),
			port.Model(modelName),
		)
		return domainerror.NewNoBackend()
	}

	selectedRoute := findRouteForBackend(availableRoutes, backend)
	backendModelName := modelName
	if selectedRoute != nil {
		backendModelName = selectedRoute.Model
	}

	uc.logger.Debug("backend选择",
		port.ReqID(reqID),
		port.Model(modelName),
		port.Backend(backend.Name()),
		port.BackendModel(backendModelName),
	)

	backendReq, err := uc.protocolConv.ToBackend(req, backend.Protocol())
	if err != nil {
		uc.logger.Error("转换协议失败",
			port.ReqID(reqID),
			port.Model(modelName),
			port.Backend(backend.Name()),
			port.BackendModel(backendModelName),
			port.Error(err),
		)
		return domainerror.NewProtocolError("failed to convert request", err)
	}

	err = uc.executeStreamingPassthroughWithRetry(ctx, backendReq, availableRoutes, backendModelName, handler)
	if err != nil {
		uc.logger.Error("请求失败(透传模式)",
			port.ReqID(reqID),
			port.Model(modelName),
			port.Backend(backend.Name()),
			port.BackendModel(backendModelName),
			port.DurationMSInt(time.Since(startTime).Milliseconds()),
			port.Error(err),
		)
		return err
	}

	uc.logger.Info("请求完成(透传模式)",
		port.ReqID(reqID),
		port.DurationMSInt(time.Since(startTime).Milliseconds()),
	)

	return nil
}

// executeStreamingWithRetry executes streaming request with retry logic.
func (uc *ProxyRequestUseCase) executeStreamingWithRetry(
	ctx context.Context,
	req *entity.Request,
	routes []*port.Route,
	backendModelName string,
	handler func(*entity.Response) error,
) error {
	adapter := NewResponseStreamingAdapter(uc, req, handler)
	return uc.executeStreamingWithRetryCommon(ctx, req, routes, backendModelName, adapter)
}

// executeStreamingPassthroughWithRetry executes streaming passthrough with retry logic.
func (uc *ProxyRequestUseCase) executeStreamingPassthroughWithRetry(
	ctx context.Context,
	req *entity.Request,
	routes []*port.Route,
	backendModelName string,
	handler func([]byte) error,
) error {
	adapter := NewPassthroughStreamingAdapter(uc, handler)
	return uc.executeStreamingWithRetryCommon(ctx, req, routes, backendModelName, adapter)
}
