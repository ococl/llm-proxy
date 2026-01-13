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
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	showVersion := flag.Bool("version", false, "显示版本信息")
	flag.BoolVar(showVersion, "v", false, "显示版本信息（简写）")
	flag.Parse()

	if *showVersion {
		fmt.Printf("LLM Proxy %s\n", Version)
		fmt.Printf("构建时间: %s\n", BuildTime)
		os.Exit(0)
	}

	configMgr, err := NewConfigManager(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	cfg := configMgr.Get()
	if err := InitLogger(cfg); err != nil {
		log.Fatalf("初始化日志失败: %v", err)
	}

	cooldown := NewCooldownManager()
	go func() {
		for {
			time.Sleep(time.Minute)
			cooldown.ClearExpired()
		}
	}()
	router := NewRouter(configMgr, cooldown)
	detector := NewDetector(configMgr)
	proxy := NewProxy(configMgr, router, cooldown, detector)

	printBanner(Version, cfg.GetListen(), len(cfg.Backends), len(cfg.Models))

	LogGeneral("INFO", "LLM Proxy %s", Version)
	LogGeneral("INFO", "访问地址: http://%s", formatListenAddress(cfg.GetListen()))
	LogGeneral("INFO", "已加载 %d 个后端，%d 个模型别名", len(cfg.Backends), len(cfg.Models))

	server := &http.Server{
		Addr:    cfg.GetListen(),
		Handler: RecoveryMiddleware(proxy),
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务器启动失败: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	LogGeneral("INFO", "正在关闭服务器...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		LogGeneral("ERROR", "服务器关闭失败: %v", err)
	}

	LogGeneral("INFO", "服务器已关闭")
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
