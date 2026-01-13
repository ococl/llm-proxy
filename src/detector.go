package main

import (
	"strconv"
	"strings"
)

type Detector struct {
	configMgr *ConfigManager
}

func NewDetector(cfg *ConfigManager) *Detector {
	return &Detector{configMgr: cfg}
}

func (d *Detector) ShouldFallback(statusCode int, body string) bool {
	cfg := d.configMgr.Get()

	for _, pattern := range cfg.Detection.ErrorCodes {
		if d.matchStatusCode(statusCode, pattern) {
			return true
		}
	}

	for _, pattern := range cfg.Detection.ErrorPatterns {
		if strings.Contains(body, pattern) {
			return true
		}
	}

	return false
}

func (d *Detector) matchStatusCode(code int, pattern string) bool {
	pattern = strings.TrimSpace(pattern)
	if strings.HasSuffix(pattern, "xx") {
		prefix := strings.TrimSuffix(pattern, "xx")
		codePrefix := strconv.Itoa(code / 100)
		return codePrefix == prefix
	}
	exact, err := strconv.Atoi(pattern)
	if err != nil {
		return false
	}
	return code == exact
}
