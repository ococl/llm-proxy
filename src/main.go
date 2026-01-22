package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	backend_adapter "llm-proxy/adapter/backend"
	config_adapter "llm-proxy/adapter/config"
	http_adapter "llm-proxy/adapter/http"
	logging_adapter "llm-proxy/adapter/logging"
	"llm-proxy/application/service"
	"llm-proxy/application/usecase"
	"llm-proxy/config"
	"llm-proxy/domain/entity"
	domain_service "llm-proxy/domain/service"
	"llm-proxy/logging"
	"llm-proxy/middleware"
	"llm-proxy/proxy"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
	testMode  = false
)

func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	showVersion := flag.Bool("version", false, "显示版本信息")
	disableColor := flag.Bool("no-color", false, "禁用控制台颜色输出")
	flag.BoolVar(showVersion, "v", false, "显示版本信息（简写）")
	flag.BoolVar(disableColor, "disable-color", false, "禁用控制台颜色输出")
	flag.Parse()

	if *showVersion {
		fmt.Printf("LLM Proxy %s\n", Version)
		fmt.Printf("构建时间: %s\n", BuildTime)
		os.Exit(0)
	}

	configMgr, err := config.NewManager(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	cfg := configMgr.Get()

	if *disableColor {
		falseValue := false
		cfg.Logging.Colorize = &falseValue
	}

	if err := logging.InitLogger(cfg); err != nil {
		log.Fatalf("初始化日志失败: %v", err)
	}

	config.LoggingConfigChangedFunc = func(c *config.Config) error {
		logging.ShutdownLogger()
		return logging.InitLogger(c)
	}

	configAdapter := config_adapter.NewConfigAdapter(configMgr)
	proxyLogger := logging_adapter.NewZapLoggerAdapter(logging.ProxySugar)

	cooldownMgr := domain_service.NewCooldownManager(time.Duration(cfg.Fallback.CooldownSeconds) * time.Second)
	shutdownCooldown := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				cooldownMgr.Cleanup()
			case <-shutdownCooldown:
				return
			}
		}
	}()

	proxy.InitHTTPClient(cfg)
	httpClient := backend_adapter.NewHTTPClient(proxy.GetHTTPClient())
	backendClient := backend_adapter.NewBackendClientAdapter(httpClient)

	loadBalancer := domain_service.NewLoadBalancer(domain_service.StrategyRandom)

	fallbackAliases := make(map[string][]entity.ModelAlias)
	for alias, targets := range cfg.Fallback.AliasFallback {
		for _, target := range targets {
			fallbackAliases[alias] = append(fallbackAliases[alias], entity.NewModelAlias(target))
		}
	}

	retryConfig := entity.RetryConfig{
		EnableBackoff:       cfg.Fallback.IsBackoffEnabled(),
		BackoffInitialDelay: time.Duration(cfg.Fallback.GetBackoffInitialDelay()) * time.Millisecond,
		BackoffMaxDelay:     time.Duration(cfg.Fallback.GetBackoffMaxDelay()) * time.Millisecond,
		BackoffMultiplier:   cfg.Fallback.GetBackoffMultiplier(),
		BackoffJitter:       cfg.Fallback.GetBackoffJitter(),
		MaxRetries:          cfg.Fallback.MaxRetries,
	}

	fallbackStrategy := domain_service.NewFallbackStrategy(cooldownMgr, fallbackAliases, retryConfig)
	retryStrategy := usecase.NewRetryStrategy(
		cfg.Fallback.MaxRetries,
		cfg.Fallback.IsBackoffEnabled(),
		time.Duration(cfg.Fallback.GetBackoffInitialDelay())*time.Millisecond,
		time.Duration(cfg.Fallback.GetBackoffMaxDelay())*time.Millisecond,
		cfg.Fallback.GetBackoffMultiplier(),
		cfg.Fallback.GetBackoffJitter(),
	)

	protocolConverter := service.NewProtocolConverter(loadSystemPrompts())

	backendRepo := config_adapter.NewBackendRepository(configAdapter)
	routeResolver := usecase.NewRouteResolveUseCase(configAdapter, backendRepo, cfg.Fallback.AliasFallback)

	proxyUseCase := usecase.NewProxyRequestUseCase(
		proxyLogger,
		configAdapter,
		routeResolver,
		protocolConverter,
		backendClient,
		retryStrategy,
		fallbackStrategy,
		loadBalancer,
		&NopMetricsProvider{},
		&NopRequestLogger{},
	)

	errorPresenter := http_adapter.NewErrorPresenter(proxyLogger)
	proxyHandler := http_adapter.NewProxyHandler(proxyUseCase, configAdapter, proxyLogger, errorPresenter)
	healthHandler := http_adapter.NewHealthHandler(configAdapter, proxyLogger)
	recoveryMiddleware := http_adapter.NewRecoveryMiddleware(proxyLogger)

	rateLimiter := middleware.NewRateLimiter(configMgr)
	concurrencyLimiter := middleware.NewConcurrencyLimiter(configMgr)

	printBanner(Version, cfg.GetListen(), len(cfg.Backends), len(cfg.Models))

	logging.GeneralSugar.Infow("LLM Proxy started",
		"version", Version,
		"address", formatListenAddress(cfg.GetListen()),
		"backends", len(cfg.Backends),
		"models", len(cfg.Models),
	)

	mux := http.NewServeMux()
	mux.Handle("/v1/chat/completions", proxyHandler)
	mux.Handle("/health", healthHandler)

	server := &http.Server{
		Addr:    cfg.GetListen(),
		Handler: recoveryMiddleware.Middleware(rateLimiter.Middleware(concurrencyLimiter.Middleware(mux))),
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("服务器启动失败: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logging.GeneralSugar.Info("正在关闭服务器...")

	close(shutdownCooldown)
	logging.ShutdownLogger()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logging.GeneralSugar.Errorw("服务器关闭失败", "error", err)
	}

	logging.GeneralSugar.Info("服务器已关闭")
}

