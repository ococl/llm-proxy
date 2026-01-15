package main

import (
	"context"
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

	"llm-proxy/backend"
	"llm-proxy/config"
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

	cooldown := backend.NewCooldownManager()
	shutdownCooldown := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				cooldown.ClearExpired()
			case <-shutdownCooldown:
				return
			}
		}
	}()

	router := proxy.NewRouter(configMgr, cooldown)
	detector := proxy.NewDetector(configMgr)
	p := proxy.NewProxy(configMgr, router, cooldown, detector)

	rateLimiter := middleware.NewRateLimiter(configMgr)
	concurrencyLimiter := middleware.NewConcurrencyLimiter(configMgr)

	printBanner(Version, cfg.GetListen(), len(cfg.Backends), len(cfg.Models))

	logging.GeneralSugar.Infow("LLM Proxy started",
		"version", Version,
		"address", formatListenAddress(cfg.GetListen()),
		"backends", len(cfg.Backends),
		"models", len(cfg.Models),
	)

	server := &http.Server{
		Addr:    cfg.GetListen(),
		Handler: middleware.RecoveryMiddleware(rateLimiter.Middleware(concurrencyLimiter.Middleware(p))),
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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
