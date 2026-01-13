package main

import (
	"flag"
	"log"
	"net/http"
	"time"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

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

	LogGeneral("INFO", "LLM Proxy 启动，监听地址: %s", cfg.Listen)
	LogGeneral("INFO", "已加载 %d 个后端，%d 个模型别名", len(cfg.Backends), len(cfg.Models))

	if err := http.ListenAndServe(cfg.Listen, proxy); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