func loadSystemPrompts() map[string]string {
	prompts := make(map[string]string)
	systemPromptsDir := "system_prompts"
	files, err := os.ReadDir(systemPromptsDir)
	if err != nil {
		return prompts
	}
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".txt") {
			continue
		}
		modelName := strings.TrimSuffix(file.Name(), ".txt")
		data, err := os.ReadFile(systemPromptsDir + "/" + file.Name())
		if err != nil {
			continue
		}
		prompts[modelName] = string(data)
	}
	return prompts
}

func formatListenAddress(listen string) string {
	if strings.HasPrefix(listen, ":") {
		ip := getLocalIP()
		return ip + listen
	}
	return listen
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

func printBanner(version, listen string, backends, models int) {
	if testMode || !shouldUseColor() {
		return
	}

	banner := `
 ╦  ╦  ╔╦╗  ╔═╗┬─┐┌─┐─┐ ┬┬ ┬
 ║  ║  ║║║  ╠═╝├┬┘│ │┌┴┬┘└┬┘
 ╩═╝╩═╝╩ ╩  ╩  ┴└─└─┘┴ └─ ┴ `

	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#BD93F9")).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B"))

	fmt.Println(titleStyle.Render(banner))
	fmt.Println()
	fmt.Println(labelStyle.Render("  Version:  ") + valueStyle.Render(version))
	fmt.Println(labelStyle.Render("  Listen:   ") + valueStyle.Render(listen))
	fmt.Println(labelStyle.Render("  Backends: ") + valueStyle.Render(fmt.Sprintf("%d loaded", backends)))
	fmt.Println(labelStyle.Render("  Models:   ") + valueStyle.Render(fmt.Sprintf("%d aliases", models)))
	fmt.Println()
}

func shouldUseColor() bool {
	cfg := logging.GetLoggingConfig()
	if cfg != nil && cfg.Colorize != nil && !*cfg.Colorize {
		return false
	}
	return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}

type NopMetricsProvider struct{}

func (n *NopMetricsProvider) IncRequestsTotal(backend string)                       {}
func (n *NopMetricsProvider) RecordDuration(backend string, duration time.Duration) {}
func (n *NopMetricsProvider) IncBackendErrors(backend string)                       {}
func (n *NopMetricsProvider) SetCircuitBreakerState(backend string, state int)      {}
func (n *NopMetricsProvider) IncActiveRequests()                                    {}
func (n *NopMetricsProvider) DecActiveRequests()                                    {}
func (n *NopMetricsProvider) GetSnapshot() map[string]interface{}                   { return nil }

type NopRequestLogger struct{}

func (n *NopRequestLogger) LogRequest(reqID string, content string) {}
func (n *NopRequestLogger) LogError(reqID string, content string)   {}
