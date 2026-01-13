package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	configMgr, err := NewConfigManager(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	cfg := configMgr.Get()
	if err := InitLogger(cfg); err != nil {
		log.Fatalf("Failed to init logger: %v", err)
	}

	cooldown := NewCooldownManager()
	router := NewRouter(configMgr, cooldown)
	detector := NewDetector(configMgr)
	proxy := NewProxy(configMgr, router, cooldown, detector)

	LogGeneral("INFO", "Starting LLM Proxy on %s", cfg.Listen)
	LogGeneral("INFO", "Loaded %d backends, %d model aliases", len(cfg.Backends), len(cfg.Models))

	if err := http.ListenAndServe(cfg.Listen, proxy); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
